package repository

import (
	"errors"
	"fmt"

	"github.com/gbtreehole/backend/internal/constants"
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
	AdjustCommentCount(id uint, delta int) error
	// WithdrawCascade 在单个事务内撤下帖子及其评论、点赞、标签关系，
	// 并取消尚未处理的审核单。
	WithdrawCascade(id uint) error
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

func (r *postRepository) AdjustCommentCount(id uint, delta int) error {
	switch {
	case delta > 0:
		if err := r.db.Model(&model.Post{}).Where("id = ?", id).
			UpdateColumn("comment_count", gorm.Expr("comment_count + ?", delta)).Error; err != nil {
			return fmt.Errorf("adjust comment count: %w", err)
		}
	case delta < 0:
		// 先读后写，避免使用 MySQL 方言函数（兼容测试用 SQLite）。
		var post model.Post
		if err := r.db.First(&post, id).Error; err != nil {
			return fmt.Errorf("find post for comment count: %w", err)
		}
		if post.CommentCount >= -delta {
			post.CommentCount += delta
		} else {
			post.CommentCount = 0
		}
		if err := r.db.Model(&post).UpdateColumn("comment_count", post.CommentCount).Error; err != nil {
			return fmt.Errorf("adjust comment count: %w", err)
		}
	}
	return nil
}

// WithdrawCascade 撤帖级联：
//  1. 帖子标记为撤回，点赞/评论计数清零；
//  2. 所有未撤回的子评论标记为撤回；
//  3. 删除帖子与评论的点赞关系；
//  4. 删除帖子标签关系并回退标签计数；
//  5. 取消该帖子/子评论待审核的审核单。
func (r *postRepository) WithdrawCascade(id uint) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var post model.Post
		if err := tx.First(&post, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("find post to withdraw: %w", err)
		}
		if post.Status == constants.PostStatusWithdrawn {
			return nil
		}

		var tagIDs []uint
		if err := tx.Model(&model.PostTag{}).Where("post_id = ?", id).Pluck("tag_id", &tagIDs).Error; err != nil {
			return fmt.Errorf("pluck post tags: %w", err)
		}
		var commentIDs []uint
		if err := tx.Model(&model.Comment{}).
			Where("post_id = ? AND status <> ?", id, constants.CommentStatusWithdrawn).
			Pluck("id", &commentIDs).Error; err != nil {
			return fmt.Errorf("pluck post comments: %w", err)
		}

		if err := tx.Model(&model.Post{}).Where("id = ?", id).
			Updates(map[string]any{
				"status":        constants.PostStatusWithdrawn,
				"like_count":    0,
				"comment_count": 0,
			}).Error; err != nil {
			return fmt.Errorf("withdraw post: %w", err)
		}
		if err := tx.Model(&model.Comment{}).
			Where("post_id = ? AND status <> ?", id, constants.CommentStatusWithdrawn).
			Update("status", constants.CommentStatusWithdrawn).Error; err != nil {
			return fmt.Errorf("withdraw comments: %w", err)
		}
		if err := tx.Where("target_type = ? AND target_id = ?", "post", id).Delete(&model.Like{}).Error; err != nil {
			return fmt.Errorf("delete post likes: %w", err)
		}
		if len(commentIDs) > 0 {
			if err := tx.Where("target_type = ? AND target_id IN ?", "comment", commentIDs).Delete(&model.Like{}).Error; err != nil {
				return fmt.Errorf("delete comment likes: %w", err)
			}
		}
		if err := tx.Where("post_id = ?", id).Delete(&model.PostTag{}).Error; err != nil {
			return fmt.Errorf("delete post tags: %w", err)
		}
		for _, tagID := range tagIDs {
			var tag model.Tag
			if err := tx.First(&tag, tagID).Error; err != nil {
				return fmt.Errorf("find tag to decrement: %w", err)
			}
			if tag.PostCount > 0 {
				tag.PostCount--
			}
			if err := tx.Model(&tag).UpdateColumn("post_count", tag.PostCount).Error; err != nil {
				return fmt.Errorf("decrement tag count: %w", err)
			}
		}
		if err := tx.Model(&model.ReviewQueue{}).
			Where("target_type = ? AND target_id = ? AND status = ?", "post", id, constants.ReviewStatusPending).
			Update("status", constants.ReviewStatusCanceled).Error; err != nil {
			return fmt.Errorf("cancel post review: %w", err)
		}
		if len(commentIDs) > 0 {
			if err := tx.Model(&model.ReviewQueue{}).
				Where("target_type = ? AND target_id IN ? AND status = ?", "comment", commentIDs, constants.ReviewStatusPending).
				Update("status", constants.ReviewStatusCanceled).Error; err != nil {
				return fmt.Errorf("cancel comment reviews: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("withdraw post cascade: %w", err)
	}
	return nil
}
