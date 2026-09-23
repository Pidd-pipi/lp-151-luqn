import { useEffect, useState } from 'react'
import { Card, List, Button, Space, Tag, Typography, message, Table, Tabs } from 'antd'
import { CheckOutlined, CloseOutlined } from '@ant-design/icons'
import { request } from '../api/client'
import type { PageResult, ReviewItem } from '../types'

export default function AdminPage() {
  const [reviews, setReviews] = useState<ReviewItem[]>([])
  const [words, setWords] = useState<{ id: number; word: string }[]>([])
  const [status, setStatus] = useState(1)

  const loadReviews = async () => {
    try {
      const data = await request<PageResult<ReviewItem>>('get', '/admin/reviews', { page: 1, page_size: 50, status })
      setReviews(data.items)
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const loadWords = async () => {
    try {
      const data = await request<{ id: number; word: string }[]>('get', '/admin/sensitive-words')
      setWords(data)
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  useEffect(() => {
    loadReviews()
    loadWords()
  }, [status])

  const action = async (queueId: number, act: 'approve' | 'reject') => {
    try {
      await request('post', '/admin/reviews/action', { queueId, action: act, note: '' })
      message.success(act === 'approve' ? '已放行' : '已屏蔽')
      loadReviews()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  return (
    <Tabs
      items={[
        {
          key: 'reviews',
          label: '审核队列',
          children: (
            <Card>
              <Tabs
                size="small"
                activeKey={String(status)}
                onChange={(key) => setStatus(Number(key))}
                items={[
                  { key: '1', label: '待审核' },
                  { key: '2', label: '已放行' },
                  { key: '3', label: '已屏蔽' },
                ]}
              />
              <List
                dataSource={reviews}
                locale={{ emptyText: '暂无审核项' }}
                renderItem={(item) => (
                  <List.Item
                    actions={
                      item.status === 1
                        ? [
                            <Button key="approve" type="primary" size="small" icon={<CheckOutlined />} onClick={() => action(item.id, 'approve')}>放行</Button>,
                            <Button key="reject" danger size="small" icon={<CloseOutlined />} onClick={() => action(item.id, 'reject')}>屏蔽</Button>,
                          ]
                        : undefined
                    }
                  >
                    <List.Item.Meta
                      title={
                        <Space>
                          <Tag color={item.targetType === 'post' ? 'blue' : 'purple'}>{item.targetType === 'post' ? '帖子' : '评论'}</Tag>
                          <Tag color="red">{item.hitWords || '敏感词'}</Tag>
                          <Typography.Text type="secondary">状态: {item.status === 1 ? '待审核' : item.status === 2 ? '已放行' : '已屏蔽'}</Typography.Text>
                        </Space>
                      }
                      description={<Typography.Paragraph>{item.content}</Typography.Paragraph>}
                    />
                  </List.Item>
                )}
              />
            </Card>
          ),
        },
        {
          key: 'words',
          label: '敏感词库',
          children: (
            <Card>
              <Table
                rowKey="id"
                dataSource={words}
                pagination={false}
                columns={[
                  { title: 'ID', dataIndex: 'id', width: 80 },
                  { title: '敏感词', dataIndex: 'word' },
                ]}
              />
            </Card>
          ),
        },
      ]}
    />
  )
}
