import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SearchCitationCard } from './SearchCitationCard'

describe('SearchCitationCard', () => {
  it('renders query, verified badge, and citation links correctly', () => {
    const cardData = {
      type: 'search_citation' as const,
      query: '西门子洗碗机 E15 故障码排查',
      results: [
        {
          title: '西门子官方家电故障代码表',
          url: 'https://siemens-home.bsh-group.cn/faq/e15',
          snippet: '拔掉电源插头，前倾45度倒出底盘积水',
          source_name: '西门子家电服务中心',
          is_official: true,
        },
        {
          title: '家电维修实录',
          url: 'https://weixiu.example.com/e15',
          snippet: '检查进水阀密封圈是否老化',
          source_name: '生活技巧家',
          is_official: false,
        },
      ],
    }

    render(<SearchCitationCard card={cardData} />)

    expect(screen.getByText('权威信源参考')).toBeInTheDocument()
    expect(screen.getByText('(2 个来源)')).toBeInTheDocument()
    expect(screen.getByText('验证无误')).toBeInTheDocument()
    expect(screen.getByText('官方信源')).toBeInTheDocument()
    expect(screen.getByText('西门子官方家电故障代码表')).toBeInTheDocument()
    expect(screen.getByText('西门子家电服务中心')).toBeInTheDocument()
    expect(screen.getByText('家电维修实录')).toBeInTheDocument()

    // Citation links should open in new tab with security attributes
    const link = screen.getByRole('link', { name: /西门子官方家电故障代码表/ })
    expect(link).toHaveAttribute('href', 'https://siemens-home.bsh-group.cn/faq/e15')
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  })
})
