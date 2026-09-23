package constants

// 统一错误码
const (
	CodeOK           = 0
	CodeBadRequest   = 40000
	CodeUnauthorized = 40100
	CodeForbidden    = 40300
	CodeNotFound     = 40400
	CodeConflict     = 40900
	CodeInternal     = 50000
)

// 状态枚举
const (
	PostStatusPublished = 1
	PostStatusPending   = 2
	PostStatusRejected  = 3

	CommentStatusPublished = 1
	CommentStatusPending   = 2
	CommentStatusRejected  = 3

	ReviewStatusPending  = 1
	ReviewStatusApproved = 2
	ReviewStatusRejected = 3
)

// 热度计算公式常量
const (
	HeatLikeWeight     = 10
	HeatCommentWeight  = 5
	HeatDecayPerMinute = 0.001
	HeatBaseSeconds    = 1
)
