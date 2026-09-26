/**
 * UX 走查脚本（痛批官模式）：以真实用户视角完整走查，主动制造异常。
 * 输出：e2e-shots/ux-*.png 截图 + 控制台/请求错误清单。
 */
import { test, type Page, expect } from '@playwright/test'

const SHOT = 'e2e-shots'
const amounts: string[] = []

const consoleErrors: string[] = []
const pageErrors: string[] = []
const failedRequests: string[] = []

function attachCollectors(page: Page) {
  page.on('console', (m) => {
    if (m.type() === 'error') consoleErrors.push(`[console] ${m.text()}`)
  })
  page.on('pageerror', (e) => pageErrors.push(`[pageerror] ${e.message}`))
  page.on('requestfailed', (r) =>
    failedRequests.push(`[requestfailed] ${r.method()} ${r.url()} :: ${r.failure()?.errorText}`),
  )
}

async function login(page: Page) {
  await page.goto('/login')
  await page.fill('input[placeholder="成员名（如：爸爸）"]', '爸爸')
  await page.fill('input[type="password"]', 'dev-baba')
  await page.click('button[type="submit"]')
  await page.waitForURL('/')
}

test.describe('UX 端到端走查', () => {
  test('完整用户旅程 + 异常制造', async ({ page }) => {
    attachCollectors(page)
    test.setTimeout(180_000)

    // ── 1. 登录页：第一印象 ──
    await page.goto('/login')
    await page.screenshot({ path: `${SHOT}/ux-01-login.png` })

    // 异常：空表单直接点登录 → 按钮无反应，没有任何提示
    await page.click('button[type="submit"]')
    await page.waitForTimeout(300)
    await page.screenshot({ path: `${SHOT}/ux-02-login-empty.png` })

    // 异常：错误令牌
    await page.fill('input[placeholder="成员名（如：爸爸）"]', '爸爸')
    await page.fill('input[type="password"]', 'wrong-token')
    await page.click('button[type="submit"]')
    await page.waitForTimeout(800)
    await page.screenshot({ path: `${SHOT}/ux-03-login-wrong.png` })

    // 正常登录
    await login(page)
    // 对话页在 /chat（V1.0-A 起首屏是 Today）
    await page.goto('/chat')

    // ── 2. 主页第一印象 ──
    await page.waitForTimeout(500)
    await page.screenshot({ path: `${SHOT}/ux-04-home.png` })

    // 异常：空消息点发送
    await page.click('button:has-text("发送")')
    await page.waitForTimeout(200)
    await page.screenshot({ path: `${SHOT}/ux-05-empty-send.png` })

    // ── 3. 核心流程：对话记账 ──
    const amt = String(3 + Math.floor(Math.random() * 90))
    amounts.push(amt)
    await page.fill('input[placeholder="输入消息…"]', `今天买菜花了 ${amt} 元`)
    await page.click('button:has-text("发送")')
    // 等流结束（done 后 status idle，发送按钮重新可用）
    await expect(page.locator('button:has-text("发送")')).toBeEnabled({ timeout: 60_000 })
    await page.waitForTimeout(500)
    await page.screenshot({ path: `${SHOT}/ux-06-expense-result.png` })

    // 撤销按钮是否存在且可点
    const undoBtn = page.locator('button:has-text("撤销")')
    const hasUndo = await undoBtn.count()
    if (hasUndo > 0) {
      await undoBtn.first().click()
      await page.waitForTimeout(800)
      await page.screenshot({ path: `${SHOT}/ux-07-after-undo.png` })
    }

    // ── 4. 异常：重复提交（连点发送） ──
    await page.fill('input[placeholder="输入消息…"]', '今晚不回家吃饭')
    for (let i = 0; i < 5; i++) {
      await page.click('button:has-text("发送")')
    }
    await expect(page.locator('button:has-text("发送")')).toBeEnabled({ timeout: 60_000 })
    await page.waitForTimeout(500)
    await page.screenshot({ path: `${SHOT}/ux-08-double-submit.png` })

    // ── 5. 异常：超长文本 ──
    const longText = '帮我记账 '.repeat(200)
    await page.fill('input[placeholder="输入消息…"]', longText)
    await page.click('button:has-text("发送")')
    await expect(page.locator('button:has-text("发送")')).toBeEnabled({ timeout: 90_000 })
    await page.waitForTimeout(500)
    await page.screenshot({ path: `${SHOT}/ux-09-long-text.png` })

    // ── 6. 会话侧边栏 ──
    await page.click('button[aria-label="会话列表"]')
    await page.waitForTimeout(500)
    await page.screenshot({ path: `${SHOT}/ux-10-sidebar.png` })

    // 新建对话
    await page.click('button:has-text("新建对话")')
    await page.waitForTimeout(500)
    await page.screenshot({ path: `${SHOT}/ux-11-new-conv.png` })

    // ── 7. 刷新页面：状态是否保持 ──
    await page.reload()
    await page.waitForTimeout(800)
    await page.screenshot({ path: `${SHOT}/ux-12-after-refresh.png` })

    // ── 8. 家务页（TabBar） ──
    await page.click('a[href="/chores"]')
    await page.waitForTimeout(400)
    await page.screenshot({ path: `${SHOT}/ux-13-chores.png` })
    // 点"打卡"——按钮有反应吗？
    const punch = page.locator('button:has-text("打卡")')
    if ((await punch.count()) > 0) {
      await punch.first().click()
      await page.waitForTimeout(500)
      await page.screenshot({ path: `${SHOT}/ux-14-chores-punch.png` })
    }

    // ── 9. 记账页 ──
    await page.click('a[href="/money"]')
    await page.waitForTimeout(500)
    await page.screenshot({ path: `${SHOT}/ux-15-money.png` })

    // ── 10. 报饭页 ──
    await page.click('a[href="/meal"]')
    await page.waitForTimeout(400)
    await page.screenshot({ path: `${SHOT}/ux-16-meal.png` })
    const notHome = page.locator('button:has-text("不在家吃")')
    if ((await notHome.count()) > 0) {
      await notHome.click()
      await page.waitForTimeout(400)
      await page.screenshot({ path: `${SHOT}/ux-17-meal-toggle.png` })
    }

    // ── 11. 设置页：TabBar 没有入口，只能手敲 URL ──
    await page.goto('/settings')
    await page.waitForTimeout(400)
    await page.screenshot({ path: `${SHOT}/ux-18-settings.png` })
    // 点"配置 →"——有反应吗？
    const cfg = page.locator('span:has-text("配置 →")')
    if ((await cfg.count()) > 0) {
      await cfg.first().click()
      await page.waitForTimeout(400)
      await page.screenshot({ path: `${SHOT}/ux-19-settings-click.png` })
    }

    // ── 12. 回对话页，检查记账页是否同步 ──
    await page.goto('/chat')
    await page.waitForTimeout(500)
    await page.goto('/money')
    await page.waitForTimeout(500)
    await page.screenshot({ path: `${SHOT}/ux-20-money-final.png` })

    // ── 汇总错误 ──
    const report = [
      `记账金额: ${amounts.join(', ')}`,
      '',
      `=== console errors (${consoleErrors.length}) ===`,
      ...consoleErrors.slice(0, 30),
      '',
      `=== page errors (${pageErrors.length}) ===`,
      ...pageErrors.slice(0, 20),
      '',
      `=== failed requests (${failedRequests.length}) ===`,
      ...failedRequests.slice(0, 20),
    ].join('\n')
    console.log('UX_WALKTHROUGH_REPORT_START')
    console.log(report)
    console.log('UX_WALKTHROUGH_REPORT_END')
  })
})
