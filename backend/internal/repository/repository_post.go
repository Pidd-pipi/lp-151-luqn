package repository

import (
	"errors"
	"fmt"

	"github.com/gbtreehole/backend/internal/model"
	"gorm.io/gorm"
)

type PostRepository interface {
	Create(post *model.Post) error
	Update(post *model.Post) error
	FindByID(id uint) (*model.Post, error)
	List(page, pageSize int, status int, featured bool, tagID uint) ([]model.Post, int64, error)
	ListByIDs(ids []uint) ([]model.Post, error)
	ListHot(limit int) ([]model.Post, error)
	ListFeatured(limit int) ([]model.Post, error)
	IncrementView(id uint) error
}

type postRepository struct {
	db *gorm.DB
}

func NewPostRepository(db *gorm.DB) PostRepository {
	return &postRepository{db: db}
}

func (r *postRepository) Create(post *model.Post) error {
	if err := r.db.Create(post).Error; err != nil {
		return fmt.Errorf("create post: %w", err)
	}
	return nil
}

func (r *postRepository) Update(post *model.Post) error {
	if err := r.db.Save(post).Error; err != nil {
		return fmt.Errorf("update post: %w", err)
	}
	return nil
}

func (r *postRepository) FindByID(id uint) (*model.Post, error) {
	var post model.Post
	if err := r.db.Preload("Identity").Preload("Tags").First(&post, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find post by id: %w", err)
	}
	return &post, nil
}

func (r *postRepository) List(page, pageSize int, status int, featured bool, tagID uint) ([]model.Post, int64, error) {
	var posts []model.Post
	var total int64
	q := r.db.Model(&model.Post{}).Preload("Identity").Preload("Tags")
	if status > 0 {
		q = q.Where("status = ?", status)
	}
	if featured {
		q = q.Where("is_featured = ?", true)
	}
	if tagID > 0 {
		q = q.Joins("JOIN post_tags ON post_tags.post_id = posts.id AND post_tags.tag_id = ?", tagID)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count posts: %w", err)
	}
	if err := q.Order("posts.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&posts).Error; err != nil {
		return nil, 0, fmt.Errorf("list posts: %w", err)
	}
	return posts, total, nil
}

func (r *postRepository) ListByIDs(ids []uint) ([]model.Post, error) {
	var posts []model.Post
	if len(ids) == 0 {
		return posts, nil
	}
	if err := r.db.Preload("Identity").Preload("Tags").Where("id IN ?", ids).Find(&posts).Error; err != nil {
		return nil, fmt.Errorf("list posts by ids: %w", err)
	}
	return posts, nil
}

func (r *postRepository) ListHot(limit int) ([]model.Post, error) {
	var posts []model.Post
	// 按热度分值（点赞*10 + 评论*5 - 时间衰减）降序
	if err := r.db.Preload("Identity").Preload("Tags").Where("status = ?", 1).
		Order("(like_count * 10 + comment_count * 5 - TIMESTAMPDIFF(MINUTE, created_at, NOW()) * 0.001) DESC").
		Limit(limit).Find(&posts).Error; err != nil {
		return nil, fmt.Errorf("list hot posts: %w", err)
	}
	return posts, nil
}

func (r *postRepository) ListFeatured(limit int) ([]model.Post, error) {
	var posts []model.Post
	if err := r.db.Preload("Identity").Preload("Tags").Where("status = ? AND is_featured = ?", 1, true).
		Order("featured_at DESC").Limit(limit).Find(&posts).Error; err != nil {
		return nil, fmt.Errorf("list featured posts: %w", err)
	}
	return posts, nil
}

func (r *postRepository) IncrementView(id uint) error {
	if err := r.db.Model(&model.Post{}).Where("id = ?", id).UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error; err != nil {
		return fmt.Errorf("increment view: %w", err)
	}
	return nil
}
