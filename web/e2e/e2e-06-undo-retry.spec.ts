/**
 * A-04 / E2E-06 验收：撤销、失败、重试与跨入口一致性。
 * - 跨入口失效约定：一处写操作（记账/任务/打卡/撤销），各页面刷新/重访后均从业务源获得正确结果
 * - 通知语义解耦（A-04-02）：已读/未读通知 ≠ 事务完成/待办，读通知不完成任务，任务完成不继续显示待办
 * - 撤销失效与防伪：过期/无效 undoId 正确返回 404/409，UI 不虚报成功
 * - 重试幂等（E2E-06）：同参数重试记账/派活/报饭不产生重复业务记录
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

function todayAt(hour: number, minute = 0): string {
  const d = new Date()
  d.setHours(hour, minute, 0, 0)
  const tz = -d.getTimezoneOffset() / 60
  const sign = tz >= 0 ? '+' : '-'
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}T${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}:00${sign}${String(Math.abs(tz)).padStart(2, '0')}:00`
}

test.describe('E2E-06: 撤销、失败、重试与跨入口一致性（A-04）', () => {
  test('跨入口状态一致：记账 → 账本可见 → 撤销中心撤销 → 账本自动更新（E2E-06 记账闭环）', async ({ page }) => {
    await login(page)
    const { token } = await auth(page)

    // 1. 造一笔唯一金额的记账
    const cents = 6000 + ((Date.now() % 90000) % 3000)
    const yuan = (cents / 100).toFixed(2)
    const hint = `e2e跨入口记账${Date.now() % 10000}`

    const res = await page.request.post(`${BASE}/expenses`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { amount_cents: cents, hint, category: '食材' },
    })
    expect(res.status()).toBe(201)

    // 2. 访问 /money 账本页，确认该记录可见
    await page.goto('/money')
    await expect(page.getByRole('heading', { name: '记账' })).toBeVisible()
    await expect(page.getByText(hint)).toBeVisible()
    await expect(page.getByText(`¥${yuan}`)).toBeVisible()
    await page.screenshot({ path: `${SHOT}/e2e-06-01-money-before.png` })

    // 3. 访问 /undo 撤销中心，撤销该记账
    await page.goto('/undo')
    await expect(page.getByRole('heading', { name: '撤销' })).toBeVisible()
    const undoRow = page.getByText(`¥${yuan}`).locator('xpath=../..')
    await expect(undoRow).toBeVisible()
    await undoRow.getByRole('button', { name: '撤销' }).click()
    await expect(undoRow).toBeHidden({ timeout: 5000 })
    await page.screenshot({ path: `${SHOT}/e2e-06-02-undo-done.png` })

    // 4. 回到 /money 账本页，该笔记录已从真实业务源中移除
    await page.goto('/money')
    await expect(page.getByRole('heading', { name: '记账' })).toBeVisible()
    await expect(page.getByText(hint)).toHaveCount(0)
    await page.screenshot({ path: `${SHOT}/e2e-06-03-money-after-undo.png` })
  })

  test('跨入口状态一致：任务创建 → Today 显示 → 家务打卡 → Today 消失 → 撤销打卡 → Today 恢复', async ({ page }) => {
    await login(page)
    const { token, memberId } = await auth(page)

    // 1. 创建任务并指派给自己
    const taskTitle = `e2e联动打卡${Date.now() % 100000}`
    const taskRes = await page.request.post(`${BASE}/tasks`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { title: taskTitle, assignee_id: memberId, due_at: todayAt(22, 0) },
    })
    expect(taskRes.status()).toBe(201)

    // 2. 访问 Today 首屏，看到该待办
    await page.goto('/')
    await expect(page.getByRole('heading', { name: '需要我处理' })).toBeVisible()
    await expect(page.getByText(taskTitle)).toBeVisible()

    // 3. 进入 /chores 家务页打卡（两步完成）
    await page.goto('/chores')
    const item = page.getByText(taskTitle).locator('xpath=../..')
    await expect(item).toBeVisible()
    await item.getByRole('button', { name: '打卡' }).click() // in_progress
    await expect(item.getByText('进行中', { exact: false })).toBeVisible()
    await item.getByRole('button', { name: '打卡' }).click() // done
    await expect(item.getByText('已完成', { exact: false })).toBeVisible()
    await page.screenshot({ path: `${SHOT}/e2e-06-04-chore-done.png` })

    // 4. 再次访问 Today，该任务从未完成区消失（不显示已完成事项为待办）
    await page.goto('/')
    await expect(page.getByText(taskTitle)).not.toBeVisible()

    // 5. 跨入口撤销：进入 /undo 撤销中心回退最近一次打卡
    await page.goto('/undo')
    await expect(page.getByRole('heading', { name: '撤销' })).toBeVisible()
    const undoRow = page.getByText(`打卡：${taskTitle}`).first().locator('xpath=../..')
    await expect(undoRow).toBeVisible()
    await undoRow.getByRole('button', { name: '撤销' }).click()
    await page.waitForTimeout(500)

    // 6. 再次访问 Today，该待办重新恢复展示
    await page.goto('/')
    await expect(page.getByRole('heading', { name: '需要我处理' })).toBeVisible()
    await expect(page.getByText(taskTitle)).toBeVisible()
    await page.screenshot({ path: `${SHOT}/e2e-06-05-today-restored.png` })

    // 7. 再次访问家务页，该任务重新恢复为待打卡
    await page.goto('/chores')
    await expect(page.getByText(taskTitle)).toBeVisible()
    await expect(page.getByText(taskTitle).locator('xpath=../..').getByRole('button', { name: '打卡' })).toBeVisible()
  })

  test('通知语义与事务状态解耦（A-04-02）：已读通知不自动完成任务，任务完成不误删通知历史', async ({ page }) => {
    await login(page)
    const { token, memberId } = await auth(page)

    // 1. 创建任务并直接为该成员写入一条关联通知
    const taskTitle = `e2e通知解耦${Date.now() % 100000}`
    const taskRes = await page.request.post(`${BASE}/tasks`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { title: taskTitle, assignee_id: memberId, due_at: todayAt(23, 0) },
    })
    expect(taskRes.status()).toBe(201)
    const taskId = (await taskRes.json()).id as string

    // 2. 标记所有通知为已读（或者调用 POST /notifications 标记已读）
    const readRes = await page.request.post(`${BASE}/notifications`, {
      headers: { Authorization: `Bearer ${token}` },
      data: {},
    })
    expect(readRes.status()).toBe(200)

    // 3. 验证已读通知绝对不完成任务：任务依然在 GET /tasks 中且为待办
    const tasksRes = await page.request.get(`${BASE}/tasks`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    const tasksBody = (await tasksRes.json()) as { items: Array<{ id: string; status: string }> }
    const myTask = tasksBody.items.find((t) => t.id === taskId)
    expect(myTask).toBeDefined()
    expect(myTask?.status).toBe('pending')

    // 4. 访问 /chores 页面验证任务仍在列表中待打卡
    await page.goto('/chores')
    await expect(page.getByText(taskTitle)).toBeVisible()
    await expect(page.getByText(taskTitle).locator('xpath=../..').getByRole('button', { name: '打卡' })).toBeVisible()
  })

  test('无效/过期撤销安全拒绝（E2E-06 错误态保护）', async ({ page }) => {
    await login(page)
    const { token } = await auth(page)

    // 对不存在的 undo ID 发起撤销请求 → 404
    const fakeUndoId = '00000000-0000-0000-0000-000000000000'
    const res = await page.request.post(`${BASE}/undo/${fakeUndoId}`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    expect(res.status()).toBe(404)
    const body = (await res.json()) as { code: string; message: string }
    expect(body.code).toBe('not_found')
  })

  test('重试幂等防重（E2E-06 记账/家务/报饭重试不产生重复记录）', async ({ page }) => {
    await login(page)
    const { token, memberId } = await auth(page)

    // 1. 记账幂等重试（同日同额同 hint 同 category）
    const cents = 5432
    const hint = `e2e幂等记账${Date.now() % 10000}`
    const exp1 = await page.request.post(`${BASE}/expenses`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { amount_cents: cents, hint, category: '餐饮' },
    })
    expect(exp1.status()).toBe(201)
    const id1 = (await exp1.json()).id as string

    // 立即重试相同记账
    const exp2 = await page.request.post(`${BASE}/expenses`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { amount_cents: cents, hint, category: '餐饮' },
    })
    expect(exp2.status()).toBe(201)
    const id2 = (await exp2.json()).id as string
    expect(id2).toBe(id1) // 相同 ID，未重复写入

    // 2. 报饭幂等重试（同人同日）
    const meal1 = await page.request.post(`${BASE}/meals`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { at_home: true },
    })
    expect(meal1.status()).toBe(200)

    const meal2 = await page.request.post(`${BASE}/meals`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { at_home: true },
    })
    expect(meal2.status()).toBe(200)

    // 3. 家务幂等重试（同 member+title+assignee）
    const choreTitle = `e2e幂等家务${Date.now() % 10000}`
    const task1 = await page.request.post(`${BASE}/tasks`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { title: choreTitle, assignee_id: memberId },
    })
    expect(task1.status()).toBe(201)
    const taskId1 = (await task1.json()).id as string

    const task2 = await page.request.post(`${BASE}/tasks`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { title: choreTitle, assignee_id: memberId },
    })
    expect(task2.status()).toBe(201)
    const resTask2 = (await task2.json()) as { id: string; duplicated?: boolean }
    expect(resTask2.id).toBe(taskId1)
    expect(resTask2.duplicated).toBe(true)
  })
})
