package model

import "time"

type ReviewQueue struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TargetType string    `gorm:"type:varchar(16);not null;index" json:"targetType"` // post / comment
	TargetID   uint      `gorm:"index;not null" json:"targetId"`
	Content    string    `gorm:"type:text" json:"content"`
	Status     int       `gorm:"default:1;index" json:"status"`
	HitWords   string    `gorm:"type:varchar(255)" json:"hitWords"`
	ReviewedBy *uint     `json:"reviewedBy,omitempty"`
	ReviewNote string    `gorm:"type:varchar(255)" json:"reviewNote"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
