/**
 * 认证状态存储（两端共享）。
 *
 * 只存后端换发的 JWT，不存成员预共享密钥（auth_token 换完即弃）。
 * 权限每次请求由后端实时校验（ADR-005 双保险），令牌过期/失效即清空。
 */
import type { MemberRole } from './enums'

const TOKEN_KEY = 'homeagent.token'

/** 登录态（对齐 openapi TokenResponse） */
export interface AuthState {
  token: string
  memberId: string
  role: MemberRole
  /** 过期时间（秒级 Unix 时间戳） */
  expiresAt: number
}

type TokenResponse = {
  token: string
  member_id: string
  role: string
  expires_at: string
}

function isBrowser(): boolean {
  return typeof window !== 'undefined' && typeof window.localStorage !== 'undefined'
}

function parseExpiry(token: string, fallbackISO: string): number {
  // JWT payload 过期时间优先，response.expires_at 兜底
  const parts = token.split('.')
  if (parts.length === 3) {
    try {
      // atob 在浏览器与 Node 18+ 均可用；base64url → base64
      const b64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
      const payload = JSON.parse(atob(b64)) as { exp?: number }
      if (typeof payload.exp === 'number' && payload.exp > 0) return payload.exp
    } catch {
      // 令牌不是合法 JWT（如脚本供应商场景），走兜底
    }
  }
  const parsed = Date.parse(fallbackISO)
  return Number.isNaN(parsed) ? 0 : Math.floor(parsed / 1000)
}

// 快照缓存：useSyncExternalStore 用 Object.is 比较快照，
// getAuth 每次返回新对象会引发无限重渲染，必须缓存引用。
// 按 localStorage 原始字符串比对：外部清空/别的标签改动都能自动重解析。
let cachedRaw = ''
let cached: AuthState | null = null

function isExpired(s: AuthState): boolean {
  return s.expiresAt > 0 && s.expiresAt <= Math.floor(Date.now() / 1000)
}

/** 保存登录态（POST /auth/token 成功后调用） */
export function saveAuth(resp: TokenResponse): AuthState {
  const state: AuthState = {
    token: resp.token,
    memberId: resp.member_id,
    role: resp.role as MemberRole,
    expiresAt: parseExpiry(resp.token, resp.expires_at),
  }
  if (isBrowser()) {
    const raw = JSON.stringify(state)
    window.localStorage.setItem(TOKEN_KEY, raw)
    cachedRaw = raw
    cached = state
  }
  emit()
  return state
}

/** 从 localStorage 解析并校验过期（不走缓存） */
function parseAuth(raw: string): AuthState | null {
  if (!raw) return null
  try {
    const state = JSON.parse(raw) as AuthState
    if (!state?.token) return null
    if (isExpired(state)) {
      clearAuth()
      return null
    }
    return state
  } catch {
    // 残留脏数据：清空
    window.localStorage.removeItem(TOKEN_KEY)
    return null
  }
}

/** 取登录态；过期或不存在返回 null */
export function getAuth(): AuthState | null {
  if (!isBrowser()) return null
  const raw = window.localStorage.getItem(TOKEN_KEY) ?? ''
  if (raw !== cachedRaw) {
    cachedRaw = raw
    cached = parseAuth(raw)
    // 解析失败时 parseAuth 已清空存储，同步缓存键
    if (cached === null && raw) cachedRaw = ''
  } else if (cached && isExpired(cached)) {
    // 缓存命中也要校验：令牌可能在持有期间过期
    clearAuth()
    return null
  }
  return cached
}

/** 取令牌字符串（供 fetch/SSE 注入 Authorization 头） */
export function getToken(): string | null {
  return getAuth()?.token ?? null
}

/** 登出/清空登录态 */
export function clearAuth(): void {
  if (isBrowser()) {
    window.localStorage.removeItem(TOKEN_KEY)
    cachedRaw = ''
    cached = null
  }
  emit()
}

// --- 跨标签同步：storage 事件 + 同标签通知 ---

const changeTarget = new EventTarget()

function emit(): void {
  changeTarget.dispatchEvent(new Event('change'))
}

/** 订阅登录态变化（含跨标签页），返回取消订阅函数 */
export function subscribeAuth(cb: () => void): () => void {
  const handler = (): void => cb()
  changeTarget.addEventListener('change', handler)
  if (isBrowser()) {
    // 其他标签改存储会触发 storage 事件；getAuth 内部按 raw 比对自动重解析
    const onStorage = (e: StorageEvent): void => {
      if (e.key === TOKEN_KEY || e.key === null) handler()
    }
    window.addEventListener('storage', onStorage)
    return () => {
      changeTarget.removeEventListener('change', handler)
      window.removeEventListener('storage', onStorage)
    }
  }
  return () => changeTarget.removeEventListener('change', handler)
}
