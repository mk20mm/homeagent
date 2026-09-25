import {
  AuditOutlined,
  CloudServerOutlined,
  DashboardOutlined,
  SafetyCertificateOutlined,
  ToolOutlined,
} from '@ant-design/icons'
import { Layout, Menu } from 'antd'
import { useSyncExternalStore, useState } from 'react'
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router'

import { clearAuth, getAuth, MEMBER_ROLE_LABEL, subscribeAuth } from '@homeagent/shared'

const { Header, Sider, Content } = Layout

const ITEMS = [
  { key: '/dashboard', icon: <DashboardOutlined />, label: '仪表盘' },
  { key: '/debug', icon: <ToolOutlined />, label: '工具调试' },
  { key: '/models', icon: <CloudServerOutlined />, label: '模型管理' },
  { key: '/permissions', icon: <SafetyCertificateOutlined />, label: '权限管理' },
  { key: '/audit', icon: <AuditOutlined />, label: '审计日志' },
]

/** 桌面管理端布局：左侧栏 + 路由 Outlet（对齐 web-01~03 设计图） */
export function AdminLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const [collapsed, setCollapsed] = useState(false)

  // 路由守卫：无登录态去登录页（订阅变化，令牌失效即时跳转）
  const auth = useSyncExternalStore(subscribeAuth, getAuth, () => null)
  if (!auth) return <Navigate to="/login" replace />

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider collapsible collapsed={collapsed} onCollapse={setCollapsed} theme="dark">
        <div
          style={{
            height: 48,
            margin: 8,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#fff',
            fontWeight: 600,
            whiteSpace: 'nowrap',
            overflow: 'hidden',
          }}
        >
          {collapsed ? '家事' : '家事 Agent · 管理端'}
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[location.pathname]}
          items={ITEMS}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <Layout>
        <Header
          style={{
            padding: '0 24px',
            background: '#fff',
            fontWeight: 600,
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <span>家事协作中枢 · 管理端</span>
          <span style={{ fontWeight: 400, fontSize: 13, color: '#8e8e93' }}>
            {MEMBER_ROLE_LABEL[auth.role]}{' '}
            <a
              style={{ marginLeft: 12 }}
              onClick={() => {
                clearAuth()
                navigate('/login', { replace: true })
              }}
            >
              退出登录
            </a>
          </span>
        </Header>
        <Content style={{ margin: 24, padding: 24, background: '#fff', borderRadius: 12 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
