/**
 * 会话状态：zustand 全局 store（CONVENTIONS-frontend §3）。
 * 消息流是状态机（idle/streaming/tool_running/error），用 reducer 思路管理。
 *
 * 流式消息 id 由 store 持有（streamId），避免 useSSE 回调闭包拿到过期 id。
 */
import { create } from 'zustand'

import type { ChatStatus, MessageRole } from '@homeagent/shared'

import { api, unwrap } from '../api/client'

export interface ChatMessage {
  id: string
  role: MessageRole
  content: string
  toolName?: string
  /** 工具结果卡片（前端渲染 + 撤销按钮） */
  card?: {
    type: string
    [key: string]: unknown
  }
  /** 该卡片是否可撤销（24h 窗口内） */
  undoable?: boolean
  undoId?: string
  /** 撤销已生效（前端乐观回滚） */
  undone?: boolean
}

export interface ConversationItem {
  id: string
  title: string
  lastMessageAt?: string
}

interface ChatState {
  messages: ChatMessage[]
  status: ChatStatus
  /** 当前流式 assistant 消息 id */
  streamId: string | null
  currentModelId?: string
  /** 当前会话 id（新建对话/切换会话用） */
  currentConversationId: string | null
  /** 会话列表（侧边栏） */
  conversations: ConversationItem[]

  /** 发送前调用：追加上用户消息 + 空的 assistant 占位，进入 streaming */
  beginStream: (userContent: string) => void
  appendToken: (token: string) => void
  appendCard: (card: ChatMessage['card'], undoable: boolean, undoId?: string) => void
  updateCard: (messageId: string, next: Partial<ChatMessage['card']>) => void
  markUndone: (undoId: string) => void
  setStatus: (s: ChatStatus) => void
  setModel: (id: string) => void
  clear: () => void

  /** 拉取会话列表 */
  loadConversations: () => Promise<void>
  /** 切换会话：加载历史消息 */
  selectConversation: (id: string) => Promise<void>
  /** 新建会话（显式创建，拿到 id） */
  newConversation: () => Promise<void>
  /** 删除会话（软删除）；删的是当前会话则清空消息 */
  deleteConversation: (id: string) => Promise<void>
}

function newId(): string {
  return typeof crypto !== 'undefined' && crypto.randomUUID
    ? crypto.randomUUID()
    : `m${Date.now()}${Math.random().toString(36).slice(2, 8)}`
}

export const useChatStore = create<ChatState>((set, get) => ({
  messages: [],
  status: 'idle',
  streamId: null,
  currentConversationId: null,
  conversations: [],

  beginStream: (userContent) =>
    set((s) => {
      const streamId = newId()
      return {
        status: 'streaming',
        streamId,
        messages: [
          ...s.messages,
          { id: newId(), role: 'user', content: userContent },
          { id: streamId, role: 'assistant', content: '' },
        ],
      }
    }),

  appendToken: (token) =>
    set((s) => ({
      messages: s.messages.map((m) =>
        m.id === s.streamId ? { ...m, content: m.content + token } : m,
      ),
    })),

  appendCard: (card, undoable, undoId) =>
    set((s) => ({
      messages: s.messages.map((m) =>
        m.id === s.streamId ? { ...m, card, undoable, undoId } : m,
      ),
    })),

  markUndone: (undoId) =>
    set((s) => ({
      messages: s.messages.map((m) =>
        m.undoId === undoId ? { ...m, undone: true, undoable: false } : m,
      ),
    })),

  updateCard: (messageId, next) =>
    set((s) => ({
      messages: s.messages.map((m) =>
        m.id === messageId && m.card ? { ...m, card: { ...m.card, ...next } } : m,
      ),
    })),

  setStatus: (status) => set({ status }),
  setModel: (currentModelId) => set({ currentModelId }),
  clear: () => set({ messages: [], status: 'idle', streamId: null }),

  loadConversations: async () => {
    const data = unwrap(
      await api.GET('/conversations', { params: { query: { page_size: 50 } } }),
    )
    set({
      conversations: data.items.map((c) => ({
        id: c.id,
        title: c.title || '新对话',
        lastMessageAt: c.last_message_at ?? undefined,
      })),
    })
  },

  selectConversation: async (id) => {
    const data = unwrap(
      await api.GET('/conversations/{conversationId}/messages', {
        params: { path: { conversationId: id } },
      }),
    )
    // 只展示 user/assistant 气泡；tool 回执已在流的 card 里展示，历史不重复渲染
    const messages: ChatMessage[] = data.items
      .filter((m) => m.role === 'user' || m.role === 'assistant')
      .map((m) => ({
        id: m.id,
        role: m.role as MessageRole,
        content: m.content,
      }))
    set({
      currentConversationId: id,
      messages,
      status: 'idle',
      streamId: null,
    })
  },

  newConversation: async () => {
    const data = unwrap(await api.POST('/conversations', { body: {} }))
    set({
      currentConversationId: data.id,
      messages: [],
      status: 'idle',
      streamId: null,
    })
  },

  deleteConversation: async (id) => {
    unwrap(
      await api.DELETE('/conversations/{conversationId}', {
        params: { path: { conversationId: id } },
      }),
    )
    const isCurrent = get().currentConversationId === id
    set((s) => ({
      conversations: s.conversations.filter((c) => c.id !== id),
      ...(isCurrent
        ? { currentConversationId: null, messages: [], status: 'idle' as const, streamId: null }
        : {}),
    }))
  },
}))
