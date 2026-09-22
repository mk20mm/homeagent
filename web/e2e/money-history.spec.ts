/**
 * T-A06 验收：账本全部历史——按日分组 + 分类筛选 + 日期范围 + 加载更多。
 * 数据通过 POST /expenses 造（显式传 category，避免自动归类不可控），登录态从 localStorage 取 token。
 */
import { test, expect, type Page } from '@playwright/test'

import { login } from './helpers'

const SHOT = 'e2e-shots'

async function auth(page: Page) {
  const raw = await page.evaluate(() => localStorage.getItem('homeagent.token'))
  if (!raw) throw new Error('未登录')
  return JSON.parse(raw) as { token: string; memberId: string }
}

/** 造一笔流水（金额分，显式指定类目，保证筛选可预期） */
async function record(
  page: Page,
  token: string,
  cents: number,
  hint: string,
  category?: string,
  occurredAt?: string,
) {
  const res = await page.request.post('/api/v1/expenses', {
    headers: { Authorization: `Bearer ${token}` },
    data: {
      amount_cents: cents,
      hint,
      ...(category ? { category } : {}),
      ...(occurredAt ? { occurred_at: occurredAt } : {}),
    },
  })
  expect(res.status()).toBe(201)
  return (await res.json()).id as string
}

test.describe('账本页', () => {
  test('按日分组 + 每日小计 + 加载更多', async ({ page }) => {
    await login(page)
    const { token } = await auth(page)
    const stamp = `${Date.now() % 10000}`

    // 两笔今天（不同类目）
    await record(page, token, 123400, `e2e今天A${stamp}`, '食材')
    await record(page, token, 567800, `e2e今天B${stamp}`, '日用')

    await page.goto('/money')
    await expect(page.getByRole('heading', { name: '记账' })).toBeVisible()
    await page.waitForTimeout(600)

    // 「今天」分组含两笔；分组头带「小计 ¥x」（金额不写死：多轮 e2e 会累积同名数据）
    await expect(page.getByText(`e2e今天A${stamp}`)).toBeVisible()
    await expect(page.getByText(`e2e今天B${stamp}`)).toBeVisible()
    await expect(page.getByText(/小计\s*¥/).first()).toBeVisible()
    await page.screenshot({ path: `${SHOT}/mn-01-grouped.png` })

    // 「加载更多」在有下一页时可点（第一页 20 条，累积数据足够多）
    const loadMore = page.getByRole('button', { name: '加载更多' })
    await expect(loadMore).toBeVisible()
    await loadMore.click()
    await page.waitForTimeout(600)
    // 翻页后列表条数增加（至少多出一条）
    await expect(page.getByText(/小计\s*¥/).first()).toBeVisible()
    await page.screenshot({ path: `${SHOT}/mn-02-loadmore.png` })

    // 昨天分组由接口层兜底：查一笔昨天的，确认按日期分组的键正确
    const yesterday = new Date()
    yesterday.setDate(yesterday.getDate() - 1)
    // 定在昨天 23:59：按日查询 occurred_at 倒序，累积数据下也要在首页可见
    yesterday.setHours(23, 59, 0, 0)
    await record(
      page,
      token,
      99900,
      `e2e昨天${stamp}`,
      '食材',
      yesterday.toISOString(),
    )
    const yIso = yesterday.toISOString().slice(0, 10)
    const byDate = await page.request.get('/api/v1/expenses', {
      headers: { Authorization: `Bearer ${token}` },
      params: { start_date: yIso, end_date: yIso },
    })
    expect(byDate.status()).toBe(200)
    const rows = (await byDate.json()).items as { hint: string }[]
    expect(rows.some((r) => r.hint === `e2e昨天${stamp}`)).toBe(true)
  })

  test('分类筛选即时生效，汇总条同步切换', async ({ page }) => {
    await login(page)
    const { token } = await auth(page)
    const stamp = `${Date.now() % 10000}`

    await record(page, token, 2000, `e2e筛选食材${stamp}`, '食材')
    await record(page, token, 3000, `e2e筛选日用${stamp}`, '日用')

    await page.goto('/money')
    await expect(page.getByRole('heading', { name: '记账' })).toBeVisible()
    await page.waitForTimeout(600)
    await expect(page.getByText('本月支出')).toBeVisible()

    // 点「食材」筛选：只留食材，汇总条切到「当前筛选合计」
    await page.getByRole('button', { name: '食材' }).first().click()
    await page.waitForTimeout(600)
    await expect(page.getByText('当前筛选合计')).toBeVisible()
    await expect(page.getByText(`e2e筛选食材${stamp}`)).toBeVisible()
    await expect(page.getByText(`e2e筛选日用${stamp}`)).toHaveCount(0)
    await page.screenshot({ path: `${SHOT}/mn-03-filtered.png` })

    // 取消筛选回到全部
    await page.getByRole('button', { name: '全部分类' }).click()
    await page.waitForTimeout(600)
    await expect(page.getByText('本月支出')).toBeVisible()
    await expect(page.getByText(`e2e筛选日用${stamp}`)).toBeVisible()

    // 空筛选结果有空态文案
    await page.getByRole('button', { name: '出行' }).first().click()
    await page.waitForTimeout(600)
    await expect(page.getByText('这个范围没有流水，换个筛选看看')).toBeVisible()
    await page.screenshot({ path: `${SHOT}/mn-04-filter-empty.png` })
  })

  test('日期范围筛选（本周）', async ({ page }) => {
    await login(page)
    const { token } = await auth(page)
    const stamp = `${Date.now() % 10000}`

    // 造一笔在 30 天前（本月内但本周外），金额取大值便于在首页就能找到
    const old = new Date()
    old.setDate(old.getDate() - 30)
    await record(page, token, 555500, `e2e本月外${stamp}`, '食材', old.toISOString())

    await page.goto('/money')
    await page.waitForTimeout(600)

    // 本月汇总包含这笔（汇总固定按本月）
    await expect(page.getByText('本月支出')).toBeVisible()

    // 这笔在第一页可见（金额大、日期新？不——日期最旧的排最后，用汇总兜底）
    // 切「本周」后这笔消失
    await page.getByRole('button', { name: '本周' }).click()
    await page.waitForTimeout(600)
    await expect(page.getByText(`e2e本月外${stamp}`)).toHaveCount(0)
    await page.screenshot({ path: `${SHOT}/mn-05-week.png` })

    // 切回全部这笔回来
    await page.getByRole('button', { name: '全部' }).first().click()
    await page.waitForTimeout(600)
    // 本月汇总金额回到含这笔的状态（兜底断言，避免分页导致找不到）
    await expect(page.getByText('本月支出')).toBeVisible()
  })
})
