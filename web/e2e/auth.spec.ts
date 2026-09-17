import { expect, test } from '@playwright/test'

import { login, MEMBER } from './helpers'

test.describe('登录守卫', () => {
  test('未登录访问主页跳转到登录页', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveURL(/\/login/)
  })

  test('正确凭据登录后进入主页', async ({ page }) => {
    await login(page)
    await expect(page.getByText('家事助手')).toBeVisible()
  })

  test('错误令牌显示错误提示且不跳转', async ({ page }) => {
    await page.goto('/login')
    await page.getByPlaceholder('成员名（如：爸爸）').fill(MEMBER.name)
    await page.getByPlaceholder('登录令牌').fill('wrong-token')
    await page.getByRole('button', { name: '登录' }).click()
    await expect(page).toHaveURL(/\/login/)
    // 现状：错误令牌也提示"登录已过期"——文案不贴切，记为待修问题
    await expect(page.locator('body')).toContainText(/过期|错误|失败|不正确/)
  })
})
