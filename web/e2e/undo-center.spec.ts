/**
 * T-A04 验收：撤销中心——24h 内可撤销项列表 + 一次点击撤销（乐观移除）。
 * 先用 API 造一笔记账（产生 undo 记录），再在 /undo 页面撤销它。
 */
import { test, expect, type Page } from '@playwright/test'

import { login } from './helpers'

const SHOT = 'e2e-shots'

async function auth(page: Page) {
  const raw = await page.evaluate(() => localStorage.getItem('homeagent.token'))
  if (!raw) throw new Error('未登录')
  return JSON.parse(raw) as { token: string; memberId: string }
}

test.describe('撤销中心', () => {
  test('列表展示摘要 → 撤销后乐观移除', async ({ page }) => {
    await login(page)
    const { token } = await auth(page)

    // 造一笔唯一金额的记账（产生 undo 记录）：用时间戳避免和历史残留撞金额
    const cents = 1000 + (Date.now() % 99000) % 9000 // ¥10.xx ~ ¥100.xx，ms 级唯一
    const yuan = (cents / 100).toFixed(2)

    // 先确认历史上没有同金额残留（否则后续断言会误判）
    await page.goto('/undo')
    await page.waitForTimeout(400)
    await expect(page.getByText(`¥${yuan}`)).toHaveCount(0)

    const res = await page.request.post('/api/v1/expenses', {
      headers: { Authorization: `Bearer ${token}` },
      data: { amount_cents: cents, hint: '撤销中心验收', category: '食材' },
    })
    expect(res.status()).toBe(201)

    // 从设置页进撤销中心（顺带验收 T20 的入口；整行是 button，点任意位置即可）
    await page.goto('/settings')
    await page.waitForTimeout(300)
    await page.getByRole('button', { name: /可撤销的操作/ }).click()
    await expect(page).toHaveURL(/\/undo/)
    await page.waitForTimeout(400)
    await page.screenshot({ path: `${SHOT}/ud-01-list.png` })

    // 定位刚记的那一条
    const row = page.getByText(`¥${yuan}`).locator('xpath=../..')
    await expect(row).toBeVisible()
    await expect(row.getByText(/剩 \d+/)).toBeVisible()

    // 撤销 → 乐观移除
    await row.getByRole('button', { name: '撤销' }).click()
    await expect(row).toBeHidden({ timeout: 5000 })
    await page.screenshot({ path: `${SHOT}/ud-02-after-undo.png` })

    // 列表里不再有这条（服务端确认后 reload 也保持没了）
    await page.reload()
    await page.waitForTimeout(400)
    await expect(page.getByText(`¥${yuan}`)).toHaveCount(0)
  })

  test('空态文案', async ({ page }) => {
    // 用一个新家庭隔离不了（单家庭），这里只验证页面能打开且有空态兜底
    await login(page)
    await page.goto('/undo')
    await expect(page.getByRole('heading', { name: '撤销' })).toBeVisible()
    await page.waitForTimeout(400)
    // 有数据时显示列表，无数据时显示空态——两者都是合法渲染
    const listVisible = (await page.locator('button:has-text("撤销")').count()) > 0
    const emptyVisible = (await page.getByText('最近没有可撤销的操作').count()) > 0
    expect(listVisible || emptyVisible).toBeTruthy()
  })
})
