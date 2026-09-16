/**
 * useSSE：对话流式接收（管理端调试台用）。与 web/ 同一份 SSE 协议。
 * 事件类型：token（文本增量）/ tool_call（工具回执）/ done（结束）。
 */
import { useCallback, useEffect, useRef, useState } from 'react'

import type { ChatStatus } from '@homeagent/shared'

export type SSEEvent =
  | { type: 'token'; content: string }
  | { type: 'tool_call'; tool: string; card: unknown }
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

      fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: ctrl.signal,
      })
        .then(async (res) => {
          if (!res.ok || !res.body) throw new Error(`SSE 连接失败: ${res.status}`)
          const reader = res.body.getReader()
          const decoder = new TextDecoder()
          let buffer = ''

          for (;;) {
            const { done, value } = await reader.read()
            if (done) break
            buffer += decoder.decode(value, { stream: true })
            const lines = buffer.split('\n')
            buffer = lines.pop() ?? ''
            for (const line of lines) {
              if (line.startsWith('data:')) {
                try {
                  const payload = JSON.parse(line.slice(5).trim()) as SSEEvent
                  if (payload.type === 'tool_call') setStatus('tool_running')
                  cbRef.current.onEvent(payload)
                  if (payload.type === 'done') setStatus('idle')
                } catch {
                  // 非 JSON 行忽略，不中断流
                }
              }
            }
          }
          setStatus('idle')
        })
        .catch((err: Error) => {
          if (err.name === 'AbortError') return // 客户端主动断开
          setStatus('error')
          cbRef.current.onError?.(err)
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
