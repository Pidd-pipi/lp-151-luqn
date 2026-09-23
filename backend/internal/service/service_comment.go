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

var (
	ErrCommentNotFound  = errors.New("comment not found")
	ErrCommentWithdrawn = errors.New("comment has been withdrawn")
)

type CommentService interface {
	Create(identityID, postID uint, content string) (*model.Comment, []string, bool, error)
	Update(identityID, commentID uint, content string) (*model.Comment, []string, bool, error)
	Withdraw(identityID, commentID uint) error
	ListByPostID(postID uint, page, pageSize int, viewerIdentityID uint) ([]model.Comment, int64, error)
}

type commentService struct {
	comments  repository.CommentRepository
	posts     repository.PostRepository
	likes     repository.LikeRepository
	sensitive SensitiveWordService
	review    ReviewService
	logger    *slog.Logger
}

func NewCommentService(comments repository.CommentRepository, posts repository.PostRepository, likes repository.LikeRepository, sensitive SensitiveWordService, review ReviewService, logger *slog.Logger) CommentService {
	return &commentService{comments: comments, posts: posts, likes: likes, sensitive: sensitive, review: review, logger: logger}
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
		s.adjustPostCommentCount(postID, 1)
	} else {
		if err := s.review.Enqueue("comment", comment.ID, content, hits); err != nil {
			s.logger.Error("enqueue comment review", "error", err)
		}
	}
	return comment, hits, blocked, nil
}

// Update 编辑自己的评论：仅作者可操作；内容重新检测敏感词，命中则转入待审核。
func (s *commentService) Update(identityID, commentID uint, content string) (*model.Comment, []string, bool, error) {
	comment, err := s.comments.FindByID(commentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, false, ErrCommentNotFound
		}
		return nil, nil, false, err
	}
	if comment.IdentityID != identityID {
		return nil, nil, false, ErrForbidden
	}
	if comment.Status == constants.CommentStatusWithdrawn {
		return nil, nil, false, ErrCommentWithdrawn
	}
	hits, blocked := s.sensitive.Detect(content)
	wasPublished := comment.Status == constants.CommentStatusPublished
	comment.Content = content
	if blocked {
		comment.Status = constants.CommentStatusPending
	} else {
		comment.Status = constants.CommentStatusPublished
	}
	comment.UpdatedAt = time.Now()
	if err := s.comments.Update(comment); err != nil {
		return nil, hits, blocked, err
	}
	// 评论可见性变化时同步帖子评论数
	switch {
	case wasPublished && blocked:
		s.adjustPostCommentCount(comment.PostID, -1)
	case !wasPublished && !blocked:
		s.adjustPostCommentCount(comment.PostID, 1)
	}
	if blocked {
		if err := s.review.Enqueue("comment", comment.ID, content, hits); err != nil {
			s.logger.Error("enqueue comment review", "error", err)
		}
	}
	return comment, hits, blocked, nil
}

// Withdraw 撤回自己的评论：评论不再可见，帖子评论数同步减少。
func (s *commentService) Withdraw(identityID, commentID uint) error {
	comment, err := s.comments.FindByID(commentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrCommentNotFound
		}
		return err
	}
	if comment.IdentityID != identityID {
		return ErrForbidden
	}
	if comment.Status == constants.CommentStatusWithdrawn {
		return nil
	}
	wasPublished := comment.Status == constants.CommentStatusPublished
	comment.Status = constants.CommentStatusWithdrawn
	comment.UpdatedAt = time.Now()
	if err := s.comments.Update(comment); err != nil {
		return err
	}
	if err := s.likes.DeleteByTargets("comment", []uint{commentID}); err != nil {
		return err
	}
	if wasPublished {
		s.adjustPostCommentCount(comment.PostID, -1)
	}
	return nil
}

func (s *commentService) ListByPostID(postID uint, page, pageSize int, viewerIdentityID uint) ([]model.Comment, int64, error) {
	return s.comments.ListByPostID(postID, page, pageSize, viewerIdentityID)
}

// adjustPostCommentCount 调整帖子评论数，失败只记录日志不阻塞主流程。
func (s *commentService) adjustPostCommentCount(postID uint, delta int) {
	post, err := s.posts.FindByID(postID)
	if err != nil {
		s.logger.Error("find post for comment count", "error", err)
		return
	}
	post.CommentCount += delta
	if post.CommentCount < 0 {
		post.CommentCount = 0
	}
	post.UpdatedAt = time.Now()
	if err := s.posts.Update(post); err != nil {
		s.logger.Error("update post comment count", "error", err)
	}
}
