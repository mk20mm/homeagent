import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { ResultCard } from './ResultCard'

describe('ResultCard', () => {
  it('撤销窗口内显示撤销按钮并触发回调', async () => {
    const onUndo = vi.fn()
    render(
      <ResultCard title="记账成功" undoable onUndo={onUndo}>
        买菜 ¥12.50
      </ResultCard>,
    )

    expect(screen.getByText('记账成功')).toBeInTheDocument()
    const undo = screen.getByRole('button', { name: '撤销' })
    await userEvent.click(undo)
    expect(onUndo).toHaveBeenCalledOnce()
  })

  it('不可撤销时不渲染撤销按钮', () => {
    render(<ResultCard title="仅展示">内容</ResultCard>)
    expect(screen.queryByRole('button', { name: '撤销' })).not.toBeInTheDocument()
  })
})
