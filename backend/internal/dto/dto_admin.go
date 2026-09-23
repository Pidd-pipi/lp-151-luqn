package dto

type ReviewRequest struct {
	QueueID uint   `json:"queueId" validate:"required"`
	Action  string `json:"action" validate:"required,oneof=approve reject"`
	Note    string `json:"note" validate:"omitempty,max=255"`
}

type CreateSensitiveWordRequest struct {
	Word string `json:"word" validate:"required,min=1,max=128"`
}

type FeatureRequest struct {
	PostID   uint `json:"postId" validate:"required"`
	Featured bool `json:"featured"`
}

type ReviewItemResponse struct {
	ID         uint   `json:"id"`
	TargetType string `json:"targetType"`
	TargetID   uint   `json:"targetId"`
	Content    string `json:"content"`
	Status     int    `json:"status"`
	HitWords   string `json:"hitWords"`
	ReviewNote string `json:"reviewNote"`
	CreatedAt  string `json:"createdAt"`
}

type ListReviewRequest struct {
	Page     int `json:"page" form:"page" validate:"omitempty,min=1"`
	PageSize int `json:"pageSize" form:"page_size" validate:"omitempty,min=1,max=100"`
	Status   int `json:"status" form:"status" validate:"omitempty,oneof=1 2 3"`
}
