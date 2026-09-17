import { expect, type Page } from '@playwright/test'

/** 测试成员（种子数据：爸爸 / parent 角色） */
export const MEMBER = { name: '爸爸', token: 'dev-baba' }

/** 填表单登录并等待进入主页 */
export async function login(page: Page) {
  await page.goto('/login')
  await page.getByPlaceholder('成员名（如：爸爸）').fill(MEMBER.name)
  await page.getByPlaceholder('登录令牌').fill(MEMBER.token)
  await page.getByRole('button', { name: '登录' }).click()
  await page.waitForURL('/')
}

/** 随机金额（元，2 位小数）：避免命中记账幂等键 */
export function randomAmount(): string {
  return (Math.random() * 99 + 1).toFixed(2)
}

/** 等待一轮对话流结束（发送按钮重新可用、"正在思考"消失） */
export async function waitForStreamDone(page: Page, timeout = 60_000) {
  const sendBtn = page.getByRole('button', { name: '发送' })
  // 先等到进入 streaming（按钮 disabled），再等它恢复
  await expect(sendBtn).toBeDisabled({ timeout: 10_000 }).catch(() => {})
  await expect(sendBtn).toBeEnabled({ timeout })
}
