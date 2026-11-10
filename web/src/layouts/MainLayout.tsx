import { useState } from 'react'
import { useNavigate, useOutlet, useLocation } from 'react-router-dom'
import { Avatar, Dropdown, Layout, Menu, Typography } from 'antd'
import {
  BarChartOutlined,
  InboxOutlined,
  LogoutOutlined,
  SettingOutlined,
  TeamOutlined,
  ClusterOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { clearSession, getUser, type Role } from '@/auth/session'

const { Header, Sider, Content } = Layout
const { Text } = Typography

/** 角色中文名，用于顶栏展示。 */
const ROLE_LABEL: Record<Role, string> = {
  admin: '管理员',
  operator: '操作员',
  inspector: '质检员',
  viewer: '只读用户',
}

/**
 * 主布局：左侧导航 + 顶栏用户区。
 *
 * 导航项按 spec §7.1 的结构，但只保留**已有后端支撑**的页面：
 * 报表与系统管理尚未实现，仓储管理依赖的包装四表在 Center 恒为空，
 * 因此不放死链。后续页面落地时在此追加菜单项与路由。
 */
export default function MainLayout() {
  const outlet = useOutlet()
  const navigate = useNavigate()
  const location = useLocation()
  const [collapsed, setCollapsed] = useState(false)

  const user = getUser()

  // 选中态：优先匹配完整路径（如 /system/users），否则退回首段。
  const pathname = location.pathname
  const selectedKey = pathname.startsWith('/system/')
    ? pathname
    : '/' + (pathname.split('/')[1] ?? '')

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider collapsible collapsed={collapsed} onCollapse={setCollapsed}>
        <div
          style={{
            height: 48,
            margin: 8,
            color: '#fff',
            display: 'grid',
            placeItems: 'center',
            fontWeight: 600,
            letterSpacing: 1,
          }}
        >
          {collapsed ? 'SR' : 'SilkRoad'}
        </div>

        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          onClick={({ key }) => navigate(key)}
          items={[
            { key: '/lots', icon: <InboxOutlined />, label: '批次管理' },
            { key: '/reports', icon: <BarChartOutlined />, label: '报表' },
            {
              key: 'system',
              icon: <SettingOutlined />,
              label: '系统管理',
              children: [
                { key: '/system/users', icon: <TeamOutlined />, label: '用户' },
                { key: '/system/edges', icon: <ClusterOutlined />, label: '设备' },
              ],
            },
          ]}
        />
      </Sider>

      <Layout>
        <Header
          style={{
            background: '#fff',
            padding: '0 24px',
            display: 'flex',
            justifyContent: 'flex-end',
            alignItems: 'center',
            gap: 12,
          }}
        >
          <Text type="secondary">{user ? ROLE_LABEL[user.role] : ''}</Text>

          <Dropdown
            menu={{
              items: [
                {
                  key: 'logout',
                  icon: <LogoutOutlined />,
                  label: '退出登录',
                  onClick: () => {
                    clearSession()
                    navigate('/login', { replace: true })
                  },
                },
              ],
            }}
          >
            <span style={{ cursor: 'pointer', display: 'inline-flex', alignItems: 'center', gap: 8 }}>
              <Avatar size="small" icon={<UserOutlined />} />
              {user?.full_name ?? user?.username}
            </span>
          </Dropdown>
        </Header>

        <Content style={{ margin: 16 }}>{outlet}</Content>
      </Layout>
    </Layout>
  )
}
