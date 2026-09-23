import { useEffect, useState } from 'react'
import { Layout, Menu, Typography, Space, Button, Avatar } from 'antd'
import { HomeOutlined, EditOutlined, UserOutlined, SafetyOutlined } from '@ant-design/icons'
import { Routes, Route, useNavigate, useLocation } from 'react-router-dom'
import FeedPage from './pages/FeedPage'
import PostDetailPage from './pages/PostDetailPage'
import ComposePage from './pages/ComposePage'
import IdentityPage from './pages/IdentityPage'
import AdminPage from './pages/AdminPage'
import { getIdentity } from './utils/storage'
import type { Identity } from './types'

const { Header, Content } = Layout

export default function App() {
  const navigate = useNavigate()
  const location = useLocation()
  const [identity, setIdentity] = useState<Identity | null>(getIdentity())

  const menuItems = [
    { key: '/', icon: <HomeOutlined />, label: '首页' },
    { key: '/compose', icon: <EditOutlined />, label: '发布' },
    { key: '/identity', icon: <UserOutlined />, label: '身份' },
    { key: '/admin', icon: <SafetyOutlined />, label: '审核' },
  ]

  useEffect(() => {
    const update = () => setIdentity(getIdentity())
    window.addEventListener('storage', update)
    window.addEventListener('gbtreehole:identity-changed', update)
    return () => {
      window.removeEventListener('storage', update)
      window.removeEventListener('gbtreehole:identity-changed', update)
    }
  }, [])

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', paddingInline: 24 }}>
        <Space size="large">
          <Typography.Title level={4} style={{ color: '#fff', margin: 0 }}>
            匿名树洞社区
          </Typography.Title>
          <Menu
            theme="dark"
            mode="horizontal"
            selectedKeys={[location.pathname === '/' ? '/' : `/${location.pathname.split('/')[1]}`]}
            items={menuItems}
            onClick={(e) => navigate(e.key)}
            style={{ flex: 1, minWidth: 320 }}
          />
        </Space>
        <Space>
          {identity ? (
            <Button type="text" style={{ color: '#fff' }} icon={<Avatar size="small" src={identity.avatar} />} onClick={() => navigate('/identity')}>
              {identity.nickname}
            </Button>
          ) : (
            <Button type="primary" onClick={() => navigate('/identity')}>
              创建匿名身份
            </Button>
          )}
        </Space>
      </Header>
      <Content className="app-shell">
        <Routes>
          <Route path="/" element={<FeedPage />} />
          <Route path="/posts/:id" element={<PostDetailPage />} />
          <Route path="/compose" element={<ComposePage />} />
          <Route path="/identity" element={<IdentityPage />} />
          <Route path="/admin" element={<AdminPage />} />
        </Routes>
      </Content>
    </Layout>
  )
}
