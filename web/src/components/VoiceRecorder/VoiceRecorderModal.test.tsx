import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { VoiceRecorderModal } from './VoiceRecorderModal'

describe('VoiceRecorderModal', () => {
  beforeEach(() => {
    // Mock navigator.mediaDevices
    Object.defineProperty(navigator, 'mediaDevices', {
      value: {
        getUserMedia: vi.fn().mockRejectedValue(new Error('no mic in jsdom')),
      },
      configurable: true,
      writable: true,
    })
  })

  it('does not render content when isOpen is false', () => {
    const { container } = render(
      <VoiceRecorderModal isOpen={false} onClose={() => {}} onFinish={() => {}} />,
    )
    expect(container.firstChild).toBeNull()
  })

  it('renders title, timer, hint, and buttons when isOpen is true', () => {
    const onClose = vi.fn()
    const onFinish = vi.fn()

    render(<VoiceRecorderModal isOpen={true} onClose={onClose} onFinish={onFinish} />)

    expect(screen.getByText('正在聆听…')).toBeInTheDocument()
    expect(screen.getByText(/AI 实时理解/)).toBeInTheDocument()
    expect(screen.getByText('00:00 / 01:00')).toBeInTheDocument()
    expect(screen.getByText(/请说话，支持记账/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /取消/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /转文字填入/ })).toBeInTheDocument()

    // Clicking cancel triggers onClose
    fireEvent.click(screen.getByRole('button', { name: /取消/ }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })
})
