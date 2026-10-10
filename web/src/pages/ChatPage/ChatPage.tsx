/**
 * 对话主页：AI 交互入口，苹果极简设计系统（Apple Minimalist + Grok 毛玻璃胶囊）。
 * 保持全站色调与视觉语言绝对统一（浅灰底 #F2F3F7 + 纯白卡片 + 苹果蓝 + 毛玻璃悬浮胶囊）。
 * 消息先乐观渲染，工具结果以服务器回执为准（CONVENTIONS-frontend §4）。
 */
import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router'

import { ERROR_MESSAGE, type ErrorCode } from '@homeagent/shared'

import { api, ApiError, unwrap } from '../../api/client'
import { KnowledgeCitationCard } from '../../components/Cards/KnowledgeCitationCard'
import { SearchCitationCard } from '../../components/Cards/SearchCitationCard'
import { ConversationSidebar } from '../../components/ConversationSidebar'
import { ExpenseCard } from '../../components/ExpenseCard'
import { ResultCard } from '../../components/ResultCard'
import { ThoughtProcess } from '../../components/ThoughtProcess'
import { StructuredContent } from '../../components/StructuredContent/StructuredContent'
import { VoiceRecorderModal } from '../../components/VoiceRecorder/VoiceRecorderModal'
import { useSSE, type SSEEvent } from '../../hooks/useSSE'
import { useChatStore, type ChatMessage } from '../../stores/chat'

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
    setRunId,
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
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [voiceModalOpen, setVoiceModalOpen] = useState(false)
  const [webSearchActive, setWebSearchActive] = useState(false)

  const dropdownRef = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()

  const { connect, abort } = useSSE({
    url: '/api/v1/chat',
    onEvent: (e: SSEEvent) => {
      if (e.type === 'run_started') {
        setRunId(e.run_id)
      } else if (e.type === 'token') {
        appendToken(e.content)
      } else if (e.type === 'tool_call') {
        if (e.run_id) setRunId(e.run_id)
        setStatus('tool_running')
        appendCard(e.card as ChatMessage['card'], Boolean(e.undo_id), e.undo_id, e.tool)
      } else if (e.type === 'done') {
        if (e.run_id) setRunId(e.run_id)
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
    let content = input.trim()
    if (!content || status !== 'idle') return

    if (webSearchActive && !content.startsWith('帮我查') && !content.startsWith('搜索') && !content.startsWith('查一下')) {
      content = `帮我联网查一下：${content}`
    }

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

  const handleCopyMessage = (msgId: string, text: string) => {
    if (!text) return
    void navigator.clipboard.writeText(text)
    setCopiedId(msgId)
    setTimeout(() => setCopiedId(null), 1500)
  }

  const currentModel = models.find((m) => m.id === currentModelId)

  return (
    <div className={styles.page}>
      <ConversationSidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)} />

      {/* 顶部应用导航栏（单行排布，保证在移动端绝不拥挤折行） */}
      <header className={styles.header}>
        <div className={styles.headerLeft}>
          <button
            type="button"
            className={styles.menuBtn}
            aria-label="会话列表"
            title="会话列表"
            onClick={() => setSidebarOpen(true)}
          >
            ☰
          </button>
          <h1 className={styles.title}>家事助手</h1>
          <button
            type="button"
            className={styles.knowledgeHeaderBtn}
            onClick={() => navigate('/knowledge')}
            title="打开家庭知识库"
          >
            <span>📚</span>
            <span>知识库</span>
          </button>
        </div>

        {/* 苹果毛玻璃胶囊模型选择器 */}
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
            <span
              className={`${styles.chevron} ${modelDropdownOpen ? styles.chevronOpen : ''}`}
            >
              ▾
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

      {/* 消息流主视口 */}
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
              <button
                type="button"
                className={styles.promptChip}
                onClick={() => void handlePromptClick('查查净水器滤芯怎么换')}
              >
                <span>📖</span> 查查净水器滤芯怎么换
              </button>
            </div>
          </div>
        )}

        {messages.map((m: ChatMessage, idx: number) => {
          const isStreamingThis = m.id === useChatStore.getState().streamId
          const isEmpty = !m.content && !m.card
          if (isEmpty && !isStreamingThis) return null

          if (m.role === 'user') {
            return (
              <div key={m.id} className={`${styles.bubble} ${styles.user}`}>
                {m.content}
              </div>
            )
          }

          // Assistant 消息卡片
          return (
            <div key={m.id} className={`${styles.bubble} ${styles.ai}`}>
              {/* 助手消息头部 */}
              <div className={styles.aiHeader}>
                <div className={styles.aiBrand}>
                  <span className={styles.aiIcon}>✦</span>
                  <span className={styles.aiName}>AI 助手</span>
                  <span className={styles.aiModel}>
                    {currentModel ? currentModel.display_name : '默认'}
                  </span>
                </div>
                {isStreamingThis && (
                  <div className={styles.thinkingInline}>
                    <span className={styles.pulseDot} />
                    <span>{status === 'tool_running' ? '正在执行操作…' : '正在思考…'}</span>
                  </div>
                )}
              </div>

              {/* 思考过程与执行轨迹组件：展示真实推演链路，支持一键打开无限画布 */}
              {(isStreamingThis || m.content || m.card) && (
                <ThoughtProcess
                  prompt={m.prompt || (idx > 0 && messages[idx - 1]?.role === 'user' ? messages[idx - 1].content : '')}
                  toolName={m.toolName}
                  card={m.card}
                  runId={m.runId}
                  isStreaming={isStreamingThis && (status === 'streaming' || status === 'tool_running')}
                  status={status}
                  defaultOpen={isStreamingThis}
                  onCancel={abort}
                  onSteer={(instruction) => {
                    appendToken(`\n\n[人类实时干预指令: ${instruction}]\n`)
                  }}
                />
              )}

              {/* 消息正文：支持 Markdown、步骤卡片化、结论高亮与快捷闭环动作 */}
              {m.content && (
                <div className={styles.aiContent}>
                  <StructuredContent
                    content={m.content}
                    onQuickAction={(actionText) => void handlePromptClick(actionText)}
                  />
                </div>
              )}

              {/* 工具结果卡片 */}
              {m.card &&
                (m.card.type === 'expense' && !m.undone ? (
                  <ExpenseCard
                    card={m.card as Extract<ChatMessage['card'], { type: 'expense' }>}
                    undoable={Boolean(m.undoable) && !m.undone}
                    onUndo={() => void handleUndo(m.undoId)}
                    onUpdated={(next) => updateCard(m.id, next)}
                  />
                ) : m.card.type === 'search_citation' ? (
                  <SearchCitationCard
                    card={
                      m.card as React.ComponentProps<typeof SearchCitationCard>['card']
                    }
                  />
                ) : m.card.type === 'knowledge_citation' ? (
                  <KnowledgeCitationCard
                    card={
                      m.card as React.ComponentProps<typeof KnowledgeCitationCard>['card']
                    }
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

              {/* 回复底部快捷操作栏 */}
              {m.content && !isStreamingThis && (
                <div className={styles.actionToolbar}>
                  <div className={styles.toolbarActions}>
                    <button
                      type="button"
                      className={styles.toolbarBtn}
                      onClick={() => handleCopyMessage(m.id, m.content)}
                      title="复制回复"
                    >
                      {copiedId === m.id ? '✓ 已复制' : '复制'}
                    </button>
                    <button
                      type="button"
                      className={styles.toolbarBtn}
                      onClick={() => void handlePromptClick(m.content)}
                      title="重新生成"
                    >
                      重新提问
                    </button>
                  </div>
                  <span className={styles.toolbarTag}>✦ 家事中枢</span>
                </div>
              )}
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

      {/* 底部悬浮毛玻璃胶囊输入区（与 TabBar 完美贴合，绝不遮挡） */}
      <footer className={styles.inputContainer}>
        <div className={styles.inputCapsule}>
          <button
            type="button"
            className={`${styles.webSearchBadge} ${webSearchActive ? styles.webSearchBadgeActive : ''}`}
            onClick={() => setWebSearchActive((v) => !v)}
            title="切换智能联网搜索"
          >
            <span>🌐</span>
            <span>{webSearchActive ? '联网开启' : '智能联网'}</span>
          </button>
          <input
            className={styles.input}
            value={input}
            placeholder={webSearchActive ? '输入联网检索问题…' : '输入消息…'}
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
            className={styles.micBtn}
            onClick={() => setVoiceModalOpen(true)}
            title="按住/点击语音录音（100% 本地免费识别）"
            aria-label="语音录音输入"
          >
            🎙️
          </button>
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

      {/* 100% 本地免费语音转文字录音抽屉（对齐 Stitch iOS 18 拟态磨砂设计） */}
      <VoiceRecorderModal
        isOpen={voiceModalOpen}
        onClose={() => setVoiceModalOpen(false)}
        onFinish={(text) => {
          setInput((prev) => (prev ? prev + ' ' + text : text))
        }}
      />
    </div>
  )
}
