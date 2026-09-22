import { expect, test } from '@playwright/test'

import { login } from './helpers'

const SHOT = 'e2e-shots/notifications'

/** 等待调度器下一分钟 tick 生成通知（最坏等 70 秒） */
async function waitForNotification(page: import('@playwright/test').APIRequestContext, token: string) {
  for (let i = 0; i < 8; i++) {
    const res = await page.get('/api/v1/notifications', {
      headers: { Authorization: `Bearer ${token}` },
    })
    if (res.status() === 200) {
      const body = (await res.json()) as { unread_count: number }
      if (body.unread_count > 0) return true
    }
    await new Promise((r) => setTimeout(r, 10_000))
  }
  return false
}

test.describe('通知中心（T-A10 / ADR-006）', () => {
  test('铃铛角标显示未读数，抽屉可查看与已读', async ({ page, request }) => {
    await login(page)
    await expect(page.getByRole('heading', { name: '家事助手' })).toBeVisible()

    // 造一个 30 分钟后到期的任务（指派给自己），调度器 tick 后生成未读通知
    const authRes = await request.post('/api/v1/auth/token', {
      data: { name: '爸爸', auth_token: 'dev-baba' },
    })
    const { token, member_id } = (await authRes.json()) as {
      token: string
      member_id: string
    }
    const code = Date.now() % 10000
    const title = `e2e通知验收${code}`
    const due = new Date(Date.now() + 30 * 60_000).toISOString()
    await request.post('/api/v1/tasks', {
      headers: { Authorization: `Bearer ${token}` },
      data: { title, assignee_id: member_id, due_at: due },
    })

    // 等调度器生成通知（每分钟 tick）
    const got = await waitForNotification(request, token)
    expect(got, '调度器应生成未读通知').toBe(true)

    // 角标出现
    const bell = page.getByRole('button', { name: '通知' })
    await expect(bell).toBeVisible()
    await expect(bell.locator('span')).toBeVisible({ timeout: 40_000 })
    await page.screenshot({ path: `${SHOT}/n-01-badge.png` })

    // 打开抽屉，看到未读项（断言到本次造的任务标题，避免累积数据下空洞通过）
    await bell.click()
    await expect(page.getByRole('heading', { name: '通知' })).toBeVisible()
    await expect(page.getByText(`任务快到期：${title}`)).toBeVisible()
    await page.screenshot({ path: `${SHOT}/n-02-drawer.png` })

    // 标记单条已读
    const readBtn = page.getByRole('button', { name: '已读' }).first()
    await expect(readBtn).toBeVisible()
    await readBtn.click()
    await expect(readBtn).toBeHidden()
    await page.screenshot({ path: `${SHOT}/n-03-read.png` })
  })
})
