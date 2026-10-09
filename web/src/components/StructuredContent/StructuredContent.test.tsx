import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { StructuredContent } from './StructuredContent'

describe('StructuredContent', () => {
  it('renders numbered step items as structured cards', () => {
    const content = `水龙头图标红灯常见原因：

1. **进水问题**：水龙头没开或进水滤网堵塞。
   → 先把水龙头拧开，清洗进水滤网。
2. **排水堵塞**：排水泵入口被杂物堵塞。
   → 拆底部滤网清洗。

建议按 1→2 顺序排查。

需要我帮你建个维修待办任务吗？`

    const onQuickAction = vi.fn()
    render(<StructuredContent content={content} onQuickAction={onQuickAction} />)

    expect(screen.getByText('建议排查指引与步骤')).toBeInTheDocument()
    expect(screen.getByText('进水问题')).toBeInTheDocument()
    expect(screen.getByText('排水堵塞')).toBeInTheDocument()
    expect(screen.getByText('先把水龙头拧开，清洗进水滤网。')).toBeInTheDocument()

    // 核心建议 callout
    expect(screen.getByText('核心建议：')).toBeInTheDocument()

    // 快捷行动胶囊
    const chip = screen.getByText('帮我建个洗碗机排查待办')
    expect(chip).toBeInTheDocument()
    fireEvent.click(chip)
    expect(onQuickAction).toHaveBeenCalledWith('帮我建个洗碗机排查待办')
  })
})
