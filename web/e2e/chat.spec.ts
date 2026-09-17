import { expect, test } from '@playwright/test'

import { login, randomAmount, waitForStreamDone } from './helpers'

test.describe('对话记账 + 撤销', () => {
  test('记账后出现工具卡片与撤销按钮', async ({ page }) => {
    await login(page)
    const amount = randomAmount()
    await page.getByPlaceholder('输入消息…').fill(`e2e测试买水果花了 ${amount}`)
    await page.getByRole('button', { name: '发送' }).click()

    // 真实 LLM 流式回复：等待撤销按钮（工具执行回执）出现
    await expect(page.getByRole('button', { name: '撤销' }).first()).toBeVisible({
      timeout: 60_000,
    })
  })

  test('撤销后卡片标记已撤销', async ({ page }) => {
    await login(page)
    const amount = randomAmount()
    await page.getByPlaceholder('输入消息…').fill(`e2e测试买水果花了 ${amount}`)
    await page.getByRole('button', { name: '发送' }).click()

    const undoBtn = page.getByRole('button', { name: '撤销' }).first()
    await expect(undoBtn).toBeVisible({ timeout: 60_000 })
    await undoBtn.click()

    await expect(page.getByText('已撤销').first()).toBeVisible()
    // 撤销按钮消失（不可重复撤销）
    await expect(undoBtn).toBeHidden()
  })

  test('对话流正常结束（done 事件）', async ({ page }) => {
    await login(page)
    const amount = randomAmount()
    await page.getByPlaceholder('输入消息…').fill(`e2e测试买零食花了 ${amount}`)
    await page.getByRole('button', { name: '发送' }).click()

    // 流结束后发送按钮恢复可用、输入框清空
    await waitForStreamDone(page, 60_000)
    await expect(page.getByPlaceholder('输入消息…')).toHaveValue('')
  })
})
