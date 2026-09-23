import { useEffect, useState } from 'react'
import { Card, Tabs, Tag, Space, Typography, Button, Empty, message, Row, Col } from 'antd'
import { LikeOutlined, CommentOutlined, EyeOutlined, FireOutlined, StarOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { request } from '../api/client'
import type { PageResult, Post } from '../types'
import { getIdentity } from '../utils/storage'

export default function FeedPage() {
  const navigate = useNavigate()
  const [posts, setPosts] = useState<Post[]>([])
  const [featured, setFeatured] = useState<Post[]>([])
  const [loading, setLoading] = useState(false)
  const [view, setView] = useState<'latest' | 'hot'>('latest')

  const load = async (mode: 'latest' | 'hot') => {
    setLoading(true)
    try {
      if (mode === 'latest') {
        const data = await request<PageResult<Post>>('get', '/posts', { page: 1, page_size: 20 })
        setPosts(data.items)
      } else {
        const data = await request<Post[]>('get', '/posts/hot')
        setPosts(data)
      }
    } catch (e) {
      message.error((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const loadFeatured = async () => {
    try {
      const data = await request<Post[]>('get', '/posts/featured')
      setFeatured(data)
    } catch {
      // 精选加载失败不影响主列表
    }
  }

  useEffect(() => {
    load(view)
    loadFeatured()
  }, [view])

  const like = async (postId: number, e: React.MouseEvent) => {
    e.stopPropagation()
    if (!getIdentity()) {
      message.warning('请先创建匿名身份')
      return
    }
    try {
      await request<{ liked: boolean; likeCount: number }>('post', '/likes/toggle', { targetType: 'post', targetId: postId })
      load(view)
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const renderPost = (post: Post) => (
    <Card
      key={post.id}
      className="post-card"
      hoverable
      onClick={() => navigate(`/posts/${post.id}`)}
      style={{ marginBottom: 16 }}
    >
      <Space align="start" style={{ width: '100%' }}>
        <img src={post.avatar} alt={post.nickname} style={{ width: 40, height: 40, borderRadius: '50%' }} />
        <div style={{ flex: 1 }}>
          <Space direction="vertical" size={4} style={{ width: '100%' }}>
            <Space>
              <Typography.Text strong>{post.nickname}</Typography.Text>
              <Typography.Text type="secondary">{post.createdAt}</Typography.Text>
            </Space>
            {post.title && <Typography.Title level={5} style={{ margin: 0 }}>{post.title}</Typography.Title>}
            <Typography.Paragraph style={{ marginBottom: 8, whiteSpace: 'pre-wrap' }}>{post.content}</Typography.Paragraph>
            {post.images?.length > 0 && (
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 8 }}>
                {post.images.map((url, idx) => (
                  <img key={idx} src={url} alt={`${post.nickname}-${idx}`} style={{ width: 120, height: 120, objectFit: 'cover', borderRadius: 8 }} />
                ))}
              </div>
            )}
            {post.tags.map((tag) => (
              <Tag key={tag.id} color="blue">{tag.name}</Tag>
            ))}
            <Space size="large">
              <Button size="small" type={post.liked ? 'primary' : 'text'} icon={<LikeOutlined />} onClick={(e) => like(post.id, e)}>
                {post.likeCount}
              </Button>
              <Typography.Text type="secondary"><CommentOutlined /> {post.commentCount}</Typography.Text>
              <Typography.Text type="secondary"><EyeOutlined /> {post.viewCount}</Typography.Text>
              {post.isFeatured && <Typography.Text type="warning"><StarOutlined /> 精选</Typography.Text>}
            </Space>
          </Space>
        </div>
      </Space>
    </Card>
  )

  return (
    <div>
      {featured.length > 0 && (
        <Card title={<span><StarOutlined style={{ color: '#faad14' }} /> 每日精选</span>} style={{ marginBottom: 16 }}>
          <Row gutter={12}>
            {featured.slice(0, 3).map((p) => (
              <Col span={8} key={p.id}>
                <Card size="small" hoverable onClick={() => navigate(`/posts/${p.id}`)}>
                  <Typography.Text strong>{p.nickname}</Typography.Text>
                  <Typography.Paragraph ellipsis={{ rows: 2 }} style={{ marginBottom: 0 }}>{p.content}</Typography.Paragraph>
                  <Typography.Text type="secondary"><FireOutlined /> {p.likeCount} 赞 · {p.commentCount} 评论</Typography.Text>
                </Card>
              </Col>
            ))}
          </Row>
        </Card>
      )}
      <Card>
        <Tabs
          activeKey={view}
          onChange={(key) => setView(key as 'latest' | 'hot')}
          items={[
            { key: 'latest', label: '最新帖子' },
            { key: 'hot', label: '热度排行' },
          ]}
        />
        {loading ? <Typography.Text>加载中...</Typography.Text> : posts.length === 0 ? <Empty description="还没有帖子，快去发布第一条吧" /> : posts.map(renderPost)}
      </Card>
    </div>
  )
}
