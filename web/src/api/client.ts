/**
 * API client：openapi-fetch + 生成类型。禁止手写 fetch 绕过类型（CONVENTIONS-frontend §1）。
 * schema.d.ts 由 `make gen-web`（openapi-typescript）生成，勿手改。
 */
import createClient from 'openapi-fetch'

import { clearAuth, getToken } from '@homeagent/shared'

import type { components, paths } from './schema'

/** 后端错误码枚举：唯一真相源是 openapi 契约（对齐 apperr.Code） */
export type ApiErrorCode = components['schemas']['Error']['code']

export function getApiBaseUrl(): string {
  if (typeof window === 'undefined') return '/api/v1'
  const custom = localStorage.getItem('homeagent_server_url')
  if (custom) return custom.replace(/\/+$/, '') + '/api/v1'

  // 在 Capacitor 原生安卓环境中，页面 origin 是 https://localhost 或 http://localhost
  // 非 5173 端口说明是在独立 APK 里运行，需连到局域网开发机
  const isCapacitor =
    (window.location.protocol === 'capacitor:' || window.location.hostname === 'localhost') &&
    window.location.port !== '5173'
  if (isCapacitor) {
    return 'http://192.168.0.109:8080/api/v1'
  }
  return window.location.origin ? `${window.location.origin}/api/v1` : '/api/v1'
}

export const api = createClient<paths>({
  // 绝对 baseUrl：openapi-fetch 内部 new Request() 需要完整来源，
  // 相对路径在浏览器靠 document 兜底，但 SSR/测试环境会抛 Failed to parse URL
  baseUrl:
    typeof window !== 'undefined' && window.location?.origin
      ? `${window.location.origin}/api/v1`
      : '/api/v1',
  // 延迟解析 fetch，同时在原生 WebView 环境将本地 localhost 替换为真实后端基址
  fetch: (input: RequestInfo | URL, init?: RequestInit) => {
    let url = typeof input === 'string' ? input : input instanceof Request ? input.url : String(input)
    const base = getApiBaseUrl()
    if (
      (url.startsWith('https://localhost/api/v1') || url.startsWith('http://localhost/api/v1')) &&
      !base.includes('localhost')
    ) {
      url = url.replace(/^https?:\/\/localhost\/api\/v1/, base)
      if (input instanceof Request) {
        return fetch(new Request(url, input), init)
      }
      return fetch(url, init)
    }
    return fetch(input, init)
  },
})

// 认证中间件：每个请求带 JWT；401（令牌失效）立即清空登录态
api.use({
  onRequest: ({ request }) => {
    const token = getToken()
    if (token) request.headers.set('Authorization', `Bearer ${token}`)
    return request
  },
  onResponse: ({ response }) => {
    if (response.status === 401) clearAuth()
    return response
  },
})

/**
 * 错误归一化：后端返回 {code, message, trace_id}（openapi Error）。
 * 页面层用 ERROR_MESSAGE 展示中文，禁止吞错误（CONVENTIONS-frontend §6）。
 */
export class ApiError extends Error {
  code: ApiErrorCode
  traceId?: string

  constructor(code: ApiErrorCode, message: string, traceId?: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.traceId = traceId
  }
}

/** 从 openapi-fetch 的 {data, error} 提取数据或抛 ApiError */
export function unwrap<T>(res: {
  data?: T
  error?: { code?: string; message?: string; trace_id?: string }
}): T {
  // 有 error 才算失败（204 成功无响应体时 data 为 undefined，不能误判为错误）
  if (res.error) {
    throw new ApiError(
      (res.error.code as ApiErrorCode) ?? 'internal',
      res.error.message ?? '系统内部错误',
      res.error.trace_id,
    )
  }
  return res.data as T
}
