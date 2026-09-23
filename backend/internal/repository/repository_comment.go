package repository

import (
	"errors"
	"fmt"

	"github.com/gbtreehole/backend/internal/constants"
	"github.com/gbtreehole/backend/internal/model"
	"gorm.io/gorm"
)

type CommentRepository interface {
	Create(comment *model.Comment) error
	Update(comment *model.Comment) error
	FindByID(id uint) (*model.Comment, error)
	// ListVisibleByPostID 返回公开评论，另额外包含当前查看者本人未撤回的评论（审核中/被屏蔽）。
	ListVisibleByPostID(postID uint, viewerID uint, page, pageSize int) ([]model.Comment, int64, error)
	ListByIDs(ids []uint) ([]model.Comment, error)
	// WithdrawCascade 在单个事务内撤回评论、删除其点赞并取消待审核单；
	// 当评论原本处于公开状态时同步减少帖子评论数。
	WithdrawCascade(id uint) error
}

type commentRepository struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) CommentRepository {
	return &commentRepository{db: db}
}

func (r *commentRepository) Create(comment *model.Comment) error {
	if err := r.db.Create(comment).Error; err != nil {
		return fmt.Errorf("create comment: %w", err)
	}
	return nil
}

func (r *commentRepository) Update(comment *model.Comment) error {
	if err := r.db.Save(comment).Error; err != nil {
		return fmt.Errorf("update comment: %w", err)
	}
	return nil
}

func (r *commentRepository) FindByID(id uint) (*model.Comment, error) {
	var comment model.Comment
	if err := r.db.Preload("Identity").First(&comment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find comment by id: %w", err)
	}
	return &comment, nil
}

func (r *commentRepository) ListVisibleByPostID(postID uint, viewerID uint, page, pageSize int) ([]model.Comment, int64, error) {
	var comments []model.Comment
	var total int64
	q := r.db.Model(&model.Comment{}).Preload("Identity").Where("post_id = ?", postID)
	if viewerID > 0 {
		q = q.Where("status = ? OR (identity_id = ? AND status <> ?)",
			constants.CommentStatusPublished, viewerID, constants.CommentStatusWithdrawn)
	} else {
		q = q.Where("status = ?", constants.CommentStatusPublished)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count comments: %w", err)
	}
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&comments).Error; err != nil {
		return nil, 0, fmt.Errorf("list comments: %w", err)
	}
	return comments, total, nil
}

func (r *commentRepository) ListByIDs(ids []uint) ([]model.Comment, error) {
	var comments []model.Comment
	if len(ids) == 0 {
		return comments, nil
	}
	if err := r.db.Preload("Identity").Where("id IN ?", ids).Find(&comments).Error; err != nil {
		return nil, fmt.Errorf("list comments by ids: %w", err)
	}
	return comments, nil
}

// WithdrawCascade 撤回评论：标记撤回、删除点赞、取消待审核单，
// 公开评论同时把帖子的评论数减一。
func (r *commentRepository) WithdrawCascade(id uint) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var comment model.Comment
		if err := tx.First(&comment, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("find comment to withdraw: %w", err)
		}
		if comment.Status == constants.CommentStatusWithdrawn {
			return nil
		}
		wasPublished := comment.Status == constants.CommentStatusPublished

		if err := tx.Model(&model.Comment{}).Where("id = ?", id).
			Updates(map[string]any{"status": constants.CommentStatusWithdrawn, "like_count": 0}).Error; err != nil {
			return fmt.Errorf("withdraw comment: %w", err)
		}
		if err := tx.Where("target_type = ? AND target_id = ?", "comment", id).Delete(&model.Like{}).Error; err != nil {
			return fmt.Errorf("delete comment likes: %w", err)
		}
		if err := tx.Model(&model.ReviewQueue{}).
			Where("target_type = ? AND target_id = ? AND status = ?", "comment", id, constants.ReviewStatusPending).
			Update("status", constants.ReviewStatusCanceled).Error; err != nil {
			return fmt.Errorf("cancel comment review: %w", err)
		}
		if wasPublished {
			var p model.Post
			if err := tx.First(&p, comment.PostID).Error; err != nil {
				return fmt.Errorf("find post to decrement comment count: %w", err)
			}
			if p.CommentCount > 0 {
				p.CommentCount--
			}
			if err := tx.Model(&p).UpdateColumn("comment_count", p.CommentCount).Error; err != nil {
				return fmt.Errorf("decrement post comment count: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("withdraw comment cascade: %w", err)
	}
	return nil
}
