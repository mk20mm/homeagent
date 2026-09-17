import { defineConfig } from 'vitest/config'

// auth 模块用 localStorage/EventTarget，测试跑在 jsdom
export default defineConfig({
  test: {
    environment: 'jsdom',
  },
})
