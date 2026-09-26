/**
 * 登录页：成员名 + 预共享令牌换 JWT（POST /auth/token）。
 * 令牌换完即弃，前端只存 JWT（ADR-005：权限每次实时校验）。
 */
import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router'

import { ERROR_MESSAGE, type ErrorCode, getAuth, saveAuth, type MemberRole } from '@homeagent/shared'

import { api, ApiError, unwrap } from '../../api/client'
import { tokens } from '../../styles/tokens'

import styles from './LoginPage.module.css'

export function LoginPage() {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [authToken, setAuthToken] = useState('')
  const [serverUrl, setServerUrl] = useState(() => localStorage.getItem('homeagent_server_url') || '')
  const [showConfig, setShowConfig] = useState(false)
  const [error, setError] = useState<string>()
  const [loading, setLoading] = useState(false)

  const handleServerChange = (val: string) => {
    setServerUrl(val)
    if (val.trim()) {
      localStorage.setItem('homeagent_server_url', val.trim())
    } else {
      localStorage.removeItem('homeagent_server_url')
    }
  }

  // 已登录直接进主页
  if (getAuth()) return <Navigate to="/" replace />

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

        <div style={{ marginTop: '16px', textAlign: 'center' }}>
          <button
            type="button"
            style={{
              background: 'none',
              border: 'none',
              color: '#8e8e93',
              fontSize: '12px',
              cursor: 'pointer',
              padding: '6px 10px',
            }}
            onClick={() => setShowConfig(!showConfig)}
          >
            ⚙️ {showConfig ? '收起配置' : '服务器设置 (局域网/原生)'}
          </button>
        </div>

        {showConfig && (
          <div style={{ marginTop: '10px', width: '100%' }}>
            <input
              className={styles.input}
              style={{ fontSize: '13px', padding: '10px 12px' }}
              placeholder="服务器地址 (如 http://192.168.0.109:8080)"
              value={serverUrl}
              onChange={(e) => handleServerChange(e.target.value)}
            />
          </div>
        )}
      </form>
    </div>
  )
}
