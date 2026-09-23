package repository

import (
	"errors"
	"fmt"

	"github.com/gbtreehole/backend/internal/model"
	"gorm.io/gorm"
)

type LikeRepository interface {
	Create(like *model.Like) error
	Find(identityID uint, targetType string, targetID uint) (*model.Like, error)
	Delete(id uint) error
	CountByTarget(targetType string, targetID uint) (int64, error)
	IsLiked(identityID uint, targetType string, targetIDs []uint) (map[uint]bool, error)
}

type likeRepository struct {
	db *gorm.DB
}

func NewLikeRepository(db *gorm.DB) LikeRepository {
	return &likeRepository{db: db}
}

func (r *likeRepository) Create(like *model.Like) error {
	if err := r.db.Create(like).Error; err != nil {
		return fmt.Errorf("create like: %w", err)
	}
	return nil
}

func (r *likeRepository) Find(identityID uint, targetType string, targetID uint) (*model.Like, error) {
	var like model.Like
	if err := r.db.Where("identity_id = ? AND target_type = ? AND target_id = ?", identityID, targetType, targetID).First(&like).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find like: %w", err)
	}
	return &like, nil
}

func (r *likeRepository) Delete(id uint) error {
	if err := r.db.Delete(&model.Like{}, id).Error; err != nil {
		return fmt.Errorf("delete like: %w", err)
	}
	return nil
}

func (r *likeRepository) CountByTarget(targetType string, targetID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&model.Like{}).Where("target_type = ? AND target_id = ?", targetType, targetID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count like: %w", err)
	}
	return count, nil
}

func (r *likeRepository) IsLiked(identityID uint, targetType string, targetIDs []uint) (map[uint]bool, error) {
	result := make(map[uint]bool)
	if len(targetIDs) == 0 {
		return result, nil
	}
	var likes []model.Like
	if err := r.db.Where("identity_id = ? AND target_type = ? AND target_id IN ?", identityID, targetType, targetIDs).Find(&likes).Error; err != nil {
		return nil, fmt.Errorf("list likes: %w", err)
	}
	for _, l := range likes {
		result[l.TargetID] = true
	}
	return result, nil
}
