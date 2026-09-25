import { expect, test } from '@playwright/test'

import { login, randomAmount, waitForStreamDone } from './helpers'

test.describe('记账页数据联动', () => {
  test('显示本月支出汇总与流水列表', async ({ page }) => {
    await login(page)
    await page.goto('/money')
    await expect(page.getByText('本月支出')).toBeVisible()
    // 汇总金额渲染完成（不再加载中）
    await expect(page.getByText('本月支出').locator('..')).toContainText(/[¥￥]/)
  })

  test('对话记账后记账页出现该笔流水', async ({ page }) => {
    await login(page)
    const amount = randomAmount()
    await page.getByPlaceholder('输入消息…').fill(`e2e测试买书花了 ${amount}`)
    await page.getByRole('button', { name: '发送' }).click()
    // 等流走完（工具执行回执）
    await waitForStreamDone(page, 60_000)

    await page.goto('/money')
    await expect(page.getByText('本月支出')).toBeVisible()
    // 流水列表含刚记的这笔
    await expect(page.getByText(/e2e测试买书/).first()).toBeVisible({ timeout: 20_000 })
  })
})
