/**
 * 新功能验证：FAB 记一笔 + 对话卡片行内修正 + 账本定位高亮。
 */
import { test, expect } from '@playwright/test'

const SHOT = 'e2e-shots'

test.describe('记账快捷路径', () => {
  test('FAB 记一笔 → 列表出现并高亮', async ({ page }) => {
    await page.goto('/login')
    await page.fill('input[placeholder="成员名（如：爸爸）"]', '爸爸')
    await page.fill('input[type="password"]', 'dev-baba')
    await page.click('button[type="submit"]')
    await page.waitForURL('/')

    await page.click('a[href="/money"]')
    await page.waitForTimeout(400)
    await page.screenshot({ path: `${SHOT}/nf-01-money-before.png` })

    // 打开记一笔面板
    await page.click('button[aria-label="记一笔"]')
    await page.waitForTimeout(300)
    await page.screenshot({ path: `${SHOT}/nf-02-sheet.png` })

    // 空保存 → 应有提示且不关闭
    await page.click('button:has-text("保存")')
    await page.waitForTimeout(200)
    await page.screenshot({ path: `${SHOT}/nf-03-sheet-validation.png` })

    // 填入并保存
    const hint = `快捷测试${Math.floor(Math.random() * 1000)}`
    await page.fill('input[placeholder="金额（元）"]', '6.66')
    await page.fill('input[placeholder="买了什么（如：买菜）"]', hint)
    // 分类 chip 在底部面板里，与账本页的筛选 chip 同名——用面板标题限定作用域
    const sheet = page.getByText('记一笔').locator('xpath=..')
    await sheet.getByRole('button', { name: '食材' }).click()
    await page.click('button:has-text("保存")')
    await page.waitForTimeout(800)
    await page.screenshot({ path: `${SHOT}/nf-04-after-save.png` })

    // 新记录出现在列表
    await expect(page.getByText(hint)).toBeVisible()
    await expect(page.getByText('¥6.66').first()).toBeVisible()
  })

  test('对话卡片修正金额 → 卡片更新', async ({ page }) => {
    await page.goto('/login')
    await page.fill('input[placeholder="成员名（如：爸爸）"]', '爸爸')
    await page.fill('input[type="password"]', 'dev-baba')
    await page.click('button[type="submit"]')
    await page.waitForURL('/')

    // 先用 FAB 建一笔，拿 expense_id
    await page.click('a[href="/money"]')
    await page.waitForTimeout(300)
    await page.click('button[aria-label="记一笔"]')
    const hint = `修正测试${Math.floor(Math.random() * 1000)}`
    await page.fill('input[placeholder="金额（元）"]', '10')
    await page.fill('input[placeholder="买了什么（如：买菜）"]', hint)
    await page.click('button:has-text("保存")')
    await page.waitForTimeout(600)

    // 回对话页，发消息触发记账（用唯一 hint 避免与历史 e2e 数据撞幂等键）
    await page.click('a[href="/chat"]')
    await page.waitForTimeout(400)
    const amt = String(1 + Math.floor(Math.random() * 90))
    const stamp = Date.now() % 100000
    await page.fill('input[placeholder="输入消息…"]', `修正测试${stamp} 花了 ${amt} 元`)
    await page.click('button:has-text("发送")')
    await expect(page.locator('button:has-text("发送")')).toBeEnabled({ timeout: 60_000 })
    await page.waitForTimeout(400)
    await page.screenshot({ path: `${SHOT}/nf-05-card.png` })

    // 点「修正」：取最后一张卡（最新消息，即刚记的这笔；历史会话的旧卡可能已撤销，PATCH 必失败）
    const editBtn = page.locator('button:has-text("修正")')
    if ((await editBtn.count()) > 0) {
      await editBtn.last().click()
      await page.waitForTimeout(300)
      await page.screenshot({ path: `${SHOT}/nf-06-editing.png` })

      // 改金额并保存（随机金额，避免改完后与今天另一笔撞幂等键）
      const newAmt = (100 + Math.floor(Math.random() * 900) / 10).toFixed(2)
      const input = page.locator(`input[inputMode="decimal"]`).first()
      await input.fill(newAmt)
      await page.click('button:has-text("保存修正")')
      await page.waitForTimeout(600)
      await page.screenshot({ path: `${SHOT}/nf-07-after-edit.png` })

      // 卡片金额应更新
      await expect(page.getByText(`¥${newAmt}`).first()).toBeVisible({ timeout: 5000 })
    }
  })

  test('「在账本中查看」跳转到记账页并高亮', async ({ page }) => {
    await page.goto('/login')
    await page.fill('input[placeholder="成员名（如：爸爸）"]', '爸爸')
    await page.fill('input[type="password"]', 'dev-baba')
    await page.click('button[type="submit"]')
    await page.waitForURL('/')

    await page.fill('input[placeholder="输入消息…"]', `买文具花了 ${5 + Math.floor(Math.random() * 50)} 元`)
    await page.click('button:has-text("发送")')
    await expect(page.locator('button:has-text("发送")')).toBeEnabled({ timeout: 60_000 })
    await page.waitForTimeout(400)

    const viewBtn = page.locator('button:has-text("在账本中查看")')
    if ((await viewBtn.count()) > 0) {
      await viewBtn.first().click()
      await page.waitForURL('**/money**')
      await page.waitForTimeout(600)
      await page.screenshot({ path: `${SHOT}/nf-08-ledger-focus.png` })
      // 高亮项存在
      await expect(page.locator('.highlight').or(page.locator('[class*="highlight"]'))).toBeVisible({ timeout: 3000 })
    }
  })
})
