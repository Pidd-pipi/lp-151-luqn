export interface Identity {
  id: number
  identityKey: string
  nickname: string
  avatar: string
  createdAt: string
}

export interface Tag {
  id: number
  name: string
  postCount: number
}

// 内容状态：1 已发布 2 审核中 3 已屏蔽 4 已撤回
export type ContentStatus = 1 | 2 | 3 | 4
// 审核单状态：1 待审核 2 已放行 3 已拒绝 4 已取消
export type ReviewStatus = 0 | 1 | 2 | 3 | 4

export interface Post {
  id: number
  identityId: number
  nickname: string
  avatar: string
  title: string
  content: string
  images: string[]
  status: ContentStatus
  likeCount: number
  commentCount: number
  viewCount: number
  isFeatured: boolean
  liked: boolean
  tags: Tag[]
  reviewStatus: ReviewStatus
  reviewNote: string
  hitWords: string
  createdAt: string
  updatedAt: string
}

export interface Comment {
  id: number
  postId: number
  identityId: number
  nickname: string
  avatar: string
  content: string
  status: ContentStatus
  likeCount: number
  liked: boolean
  reviewStatus: ReviewStatus
  reviewNote: string
  hitWords: string
  createdAt: string
  updatedAt: string
}

export interface ReviewItem {
  id: number
  targetType: string
  targetId: number
  content: string
  status: number
  hitWords: string
  reviewNote: string
  createdAt: string
}

export interface PageResult<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}
