import { Tag, Tooltip, Typography } from 'antd'
import { ClockCircleOutlined, StopOutlined, UndoOutlined } from '@ant-design/icons'
import type { ContentStatus, ReviewStatus } from '../types'

// 作者视角的内容处理状态徽标：撤回、审核中、被屏蔽。已发布不展示。
export function ContentStatusBadge({ status }: { status: ContentStatus }) {
  switch (status) {
    case 2:
      return (
        <Tooltip title="内容命中敏感词，管理员放行后将重新公开">
          <Tag icon={<ClockCircleOutlined />} color="processing">审核中</Tag>
        </Tooltip>
      )
    case 3:
      return (
        <Tooltip title="内容未通过审核，仅你自己可见">
          <Tag icon={<StopOutlined />} color="error">已屏蔽</Tag>
        </Tooltip>
      )
    case 4:
      return <Tag icon={<UndoOutlined />} color="default">已撤回</Tag>
    default:
      return null
  }
}

// 审核处理进度：在内容仍公开（编辑重审中除外）时不展示。
export function ReviewProgressTag({ reviewStatus, reviewNote, hitWords }: {
  reviewStatus: ReviewStatus
  reviewNote?: string
  hitWords?: string
}) {
  if (!reviewStatus) return null
  const note = [hitWords ? `命中：${hitWords}` : '', reviewNote || ''].filter(Boolean).join('；')
  switch (reviewStatus) {
    case 1:
      return <Tooltip title={note || '等待管理员处理'}><Tag color="orange">待审核</Tag></Tooltip>
    case 2:
      return <Tooltip title={note || '已放行'}><Tag color="success">审核放行</Tag></Tooltip>
    case 3:
      return <Tooltip title={note || '未通过审核'}><Tag color="error">审核拒绝</Tag></Tooltip>
    case 4:
      return <Tooltip title={note || '因内容更新自动取消'}><Tag>审核已取消</Tag></Tooltip>
    default:
      return null
  }
}

// 审核中/被屏蔽内容在作者视角的提示语。
export function OwnerNotice({ status, reviewStatus, reviewNote }: {
  status: ContentStatus
  reviewStatus: ReviewStatus
  reviewNote?: string
}) {
  if (status === 4) {
    return (
      <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
        该内容已撤回，列表与详情对其他人不再展示。
      </Typography.Paragraph>
    )
  }
  if (status === 2) {
    return (
      <Typography.Paragraph type="warning" style={{ marginBottom: 8 }}>
        内容命中敏感词，已进入审核队列，当前仅你自己可见。{reviewStatus === 1 ? '请耐心等待管理员处理。' : ''}
      </Typography.Paragraph>
    )
  }
  if (status === 3) {
    return (
      <Typography.Paragraph type="danger" style={{ marginBottom: 8 }}>
        该内容未通过审核{reviewNote ? `：${reviewNote}` : ''}，当前仅你自己可见。你可以修改内容后重新提交。
      </Typography.Paragraph>
    )
  }
  return null
}
