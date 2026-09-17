/**
 * 对话主页：AI 交互入口，底部输入框 + 模型切换（page-01-chat）。
 * 消息先乐观渲染，工具结果以服务器回执为准（CONVENTIONS-frontend §4）。
 */
import { useEffect, useState } from 'react'

import { ERROR_MESSAGE, type ErrorCode } from '@homeagent/shared'

import { api, ApiError, unwrap } from '../../api/client'
import { ConversationSidebar } from '../../components/ConversationSidebar'
import { ResultCard } from '../../components/ResultCard'
import { useSSE, type SSEEvent } from '../../hooks/useSSE'
import { useChatStore, type ChatMessage } from '../../stores/chat'
import { tokens } from '../../styles/tokens'

import styles from './ChatPage.module.css'

interface ModelOption {
  id: string
  display_name: string
  is_default?: boolean
}

export function ChatPage() {
  const {
    messages,
    status,
    currentModelId,
    currentConversationId,
    beginStream,
    appendToken,
    appendCard,
    markUndone,
    setStatus,
    setModel,
    loadConversations,
    selectConversation,
    newConversation,
  } = useChatStore()
  const [input, setInput] = useState('')
  const [error, setError] = useState<string>()
  const [models, setModels] = useState<ModelOption[]>([])
  const [sidebarOpen, setSidebarOpen] = useState(false)

  const { connect } = useSSE({
    url: '/api/v1/chat',
    onEvent: (e: SSEEvent) => {
      if (e.type === 'token') {
        appendToken(e.content)
      } else if (e.type === 'tool_call') {
        appendCard(e.card as ChatMessage['card'], Boolean(e.undo_id), e.undo_id)
      } else if (e.type === 'done') {
        setStatus('idle')
        void loadConversations() // 刷新侧边栏标题/时间
      }
    },
    onError: (err: Error) => {
      setStatus('error')
      setError(err.message)
    },
  })

  // 模型清单（会话内切换，工具集不变）
  useEffect(() => {
    void (async () => {
      try {
        const data = unwrap(await api.GET('/models'))
        setModels(data.models)
        const def = data.models.find((m) => m.is_default)
        if (def) setModel(def.id)
      } catch {
        // 清单加载失败不阻塞对话，沿用后端默认模型
      }
    })()
  }, [setModel])

  // 会话列表：进入时加载并选中最近会话（没有则停在欢迎页，首条消息时自动新建）
  useEffect(() => {
    void (async () => {
      await loadConversations()
      const state = useChatStore.getState()
      if (!state.currentConversationId && state.conversations.length > 0) {
        await selectConversation(state.conversations[0].id)
      }
    })()
  }, [loadConversations, selectConversation])

  const handleUndo = async (undoId?: string) => {
    if (!undoId) return
    setError(undefined)
    try {
      await unwrap(await api.POST('/undo/{undoId}', { params: { path: { undoId } } }))
      markUndone(undoId) // 乐观回滚
    } catch (e) {
      const code: ErrorCode = e instanceof ApiError ? e.code : 'internal'
      setError(ERROR_MESSAGE[code])
    }
  }

  const handleSend = async () => {
    const content = input.trim()
    if (!content || status !== 'idle') return

    setError(undefined)
    setInput('')
    // 无当前会话则显式新建（拿到 id 同步侧边栏；后端 Ensure 也会兜底）
    if (!currentConversationId) {
      await newConversation()
    }
    // 用户消息 + assistant 占位（store 持有 streamId，避免回调闭包过期）
    beginStream(content)

    connect({
      content,
      model_id: currentModelId,
      conversation_id: useChatStore.getState().currentConversationId,
    })
  }

  return (
    <div className={styles.page}>
      <ConversationSidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)} />
      <header className={styles.header}>
        <button
          type="button"
          className={styles.menuBtn}
          aria-label="会话列表"
          onClick={() => setSidebarOpen(true)}
        >
          ☰
        </button>
        <h1 className={styles.title} style={{ fontSize: tokens.fontSize.title }}>
          家事助手
        </h1>
        <select
          className={styles.modelSelect}
          aria-label="切换模型"
          value={currentModelId ?? ''}
          onChange={(e) => setModel(e.target.value)}
        >
          {models.length === 0 && <option value="">默认模型</option>}
          {models.map((m) => (
            <option key={m.id} value={m.id}>
              {m.display_name}
              {m.is_default ? '（默认）' : ''}
            </option>
          ))}
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
              <ResultCard
                title="执行结果"
                undoable={Boolean(m.undoable) && !m.undone}
                onUndo={() => void handleUndo(m.undoId)}
              >
                {m.undone ? (
                  <span className={styles.undone}>已撤销</span>
                ) : m.card.duplicated ? (
                  <span className={styles.undone}>今天已记过这笔，未重复记账</span>
                ) : (
                  JSON.stringify(m.card)
                )}
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
