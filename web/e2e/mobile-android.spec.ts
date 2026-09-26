import { expect, test } from '@playwright/test'
import { login } from './helpers'

test.describe('移动端与安卓 App 原生体验走查', () => {
  test.use({
    viewport: { width: 412, height: 915 }, // 标准 Android (Pixel 7) 视口
    userAgent:
      'Mozilla/5.0 (Linux; Android 14; Pixel 7 Build/UQ1A.240205.002) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Mobile Safari/537.36',
    hasTouch: true,
    isMobile: true,
  })

  test('PWA 与原生 App Shell 配置有效性', async ({ page }) => {
    await page.goto('/')
    // 1. 验证 PWA Manifest 链接
    const manifestLink = page.locator('link[rel="manifest"]')
    await expect(manifestLink).toHaveAttribute('href', '/manifest.webmanifest')

    // 2. 验证 Android 与移动端 Meta 标签
    const themeColor = page.locator('meta[name="theme-color"]')
    await expect(themeColor).toHaveAttribute('content', '#F2F3F7')

    const mobileCapable = page.locator('meta[name="mobile-web-app-capable"]')
    await expect(mobileCapable).toHaveAttribute('content', 'yes')

    // 3. 验证 manifest 内容
    const res = await page.request.get('/manifest.webmanifest')
    expect(res.status()).toBe(200)
    const json = await res.json()
    expect(json.name).toBe('家事 Agent')
    expect(json.short_name).toBe('家事')
    expect(json.display).toBe('standalone')
    expect(json.icons?.length).toBeGreaterThan(0)
  })

  test('移动端登录与 100dvh 视口容器布局', async ({ page }) => {
    await login(page)

    // 验证主聊天页头部与胶囊输入区在移动端视口正常显示
    await expect(page.locator('header')).toBeVisible()
    const input = page.getByPlaceholder('输入消息…')
    await expect(input).toBeVisible()
    await expect(input).toHaveAttribute('enterkeyhint', 'send')

    // 验证 Grok 模型胶囊在移动端正常加载
    const modelPill = page.locator('[aria-label="切换模型"]')
    await expect(modelPill).toBeVisible()
  })

  test('底部 TabBar 5 入口全链路触控导航', async ({ page }) => {
    await login(page)

    // 1. 家务
    await page.click('nav a:has-text("家务")')
    await page.waitForURL('/chores')
    await expect(page.locator('h1')).toHaveText('家务任务')

    // 2. 财务
    await page.click('nav a:has-text("财务")')
    await page.waitForURL('/money')
    await expect(page.locator('h1')).toHaveText('财务')
    // 验证记账页 FAB 在移动端正确悬浮
    await expect(page.locator('button[aria-label="记一笔"]')).toBeVisible()

    // 3. 厨房（做饭流程与菜谱）
    await page.click('nav a:has-text("厨房")')
    await page.waitForURL('/kitchen')
    await expect(page.locator('h1')).toHaveText('厨房')
    // 验证大尺寸烹饪模式按钮
    await expect(page.locator('button:has-text("开启做饭大字免脏屏模式")')).toBeVisible()

    // 4. 设置
    await page.click('nav a:has-text("设置")')
    await page.waitForURL('/settings')
    await expect(page.locator('h1')).toHaveText('系统管理')
    await expect(page.locator('text=退出登录')).toBeVisible()

    // 5. 对话
    await page.click('nav a:has-text("对话")')
    await page.waitForURL('/')
    await expect(page.locator('header')).toBeVisible()
  })

  test('移动端会话抽屉抽拉与遮罩触摸交互', async ({ page }) => {
    await login(page)

    // 打开抽屉
    const menuBtn = page.getByRole('button', { name: '会话列表' })
    await menuBtn.click()

    const drawer = page.locator('aside')
    await expect(drawer).toBeVisible()
    await expect(drawer.locator('text=＋ 新建对话')).toBeVisible()

    // 点击半透明遮罩背景
    const overlay = page.locator('div[aria-hidden="true"]')
    await overlay.click({ position: { x: 380, y: 300 } })
    await expect(drawer).not.toBeVisible()
  })
})
