import { useEffect, useState } from 'react'
import { Card, Form, Input, Select, Button, message, Alert, Space } from 'antd'
import { useNavigate } from 'react-router-dom'
import { request } from '../api/client'
import type { Tag } from '../types'
import { getIdentity } from '../utils/storage'

export default function ComposePage() {
  const navigate = useNavigate()
  const [tags, setTags] = useState<Tag[]>([])
  const [blockedInfo, setBlockedInfo] = useState<string | null>(null)
  const [form] = Form.useForm()

  useEffect(() => {
    request<Tag[]>('get', '/tags').then(setTags).catch(() => {})
  }, [])

  const submit = async (values: { title?: string; content: string; tags?: string[]; images?: string[] }) => {
    if (!getIdentity()) {
      message.warning('请先创建匿名身份')
      return
    }
    try {
      const data = await request<{ post: unknown; blocked: boolean; hitWords: string[] }>('post', '/posts', {
        title: values.title || '',
        content: values.content,
        images: values.images || [],
        tags: values.tags || [],
      })
      if (data.blocked) {
        setBlockedInfo(`内容命中敏感词：${data.hitWords.join('、')}，已进入审核队列`)
      } else {
        message.success('发布成功')
        navigate('/')
      }
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  return (
    <Card title="发布匿名帖子">
      <Form form={form} layout="vertical" onFinish={submit}>
        <Form.Item name="title" label="标题">
          <Input placeholder="可选" maxLength={255} />
        </Form.Item>
        <Form.Item name="content" label="内容" rules={[{ required: true, message: '请输入内容' }]}>
          <Input.TextArea rows={6} maxLength={5000} placeholder="倾诉你的故事..." />
        </Form.Item>
        <Form.Item name="tags" label="话题标签">
          <Select mode="tags" placeholder="选择或输入新标签" options={tags.map((t) => ({ label: t.name, value: t.name }))} />
        </Form.Item>
        <Form.Item name="images" label="图片链接">
          <Select mode="tags" placeholder="粘贴图片 URL 后回车，最多 9 张" />
        </Form.Item>
        {blockedInfo && <Alert type="warning" message={blockedInfo} style={{ marginBottom: 16 }} showIcon />}
        <Space>
          <Button type="primary" htmlType="submit">发布</Button>
          <Button onClick={() => navigate('/')}>取消</Button>
        </Space>
      </Form>
    </Card>
  )
}
