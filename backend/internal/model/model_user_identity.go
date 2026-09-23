package model

import "time"

type UserIdentity struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	IdentityKey string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"identityKey"`
	Nickname    string    `gorm:"type:varchar(64);not null" json:"nickname"`
	Avatar      string    `gorm:"type:varchar(255)" json:"avatar"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
