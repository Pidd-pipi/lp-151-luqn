import { useCallback, useEffect, useState } from 'react'
import {
  Card, Space, Typography, Button, Input, List, message, Tag, Avatar, Modal, Form, Popconfirm, Alert, Empty,
} from 'antd'
import { LikeOutlined, CommentOutlined, EyeOutlined, EditOutlined, UndoOutlined } from '@ant-design/icons'
import { useParams, useNavigate } from 'react-router-dom'
import { request } from '../api/client'
import type { Comment, PageResult, Post } from '../types'
import { getIdentity } from '../utils/storage'
import { ContentStatusBadge, OwnerNotice, ReviewProgressTag } from '../components/ModerationStatus'

interface SubmitResult {
  blocked: boolean
  hitWords: string[]
}

export default function PostDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [post, setPost] = useState<Post | null>(null)
  const [comments, setComments] = useState<Comment[]>([])
  const [commentText, setCommentText] = useState('')
  const [notFound, setNotFound] = useState(false)
  const [postForm] = Form.useForm()
  const [postEditing, setPostEditing] = useState(false)
  const [commentForm] = Form.useForm()
  const [editingComment, setEditingComment] = useState<Comment | null>(null)

  const current = getIdentity()
  const postId = Number(id)

  const load = useCallback(async () => {
    try {
      const postData = await request<Post>('get', `/posts/${id}`)
      setPost(postData)
      setNotFound(false)
      const commentData = await request<PageResult<Comment>>('get', `/posts/${id}/comments`, { page: 1, page_size: 50 })
      setComments(commentData.items)
    } catch (e) {
      setPost(null)
      setNotFound(true)
      message.error((e as Error).message)
    }
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  const requireIdentity = (): boolean => {
    if (!getIdentity()) {
      message.warning('请先创建匿名身份')
      return false
    }
    return true
 }

  const likePost = async () => {
    if (!requireIdentity()) return
    try {
      await request('post', '/likes/toggle', { targetType: 'post', targetId: postId })
      load()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const likeComment = async (commentId: number) => {
    if (!requireIdentity()) return
    try {
      await request('post', '/likes/toggle', { targetType: 'comment', targetId: commentId })
      load()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const submitComment = async () => {
    if (!requireIdentity()) return
    if (!commentText.trim()) return
    try {
      const data = await request<SubmitResult>('post', '/comments', { postId, content: commentText.trim() })
      setCommentText('')
      if (data.blocked) {
        message.warning(`评论命中敏感词：${data.hitWords.join('、')}，已进入审核队列，仅你自己可见`)
      } else {
        message.success('评论已发布')
      }
      load()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const withdrawPost = async () => {
    try {
      await request('post', `/posts/${postId}/withdraw`)
      message.success('帖子已撤回')
      navigate('/')
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const savePost = async (values: { title?: string; content: string; images?: string | string[] }) => {
    const images = Array.isArray(values.images)
      ? values.images
      : (values.images || '').split(/[\n,，\s]+/).map((s) => s.trim()).filter(Boolean)
    try {
      const data = await request<SubmitResult>('put', `/posts/${postId}`, {
        title: values.title || '',
        content: values.content,
        images,
      })
      setPostEditing(false)
      if (data.blocked) {
        message.warning(`修改后的内容命中敏感词：${data.hitWords.join('、')}，已重新进入审核`)
      } else {
        message.success('帖子已更新')
      }
      load()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const withdrawComment = async (commentId: number) => {
    try {
      await request('post', `/comments/${commentId}/withdraw`)
      message.success('评论已撤回')
      load()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const saveComment = async (values: { content: string }) => {
    if (!editingComment) return
    try {
      const data = await request<SubmitResult>('put', `/comments/${editingComment.id}`, { content: values.content })
      setEditingComment(null)
      if (data.blocked) {
        message.warning(`修改后的评论命中敏感词：${data.hitWords.join('、')}，已重新进入审核`)
      } else {
        message.success('评论已更新')
      }
      load()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const openPostEditor = () => {
    postForm.setFieldsValue({ title: post?.title || '', content: post?.content, images: post?.images || [] })
    setPostEditing(true)
  }

  if (notFound) {
    return (
      <div>
        <Button type="link" onClick={() => navigate(-1)}>返回</Button>
        <Empty description="帖子不存在或已被撤回" style={{ marginTop: 48 }} />
      </div>
    )
  }
  if (!post) return <Typography.Text>加载中...</Typography.Text>

  const isOwner = current?.id === post.identityId
  const withdrawn = post.status === 4

  return (
    <div>
      <Button type="link" onClick={() => navigate(-1)}>返回</Button>
      <Card className="post-card">
        <Space align="start" style={{ width: '100%' }}>
          <Avatar size={48} src={post.avatar} />
          <div style={{ flex: 1 }}>
            <Space wrap>
              <Typography.Text strong>{post.nickname}</Typography.Text>
              <Typography.Text type="secondary">{post.createdAt}</Typography.Text>
              {isOwner && <ContentStatusBadge status={post.status} />}
              {isOwner && <ReviewProgressTag reviewStatus={post.reviewStatus} reviewNote={post.reviewNote} hitWords={post.hitWords} />}
              {isOwner && !withdrawn && (
                <Space size="small">
                  <Button size="small" icon={<EditOutlined />} onClick={openPostEditor}>编辑</Button>
                  <Popconfirm
                    title="撤回该帖子？"
                    description="帖子的评论、点赞和标签关系将一起撤下，且无法恢复。"
                    okText="撤回"
                    okButtonProps={{ danger: true }}
                    cancelText="取消"
                    onConfirm={withdrawPost}
                  >
                    <Button size="small" danger icon={<UndoOutlined />}>撤回</Button>
                  </Popconfirm>
                </Space>
              )}
            </Space>
            {isOwner && <div style={{ marginTop: 8 }}><OwnerNotice status={post.status} reviewStatus={post.reviewStatus} reviewNote={post.reviewNote} /></div>}
            {post.title && <Typography.Title level={4} style={{ margin: '8px 0' }}>{post.title}</Typography.Title>}
            <Typography.Paragraph style={{ whiteSpace: 'pre-wrap' }}>{post.content}</Typography.Paragraph>
            {post.images?.length > 0 && (
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', margin: '8px 0' }}>
                {post.images.map((url, idx) => (
                  <img key={idx} src={url} alt={`${post.nickname}-${idx}`} style={{ width: 160, height: 160, objectFit: 'cover', borderRadius: 8 }} />
                ))}
              </div>
            )}
            {post.tags?.map((tag) => (
              <Tag key={tag.id} color="blue">{tag.name}</Tag>
            ))}
            <Space size="large" style={{ marginTop: 8 }}>
              <Button type={post.liked ? 'primary' : 'text'} icon={<LikeOutlined />} onClick={likePost} disabled={post.status !== 1}>
                {post.likeCount}
              </Button>
              <Typography.Text type="secondary"><CommentOutlined /> {post.commentCount}</Typography.Text>
              <Typography.Text type="secondary"><EyeOutlined /> {post.viewCount}</Typography.Text>
            </Space>
          </div>
        </Space>
      </Card>

      <Card title={`评论 (${post.commentCount})`} style={{ marginTop: 16 }}>
        {post.status !== 1 ? (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 12 }}
            message="帖子未公开发布，暂时不能发表新评论。"
          />
        ) : (
          <>
            <Space.Compact style={{ width: '100%', marginBottom: 16 }}>
              <Input.TextArea value={commentText} onChange={(e) => setCommentText(e.target.value)} placeholder="写下你的匿名评论..." autoSize={{ minRows: 2, maxRows: 4 }} maxLength={1000} showCount />
            </Space.Compact>
            <Button type="primary" onClick={submitComment} style={{ marginTop: 8 }}>发表评论</Button>
          </>
        )}
        <List
          style={{ marginTop: 16 }}
          dataSource={comments}
          locale={{ emptyText: '暂无评论' }}
          renderItem={(item) => {
            const itemOwner = current?.id === item.identityId
            const itemWithdrawn = item.status === 4
            return (
              <List.Item
                actions={[
                  <Button key="like" type="text" size="small" icon={<LikeOutlined />}
                    disabled={item.status !== 1} onClick={() => likeComment(item.id)}>
                    {item.likeCount}
                  </Button>,
                  ...(itemOwner && !itemWithdrawn ? [
                    <Button key="edit" type="text" size="small" icon={<EditOutlined />}
                      onClick={() => {
                        commentForm.setFieldsValue({ content: item.content })
                        setEditingComment(item)
                      }}>
                      编辑
                    </Button>,
                    <Popconfirm
                      key="withdraw"
                      title="撤回该评论？"
                      description="撤回后不可恢复。"
                      okText="撤回"
                      okButtonProps={{ danger: true }}
                      cancelText="取消"
                      onConfirm={() => withdrawComment(item.id)}
                    >
                      <Button type="text" size="small" danger icon={<UndoOutlined />}>撤回</Button>
                    </Popconfirm>,
                  ] : []),
                ]}
              >
                <List.Item.Meta
                  avatar={<Avatar src={item.avatar} />}
                  title={
                    <Space wrap>
                      <Typography.Text strong>{item.nickname}</Typography.Text>
                      <Typography.Text type="secondary">{item.createdAt}</Typography.Text>
                      {itemOwner && <ContentStatusBadge status={item.status} />}
                      {itemOwner && <ReviewProgressTag reviewStatus={item.reviewStatus} reviewNote={item.reviewNote} hitWords={item.hitWords} />}
                    </Space>
                  }
                  description={
                    itemWithdrawn
                      ? <Typography.Text type="secondary">该评论已撤回</Typography.Text>
                      : <Typography.Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}>{item.content}</Typography.Paragraph>
                  }
                />
              </List.Item>
            )
          }}
        />
      </Card>

      <Modal
        title="编辑帖子"
        open={postEditing}
        onCancel={() => setPostEditing(false)}
        onOk={() => postForm.submit()}
        okText="保存"
        cancelText="取消"
        destroyOnClose
      >
        <Alert type="info" showIcon style={{ marginBottom: 12 }}
          message="保存后会重新检测敏感词，命中将先进入审核，审核期间内容仅你自己可见。" />
        <Form form={postForm} layout="vertical" onFinish={savePost}>
          <Form.Item name="title" label="标题">
            <Input maxLength={255} />
          </Form.Item>
          <Form.Item name="content" label="内容" rules={[{ required: true, message: '请输入内容' }]}>
            <Input.TextArea rows={6} maxLength={5000} showCount />
          </Form.Item>
          <Form.Item name="images" label="图片链接">
            <Input.TextArea rows={2} placeholder="多个 URL 用回车分隔" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="编辑评论"
        open={!!editingComment}
        onCancel={() => setEditingComment(null)}
        onOk={() => commentForm.submit()}
        okText="保存"
        cancelText="取消"
        destroyOnClose
      >
        <Alert type="info" showIcon style={{ marginBottom: 12 }}
          message="保存后会重新检测敏感词，命中将先进入审核并从列表临时隐藏。" />
        <Form form={commentForm} layout="vertical" onFinish={saveComment}
          initialValues={{ content: editingComment?.content }}>
          <Form.Item name="content" rules={[{ required: true, message: '请输入内容' }]}>
            <Input.TextArea rows={4} maxLength={1000} showCount />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
