/**
 * 对话主页：AI 交互入口，底部输入框 + 模型切换（page-01-chat）。
 * 消息先乐观渲染，工具结果以服务器回执为准（CONVENTIONS-frontend §4）。
 */
import { useState } from 'react'

import { ERROR_MESSAGE, type ErrorCode } from '@homeagent/shared'

import { api, ApiError, unwrap } from '../../api/client'
import { ResultCard } from '../../components/ResultCard'
import { useSSE, type SSEEvent } from '../../hooks/useSSE'
import { useChatStore, type ChatMessage } from '../../stores/chat'
import { tokens } from '../../styles/tokens'

import styles from './ChatPage.module.css'

export function ChatPage() {
  const { messages, status, appendMessage, appendToken, appendCard, setStatus } = useChatStore()
  const [input, setInput] = useState('')
  const [error, setError] = useState<string>()

  const { connect, abort } = useSSE({
    url: '/api/v1/chat',
    onEvent: (e: SSEEvent) => {
      if (e.type === 'token') {
        // 乐观渲染：token 直接追加到当前 assistant 消息
        appendToken('streaming', e.content)
      } else if (e.type === 'tool_call') {
        appendCard('streaming', e.card as ChatMessage['card'], true)
      } else if (e.type === 'done') {
        setStatus('idle')
      }
    },
    onError: (err: Error) => {
      setStatus('error')
      setError(err.message)
    },
  })

  const handleSend = async () => {
    const content = input.trim()
    if (!content || status !== 'idle') return

    setError(undefined)
    setInput('')
    appendMessage({ id: 'streaming', role: 'user', content })

    try {
      // 骨架阶段：SSE 端点返回固定事件流，验证链路
      connect({ content })
      // 会话管理（创建/取历史）后续接入
      const res = await api.POST('/conversations', { body: {} })
      unwrap(res)
    } catch (e) {
      const code: ErrorCode = e instanceof ApiError ? e.code : 'internal'
      setError(ERROR_MESSAGE[code])
      setStatus('idle')
    }
  }

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <h1 className={styles.title} style={{ fontSize: tokens.fontSize.title }}>
          家事助手
        </h1>
        <select
          className={styles.modelSelect}
          aria-label="切换模型"
          // TODO: 接入 GET /models 返回的已启用模型清单
        >
          <option>默认模型</option>
        </select>
      </header>

      <div className={styles.messages}>
        {messages.length === 0 && (
          <div className={styles.empty}>
            <p>说点什么，我来跑腿：</p>
            <p className={styles.hint}>「今天买菜花了 120」「今晚不回家吃」「提醒媳妇洗碗」</p>
          </div>
        )}
        {messages.map((m: ChatMessage) => (
          <div
            key={m.id}
            className={`${styles.bubble} ${m.role === 'user' ? styles.user : styles.ai}`}
          >
            {m.content}
            {m.card && (
              <ResultCard title="执行结果" undoable={m.undoable} onUndo={() => abort()}>
                {JSON.stringify(m.card)}
              </ResultCard>
            )}
          </div>
        ))}
        {status === 'streaming' && <div className={styles.ai}>正在思考…</div>}
        {status === 'tool_running' && <div className={styles.ai}>正在执行操作…</div>}
        {error && <div className={styles.error}>{error}</div>}
      </div>

      <footer className={styles.inputBar}>
        <input
          className={styles.input}
          value={input}
          placeholder="输入消息…"
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') void handleSend()
          }}
        />
        <button
          type="button"
          className={styles.send}
          onClick={() => void handleSend()}
          disabled={status !== 'idle'}
        >
          发送
        </button>
      </footer>
    </div>
  )
}
