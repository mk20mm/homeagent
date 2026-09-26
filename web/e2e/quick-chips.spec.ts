import { expect, test } from '@playwright/test'

import { login, waitForStreamDone } from './helpers'

const SHOT = 'e2e-shots/quick'

test.describe('快捷 chips + 可点空态（T-A05）', () => {
  test('输入框上方常驻 chips，点击直接发送', async ({ page }) => {
    await login(page)
    // chips 在对话页（V1.0-A 起首屏是 Today，对话移到 /chat）
    await page.goto('/chat')
    await expect(page.getByRole('heading', { name: '家事助手' })).toBeVisible()
    await expect(page.getByRole('group', { name: '快捷操作' })).toBeVisible()

    // 常驻项：「记一笔」（爸爸有 expense.write）
    const chip = page.getByRole('button', { name: '记一笔' })
    await expect(chip).toBeVisible()

    // draft 类 chip：点击只填入输入框，不发送
    await chip.click()
    await expect(page.getByPlaceholder('输入消息…')).toHaveValue(
      '记一笔：买菜花了 ',
    )
    await page.screenshot({ path: `${SHOT}/q-01-draft-chip.png` })

    // 清空后点一个非 draft chip（本月预算 = query_budget，只读）
    await page.getByPlaceholder('输入消息…').fill('')
    const budget = page.getByRole('button', { name: '本月预算' })
    await expect(budget).toBeVisible()
    await budget.click()

    // 直接发送：用户气泡出现（限定 main 区域，避开侧边栏会话标题）
    const main = page.getByRole('main')
    await expect(
      main.getByText('这个月预算还剩多少？').filter({ visible: true }).first(),
    ).toBeVisible()
    await waitForStreamDone(page, 60_000)
    await page.screenshot({ path: `${SHOT}/q-02-chip-sent.png` })
  })

  test('欢迎页示例可点击，一次点击直接发送', async ({ page }) => {
    await login(page)

    // 侧边栏「＋ 新建对话」进入空态
    await page.getByRole('button', { name: '会话列表' }).click()
    await page.getByRole('button', { name: /新建对话/ }).click()
    await page.waitForTimeout(400)

    // 空态示例按钮存在（至少一条）；filter(visible) 避开侧边栏隐藏的历史标题
    const main = page.getByRole('main')
    const example = main
      .getByRole('button', { name: '今天买菜花了 120' })
      .filter({ visible: true })
      .first()
    await expect(example).toBeVisible()
    await example.click()

    // 直接发送：气泡可见（侧边栏历史标题是 hidden，filter 掉）
    await expect(
      main.getByText('今天买菜花了 120').filter({ visible: true }),
    ).toBeVisible()
    await waitForStreamDone(page, 60_000)
    await page.screenshot({ path: `${SHOT}/q-03-example-sent.png` })
  })

  test('chips 按成员权限过滤：孩子看不到记账与派任务', async ({ page }) => {
    await page.goto('/login')
    await page.getByPlaceholder('成员名（如：爸爸）').fill('孩子')
    await page.getByPlaceholder('登录令牌').fill('dev-haizi')
    await page.getByRole('button', { name: '登录' }).click()
    await page.waitForURL('/')

    // chips 在对话页（V1.0-A 起首屏是 Today）
    await page.goto('/chat')
    await expect(page.getByRole('group', { name: '快捷操作' })).toBeVisible()
    // 孩子没有 expense.write / task.write
    await expect(page.getByRole('button', { name: '记一笔' })).toBeHidden()
    await expect(page.getByRole('button', { name: '提醒谁洗碗' })).toBeHidden()
    // 孩子有 task.read（打卡/我的任务）与 meal.write（报饭）
    await expect(page.getByRole('button', { name: '洗碗打卡' })).toBeVisible()
    await page.screenshot({ path: `${SHOT}/q-04-kid-filtered.png` })
  })
})
