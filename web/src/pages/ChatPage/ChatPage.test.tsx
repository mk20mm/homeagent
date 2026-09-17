import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { useChatStore } from '../../stores/chat'
import { server } from '../../test/server'
import { ChatPage } from './ChatPage'

/** 构造一段 SSE 流（token → tool_call → done） */
function sseResponse(events: object[]): Response {
  const encoder = new TextEncoder()
  const stream = new ReadableStream({
    start(controller) {
      for (const e of events) {
        controller.enqueue(encoder.encode(`data: ${JSON.stringify(e)}\n\n`))
      }
      controller.close()
    },
  })
  return new Response(stream, {
    headers: { 'Content-Type': 'text/event-stream' },
  })
}

const CARD = { type: 'expense', amount: 120, category: '食材', hint: '买菜', time: '10:26' }

describe('ChatPage 对话联调', () => {
  beforeEach(() => {
    useChatStore.getState().clear()
    localStorage.clear()
    // ChatPage 进入时加载会话列表；首条消息无会话时自动新建
    server.use(
      http.get('/api/v1/conversations', () => HttpResponse.json({ items: [] })),
      http.post('/api/v1/conversations', () =>
        HttpResponse.json({ id: 'conv-1', member_id: 'm1' }, { status: 201 }),
      ),
    )
  })
  afterEach(() => useChatStore.getState().clear())

  it('流式消息进 assistant 气泡，工具卡片带撤销按钮', async () => {
    server.use(
      http.get('/api/v1/models', () =>
        HttpResponse.json({
          models: [
            { id: 'm1', model_name: 'deepseek-chat', display_name: 'DeepSeek', is_default: true },
          ],
        }),
      ),
      http.post('/api/v1/chat', () =>
        sseResponse([
          { type: 'token', content: '已记账' },
          { type: 'tool_call', tool: 'record_expense', card: CARD, undo_id: 'undo-1' },
          { type: 'done' },
        ]),
      ),
    )

    render(<ChatPage />)
    fireEvent.change(screen.getByPlaceholderText('输入消息…'), {
      target: { value: '今天买菜花了120' },
    })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))

    // 用户消息与 assistant 回复各自独立气泡（token 追加到 assistant 占位）
    await waitFor(() => expect(screen.getByText('已记账')).toBeInTheDocument())
    expect(screen.getByText('今天买菜花了120')).toBeInTheDocument()
    // 工具卡片渲染（与用户消息分属不同气泡）
    expect(screen.getAllByText(/120/).length).toBeGreaterThanOrEqual(2)

    // 撤销按钮可点
    expect(screen.getByRole('button', { name: '撤销' })).toBeInTheDocument()
  })

  it('撤销走 POST /undo 且本地立即回滚', async () => {
    const requests: string[] = []
    server.use(
      http.get('/api/v1/models', () => HttpResponse.json({ models: [] })),
      http.post('/api/v1/chat', () =>
        sseResponse([
          { type: 'tool_call', tool: 'record_expense', card: CARD, undo_id: 'undo-2' },
          { type: 'done' },
        ]),
      ),
      http.post('/api/v1/undo/:undoId', ({ request }) => {
        requests.push(`POST ${request.url}`)
        return HttpResponse.json({ undone: true })
      }),
    )

    render(<ChatPage />)
    fireEvent.change(screen.getByPlaceholderText('输入消息…'), {
      target: { value: '报饭' },
    })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))

    await waitFor(() => expect(screen.getByRole('button', { name: '撤销' })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: '撤销' }))

    await waitFor(() => expect(requests.length).toBeGreaterThan(0))
    expect(requests[0]).toContain('/api/v1/undo/undo-2')
    await waitFor(() => expect(screen.getByText('已撤销')).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: '撤销' })).not.toBeInTheDocument()
  })

  it('SSE error 事件走错误态', async () => {
    server.use(
      http.get('/api/v1/models', () => HttpResponse.json({ models: [] })),
      http.post('/api/v1/chat', () =>
        sseResponse([{ type: 'error', error: '服务暂时不可用，请稍后重试' }]),
      ),
    )

    render(<ChatPage />)
    fireEvent.change(screen.getByPlaceholderText('输入消息…'), { target: { value: 'hi' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))

    await waitFor(() => expect(screen.getByText('服务暂时不可用，请稍后重试')).toBeInTheDocument())
  })
})
