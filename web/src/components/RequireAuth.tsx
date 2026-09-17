/**
 * 路由守卫：无登录态重定向到 /login。
 * 用 useSyncExternalStore 订阅登录态变化（含跨标签页），避免令牌失效后仍停留页面。
 */
import { useSyncExternalStore } from 'react'
import { Navigate } from 'react-router'

import { getAuth, subscribeAuth } from '@homeagent/shared'

export function RequireAuth({ children }: { children: React.ReactNode }) {
  const auth = useSyncExternalStore(subscribeAuth, getAuth, () => null)
  if (!auth) return <Navigate to="/login" replace />
  return <>{children}</>
}
