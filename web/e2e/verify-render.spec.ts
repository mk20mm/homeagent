import { test } from '@playwright/test'

const SHOT = 'e2e-shots'

test('验证新功能渲染', async ({ page }) => {
  await page.goto('/login')
  await page.fill('input[placeholder="成员名（如：爸爸）"]', '爸爸')
  await page.fill('input[type="password"]', 'dev-baba')
  await page.click('button[type="submit"]')
  await page.waitForURL('/')

  // 记账页：FAB 应可见
  await page.click('a[href="/money"]')
  await page.waitForTimeout(500)
  await page.screenshot({ path: `${SHOT}/verify-01-money-fab.png` })

  // 打开记一笔弹层
  await page.click('button[aria-label="记一笔"]')
  await page.waitForTimeout(400)
  await page.screenshot({ path: `${SHOT}/verify-02-sheet.png` })

  // 关闭弹层，回对话页
  await page.keyboard.press('Escape')
  await page.waitForTimeout(300)
  await page.click('a[href="/"]')
  await page.waitForTimeout(500)
  await page.screenshot({ path: `${SHOT}/verify-03-chat.png` })
})
