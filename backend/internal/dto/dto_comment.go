package dto

type CreateCommentRequest struct {
	PostID  uint   `json:"postId" validate:"required"`
	Content string `json:"content" validate:"required,min=1,max=1000"`
}

// UpdateCommentRequest 作者编辑评论内容。
type UpdateCommentRequest struct {
	Content string `json:"content" validate:"required,min=1,max=1000"`
}

type ListCommentRequest struct {
	Page     int `json:"page" form:"page" validate:"omitempty,min=1"`
	PageSize int `json:"pageSize" form:"page_size" validate:"omitempty,min=1,max=100"`
}

type CommentResponse struct {
	ID           uint   `json:"id"`
	PostID       uint   `json:"postId"`
	IdentityID   uint   `json:"identityId"`
	Nickname     string `json:"nickname"`
	Avatar       string `json:"avatar"`
	Content      string `json:"content"`
	Status       int    `json:"status"`
	LikeCount    int    `json:"likeCount"`
	Liked        bool   `json:"liked"`
	ReviewStatus int    `json:"reviewStatus"`
	ReviewNote   string `json:"reviewNote"`
	HitWords     string `json:"hitWords"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}
