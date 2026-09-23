import { useEffect, useState } from 'react'
import { Card, Button, Input, Form, List, Avatar, Space, message, Typography, Tag } from 'antd'
import { PlusOutlined, SwapOutlined } from '@ant-design/icons'
import { request } from '../api/client'
import { saveIdentities, getIdentities, saveIdentity, saveToken, getIdentity } from '../utils/storage'
import type { Identity } from '../types'

export default function IdentityPage() {
  const [identities, setIdentities] = useState<Identity[]>(getIdentities())
  const [current, setCurrent] = useState<Identity | null>(getIdentity())
  const [form] = Form.useForm()

  useEffect(() => {
    saveIdentities(identities)
  }, [identities])

  const create = async (values: { nickname?: string; avatar?: string }) => {
    try {
      const data = await request<{ identity: Identity; token: string }>('post', '/auth/identities', {
        nickname: values.nickname || '',
        avatar: values.avatar || '',
      })
      setIdentities((prev) => [...prev, data.identity])
      setCurrent(data.identity)
      saveIdentity(data.identity)
      saveToken(data.token)
      window.dispatchEvent(new Event('gbtreehole:identity-changed'))
      form.resetFields()
      message.success('身份创建成功')
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const switchIdentity = async (identity: Identity) => {
    try {
      const data = await request<{ identity: Identity; token: string }>('post', '/auth/login', { identityKey: identity.identityKey })
      setCurrent(data.identity)
      saveIdentity(data.identity)
      saveToken(data.token)
      window.dispatchEvent(new Event('gbtreehole:identity-changed'))
      message.success(`已切换到 ${data.identity.nickname}`)
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  return (
    <div>
      <Card title="创建匿名身份">
        <Form form={form} layout="inline" onFinish={create}>
          <Form.Item name="nickname" label="昵称">
            <Input placeholder="留空自动生成" maxLength={32} />
          </Form.Item>
          <Form.Item name="avatar" label="头像 URL">
            <Input placeholder="可选" maxLength={255} style={{ width: 240 }} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" icon={<PlusOutlined />}>创建身份</Button>
          </Form.Item>
        </Form>
        <Typography.Paragraph type="secondary" style={{ marginTop: 8 }}>
          每个身份拥有唯一的身份密钥，密钥只保存在本地浏览器，用于管理自己发布的内容。
        </Typography.Paragraph>
      </Card>
      <Card title="我的匿名身份" style={{ marginTop: 16 }}>
        <List
          dataSource={identities}
          locale={{ emptyText: '还没有身份，先创建一个吧' }}
          renderItem={(item) => (
            <List.Item
              actions={[
                current?.id === item.id ? <Tag color="green">当前</Tag> : <Button size="small" icon={<SwapOutlined />} onClick={() => switchIdentity(item)}>切换</Button>,
              ]}
            >
              <List.Item.Meta
                avatar={<Avatar src={item.avatar} />}
                title={<Space><Typography.Text strong>{item.nickname}</Typography.Text>{current?.id === item.id && <Typography.Text type="secondary">使用中</Typography.Text>}</Space>}
                description={<Typography.Text copyable style={{ fontSize: 12 }}>{item.identityKey}</Typography.Text>}
              />
            </List.Item>
          )}
        />
      </Card>
    </div>
  )
}
