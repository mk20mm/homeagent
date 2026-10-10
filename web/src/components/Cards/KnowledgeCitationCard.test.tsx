import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { MemoryRouter } from 'react-router'
import { KnowledgeCitationCard } from './KnowledgeCitationCard'

describe('KnowledgeCitationCard', () => {
  const mockCard = {
    type: 'knowledge_citation' as const,
    query: '净水器滤芯怎么换',
    results: [
      {
        document_id: 'doc-1',
        document_title: '美的净水器使用说明书.pdf',
        category: 'manual',
        visibility: 'family',
        chunk_index: 0,
        content: '第一步：关掉水龙头。第二步：旋转拆卸RO滤芯。',
        score: 0.95,
        is_private: false,
      },
      {
        document_id: 'doc-2',
        document_title: '个人健康体检记录',
        category: 'health',
        visibility: 'private',
        chunk_index: 1,
        content: '甘油三酯指标2.1，注意低脂饮食。',
        score: 0.88,
        is_private: true,
      },
    ],
  }

  it('renders citation items with badges, titles, and scores', () => {
    render(
      <MemoryRouter>
        <KnowledgeCitationCard card={mockCard} />
      </MemoryRouter>,
    )

    expect(screen.getByText('家庭知识库引用')).toBeInTheDocument()
    expect(screen.getByText(/命中 2 处/)).toBeInTheDocument()
    expect(screen.getByText('🌐 家庭共享')).toBeInTheDocument()
    expect(screen.getByText('🔒 个人私密')).toBeInTheDocument()
    expect(screen.getByText('美的净水器使用说明书.pdf')).toBeInTheDocument()
    expect(screen.getByText('个人健康体检记录')).toBeInTheDocument()
    expect(screen.getByText('95% 匹配')).toBeInTheDocument()
    expect(screen.getByText('88% 匹配')).toBeInTheDocument()
  })

  it('toggles expand and collapse on click', () => {
    render(
      <MemoryRouter>
        <KnowledgeCitationCard card={mockCard} />
      </MemoryRouter>,
    )

    const docHeader = screen.getByText('美的净水器使用说明书.pdf')
    fireEvent.click(docHeader)
    expect(screen.getByText(/旋转拆卸RO滤芯/)).toBeInTheDocument()
  })
})
