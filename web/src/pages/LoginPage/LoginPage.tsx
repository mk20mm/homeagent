/**
 * 登录页：成员名 + 预共享令牌换 JWT（POST /auth/token）。
 * 令牌换完即弃，前端只存 JWT（ADR-005：权限每次实时校验）。
 */
import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router'

import {
  ERROR_MESSAGE,
  type ErrorCode,
  getAuth,
  saveAuth,
  type MemberRole,
} from '@homeagent/shared'

import { api, ApiError, unwrap } from '../../api/client'
import { tokens } from '../../styles/tokens'

import styles from './LoginPage.module.css'

export function LoginPage() {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [authToken, setAuthToken] = useState('')
  const [error, setError] = useState<string>()
  const [loading, setLoading] = useState(false)

  // 已登录直接进主页
  if (getAuth()) return <Navigate to="/" replace />

  // 会话被后端踢出（401）：提示「登录已过期」，不是首次凭据错误（T-A07）
  if (!error) {
    try {
      if (sessionStorage.getItem('homeagent.session_expired')) {
        sessionStorage.removeItem('homeagent.session_expired')
        setError(ERROR_MESSAGE.unauthorized)
      }
    } catch {
      // sessionStorage 不可用时静默
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !authToken.trim() || loading) return

    setError(undefined)
    setLoading(true)
    try {
      const data = unwrap(
        await api.POST('/auth/token', {
          body: { name: name.trim(), auth_token: authToken.trim() },
        }),
      )
      saveAuth({
        token: data.token,
        member_id: data.member_id,
        role: data.role as MemberRole,
        expires_at: data.expires_at,
      })
      navigate('/', { replace: true })
    } catch (err) {
      const code: ErrorCode = err instanceof ApiError ? err.code : 'internal'
      setError(ERROR_MESSAGE[code])
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className={styles.page} style={{ fontFamily: tokens.fontFamily }}>
      <form className={styles.card} onSubmit={handleSubmit}>
        <h1 className={styles.title} style={{ fontSize: tokens.fontSize.title }}>
          家事助手
        </h1>
        <p className={styles.subtitle}>登录后即可让 Agent 跑腿</p>

        <input
          className={styles.input}
          placeholder="成员名（如：爸爸）"
          autoComplete="username"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <input
          className={styles.input}
          type="password"
          placeholder="登录令牌"
          autoComplete="current-password"
          value={authToken}
          onChange={(e) => setAuthToken(e.target.value)}
        />

        {error && <div className={styles.error}>{error}</div>}

        <button type="submit" className={styles.submit} disabled={loading}>
          {loading ? '登录中…' : '登录'}
        </button>

        <p className={styles.tokenHint}>
          令牌由家庭管理员在「系统管理」中生成，每成员一个，问管理员要即可。
        </p>
      </form>
    </div>
  )
}
