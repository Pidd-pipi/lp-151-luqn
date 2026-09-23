package repository

import (
	"errors"
	"fmt"

	"github.com/gbtreehole/backend/internal/model"
	"gorm.io/gorm"
)

type TagRepository interface {
	Create(tag *model.Tag) error
	FindByID(id uint) (*model.Tag, error)
	FindByName(name string) (*model.Tag, error)
	List() ([]model.Tag, error)
	IncrementPostCount(id uint) error
	DecrementPostCount(id uint) error
	LinkPostTag(postID, tagID uint) error
	UnlinkPostTag(postID, tagID uint) error
	ListTagsByPostID(postID uint) ([]model.Tag, error)
}

type tagRepository struct {
	db *gorm.DB
}

func NewTagRepository(db *gorm.DB) TagRepository {
	return &tagRepository{db: db}
}

func (r *tagRepository) Create(tag *model.Tag) error {
	if err := r.db.Create(tag).Error; err != nil {
		return fmt.Errorf("create tag: %w", err)
	}
	return nil
}

func (r *tagRepository) FindByID(id uint) (*model.Tag, error) {
	var tag model.Tag
	if err := r.db.First(&tag, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find tag by id: %w", err)
	}
	return &tag, nil
}

func (r *tagRepository) FindByName(name string) (*model.Tag, error) {
	var tag model.Tag
	if err := r.db.Where("name = ?", name).First(&tag).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find tag by name: %w", err)
	}
	return &tag, nil
}

func (r *tagRepository) List() ([]model.Tag, error) {
	var tags []model.Tag
	if err := r.db.Order("post_count DESC").Find(&tags).Error; err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	return tags, nil
}

func (r *tagRepository) IncrementPostCount(id uint) error {
	if err := r.db.Model(&model.Tag{}).Where("id = ?", id).UpdateColumn("post_count", gorm.Expr("post_count + 1")).Error; err != nil {
		return fmt.Errorf("increment tag post count: %w", err)
	}
	return nil
}

func (r *tagRepository) DecrementPostCount(id uint) error {
	if err := r.db.Model(&model.Tag{}).Where("id = ?", id).UpdateColumn("post_count", gorm.Expr("GREATEST(post_count - 1, 0)")).Error; err != nil {
		return fmt.Errorf("decrement tag post count: %w", err)
	}
	return nil
}

func (r *tagRepository) LinkPostTag(postID, tagID uint) error {
	pt := &model.PostTag{PostID: postID, TagID: tagID}
	if err := r.db.Create(pt).Error; err != nil {
		return fmt.Errorf("link post tag: %w", err)
	}
	return nil
}

func (r *tagRepository) UnlinkPostTag(postID, tagID uint) error {
	if err := r.db.Where("post_id = ? AND tag_id = ?", postID, tagID).Delete(&model.PostTag{}).Error; err != nil {
		return fmt.Errorf("unlink post tag: %w", err)
	}
	return nil
}

func (r *tagRepository) ListTagsByPostID(postID uint) ([]model.Tag, error) {
	var tags []model.Tag
	if err := r.db.Joins("JOIN post_tags ON post_tags.tag_id = tags.id AND post_tags.post_id = ?", postID).Find(&tags).Error; err != nil {
		return nil, fmt.Errorf("list tags by post id: %w", err)
	}
	return tags, nil
}
