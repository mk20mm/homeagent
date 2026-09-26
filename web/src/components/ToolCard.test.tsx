import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router'

import { ToolCard } from './ToolCard'

/** MemoryRouter + 路由表：深链跳转后由出口组件显示 pathname，便于断言目标 */
function renderCard(card: { type: string; [key: string]: unknown }) {
  render(
    <MemoryRouter initialEntries={['/chat']}>
      <Routes>
        <Route path="/chat" element={<ToolCard card={card} />} />
        <Route path="*" element={<LocationProbe />} />
      </Routes>
    </MemoryRouter>,
  )
}

function LocationProbe() {
  const { pathname } = useLocation()
  return <div>landed:{pathname}</div>
}

describe('ToolCard（A-03 结构化结果卡片）', () => {
  it('task：显示任务名/负责人，可跳家务页', async () => {
    renderCard({
      type: 'task',
      task_id: 't1',
      title: '洗碗',
      assignee: '媳妇',
      due_at: '今晚',
      status: 'pending',
    })
    expect(screen.getByText('已派任务「洗碗」')).toBeInTheDocument()
    expect(screen.getByText('负责人：媳妇')).toBeInTheDocument()
    expect(screen.getByText('截止：今晚')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '去家务页查看' }))
    expect(screen.getByText('landed:/chores')).toBeInTheDocument()
    // 关键：不出现原始 JSON
    expect(screen.queryByText(/"type":"task"/)).not.toBeInTheDocument()
  })

  it('task 幂等命中：给准确文案，不说「记账」', () => {
    renderCard({ type: 'task', task_id: 't1', title: '洗碗', duplicated: true })
    expect(screen.getByText('这个任务已经派过了，未重复创建')).toBeInTheDocument()
  })

  it('task_done：显示打卡完成，可跳家务页', async () => {
    renderCard({
      type: 'task_done',
      task_id: 't1',
      title: '倒垃圾',
      points: '+1',
    })
    expect(screen.getByText('已完成「倒垃圾」打卡')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '去家务页查看' }))
    expect(screen.getByText('landed:/chores')).toBeInTheDocument()
  })

  it('meal：显示在家/不在家与日期，可跳报饭页', async () => {
    renderCard({
      type: 'meal',
      at_home: false,
      date: '今天',
      note: '加班',
    })
    expect(screen.getByText('已报饭（今天）：今晚不在家吃')).toBeInTheDocument()
    expect(screen.getByText('加班')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '去报饭页查看' }))
    expect(screen.getByText('landed:/meal')).toBeInTheDocument()
  })

  it('event：显示时间/重复/可见，可跳日程详情', async () => {
    renderCard({
      type: 'event',
      event_id: 'ev-1',
      title: '开家长会',
      start_at: '2026-10-07T15:00:00+08:00',
      repeat: 'weekly',
      visibility: 'family',
    })
    expect(screen.getByText('已建日程「开家长会」')).toBeInTheDocument()
    expect(screen.getByText('时间：10月7日 15:00')).toBeInTheDocument()
    expect(screen.getByText('重复：每周')).toBeInTheDocument()
    expect(screen.getByText('可见：全家可见')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '去日程详情' }))
    expect(screen.getByText('landed:/events/ev-1')).toBeInTheDocument()
  })

  it('event private：可见范围标注仅自己', () => {
    renderCard({
      type: 'event',
      event_id: 'ev-2',
      title: '私事',
      start_at: '2026-10-08T20:00:00+08:00',
      repeat: 'once',
      visibility: 'private',
    })
    expect(screen.getByText('可见：仅自己可见')).toBeInTheDocument()
  })

  it('event 幂等命中：给准确文案', () => {
    renderCard({
      type: 'event',
      event_id: 'ev-1',
      title: '开家长会',
      duplicated: true,
    })
    expect(screen.getByText('这个日程已经建过了，未重复创建')).toBeInTheDocument()
  })

  it('未知类型兜底：不展示原始 JSON', () => {
    renderCard({ type: 'something_new', foo: 'bar' })
    expect(screen.getByText('已处理')).toBeInTheDocument()
    expect(screen.queryByText(/"foo":"bar"/)).not.toBeInTheDocument()
  })
})
