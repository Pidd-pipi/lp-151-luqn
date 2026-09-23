package dto

type TagResponse struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	PostCount int    `json:"postCount"`
}

type CreateTagRequest struct {
	Name string `json:"name" validate:"required,min=1,max=32"`
}
