package service

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gbtreehole/backend/internal/constants"
	"github.com/gbtreehole/backend/internal/model"
	"github.com/gbtreehole/backend/internal/repository"
)

type ReviewService interface {
	Enqueue(targetType string, targetID uint, content string, hitWords []string) error
	// Resubmit 编辑后重新送审：有待审单则更新内容与命中词，否则新建。
	Resubmit(targetType string, targetID uint, content string, hitWords []string) error
	// CancelPending 取消某内容未处理的审核单（编辑后已不含敏感词 / 撤回时使用）。
	CancelPending(targetType string, targetID uint) error
	// LatestReviews 查询每个内容最新的审核单，供作者查看处理状态。
	LatestReviews(targetType string, targetIDs []uint) (map[uint]model.ReviewQueue, error)
	List(page, pageSize int, status int) ([]model.ReviewQueue, int64, error)
	Approve(queueID uint, adminID uint, note string) error
	Reject(queueID uint, adminID uint, note string) error
}

type reviewService struct {
	queue    repository.ReviewQueueRepository
	posts    repository.PostRepository
	comments repository.CommentRepository
	logger   *slog.Logger
}

func NewReviewService(queue repository.ReviewQueueRepository, posts repository.PostRepository, comments repository.CommentRepository, logger *slog.Logger) ReviewService {
	return &reviewService{queue: queue, posts: posts, comments: comments, logger: logger}
}

func (s *reviewService) Enqueue(targetType string, targetID uint, content string, hitWords []string) error {
	item := &model.ReviewQueue{
		TargetType: targetType,
		TargetID:   targetID,
		Content:    content,
		Status:     constants.ReviewStatusPending,
		HitWords:   strings.Join(hitWords, ","),
	}
	if err := s.queue.Create(item); err != nil {
		return err
	}
	return nil
}

func (s *reviewService) Resubmit(targetType string, targetID uint, content string, hitWords []string) error {
	if pending, err := s.queue.FindPending(targetType, targetID); err == nil {
		pending.Content = content
		pending.HitWords = strings.Join(hitWords, ",")
		pending.ReviewedBy = nil
		pending.ReviewNote = ""
		pending.UpdatedAt = time.Now()
		return s.queue.Update(pending)
	} else if !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	return s.Enqueue(targetType, targetID, content, hitWords)
}

func (s *reviewService) CancelPending(targetType string, targetID uint) error {
	pending, err := s.queue.FindPending(targetType, targetID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return err
	}
	pending.Status = constants.ReviewStatusCanceled
	pending.ReviewNote = "内容已更新，审核自动取消"
	pending.UpdatedAt = time.Now()
	return s.queue.Update(pending)
}

func (s *reviewService) LatestReviews(targetType string, targetIDs []uint) (map[uint]model.ReviewQueue, error) {
	return s.queue.LatestByTargets(targetType, targetIDs)
}

func (s *reviewService) List(page, pageSize int, status int) ([]model.ReviewQueue, int64, error) {
	return s.queue.List(page, pageSize, status)
}

func (s *reviewService) Approve(queueID uint, adminID uint, note string) error {
	item, err := s.queue.FindByID(queueID)
	if err != nil {
		return err
	}
	if item.Status != constants.ReviewStatusPending {
		return fmt.Errorf("review item not pending")
	}
	if err := s.approveTarget(item.TargetType, item.TargetID); err != nil {
		return err
	}
	item.Status = constants.ReviewStatusApproved
	item.ReviewedBy = &adminID
	item.ReviewNote = note
	item.UpdatedAt = time.Now()
	if err := s.queue.Update(item); err != nil {
		return err
	}
	return nil
}

func (s *reviewService) Reject(queueID uint, adminID uint, note string) error {
	item, err := s.queue.FindByID(queueID)
	if err != nil {
		return err
	}
	if item.Status != constants.ReviewStatusPending {
		return fmt.Errorf("review item not pending")
	}
	if err := s.rejectTarget(item.TargetType, item.TargetID); err != nil {
		return err
	}
	item.Status = constants.ReviewStatusRejected
	item.ReviewedBy = &adminID
	item.ReviewNote = note
	item.UpdatedAt = time.Now()
	if err := s.queue.Update(item); err != nil {
		return err
	}
	return nil
}

func (s *reviewService) approveTarget(targetType string, targetID uint) error {
	switch targetType {
	case "post":
		post, err := s.posts.FindByID(targetID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil
			}
			return err
		}
		// 撤回的内容不再随审核改变状态
		if post.Status == constants.PostStatusWithdrawn {
			return nil
		}
		post.Status = constants.PostStatusPublished
		post.UpdatedAt = time.Now()
		return s.posts.Update(post)
	case "comment":
		comment, err := s.comments.FindByID(targetID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil
			}
			return err
		}
		if comment.Status == constants.CommentStatusWithdrawn {
			return nil
		}
		wasNotPublished := comment.Status != constants.CommentStatusPublished
		comment.Status = constants.CommentStatusPublished
		comment.UpdatedAt = time.Now()
		if err := s.comments.Update(comment); err != nil {
			return err
		}
		if wasNotPublished {
			if err := s.posts.AdjustCommentCount(comment.PostID, 1); err != nil {
				s.logger.Error("increment post comment count on approve", "error", err)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown target type: %s", targetType)
	}
}

func (s *reviewService) rejectTarget(targetType string, targetID uint) error {
	switch targetType {
	case "post":
		post, err := s.posts.FindByID(targetID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil
			}
			return err
		}
		if post.Status == constants.PostStatusWithdrawn {
			return nil
		}
		post.Status = constants.PostStatusRejected
		post.UpdatedAt = time.Now()
		return s.posts.Update(post)
	case "comment":
		comment, err := s.comments.FindByID(targetID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil
			}
			return err
		}
		if comment.Status == constants.CommentStatusWithdrawn {
			return nil
		}
		// 从公开状态屏蔽时同步扣减帖子评论数
		if comment.Status == constants.CommentStatusPublished {
			if err := s.posts.AdjustCommentCount(comment.PostID, -1); err != nil {
				s.logger.Error("decrement post comment count on reject", "error", err)
			}
		}
		comment.Status = constants.CommentStatusRejected
		comment.UpdatedAt = time.Now()
		return s.comments.Update(comment)
	default:
		return fmt.Errorf("unknown target type: %s", targetType)
	}
}
