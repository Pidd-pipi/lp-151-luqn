package dto

type LikeRequest struct {
	TargetType string `json:"targetType" validate:"required,oneof=post comment"`
	TargetID   uint   `json:"targetId" validate:"required"`
}

type LikeResponse struct {
	Liked     bool  `json:"liked"`
	LikeCount int64 `json:"likeCount"`
}
