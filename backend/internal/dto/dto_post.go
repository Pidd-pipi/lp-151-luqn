package dto

type CreatePostRequest struct {
	Title   string   `json:"title" validate:"omitempty,max=255"`
	Content string   `json:"content" validate:"required,min=1,max=5000"`
	Images  []string `json:"images" validate:"omitempty,max=9,dive,max=500"`
	Tags    []string `json:"tags" validate:"omitempty,max=10,dive,max=32"`
}

// UpdatePostRequest 作者编辑帖子，仅标题/正文/图片可改，标签沿用创建时选择。
type UpdatePostRequest struct {
	Title   string   `json:"title" validate:"omitempty,max=255"`
	Content string   `json:"content" validate:"required,min=1,max=5000"`
	Images  []string `json:"images" validate:"omitempty,max=9,dive,max=500"`
}

type ListPostRequest struct {
	Page     int  `json:"page" form:"page" validate:"omitempty,min=1"`
	PageSize int  `json:"pageSize" form:"page_size" validate:"omitempty,min=1,max=100"`
	TagID    uint `json:"tagId" form:"tag_id"`
	Featured bool `json:"featured" form:"featured"`
}

type PostResponse struct {
	ID           uint          `json:"id"`
	IdentityID   uint          `json:"identityId"`
	Nickname     string        `json:"nickname"`
	Avatar       string        `json:"avatar"`
	Title        string        `json:"title"`
	Content      string        `json:"content"`
	Images       []string      `json:"images"`
	Status       int           `json:"status"`
	LikeCount    int           `json:"likeCount"`
	CommentCount int           `json:"commentCount"`
	ViewCount    int           `json:"viewCount"`
	IsFeatured   bool          `json:"isFeatured"`
	Liked        bool          `json:"liked"`
	Tags         []TagResponse `json:"tags"`
	// ReviewStatus 最近一次审核单状态：1待审 2放行 3拒绝 4已取消；0表示无审核记录。
	ReviewStatus int    `json:"reviewStatus"`
	ReviewNote   string `json:"reviewNote"`
	HitWords     string `json:"hitWords"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type PageResult struct {
	Items    any   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}
