import path from 'node:path'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

// 移动端（家人用，对话为主入口）。PWA 打通手机与桌面（CONVENTIONS-frontend §8）。
export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.svg'],
      manifest: {
        name: '家事 Agent',
        short_name: '家事',
        description: '动嘴，系统跑腿：家务/用餐/账单/日程一个对话窗口搞定',
        theme_color: '#0A84FF',
        background_color: '#F2F3F7',
        display: 'standalone',
        lang: 'zh-CN',
      },
    }),
  ],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      // 开发期 API 走 Vite 代理，避免 CORS
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    css: true,
  },
})
