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

  test('错误令牌提示「用户名或令牌错误」，不再误报「登录已过期」（T-e2e-1）', async ({ page }) => {
    await page.goto('/login')
    await page.getByPlaceholder('成员名（如：爸爸）').fill(MEMBER.name)
    await page.getByPlaceholder('登录令牌').fill('wrong-token')
    await page.getByRole('button', { name: '登录' }).click()
    await expect(page).toHaveURL(/\/login/)
    // T-A07：首次输错令牌 = 凭据错误，不是「登录已过期」
    await expect(page.getByText('用户名或令牌错误，请联系家庭管理员')).toBeVisible()
    await expect(page.locator('body')).not.toContainText('登录已过期')
    await page.screenshot({ path: 'e2e-shots/auth/bad-credentials.png' })
  })

  test('已登录态令牌失效提示「登录已过期」并跳回登录页', async ({ page }) => {
    await login(page)
    await expect(page).toHaveURL('/')

    // 篡改本地令牌为无效值（key 与形状对齐 packages/shared/auth.ts）
    await page.evaluate(() => {
      const raw = localStorage.getItem('homeagent.token')
      if (raw) {
        const auth = JSON.parse(raw)
        auth.token = 'invalid.expired.token'
        // expiresAt 设未来，确保守卫不是因「过期」而是因「后端 401」踢人
        auth.expiresAt = Math.floor(Date.now() / 1000) + 3600
        localStorage.setItem('homeagent.token', JSON.stringify(auth))
      }
    })
    await page.reload()

    // 守卫拦截：回登录页，且文案是「登录已过期」而非「用户名或令牌错误」
    await expect(page).toHaveURL(/\/login/)
    await expect(page.getByText('登录已过期，请重新登录')).toBeVisible()
    await expect(page.locator('body')).not.toContainText('用户名或令牌错误')
    await page.screenshot({ path: 'e2e-shots/auth/expired-session.png' })
  })

  test('登录页给出令牌获取路径说明', async ({ page }) => {
    await page.goto('/login')
    await expect(page.getByText(/系统管理/)).toBeVisible()
  })
})
