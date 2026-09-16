/**
 * 会话状态：zustand 全局 store（CONVENTIONS-frontend §3）。
 * 消息流是状态机（idle/streaming/tool_running/error），用 reducer 思路管理。
 */
import { create } from 'zustand'

import type { ChatStatus, MessageRole } from '@homeagent/shared'

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
}

interface ChatState {
  messages: ChatMessage[]
  status: ChatStatus
  currentModelId?: string

  appendMessage: (m: ChatMessage) => void
  appendToken: (id: string, token: string) => void
  appendCard: (id: string, card: ChatMessage['card'], undoable: boolean, undoId?: string) => void
  setStatus: (s: ChatStatus) => void
  setModel: (id: string) => void
  clear: () => void
}

export const useChatStore = create<ChatState>((set) => ({
  messages: [],
  status: 'idle',

  appendMessage: (m) => set((s) => ({ messages: [...s.messages, m] })),

  appendToken: (id, token) =>
    set((s) => ({
      messages: s.messages.map((m) => (m.id === id ? { ...m, content: m.content + token } : m)),
    })),

  appendCard: (id, card, undoable, undoId) =>
    set((s) => ({
      messages: s.messages.map((m) => (m.id === id ? { ...m, card, undoable, undoId } : m)),
    })),

  setStatus: (status) => set({ status }),
  setModel: (currentModelId) => set({ currentModelId }),
  clear: () => set({ messages: [], status: 'idle' }),
}))
