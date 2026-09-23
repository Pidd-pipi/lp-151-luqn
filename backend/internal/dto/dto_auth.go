package dto

type CreateIdentityRequest struct {
	Nickname string `json:"nickname" validate:"omitempty,max=32"`
	Avatar   string `json:"avatar" validate:"omitempty,max=255"`
}

type BindIdentityRequest struct {
	IdentityKey string `json:"identityKey" validate:"required,max=64"`
}

type IdentityResponse struct {
	ID          uint   `json:"id"`
	IdentityKey string `json:"identityKey"`
	Nickname    string `json:"nickname"`
	Avatar      string `json:"avatar"`
	CreatedAt   string `json:"createdAt"`
}

type TokenResponse struct {
	Token string `json:"token"`
}
