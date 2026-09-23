package model

import "time"

type Tag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"name"`
	PostCount int       `gorm:"default:0" json:"postCount"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type PostTag struct {
	PostID uint `gorm:"primaryKey" json:"postId"`
	TagID  uint `gorm:"primaryKey" json:"tagId"`
}
