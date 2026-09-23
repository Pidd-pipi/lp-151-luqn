import { useEffect, useState } from 'react'
import { Card, Space, Typography, Button, Input, List, message, Tag, Avatar, Modal, Form, Select, Popconfirm, Alert } from 'antd'
import { LikeOutlined, CommentOutlined, EyeOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useParams, useNavigate } from 'react-router-dom'
import { request } from '../api/client'
import type { Comment, PageResult, Post, Tag as TagItem } from '../types'
import { getIdentity } from '../utils/storage'

interface PostFormValues {
  title?: string
  content: string
  tags?: string[]
  images?: string[]
}

export default function PostDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [post, setPost] = useState<Post | null>(null)
  const [comments, setComments] = useState<Comment[]>([])
  const [commentText, setCommentText] = useState('')
  const [allTags, setAllTags] = useState<TagItem[]>([])
  const [editPostOpen, setEditPostOpen] = useState(false)
  const [editingComment, setEditingComment] = useState<Comment | null>(null)
  const [editPostForm] = Form.useForm<PostFormValues>()
  const [editCommentForm] = Form.useForm<{ content: string }>()
  const identity = getIdentity()

  const load = async () => {
    try {
      const postData = await request<Post>('get', `/posts/${id}`)
      setPost(postData)
      const commentData = await request<PageResult<Comment>>('get', `/posts/${id}/comments`, { page: 1, page_size: 20 })
      setComments(commentData.items)
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  useEffect(() => {
    load()
    request<TagItem[]>('get', '/tags').then(setAllTags).catch(() => {})
  }, [id])

  const likePost = async () => {
    if (!identity) {
      message.warning('请先创建匿名身份')
      return
    }
    try {
      await request('post', '/likes/toggle', { targetType: 'post', targetId: Number(id) })
      load()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const likeComment = async (commentId: number) => {
    if (!identity) {
      message.warning('请先创建匿名身份')
      return
    }
    try {
      await request('post', '/likes/toggle', { targetType: 'comment', targetId: commentId })
      load()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const submitComment = async () => {
    if (!identity) {
      message.warning('请先创建匿名身份')
      return
    }
    if (!commentText.trim()) return
    try {
      await request('post', '/comments', { postId: Number(id), content: commentText.trim() })
      setCommentText('')
      message.success('评论已发布')
      load()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const openEditPost = () => {
    if (!post) return
    editPostForm.setFieldsValue({
      title: post.title,
      content: post.content,
      tags: post.tags.map((t) => t.name),
      images: post.images || [],
    })
    setEditPostOpen(true)
  }

  const submitEditPost = async (values: PostFormValues) => {
    try {
      const data = await request<{ post: Post; blocked: boolean; hitWords: string[] }>('put', `/posts/${id}`, {
        title: values.title || '',
        content: values.content,
        images: values.images || [],
        tags: values.tags || [],
      })
      if (data.blocked) {
        message.warning(`内容命中敏感词：${data.hitWords.join('、')}，已进入审核，通过前仅自己可见`)
      } else {
        message.success('帖子已更新')
      }
      setEditPostOpen(false)
      load()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const withdrawPost = async () => {
    try {
      await request('delete', `/posts/${id}`)
      message.success('帖子已撤回')
      navigate('/')
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const openEditComment = (item: Comment) => {
    editCommentForm.setFieldsValue({ content: item.content })
    setEditingComment(item)
  }

  const submitEditComment = async ({ content }: { content: string }) => {
    if (!editingComment) return
    try {
      const data = await request<{ comment: Comment; blocked: boolean; hitWords: string[] }>(
        'put',
        `/comments/${editingComment.id}`,
        { content },
      )
      if (data.blocked) {
        message.warning(`内容命中敏感词：${data.hitWords.join('、')}，已进入审核，通过前仅自己可见`)
      } else {
        message.success('评论已更新')
      }
      setEditingComment(null)
      load()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  const withdrawComment = async (commentId: number) => {
    try {
      await request('delete', `/comments/${commentId}`)
      message.success('评论已撤回')
      load()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  if (!post) return <Typography.Text>加载中...</Typography.Text>

  const isOwner = identity !== null && identity.id === post.identityId

  return (
    <div>
      <Button type="link" onClick={() => navigate(-1)}>返回</Button>
      {post.status === 2 && (
        <Alert type="warning" showIcon message="内容命中敏感词，正在审核中，通过前仅自己可见" style={{ marginBottom: 16 }} />
      )}
      {post.status === 3 && (
        <Alert type="error" showIcon message="内容未通过审核，仅自己可见，可编辑后重新提交" style={{ marginBottom: 16 }} />
      )}
      <Card className="post-card">
        <Space align="start">
          <Avatar size={48} src={post.avatar} />
          <div>
            <Space>
              <Typography.Text strong>{post.nickname}</Typography.Text>
              <Typography.Text type="secondary">{post.createdAt}</Typography.Text>
            </Space>
            {post.title && <Typography.Title level={4} style={{ margin: '8px 0' }}>{post.title}</Typography.Title>}
            <Typography.Paragraph style={{ whiteSpace: 'pre-wrap' }}>{post.content}</Typography.Paragraph>
            {post.images?.length > 0 && (
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', margin: '8px 0' }}>
                {post.images.map((url, idx) => (
                  <img key={idx} src={url} alt={`${post.nickname}-${idx}`} style={{ width: 160, height: 160, objectFit: 'cover', borderRadius: 8 }} />
                ))}
              </div>
            )}
            {post.tags.map((tag) => (
              <Tag key={tag.id} color="blue">{tag.name}</Tag>
            ))}
            <Space size="large" style={{ marginTop: 8 }}>
              <Button type={post.liked ? 'primary' : 'text'} icon={<LikeOutlined />} onClick={likePost}>
                {post.likeCount}
              </Button>
              <Typography.Text type="secondary"><CommentOutlined /> {post.commentCount}</Typography.Text>
              <Typography.Text type="secondary"><EyeOutlined /> {post.viewCount}</Typography.Text>
              {isOwner && (
                <>
                  <Button type="text" icon={<EditOutlined />} onClick={openEditPost}>编辑</Button>
                  <Popconfirm
                    title="撤回帖子"
                    description="撤回后评论、点赞与标签关系将一起撤下，且不可恢复。确定撤回吗？"
                    okText="撤回"
                    cancelText="取消"
                    okButtonProps={{ danger: true }}
                    onConfirm={withdrawPost}
                  >
                    <Button type="text" danger icon={<DeleteOutlined />}>撤回</Button>
                  </Popconfirm>
                </>
              )}
            </Space>
          </div>
        </Space>
      </Card>

      <Card title={`评论 (${comments.length})`} style={{ marginTop: 16 }}>
        <Space.Compact style={{ width: '100%', marginBottom: 16 }}>
          <Input.TextArea value={commentText} onChange={(e) => setCommentText(e.target.value)} placeholder="写下你的匿名评论..." autoSize={{ minRows: 2, maxRows: 4 }} />
        </Space.Compact>
        <Button type="primary" onClick={submitComment} style={{ marginTop: 8 }}>发表评论</Button>
        <List
          style={{ marginTop: 16 }}
          dataSource={comments}
          locale={{ emptyText: '暂无评论' }}
          renderItem={(item) => {
            const isCommentOwner = identity !== null && identity.id === item.identityId
            const actions = [
              <Button key="like" type="text" icon={<LikeOutlined />} onClick={() => likeComment(item.id)}>
                {item.likeCount}
              </Button>,
            ]
            if (isCommentOwner) {
              actions.push(
                <Button key="edit" type="text" icon={<EditOutlined />} onClick={() => openEditComment(item)}>编辑</Button>,
                <Popconfirm
                  key="withdraw"
                  title="撤回评论"
                  description="撤回后评论不再展示。确定撤回吗？"
                  okText="撤回"
                  cancelText="取消"
                  okButtonProps={{ danger: true }}
                  onConfirm={() => withdrawComment(item.id)}
                >
                  <Button type="text" danger icon={<DeleteOutlined />}>撤回</Button>
                </Popconfirm>,
              )
            }
            return (
              <List.Item actions={actions}>
                <List.Item.Meta
                  avatar={<Avatar src={item.avatar} />}
                  title={
                    <Space>
                      <Typography.Text strong>{item.nickname}</Typography.Text>
                      <Typography.Text type="secondary">{item.createdAt}</Typography.Text>
                      {item.status === 2 && <Tag color="warning">审核中</Tag>}
                      {item.status === 3 && <Tag color="error">未通过</Tag>}
                    </Space>
                  }
                  description={<Typography.Paragraph style={{ marginBottom: 0 }}>{item.content}</Typography.Paragraph>}
                />
              </List.Item>
            )
          }}
        />
      </Card>

      <Modal
        title="编辑帖子"
        open={editPostOpen}
        okText="保存"
        cancelText="取消"
        onOk={() => editPostForm.submit()}
        onCancel={() => setEditPostOpen(false)}
        destroyOnClose
      >
        <Form form={editPostForm} layout="vertical" onFinish={submitEditPost}>
          <Form.Item name="title" label="标题">
            <Input placeholder="可选" maxLength={255} />
          </Form.Item>
          <Form.Item name="content" label="内容" rules={[{ required: true, message: '请输入内容' }]}>
            <Input.TextArea rows={5} maxLength={5000} />
          </Form.Item>
          <Form.Item name="tags" label="话题标签">
            <Select mode="tags" placeholder="选择或输入新标签" options={allTags.map((t) => ({ label: t.name, value: t.name }))} />
          </Form.Item>
          <Form.Item name="images" label="图片链接">
            <Select mode="tags" placeholder="粘贴图片 URL 后回车，最多 9 张" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="编辑评论"
        open={editingComment !== null}
        okText="保存"
        cancelText="取消"
        onOk={() => editCommentForm.submit()}
        onCancel={() => setEditingComment(null)}
        destroyOnClose
      >
        <Form form={editCommentForm} layout="vertical" onFinish={submitEditComment}>
          <Form.Item name="content" label="内容" rules={[{ required: true, message: '请输入内容' }]}>
            <Input.TextArea rows={4} maxLength={1000} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
