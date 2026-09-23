package model

import "time"

type SensitiveWord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Word      string    `gorm:"type:varchar(128);uniqueIndex;not null" json:"word"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
