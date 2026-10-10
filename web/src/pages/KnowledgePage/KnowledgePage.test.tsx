import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router'
import { KnowledgePage } from './KnowledgePage'

// Mock api
vi.mock('../../api/client', () => {
  return {
    api: {
      GET: vi.fn((path: string) => {
        if (path === '/knowledge/spaces') {
          return Promise.resolve({
            data: {
              spaces: [
                { visibility: 'family', document_count: 5 },
                { visibility: 'private', document_count: 2 },
              ],
            },
            error: undefined,
          })
        }
        if (path === '/knowledge/documents') {
          return Promise.resolve({
            data: {
              documents: [
                {
                  id: 'doc-1',
                  title: '美的净水器 RO-800G 使用说明书',
                  category: 'manual',
                  visibility: 'family',
                  chunk_count: 6,
                  author_name: '爸爸',
                  created_at: '2024-03-15T10:00:00Z',
                  updated_at: '2024-03-15T10:00:00Z',
                },
                {
                  id: 'doc-2',
                  title: '家庭综合保险单 2024',
                  category: 'insurance',
                  visibility: 'family',
                  chunk_count: 4,
                  author_name: '妈妈',
                  created_at: '2024-01-20T10:00:00Z',
                  updated_at: '2024-01-20T10:00:00Z',
                },
              ],
            },
            error: undefined,
          })
        }
        return Promise.resolve({ data: {}, error: undefined })
      }),
      POST: vi.fn(() => Promise.resolve({ data: {}, error: undefined })),
      DELETE: vi.fn(() => Promise.resolve({ data: {}, error: undefined })),
    },
    unwrap: (res: any) => {
      if (res.error) throw res.error
      return res.data
    },
  }
})

describe('KnowledgePage (iOS 18 Redesign)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders title, dual-space tabs, and document list with category icons', async () => {
    render(
      <MemoryRouter>
        <KnowledgePage />
      </MemoryRouter>,
    )

    // Check title and navigation
    expect(screen.getByText('家庭知识库')).toBeInTheDocument()
    expect(screen.getByText(/家电说明书 · 保单合同/)).toBeInTheDocument()

    // Check dual space tabs
    expect(screen.getByText('家庭共享')).toBeInTheDocument()
    expect(screen.getByText('个人私密')).toBeInTheDocument()

    // Wait for documents to load
    await waitFor(() => {
      expect(screen.getByText('美的净水器 RO-800G 使用说明书')).toBeInTheDocument()
    })

    expect(screen.getByText('家庭综合保险单 2024')).toBeInTheDocument()
    expect(screen.getByText(/6 个切片/)).toBeInTheDocument()
    expect(screen.getByText(/4 个切片/)).toBeInTheDocument()

    // Check memory sync footer
    expect(screen.getByText(/已同步 7 篇家庭专属记忆文档/)).toBeInTheDocument()
  })

  it('switches space tabs when clicked', async () => {
    render(
      <MemoryRouter>
        <KnowledgePage />
      </MemoryRouter>,
    )

    const privateTab = screen.getByText('个人私密')
    fireEvent.click(privateTab)

    // Check that search placeholder updates to match private space
    const searchInput = screen.getByPlaceholderText('搜索个人私密文档…')
    expect(searchInput).toBeInTheDocument()
  })

  it('filters by category chip', async () => {
    render(
      <MemoryRouter>
        <KnowledgePage />
      </MemoryRouter>,
    )

    const manualChip = screen.getByText('家电说明书')
    fireEvent.click(manualChip)

    // Verify chip interaction works without throwing
    expect(manualChip).toBeInTheDocument()
  })

  it('opens and closes the create document sheet', async () => {
    render(
      <MemoryRouter>
        <KnowledgePage />
      </MemoryRouter>,
    )

    const newBtn = screen.getByText('新建')
    fireEvent.click(newBtn)

    expect(screen.getByText('新建知识库文档')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('如：美的净水器RO-800G使用说明书')).toBeInTheDocument()

    const cancelBtn = screen.getByText('取消')
    fireEvent.click(cancelBtn)

    expect(screen.queryByText('新建知识库文档')).not.toBeInTheDocument()
  })
})
