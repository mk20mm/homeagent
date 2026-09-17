import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { clearAuth, getAuth, saveAuth, getToken, subscribeAuth } from './auth'

describe('auth token store', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => localStorage.clear())

  it('保存并取回登录态', () => {
    const state = saveAuth({
      token: 'aaa.bbb.ccc',
      member_id: 'm1',
      role: 'parent',
      expires_at: '2099-01-01T00:00:00Z',
    })
    expect(state.role).toBe('parent')
    expect(getAuth()?.memberId).toBe('m1')
    expect(getToken()).toBe('aaa.bbb.ccc')
  })

  it('JWT exp 优先于 response.expires_at', () => {
    // payload {"exp":4102444800}
    const token = `header.${btoa(JSON.stringify({ exp: 4102444800 }))}.sig`
    saveAuth({ token, member_id: 'm1', role: 'child', expires_at: '2000-01-01T00:00:00Z' })
    expect(getAuth()?.expiresAt).toBe(4102444800)
  })

  it('过期令牌取回时被清空', () => {
    const token = `header.${btoa(JSON.stringify({ exp: 1000 }))}.sig`
    saveAuth({ token, member_id: 'm1', role: 'child', expires_at: '2099-01-01T00:00:00Z' })
    expect(getAuth()).toBeNull()
    expect(getToken()).toBeNull()
  })

  it('脏数据兜底为空', () => {
    localStorage.setItem('homeagent.token', '{not json')
    expect(getAuth()).toBeNull()
  })

  it('登录态变化通知订阅者', () => {
    let changed = 0
    const unsub = subscribeAuth(() => {
      changed++
    })
    saveAuth({ token: 't', member_id: 'm1', role: 'parent', expires_at: '2099-01-01T00:00:00Z' })
    clearAuth()
    expect(changed).toBeGreaterThanOrEqual(2)
    unsub()
  })

  it('getAuth 快照引用稳定（useSyncExternalStore 用 Object.is 比对）', () => {
    saveAuth({ token: 't', member_id: 'm1', role: 'parent', expires_at: '2099-01-01T00:00:00Z' })
    expect(getAuth()).toBe(getAuth())
    // localStorage 被外部清空后能感知，且清空态引用也稳定
    localStorage.clear()
    expect(getAuth()).toBe(null)
  })
})
