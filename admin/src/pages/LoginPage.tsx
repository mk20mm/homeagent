/**
 * 管理端登录页：成员名 + 预共享令牌换 JWT（与移动端同一端点）。
 */
import { Button, Card, Form, Input, Typography, message } from 'antd'
import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router'

import { api, ApiError, unwrap } from '@/api/client'
import { ERROR_MESSAGE, type ErrorCode, getAuth, saveAuth, type MemberRole } from '@homeagent/shared'

export function LoginPage() {
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)

  if (getAuth()) return <Navigate to="/dashboard" replace />

  const onFinish = async (values: { name: string; auth_token: string }) => {
    setLoading(true)
    try {
      const data = unwrap(
        await api.POST('/auth/token', {
          body: { name: values.name.trim(), auth_token: values.auth_token.trim() },
        }),
      )
      saveAuth({
        token: data.token,
        member_id: data.member_id,
        role: data.role as MemberRole,
        expires_at: data.expires_at,
      })
      navigate('/dashboard', { replace: true })
    } catch (err) {
      const code: ErrorCode = err instanceof ApiError ? err.code : 'internal'
      message.error(ERROR_MESSAGE[code])
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#f0f2f5' }}>
      <Card style={{ width: 380 }}>
        <Typography.Title level={3} style={{ textAlign: 'center', marginBottom: 24 }}>
          家事 Agent · 管理端
        </Typography.Title>
        <Form layout="vertical" onFinish={onFinish} disabled={loading}>
          <Form.Item name="name" label="成员名" rules={[{ required: true, message: '请输入成员名' }]}>
            <Input autoComplete="username" />
          </Form.Item>
          <Form.Item name="auth_token" label="登录令牌" rules={[{ required: true, message: '请输入登录令牌' }]}>
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" block loading={loading}>
            登录
          </Button>
        </Form>
      </Card>
    </div>
  )
}
