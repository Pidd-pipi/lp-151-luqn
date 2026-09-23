package model

import "time"

type Post struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	IdentityID    uint      `gorm:"index;not null" json:"identityId"`
	Identity      *UserIdentity `gorm:"foreignKey:IdentityID" json:"identity,omitempty"`
	Title         string    `gorm:"type:varchar(255)" json:"title"`
	Content       string    `gorm:"type:text;not null" json:"content"`
	Images        string    `gorm:"type:text" json:"images"`
	Status        int       `gorm:"default:1;index" json:"status"`
	LikeCount     int       `gorm:"default:0" json:"likeCount"`
	CommentCount  int       `gorm:"default:0" json:"commentCount"`
	ViewCount     int       `gorm:"default:0" json:"viewCount"`
	IsFeatured    bool      `gorm:"default:false;index" json:"isFeatured"`
	FeaturedAt    *time.Time `json:"featuredAt,omitempty"`
	ReviewedBy    *uint     `json:"reviewedBy,omitempty"`
	ReviewRemark  string    `gorm:"type:varchar(255)" json:"reviewRemark"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	Tags          []Tag     `gorm:"many2many:post_tags;" json:"tags,omitempty"`
}
