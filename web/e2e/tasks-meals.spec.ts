/**
 * T-A02/T-A03 验收：报饭页（申报切换 + 缺口催办）与家务页（派活 → 打卡 → 撤销闭环）。
 * 任务通过 POST /tasks 直接造（对话脚本供应商不定能识别派活意图），登录态从 localStorage 取 token。
 */
import { test, expect, type Page } from '@playwright/test'

import { login } from './helpers'

const SHOT = 'e2e-shots'

/** 从 localStorage 取登录态（homeagent.token 存 {token, memberId, role, expiresAt}） */
async function auth(page: Page) {
  const raw = await page.evaluate(() => localStorage.getItem('homeagent.token'))
  if (!raw) throw new Error('未登录')
  return JSON.parse(raw) as { token: string; memberId: string }
}

test.describe('家务任务页', () => {
  test('空态引导跳到对话页', async ({ page }) => {
    await login(page)
    await page.goto('/chores')
    await expect(page.getByRole('heading', { name: '家务任务' })).toBeVisible()
    await page.waitForTimeout(400)

    if ((await page.getByText('今天没有待办').count()) === 0) {
      test.skip(true, '当前成员已有任务，空态分支跳过')
    }
    await page.screenshot({ path: `${SHOT}/tm-01-chores-empty.png` })
    await page.getByRole('button', { name: '去跟管家说一句' }).click()
    await expect(page).toHaveURL(/\/chat/)
  })

  test('派活 → 打卡 → 撤销回退', async ({ page }) => {
    await login(page)
    const { token, memberId } = await auth(page)

    // 造一个指派给自己的任务（与工具同一幂等键：memberID+title+assignee）
    const title = `e2e 打卡${Date.now() % 10000}`
    const res = await page.request.post('/api/v1/tasks', {
      headers: { Authorization: `Bearer ${token}` },
      data: { title, assignee_id: memberId },
    })
    expect(res.status()).toBe(201)
    const undoId = (await res.json()).undo_id as string

    await page.goto('/chores')
    await expect(page.getByRole('heading', { name: '家务任务' })).toBeVisible()
    await expect(page.getByText(title)).toBeVisible()
    await page.waitForTimeout(300)
    await page.screenshot({ path: `${SHOT}/tm-02-chores.png` })

    // 打卡①：pending → in_progress，出现「撤销」（回退误打卡）
    const item = page.getByText(title).locator('xpath=../..')
    await item.getByRole('button', { name: '打卡' }).click()
    await expect(item.getByText('进行中', { exact: false })).toBeVisible({
      timeout: 5000,
    })
    await expect(item.getByRole('button', { name: '撤销' })).toBeVisible({
      timeout: 5000,
    })
    await page.screenshot({ path: `${SHOT}/tm-03-chores-progress.png` })

    // 撤销 → 回到待打卡（reload 后是干净状态，撤销按钮消失）
    await item.getByRole('button', { name: '撤销' }).click()
    await expect(item.getByRole('button', { name: '打卡' })).toBeVisible({
      timeout: 5000,
    })
    await expect(item.getByRole('button', { name: '撤销' })).toBeHidden({
      timeout: 5000,
    })
    await page.screenshot({ path: `${SHOT}/tm-04-chores-undone.png` })

    // 打卡②③：pending → in_progress → done（✅ 确认，打卡按钮消失）
    await item.getByRole('button', { name: '打卡' }).click()
    await expect(item.getByText('进行中', { exact: false })).toBeVisible({
      timeout: 5000,
    })
    await item.getByRole('button', { name: '打卡' }).click()
    await expect(item.getByText('已完成', { exact: false })).toBeVisible({
      timeout: 5000,
    })
    await expect(item.getByRole('button', { name: '打卡' })).toBeHidden({
      timeout: 5000,
    })
    await page.screenshot({ path: `${SHOT}/tm-05-chores-done.png` })

    // 清理：撤销派活本身，把任务删掉，不污染其他用例
    if (undoId) {
      await page.request.post(`/api/v1/undo/${undoId}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
    }
  })
})

test.describe('报饭页', () => {
  // 催办要用剪贴板
  test.use({ contextOptions: { permissions: ['clipboard-read', 'clipboard-write'] } })

  test('切换申报 → 已申报列表同步；缺口区催办复制文案', async ({ page }) => {
    await login(page)
    await page.goto('/meal')
    await expect(page.getByRole('heading', { name: '报饭' })).toBeVisible()
    await page.waitForTimeout(400)
    await page.screenshot({ path: `${SHOT}/tm-05-meal.png` })

    // 申报在家吃
    const atHomeBtn = page.getByRole('button', { name: '🏠 在家吃' })
    const awayBtn = page.getByRole('button', { name: '🚪 不在家吃' })
    await atHomeBtn.click()
    await page.waitForTimeout(500)
    await expect(atHomeBtn).toHaveClass(/On/)
    await expect(page.getByText('已申报')).toBeVisible()
    await page.screenshot({ path: `${SHOT}/tm-06-meal-athome.png` })

    // 切换到不在家吃
    await awayBtn.click()
    await page.waitForTimeout(500)
    await expect(awayBtn).toHaveClass(/OffActive/)
    await page.screenshot({ path: `${SHOT}/tm-07-meal-away.png` })

    // 缺口区（有人没报才出现）
    const unreported = page.getByText(/还没报（缺 \d+ 人）/)
    if ((await unreported.count()) > 0) {
      await page.getByRole('button', { name: '催办' }).first().click()
      await expect(page.getByText('已复制提醒')).toBeVisible({ timeout: 5000 })
      await expect(page.getByText(/提醒文案已复制/)).toBeVisible({ timeout: 5000 })
      await page.screenshot({ path: `${SHOT}/tm-08-meal-nudged.png` })
    }
  })
})
