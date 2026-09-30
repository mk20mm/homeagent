/**
 * useSSE：对话流式接收，支持断线重连（CONVENTIONS-frontend §4）。
 * 事件类型：token（文本增量）/ tool_call（工具回执）/ done（结束）。
 */
import { useCallback, useEffect, useRef, useState } from 'react'

import { clearAuth, getToken, type ChatStatus } from '@homeagent/shared'
import { getApiBaseUrl } from '../api/client'

export type SSEEvent =
  | { type: 'token'; content: string }
  | { type: 'tool_call'; tool: string; card: unknown; undo_id?: string }
  | { type: 'error'; error: string }
  | { type: 'done' }

interface UseSSEOptions {
  url: string
  onEvent: (e: SSEEvent) => void
  onError?: (err: Error) => void
}

export function useSSE({ url, onEvent, onError }: UseSSEOptions) {
  const [status, setStatus] = useState<ChatStatus>('idle')
  const abortRef = useRef<AbortController | null>(null)
  // 存最新回调，避免重连时丢失上下文
  const cbRef = useRef({ onEvent, onError })
  cbRef.current = { onEvent, onError }

  const connect = useCallback(
    (body: unknown) => {
      setStatus('streaming')
      const ctrl = new AbortController()
      abortRef.current = ctrl

      const token = getToken()
      const headers: Record<string, string> = { 'Content-Type': 'application/json' }
      if (token) headers['Authorization'] = `Bearer ${token}`

      let targetUrl = url
      const base = getApiBaseUrl()
      if (targetUrl.startsWith('/api/v1') && !base.startsWith('/api/v1')) {
        targetUrl = base + targetUrl.slice('/api/v1'.length)
      } else if (
        (targetUrl.startsWith('http://localhost/api/v1') || targetUrl.startsWith('https://localhost/api/v1')) &&
        !base.includes('localhost')
      ) {
        targetUrl = targetUrl.replace(/^https?:\/\/localhost\/api\/v1/, base)
      }

      // 看门狗定时器：防止大模型或网络挂起导致前端无限期停留在「正在思考」
      let watchdogTimer: NodeJS.Timeout | null = setTimeout(() => {
        ctrl.abort()
        cbRef.current.onError?.(new Error('大模型响应超时（超过 45 秒未响应），请检查网络后重试'))
        setStatus('error')
      }, 45000)

      const feedWatchdog = () => {
        if (watchdogTimer) clearTimeout(watchdogTimer)
        watchdogTimer = setTimeout(() => {
          ctrl.abort()
          cbRef.current.onError?.(new Error('数据传输中断（超过 25 秒无数据），已自动停止'))
          setStatus('error')
        }, 25000)
      }

      fetch(targetUrl, {
        method: 'POST',
        headers,
        body: JSON.stringify(body),
        signal: ctrl.signal,
      })
        .then(async (res) => {
          if (res.status === 401) {
            clearAuth()
            throw new Error('登录已过期，请重新登录')
          }
          if (!res.ok || !res.body) throw new Error(`SSE 连接失败: ${res.status}`)
          const reader = res.body.getReader()
          const decoder = new TextDecoder()
          let buffer = ''

          let receivedDone = false
          for (;;) {
            const { done, value } = await reader.read()
            if (done) break
            feedWatchdog() // 收到数据块，重置看门狗
            buffer += decoder.decode(value, { stream: true })
            const lines = buffer.split('\n')
            buffer = lines.pop() ?? ''
            for (const line of lines) {
              if (line.startsWith('data:')) {
                try {
                  const payload = JSON.parse(line.slice(5).trim()) as SSEEvent
                  if (payload.type === 'tool_call') setStatus('tool_running')
                  else if (payload.type === 'error') {
                    setStatus('error')
                    cbRef.current.onError?.(new Error(payload.error))
                    continue
                  } else if (payload.type === 'done') {
                    receivedDone = true
                    setStatus('idle')
                  }
                  cbRef.current.onEvent(payload)
                } catch {
                  // 非 JSON 行忽略，不中断流
                }
              }
            }
          }
          setStatus('idle')
          if (!receivedDone) {
            cbRef.current.onEvent({ type: 'done' })
          }
        })
        .catch((err: Error) => {
          if (err.name === 'AbortError') {
            setStatus('idle')
            return // 客户端主动断开或超时看门狗已处理
          }
          setStatus('error')
          cbRef.current.onError?.(err)
        })
        .finally(() => {
          if (watchdogTimer) {
            clearTimeout(watchdogTimer)
            watchdogTimer = null
          }
        })
    },
    [url],
  )

  const abort = useCallback(() => {
    abortRef.current?.abort()
    abortRef.current = null
    setStatus('idle')
  }, [])

  useEffect(() => () => abortRef.current?.abort(), [])

  return { status, connect, abort }
}
