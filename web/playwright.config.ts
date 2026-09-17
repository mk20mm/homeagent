import { defineConfig, devices } from '@playwright/test'

/**
 * E2E 配置：对真实前后端（web 5173 + backend 8080）做端到端验证。
 * 串行执行：单家庭数据共享，并行会互相污染（会话/账单）。
 * 长超时：真实 LLM 流式响应较慢。
 */
export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  retries: 1,
  forbidOnly: !!process.env.CI,
  timeout: 90_000,
  expect: { timeout: 40_000 },
  reporter: [['list'], ['html', { outputFolder: 'e2e-report', open: 'never' }]],
  outputDir: 'e2e-results',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    actionTimeout: 20_000,
    navigationTimeout: 20_000,
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
})
