import { expect, test } from '@playwright/test'

import { login, waitForStreamDone } from './helpers'

test.describe('会话管理（侧边栏）', () => {
  test('打开侧边栏看到会话列表与新建按钮', async ({ page }) => {
    await login(page)
    await page.getByRole('button', { name: '会话列表' }).click()
    await expect(page.getByRole('button', { name: '新建对话' })).toBeVisible()
    // 至少存在历史会话（此前验收产生）
    const items = page.locator('[role="button"][tabindex="0"]')
    await expect(items).not.toHaveCount(0)
  })

  test('新建对话后消息区清空', async ({ page }) => {
    await login(page)
    // 先发一条消息让当前会话有内容
    await page.getByPlaceholder('输入消息…').fill('e2e测试你好')
    await page.getByRole('button', { name: '发送' }).click()
    await waitForStreamDone(page, 60_000)

    await page.getByRole('button', { name: '会话列表' }).click()
    await page.getByRole('button', { name: '新建对话' }).click()
    // 回到欢迎页（无消息）
    await expect(page.getByText('说点什么，我来跑腿：')).toBeVisible()
  })

  test('新建对话发消息后列表出现该会话', async ({ page }) => {
    const tag = `e2e${Date.now()}`
    await login(page)
    await page.getByRole('button', { name: '会话列表' }).click()
    await page.getByRole('button', { name: '新建对话' }).click()
    await page.getByPlaceholder('输入消息…').fill(tag)
    await page.getByRole('button', { name: '发送' }).click()
    await waitForStreamDone(page, 60_000)

    await page.getByRole('button', { name: '会话列表' }).click()
    // 在侧边栏内精确定位（排除聊天气泡同名文本）
    await expect(page.locator('aside').getByText(tag).first()).toBeVisible()
  })

  test('删除会话后从列表移除', async ({ page }) => {
    const tag = `e2e${Date.now()}`
    await login(page)
    // 先建一个带唯一标记的会话
    await page.getByRole('button', { name: '会话列表' }).click()
    await page.getByRole('button', { name: '新建对话' }).click()
    await page.getByPlaceholder('输入消息…').fill(tag)
    await page.getByRole('button', { name: '发送' }).click()
    await waitForStreamDone(page, 60_000)

    // 打开侧边栏删除该会话
    await page.getByRole('button', { name: '会话列表' }).click()
    const items = page.locator('aside [role="button"][tabindex="0"]').filter({
      hasText: tag,
    })
    await expect(items).toHaveCount(1)

    page.on('dialog', (d) => d.accept())
    await items.locator('button[aria-label="删除会话"]').click()
    await expect(items).toHaveCount(0)
  })
})
