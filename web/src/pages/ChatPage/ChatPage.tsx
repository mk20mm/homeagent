/**
 * 对话主页：AI 交互入口，底部悬浮胶囊输入框 + 苹果毛玻璃模型切换胶囊（Grok Bot × Apple Style）。
 * 消息先乐观渲染，工具结果以服务器回执为准（CONVENTIONS-frontend §4）。
 */
import { useEffect, useRef, useState } from 'react'

import { ERROR_MESSAGE, type ErrorCode } from '@homeagent/shared'

import { api, ApiError, unwrap } from '../../api/client'
import { ConversationSidebar } from '../../components/ConversationSidebar'
import { ExpenseCard } from '../../components/ExpenseCard'
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
    updateCard,
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
  const [modelDropdownOpen, setModelDropdownOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  const { connect } = useSSE({
    url: '/api/v1/chat',
    onEvent: (e: SSEEvent) => {
      if (e.type === 'token') {
        appendToken(e.content)
      } else if (e.type === 'tool_call') {
        setStatus('tool_running')
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
        if (def && !currentModelId) setModel(def.id)
      } catch {
        // 清单加载失败不阻塞对话，沿用后端默认模型
      }
    })()
  }, [setModel, currentModelId])

  // 点击外部收起模型下拉菜单
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setModelDropdownOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

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

  const handlePromptClick = async (promptText: string) => {
    if (status !== 'idle') return
    setInput(promptText)
    setError(undefined)
    if (!currentConversationId) {
      await newConversation()
    }
    beginStream(promptText)
    connect({
      content: promptText,
      model_id: currentModelId,
      conversation_id: useChatStore.getState().currentConversationId,
    })
    setInput('')
  }

  const currentModel = models.find((m) => m.id === currentModelId)

  return (
    <div className={styles.page}>
      <ConversationSidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)} />
      <header className={styles.header}>
        <div className={styles.headerLeft}>
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
        </div>

        {/* Grok Bot 风格苹果毛玻璃胶囊模型切换器 */}
        <div className={styles.modelPillContainer} ref={dropdownRef}>
          <button
            type="button"
            className={styles.modelPill}
            onClick={() => setModelDropdownOpen((v) => !v)}
            aria-label="切换模型"
            aria-haspopup="listbox"
            aria-expanded={modelDropdownOpen}
          >
            <span className={styles.modelIcon}>✦</span>
            <span className={styles.modelLabel}>
              {currentModel ? currentModel.display_name : '默认模型'}
            </span>
            <span className={`${styles.chevron} ${modelDropdownOpen ? styles.chevronOpen : ''}`}>
              ▼
            </span>
          </button>

          {modelDropdownOpen && (
            <div className={styles.modelDropdown} role="listbox">
              {models.length === 0 && (
                <div style={{ padding: '8px 12px', fontSize: 13, color: '#8e8e93' }}>
                  暂无可选模型
                </div>
              )}
              {models.map((m) => (
                <button
                  key={m.id}
                  type="button"
                  role="option"
                  aria-selected={m.id === currentModelId}
                  className={`${styles.modelItem} ${m.id === currentModelId ? styles.modelItemSelected : ''}`}
                  onClick={() => {
                    setModel(m.id)
                    setModelDropdownOpen(false)
                  }}
                >
                  <div className={styles.modelItemMain}>
                    <span>{m.display_name}</span>
                    {m.is_default && <span className={styles.defaultBadge}>默认</span>}
                  </div>
                  {m.id === currentModelId && <span className={styles.checkIcon}>✓</span>}
                </button>
              ))}
            </div>
          )}
        </div>
      </header>

      <div className={styles.messages}>
        {messages.length === 0 && (
          <div className={styles.empty}>
            <div className={styles.emptySparkle}>✦</div>
            <p className={styles.emptyPrompt}>说点什么，我来跑腿：</p>
            <p className={styles.hint}>动嘴一句话，买菜记账/查菜谱/分派家务全办妥</p>
            <div className={styles.promptCapsules}>
              <button
                type="button"
                className={styles.promptChip}
                onClick={() => void handlePromptClick('今天买菜花了 35 元')}
              >
                <span>💰</span> 今天买菜花了 35 元
              </button>
              <button
                type="button"
                className={styles.promptChip}
                onClick={() => void handlePromptClick('查查红烧肉怎么做')}
              >
                <span>🍳</span> 查查红烧肉怎么做
              </button>
              <button
                type="button"
                className={styles.promptChip}
                onClick={() => void handlePromptClick('提醒媳妇今晚洗碗')}
              >
                <span>🧹</span> 提醒媳妇今晚洗碗
              </button>
              <button
                type="button"
                className={styles.promptChip}
                onClick={() => void handlePromptClick('我还有什么待办活')}
              >
                <span>📋</span> 我还有什么待办活
              </button>
            </div>
          </div>
        )}
        {messages.map((m: ChatMessage) => {
          const isStreamingThis = m.id === useChatStore.getState().streamId
          const isEmpty = !m.content && !m.card
          if (isEmpty && !isStreamingThis) return null

          return (
            <div
              key={m.id}
              className={`${styles.bubble} ${m.role === 'user' ? styles.user : styles.ai}`}
            >
              {m.content ? (
                m.content
              ) : isStreamingThis ? (
                <span className={styles.thinkingInline}>
                  <span className={styles.pulseDot} />
                  <span>{status === 'tool_running' ? '⚡ 正在处理操作…' : '正在思考…'}</span>
                </span>
              ) : null}
              {m.card &&
                (m.card.type === 'expense' && !m.undone ? (
                  <ExpenseCard
                    card={m.card as Extract<ChatMessage['card'], { type: 'expense' }>}
                    undoable={Boolean(m.undoable) && !m.undone}
                    onUndo={() => void handleUndo(m.undoId)}
                    onUpdated={(next) => updateCard(m.id, next)}
                  />
                ) : (
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
                ))}
            </div>
          )
        })}
        {status === 'tool_running' &&
          !messages.some((m) => m.id === useChatStore.getState().streamId && !m.content) && (
            <div className={styles.statusPill}>
              <span className={styles.gearIcon}>⚡</span>
              <span>正在执行操作…</span>
            </div>
          )}
        {error && <div className={styles.error}>{error}</div>}
      </div>

      {/* 底部悬浮毛玻璃胶囊输入区（Grok + Apple 风格） */}
      <footer className={styles.inputContainer}>
        <div className={styles.inputCapsule}>
          <input
            className={styles.input}
            value={input}
            placeholder="输入消息…"
            enterKeyHint="send"
            autoCapitalize="off"
            autoCorrect="off"
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') void handleSend()
            }}
          />
          <button
            type="button"
            className={styles.sendBtn}
            onClick={() => void handleSend()}
            disabled={status !== 'idle'}
            aria-label="发送"
            title="发送"
          >
            <span className={styles.sendArrow}>↑</span>
            <span className={styles.srOnly}>发送</span>
          </button>
        </div>
      </footer>
    </div>
  )
}
