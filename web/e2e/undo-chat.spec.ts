import { expect, test } from '@playwright/test'

import { login, randomAmount, waitForStreamDone } from './helpers'

const SHOT = 'e2e-shots/undo-chat'

test.describe('对话侧撤销（T-A09，修 T-e2e-2）', () => {
  test('说「取消刚才那笔」撤销最近一次记账', async ({ page }) => {
    await login(page)
    const amount = randomAmount()

    // 1. 先记一笔
    await page.getByPlaceholder('输入消息…').fill(`e2e撤销测试买水果花了 ${amount}`)
    await page.getByRole('button', { name: '发送' }).click()
    await waitForStreamDone(page, 60_000)
    await expect(page.getByText(/撤销/).first()).toBeVisible()
    await page.screenshot({ path: `${SHOT}/u-01-recorded.png` })

    // 2. 自然语言撤销
    await page.getByPlaceholder('输入消息…').fill('取消刚才那笔')
    await page.getByRole('button', { name: '发送' }).click()
    await waitForStreamDone(page, 60_000)

    // AI 应回复已撤销（不是「没有这个能力」）
    const main = page.getByRole('main')
    await expect(main.getByText(/已撤销|已取消|撤销成功/).first()).toBeVisible({
      timeout: 30_000,
    })
    await page.screenshot({ path: `${SHOT}/u-02-undone.png` })
  })

  test('无可撤销项时明确告知，不静默成功', async ({ page }) => {
    // 用奶奶账号：undo 列表为空（无历史操作），且具备 undo_last 权限
    await page.goto('/login')
    await page.getByPlaceholder('成员名（如：爸爸）').fill('奶奶')
    await page.getByPlaceholder('登录令牌').fill('dev-nainai')
    await page.getByRole('button', { name: '登录' }).click()
    await page.waitForURL('/')

    await page.getByPlaceholder('输入消息…').fill('取消刚才那笔')
    await page.getByRole('button', { name: '发送' }).click()
    await waitForStreamDone(page, 60_000)

    // 应回复「没有可撤销的操作」类文案，而非假装成功
    const main = page.getByRole('main')
    await expect(
      main.getByText(/没有可撤销|无可撤销|没有可取消/).first(),
    ).toBeVisible({ timeout: 30_000 })
    await page.screenshot({ path: `${SHOT}/u-03-empty.png` })
  })
})
