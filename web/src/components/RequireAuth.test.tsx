import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { saveAuth } from '@homeagent/shared'

import { RequireAuth } from './RequireAuth'

function Protected() {
  return <div>受保护内容</div>
}

describe('RequireAuth 路由守卫', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => localStorage.clear())

  it('未登录重定向到 /login', () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <Routes>
          <Route path="/" element={<RequireAuth><Protected /></RequireAuth>} />
          <Route path="/login" element={<div>登录页</div>} />
        </Routes>
      </MemoryRouter>,
    )
    expect(screen.queryByText('受保护内容')).not.toBeInTheDocument()
    expect(screen.getByText('登录页')).toBeInTheDocument()
  })

  it('已登录渲染受保护内容（快照稳定，不无限重渲染）', () => {
    saveAuth({
      token: 't',
      member_id: 'm1',
      role: 'parent',
      expires_at: '2099-01-01T00:00:00Z',
    })
    render(
      <MemoryRouter initialEntries={['/']}>
        <Routes>
          <Route path="/" element={<RequireAuth><Protected /></RequireAuth>} />
          <Route path="/login" element={<div>登录页</div>} />
        </Routes>
      </MemoryRouter>,
    )
    expect(screen.getByText('受保护内容')).toBeInTheDocument()
  })
})
