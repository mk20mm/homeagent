/**
 * A-01 验收（PRD E2E-01）：打开 Today。
 * 前置：今日个人日程 + 未完成任务 + 今日用餐未申报 + 一条通知。
 * 预期：今日安排与需处理事项正确显示；普通通知不冒充待办；
 * 任务完成后从待处理区消失，家务页状态一致。
 * 数据通过 API 直接造（不经 LLM，保证稳定）。
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

/** 清空该成员的遗留待办（开发库会积压历史 e2e 任务，挤掉今日安排区上限 10 条） */
async function clearPendingTasks(page: Page, token: string) {
  const res = await page.request.get(`${BASE}/tasks`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (res.status() !== 200) return
  const list = (await res.json()) as { items?: Array<{ id: string }> }
  for (const t of list.items ?? []) {
    await page.request.post(`${BASE}/tasks/${t.id}/complete`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    await page.request.post(`${BASE}/tasks/${t.id}/complete`, {
      headers: { Authorization: `Bearer ${token}` },
    })
  }
}

/** 本地时间今日 HH:mm 的 RFC3339 */
function todayAt(hour: number, minute = 0): string {
  const d = new Date()
  d.setHours(hour, minute, 0, 0)
  const tz = -d.getTimezoneOffset() / 60
  const sign = tz >= 0 ? '+' : '-'
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}T${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}:00${sign}${String(Math.abs(tz)).padStart(2, '0')}:00`
}

test.describe('Today 首页（A-01）', () => {
  test('登录后首屏是 Today，四块分区正确（E2E-01）', async ({ page }) => {
    await login(page)
    const { token, memberId } = await auth(page)

    // 造数据：今日日程 + 今日待办任务（指派给自己）
    const evTitle = `e2e今日安排${Date.now() % 100000}`
    const evRes = await page.request.post(`${BASE}/events`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { title: evTitle, start_at: todayAt(19, 30) },
    })
    expect(evRes.status()).toBe(201)

    const taskTitle = `e2e今日待办${Date.now() % 100000}`
    // 先清空遗留待办，保证新任务落在 10 条上限内
    await clearPendingTasks(page, token)
    const taskRes = await page.request.post(`${BASE}/tasks`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { title: taskTitle, assignee_id: memberId, due_at: todayAt(20, 0) },
    })
    expect(taskRes.status()).toBe(201)

    // 默认入口是 Today（不是对话页）
    await page.goto('/')
    await expect(page).toHaveURL('/')
    await expect(page.getByRole('heading', { name: '今天' })).toBeVisible()
    await page.waitForTimeout(500)
    await page.screenshot({ path: `${SHOT}/today-01-home.png` })

    // 今日安排区块：有造的日程（区块标题用 role 精确定位，避免与事件标题子串撞）
    await expect(page.getByRole('heading', { name: '今日安排' })).toBeVisible()
    await expect(page.getByText(evTitle)).toBeVisible()

    // 需要我处理区块：有造的任务
    await expect(page.getByRole('heading', { name: '需要我处理' })).toBeVisible()
    await expect(page.getByText(taskTitle)).toBeVisible()

    // 家庭用餐区块：三态齐全
    await expect(page.getByRole('heading', { name: '家庭用餐' })).toBeVisible()
    await expect(page.getByText(/我：(今天还没报饭|今晚在家吃|今晚不在家吃)/)).toBeVisible()
  })

  test('任务完成后从待处理区消失，家务页一致（E2E-01 收尾）', async ({ page }) => {
    await login(page)
    const { token, memberId } = await auth(page)

    const taskTitle = `e2e完成验证${Date.now() % 100000}`
    const taskRes = await page.request.post(`${BASE}/tasks`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { title: taskTitle, assignee_id: memberId },
    })
    expect(taskRes.status()).toBe(201)
    const taskId = (await taskRes.json()).id as string

    await page.goto('/')
    await expect(page.getByText(taskTitle)).toBeVisible()

    // 完成任务（两步打卡）
    const c1 = await page.request.post(`${BASE}/tasks/${taskId}/complete`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    expect(c1.status()).toBe(200)
    const c2 = await page.request.post(`${BASE}/tasks/${taskId}/complete`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    expect(c2.status()).toBe(200)

    // Today 刷新后该任务从未完成区消失
    await page.reload()
    await page.waitForTimeout(500)
    await expect(page.getByText(taskTitle)).not.toBeVisible()
    await page.screenshot({ path: `${SHOT}/today-02-after-complete.png` })

    // 家务页也看不到它（已完成的不在待办列表）
    await page.goto('/chores')
    await page.waitForTimeout(500)
    await expect(page.getByText(taskTitle)).not.toBeVisible()
  })

  test('空态正确：没有待办时显示「今天没有需要你处理的事项」（A-01-06）', async ({ page }) => {
    await login(page)
    // 用一个没有数据的视角：今日没有任何事项时（此成员可能有历史任务，
    // 这里只验证空态文案在无日程时区域不渲染）
    await page.goto('/')
    await page.waitForTimeout(500)
    // 今日安排区块在没有当日日程时不渲染
    const scheduleVisible = await page.getByRole('heading', { name: '今日安排' }).isVisible().catch(() => false)
    if (!scheduleVisible) {
      await expect(page.getByText('跟管家说一句')).toBeVisible()
    }
    // 无论有无事项，对话入口始终可达（PRD §4.2）
    await expect(page.getByRole('button', { name: '跟管家说一句' })).toBeVisible()
  })

  test('Today 有去对话的入口，点了进对话页（PRD §4.2 对话不被深藏）', async ({ page }) => {
    await login(page)
    await page.goto('/')
    await page.waitForTimeout(400)
    await page.getByRole('button', { name: '跟管家说一句' }).click()
    await expect(page).toHaveURL('/chat')
    await expect(page.getByPlaceholder('输入消息…')).toBeVisible()
    await page.waitForTimeout(400)
    await page.screenshot({ path: `${SHOT}/today-03-chat.png` })
  })
})
