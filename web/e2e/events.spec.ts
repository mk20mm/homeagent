/**
 * A-02 验收（PRD E2E-02/03/04）：
 * - 日程列表入口存在、按日期分组展示
 * - 对话/接口创建的事件可在日程详情页找到，字段一致
 * - 重复事件「跳过单次」只影响目标实例（不误删整系列）
 * - 跨成员 private 事件：他人列表不可见、深链 404（防探测）
 * 事件通过 POST /events 直接造（不经 LLM，保证稳定）。
 */
import { test, expect, type Page } from '@playwright/test'

import { login } from './helpers'

const SHOT = 'e2e-shots'

const BASE = 'http://localhost:8080/api/v1'

async function auth(page: Page) {
  const raw = await page.evaluate(() => localStorage.getItem('homeagent.token'))
  if (!raw) throw new Error('未登录')
  return JSON.parse(raw) as { token: string; memberId: string }
}

async function loginAs(page: Page, name: string, token: string) {
  await page.goto('/login')
  await page.getByPlaceholder('成员名（如：爸爸）').fill(name)
  await page.getByPlaceholder('登录令牌').fill(token)
  await page.getByRole('button', { name: '登录' }).click()
  await page.waitForURL('/')
}

async function createEvent(
  page: Page,
  token: string,
  body: Record<string, unknown>,
): Promise<{ id: string }> {
  const res = await page.request.post(`${BASE}/events`, {
    headers: { Authorization: `Bearer ${token}` },
    data: body,
  })
  expect(res.status()).toBe(201)
  return (await res.json()) as { id: string }
}

test.describe('日程模块（A-02）', () => {
  test('日程列表有入口、详情页字段一致（E2E-02）', async ({ page }) => {
    await login(page)
    const { token } = await auth(page)

    const start = '2026-10-07T15:00:00+08:00'
    // 完整时间戳保证标题唯一，避免与历史残留事件撞名点到旧事件
    const title = `e2e详情${Date.now()}`
    const ev = await createEvent(page, token, {
      title,
      start_at: start,
      repeat: 'weekly',
      description: '三年级家长会',
    })

    // TabBar 有日程入口
    await page.goto('/')
    await expect(page.getByRole('link', { name: '日程' })).toBeVisible()
    await page.getByRole('link', { name: '日程' }).click()
    await expect(page).toHaveURL('/events')
    await expect(page.getByRole('heading', { name: '日程' })).toBeVisible()
    await page.waitForTimeout(500)
    await page.screenshot({ path: `${SHOT}/ev-01-list.png` })

    // 列表项可点进详情（周重复展开为多条实例，取第一条）
    const item = page.getByText(title).first()
    await expect(item).toBeVisible()
    await item.click()
    await expect(page).toHaveURL(new RegExp(`/events/${ev.id}$`))
    await page.waitForTimeout(500)
    await page.screenshot({ path: `${SHOT}/ev-02-detail.png` })

    // 详情字段与服务端一致
    await expect(page.getByText('15:00')).toBeVisible()
    await expect(page.getByText('每周')).toBeVisible()
    await expect(page.getByText('三年级家长会')).toBeVisible()
    // can_edit=false → 不渲染「修改」入口，只有「删除整条日程」
    await expect(page.getByText('修改单次或整条日程暂未开放')).toBeVisible()
    await expect(page.getByRole('button', { name: '删除整条日程' })).toBeVisible()
  })

  test('重复事件跳过单次不影响系列（E2E-03）', async ({ page }) => {
    await login(page)
    const { token } = await auth(page)

    const ev = await createEvent(page, token, {
      title: `e2e 跳过${Date.now() % 10000}`,
      start_at: '2026-10-06T09:00:00+08:00',
      repeat: 'weekly',
    })

    // 跳过 2026-10-13 那一次（第二周）
    const occurrence = '2026-10-13T01:00:00Z'
    const skipRes = await page.request.delete(
      `${BASE}/events/${ev.id}/instances/${occurrence}`,
      { headers: { Authorization: `Bearer ${token}` } },
    )
    expect(skipRes.status()).toBe(200)

    // 列表里该次消失，但系列其他实例仍在
    await page.goto('/events')
    await page.waitForTimeout(500)
    const listRes = await page.request.get(`${BASE}/events?start=2026-10-05T00:00:00Z&end=2026-11-01T00:00:00Z`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    const list = (await listRes.json()) as { items: Array<{ id: string; occurrence: string }> }
    const mine = list.items.filter((i) => i.id === ev.id)
    expect(mine.length).toBeGreaterThanOrEqual(1)
    expect(mine.some((i) => i.occurrence === occurrence)).toBe(false)
  })

  test('跨成员 private 事件深链被拒（E2E-04）', async ({ page, browser }) => {
    // 爸爸建 private 事件
    await login(page)
    const dadToken = (await auth(page)).token
    const ev = await createEvent(page, dadToken, {
      title: `e2e 私事${Date.now() % 10000}`,
      start_at: '2026-10-08T20:00:00+08:00',
      visibility: 'private',
    })

    // 孩子在独立上下文登录（同 context 共享 localStorage 会被爸爸的 token 自动登录）
    const kidCtx = await browser.newContext({ baseURL: 'http://localhost:5173' })
    const kidPage = await kidCtx.newPage()
    try {
      await loginAs(kidPage, '孩子', 'dev-haizi')
      const kidToken = (await auth(kidPage)).token

      // 孩子直接请求详情接口 → 404（与不存在同码）
      const res = await kidPage.request.get(`${BASE}/events/${ev.id}`, {
        headers: { Authorization: `Bearer ${kidToken}` },
      })
      expect(res.status()).toBe(404)

      // 孩子的日程列表里看不到该事件
      const listRes = await kidPage.request.get(
        `${BASE}/events?start=2026-10-07T00:00:00Z&end=2026-10-10T00:00:00Z`,
        { headers: { Authorization: `Bearer ${kidToken}` } },
      )
      const list = (await listRes.json()) as { items: Array<{ id: string }> }
      expect(list.items.some((i) => i.id === ev.id)).toBe(false)

      // 孩子直接打开深链 → 页面提示不存在/无权限，不展示内容
      await kidPage.goto(`/events/${ev.id}`)
      await kidPage.waitForTimeout(500)
      await expect(kidPage.getByText('日程不存在或你没有查看权限')).toBeVisible()
      await kidPage.screenshot({ path: `${SHOT}/ev-03-private-denied.png` })
    } finally {
      await kidCtx.close()
    }
  })
})
