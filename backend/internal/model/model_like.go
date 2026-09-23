package model

import "time"

type Like struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	IdentityID uint      `gorm:"index;not null" json:"identityId"`
	TargetType string    `gorm:"type:varchar(16);not null" json:"targetType"` // post / comment
	TargetID   uint      `gorm:"index;not null" json:"targetId"`
	CreatedAt  time.Time `json:"createdAt"`
}
