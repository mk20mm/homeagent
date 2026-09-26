/**
 * 财务模块深化 E2E 走查测试：
 * 1. 年月时间切换胶囊与弹窗
 * 2. Apple Card 结余总览卡片
 * 3. 记收入 (FAB 切换记收入模式 + 来源选择 + 补记日期)
 * 4. 全部/支出/收入 筛选联动
 */
import { test, expect } from '@playwright/test'

const SHOT = 'e2e-shots'

test.describe('财务模块深化 (Phase D P0)', () => {
  test('年月切换 + 记收入 + 结余 Apple Card + 收支筛选', async ({ page }) => {
    // 1. 登录
    await page.goto('/login')
    await page.fill('input[placeholder="成员名（如：爸爸）"]', '爸爸')
    await page.fill('input[type="password"]', 'dev-baba')
    await page.click('button[type="submit"]')
    await page.waitForURL('/')

    // 2. 导航至财务页 (点击 TabBar 上的「财务」)
    await page.click('a[href="/money"]')
    await page.waitForTimeout(400)
    await page.screenshot({ path: `${SHOT}/fin-01-overview.png` })

    // 3. 验证年月胶囊与时间切换
    const timeBtn = page.locator('button:has-text("年")')
    await expect(timeBtn.first()).toBeVisible()
    const initialTimeText = await timeBtn.first().innerText()

    // 点击 ◀ 上一个周期
    await page.click('button[aria-label="上一个周期"]')
    await page.waitForTimeout(300)
    await page.screenshot({ path: `${SHOT}/fin-02-prev-month.png` })

    // 点击 ▶ 下一个周期
    await page.click('button[aria-label="下一个周期"]')
    await page.waitForTimeout(300)

    // 点击胶囊打开弹窗
    await timeBtn.first().click()
    await page.waitForTimeout(300)
    await expect(page.getByText('选择时间')).toBeVisible()
    await page.screenshot({ path: `${SHOT}/fin-03-time-picker-modal.png` })

    // 点击「本月」重置
    await page.click('button:has-text("本月")')
    await page.waitForTimeout(300)

    // 4. 验证 Apple Card 结余卡片
    await expect(page.getByText('收支结余')).toBeVisible()
    await expect(page.getByText('本月收入')).toBeVisible()
    await expect(page.getByText('本月支出')).toBeVisible()

    // 5. 点击 FAB 打开记一笔，切换至记收入
    await page.click('button[aria-label="记一笔"]')
    await page.waitForTimeout(300)
    await page.screenshot({ path: `${SHOT}/fin-04-sheet-expense.png` })

    // 切换到「记收入」
    await page.click('button:has-text("记收入")')
    await page.waitForTimeout(200)
    await expect(page.getByText('收入来源')).toBeVisible()
    await page.screenshot({ path: `${SHOT}/fin-05-sheet-income.png` })

    // 填写收入金额、描述、选择来源「奖金」、选择日期「今天」
    const incomeHint = `季度奖金${Math.floor(Math.random() * 1000)}`
    await page.fill('input[placeholder="金额（元）"]', '8888')
    await page.fill('input[placeholder="收入来源说明（如：9月工资、兼职外包）"]', incomeHint)
    await page.click('button:has-text("奖金")')
    await page.click('button:has-text("今天")')

    // 保存收入
    await page.click('button:has-text("保存收入")')
    await page.waitForTimeout(800)
    await page.screenshot({ path: `${SHOT}/fin-06-after-income-save.png` })

    // 6. 验证列表中出现刚记录的收入（带 +¥8888.00 与「收入」徽章）
    await expect(page.getByText(incomeHint)).toBeVisible({ timeout: 5000 })
    await expect(page.getByText('+¥8888.00').first()).toBeVisible()
    await expect(page.getByText('奖金').first()).toBeVisible()

    // 7. 测试筛选 Tab 控制器
    // 切换到「支出」筛选：该笔收入应该被过滤隐藏
    await page.click('button:has-text("支出")')
    await page.waitForTimeout(300)
    await page.screenshot({ path: `${SHOT}/fin-07-filter-expense.png` })
    await expect(page.getByText(incomeHint)).not.toBeVisible()

    // 切换到「收入」筛选：该笔收入应该显示
    await page.click('button:has-text("收入")')
    await page.waitForTimeout(300)
    await page.screenshot({ path: `${SHOT}/fin-08-filter-income.png` })
    await expect(page.getByText(incomeHint)).toBeVisible()

    // 切换回「全部」
    await page.click('button:has-text("全部")')
    await page.waitForTimeout(300)
    await expect(page.getByText(incomeHint)).toBeVisible()
  })
})
