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
		comment.Status = constants.CommentStatusPublished
		comment.UpdatedAt = time.Now()
		return s.comments.Update(comment)
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
		comment.Status = constants.CommentStatusRejected
		comment.UpdatedAt = time.Now()
		return s.comments.Update(comment)
	default:
		return fmt.Errorf("unknown target type: %s", targetType)
	}
}
