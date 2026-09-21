import { expect, test } from '@playwright/test'

import { login } from './helpers'

const SHOT = 'e2e-shots/settings'

test.describe('设置页真实入口（T-A08）', () => {
  test('底部 Tab 栏有一级入口「我的」，点击进入设置页', async ({ page }) => {
    await login(page)
    await expect(page).toHaveURL('/')

    const tab = page.getByRole('link', { name: '我的' })
    await expect(tab).toBeVisible()
    await tab.click()
    await expect(page).toHaveURL('/settings')
    await expect(page.getByRole('heading', { name: '我的' })).toBeVisible()
    await page.screenshot({ path: `${SHOT}/s-01-tab-entry.png` })
  })

  test('设置页无死链：每一项要么可点要么是真实信息', async ({ page }) => {
    await login(page)
    await page.goto('/settings')

    const main = page.getByRole('main')

    // 家人端不出现 admin 专属项
    await expect(main.getByText('供应商')).toBeHidden()
    await expect(main.getByText('权限矩阵')).toBeHidden()
    await expect(main.getByText('操作审计')).toBeHidden()
    await expect(main.getByText('用量统计')).toBeHidden()

    // 「可撤销的操作」是真实跳转
    const undoEntry = main.getByRole('button', { name: /可撤销的操作/ })
    await expect(undoEntry).toBeVisible()
    await undoEntry.click()
    await expect(page).toHaveURL('/undo')
    await page.screenshot({ path: `${SHOT}/s-02-undo-entry.png` })
  })

  test('退出登录二次确认，确认后回登录页', async ({ page }) => {
    await login(page)
    await page.goto('/settings')

    const main = page.getByRole('main')
    const logout = main.getByRole('button', { name: '退出' })
    await expect(logout).toBeVisible()

    // 首次点击进入确认态，不立即登出（防误触）
    await logout.click()
    await expect(main.getByRole('button', { name: '确认退出' })).toBeVisible()
    await page.screenshot({ path: `${SHOT}/s-03-logout-confirm.png` })

    // 取消则保持登录
    await main.getByRole('button', { name: '取消' }).click()
    await expect(page).toHaveURL('/settings')

    // 确认退出 → 回登录页
    await logout.click()
    await main.getByRole('button', { name: '确认退出' }).click()
    await expect(page).toHaveURL(/\/login/)
  })
})
