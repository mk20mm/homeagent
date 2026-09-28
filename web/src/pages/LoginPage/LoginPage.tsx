/**
 * 登录页：成员名 + 预共享令牌换 JWT（POST /auth/token）。
 * 令牌换完即弃，前端只存 JWT（ADR-005：权限每次实时校验）。
 */
import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router'

import { ERROR_MESSAGE, getAuth, saveAuth, type MemberRole } from '@homeagent/shared'

import { api, ApiError, getApiBaseUrl, unwrap } from '../../api/client'
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

  const [diagMsg, setDiagMsg] = useState<string>()
  const [testing, setTesting] = useState(false)

  const handleTestConnection = async () => {
    setTesting(true)
    setDiagMsg('正在检测连通性…')
    try {
      const base = getApiBaseUrl().replace(/\/api\/v1\/?$/, '')
      const res = await fetch(`${base}/api/v1/health`, { method: 'GET' })
      if (res.ok) {
        setDiagMsg(`✅ 连接正常: ${base} 响应 OK`)
      } else {
        setDiagMsg(`⚠️ 后端响应异常: HTTP ${res.status}`)
      }
    } catch (e) {
      setDiagMsg(`❌ 无法连接到电脑后端: ${(e as Error).message}。请确认手机与电脑连接同一WiFi`)
    } finally {
      setTesting(false)
    }
  }

  // 已登录直接进主页
  if (getAuth()) return <Navigate to="/" replace />

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !authToken.trim()) {
      setError('请先输入成员名与登录令牌，或点击下方快捷填入')
      return
    }
    if (loading) return

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
      if (err instanceof ApiError) {
        setError(ERROR_MESSAGE[err.code] || err.message)
      } else if (err instanceof Error) {
        const base = getApiBaseUrl()
        setError(`网络连接失败: 无法连接后端 [${base}] - ${err.message}`)
      } else {
        setError('登录失败，请检查网络连接')
      }
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

        {/* 快捷填入按钮 */}
        <div style={{ display: 'flex', gap: '8px', marginBottom: '16px', justifyContent: 'center' }}>
          <button
            type="button"
            style={{
              padding: '6px 12px',
              fontSize: '12px',
              borderRadius: '16px',
              border: '1px solid #007aff',
              background: '#f0f7ff',
              color: '#007aff',
              cursor: 'pointer',
              fontWeight: 500,
            }}
            onClick={() => {
              setName('爸爸')
              setAuthToken('dev-baba')
              setError(undefined)
            }}
          >
            👨 一键填入爸爸
          </button>
          <button
            type="button"
            style={{
              padding: '6px 12px',
              fontSize: '12px',
              borderRadius: '16px',
              border: '1px solid #34c759',
              background: '#f0fff4',
              color: '#34c759',
              cursor: 'pointer',
              fontWeight: 500,
            }}
            onClick={() => {
              setName('妈妈')
              setAuthToken('dev-mama')
              setError(undefined)
            }}
          >
            👩 一键填入妈妈
          </button>
        </div>

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

        {/* 实时后端网络指示器 */}
        <div style={{ marginTop: '14px', textAlign: 'center' }}>
          <div style={{ fontSize: '11px', color: '#8e8e93', marginBottom: '4px' }}>
            目标后端: <span style={{ fontFamily: 'monospace' }}>{getApiBaseUrl()}</span>
          </div>
          <button
            type="button"
            style={{
              background: '#f2f2f7',
              border: 'none',
              borderRadius: '10px',
              padding: '4px 10px',
              fontSize: '11px',
              color: '#007aff',
              cursor: 'pointer',
            }}
            onClick={handleTestConnection}
            disabled={testing}
          >
            {testing ? '正在测试…' : '🔍 点此测试后端连通性'}
          </button>
          {diagMsg && (
            <div style={{ fontSize: '11px', marginTop: '6px', color: diagMsg.startsWith('✅') ? '#34c759' : '#ff3b30' }}>
              {diagMsg}
            </div>
          )}
        </div>

        <div style={{ marginTop: '12px', textAlign: 'center' }}>
          <button
            type="button"
            style={{
              background: 'none',
              border: 'none',
              color: '#8e8e93',
              fontSize: '11px',
              cursor: 'pointer',
              padding: '4px 8px',
            }}
            onClick={() => setShowConfig(!showConfig)}
          >
            ⚙️ {showConfig ? '收起配置' : '手动修改服务器地址'}
          </button>
        </div>

        {showConfig && (
          <div style={{ marginTop: '10px', width: '100%' }}>
            <input
              className={styles.input}
              style={{ fontSize: '12px', padding: '8px 10px' }}
              placeholder="服务器地址 (如 http://192.168.7.115:8080)"
              value={serverUrl}
              onChange={(e) => handleServerChange(e.target.value)}
            />
          </div>
        )}
      </form>
    </div>
  )
}
