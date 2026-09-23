package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gbtreehole/backend/internal/constants"
	"github.com/gbtreehole/backend/internal/model"
	"github.com/gbtreehole/backend/internal/repository"
)

var ErrCommentNotFound = errors.New("comment not found")

type CommentService interface {
	Create(identityID, postID uint, content string) (*model.Comment, []string, bool, error)
	ListByPostID(postID uint, page, pageSize int) ([]model.Comment, int64, error)
}

type commentService struct {
	comments  repository.CommentRepository
	posts     repository.PostRepository
	sensitive SensitiveWordService
	review    ReviewService
	logger    *slog.Logger
}

func NewCommentService(comments repository.CommentRepository, posts repository.PostRepository, sensitive SensitiveWordService, review ReviewService, logger *slog.Logger) CommentService {
	return &commentService{comments: comments, posts: posts, sensitive: sensitive, review: review, logger: logger}
}

func (s *commentService) Create(identityID, postID uint, content string) (*model.Comment, []string, bool, error) {
	post, err := s.posts.FindByID(postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, false, ErrPostNotFound
		}
		return nil, nil, false, err
	}
	if post.Status != constants.PostStatusPublished {
		return nil, nil, false, fmt.Errorf("post not published")
	}
	hits, blocked := s.sensitive.Detect(content)
	comment := &model.Comment{
		PostID:     postID,
		IdentityID: identityID,
		Content:    content,
		Status:     constants.CommentStatusPublished,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if blocked {
		comment.Status = constants.CommentStatusPending
	}
	if err := s.comments.Create(comment); err != nil {
		return nil, hits, blocked, err
	}
	if !blocked {
		post.CommentCount++
		post.UpdatedAt = time.Now()
		if err := s.posts.Update(post); err != nil {
			s.logger.Error("update post comment count", "error", err)
		}
	} else {
		if err := s.review.Enqueue("comment", comment.ID, content, hits); err != nil {
			s.logger.Error("enqueue comment review", "error", err)
		}
	}
	return comment, hits, blocked, nil
}

func (s *commentService) ListByPostID(postID uint, page, pageSize int) ([]model.Comment, int64, error) {
	return s.comments.ListByPostID(postID, page, pageSize, constants.CommentStatusPublished)
}
