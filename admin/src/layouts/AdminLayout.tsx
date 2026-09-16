import { ApiOutlined, AuditOutlined, DashboardOutlined, ToolOutlined } from '@ant-design/icons'
import { Layout, Menu } from 'antd'
import { useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router'

const { Header, Sider, Content } = Layout

const ITEMS = [
  { key: '/dashboard', icon: <DashboardOutlined />, label: '仪表盘' },
  { key: '/debug', icon: <ToolOutlined />, label: '工具调试' },
  { key: '/admin', icon: <ApiOutlined />, label: '配置与权限' },
  { key: '/audit', icon: <AuditOutlined />, label: '审计日志' },
]

/** 桌面管理端布局：左侧栏 + 路由 Outlet（对齐 web-01~03 设计图） */
export function AdminLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const [collapsed, setCollapsed] = useState(false)

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
        <Header style={{ padding: '0 24px', background: '#fff', fontWeight: 600 }}>
          家事协作中枢 · 管理端
        </Header>
        <Content style={{ margin: 24, padding: 24, background: '#fff', borderRadius: 12 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
