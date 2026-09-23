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
	ErrCommentNotFound       = errors.New("comment not found")
	ErrCommentAlreadyDeleted = errors.New("comment already withdrawn")
)

type CommentService interface {
	Create(identityID, postID uint, content string) (*model.Comment, []string, bool, error)
	// ListVisibleByPostID 返回公开评论以及查看者本人未撤回的评论。
	ListVisibleByPostID(postID, viewerID uint, page, pageSize int) ([]model.Comment, int64, error)
	// Update 作者编辑评论，重新检测敏感词，命中则转入审核。
	Update(identityID, commentID uint, content string) (*model.Comment, []string, bool, error)
	// Withdraw 作者撤回评论，同步减少帖子评论数。
	Withdraw(identityID, commentID uint) error
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
		return nil, nil, false, ErrPostNotVisible
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
		if err := s.posts.AdjustCommentCount(postID, 1); err != nil {
			s.logger.Error("update post comment count", "error", err)
		}
	} else {
		if err := s.review.Enqueue("comment", comment.ID, content, hits); err != nil {
			s.logger.Error("enqueue comment review", "error", err)
		}
	}
	return comment, hits, blocked, nil
}

func (s *commentService) ListVisibleByPostID(postID, viewerID uint, page, pageSize int) ([]model.Comment, int64, error) {
	// 撤回帖的评论对所有人关闭；非公开帖仅作者本人可看自己帖子下的评论。
	post, err := s.posts.FindByID(postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, 0, ErrPostNotFound
		}
		return nil, 0, err
	}
	if post.Status == constants.PostStatusWithdrawn {
		return nil, 0, ErrPostNotVisible
	}
	if post.Status != constants.PostStatusPublished && post.IdentityID != viewerID {
		return nil, 0, ErrPostNotVisible
	}
	return s.comments.ListVisibleByPostID(postID, viewerID, page, pageSize)
}

// Update 编辑评论：仅作者可操作；重新检测敏感词。
// 公开内容命中敏感词时下架待审并扣减帖子评论数；
// 待审内容改为不含敏感词时恢复公开并补回评论数；旧审核单随之取消或复用。
func (s *commentService) Update(identityID, commentID uint, content string) (*model.Comment, []string, bool, error) {
	comment, err := s.comments.FindByID(commentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, false, ErrCommentNotFound
		}
		return nil, nil, false, err
	}
	if comment.IdentityID != identityID {
		return nil, nil, false, ErrOperationForbidden
	}
	if comment.Status == constants.CommentStatusWithdrawn {
		return nil, nil, false, ErrCommentAlreadyDeleted
	}

	wasPublished := comment.Status == constants.CommentStatusPublished
	comment.Content = content
	comment.UpdatedAt = time.Now()

	hits, blocked := s.sensitive.Detect(content)
	switch {
	case blocked:
		comment.Status = constants.CommentStatusPending
		if err := s.comments.Update(comment); err != nil {
			return nil, nil, false, err
		}
		if wasPublished {
			if err := s.posts.AdjustCommentCount(comment.PostID, -1); err != nil {
				s.logger.Error("decrement post comment count", "error", err)
			}
		}
		if err := s.review.Resubmit("comment", commentID, content, hits); err != nil {
			s.logger.Error("resubmit comment review", "error", err)
		}
		return comment, hits, true, nil
	default:
		comment.Status = constants.CommentStatusPublished
		if err := s.comments.Update(comment); err != nil {
			return nil, nil, false, err
		}
		if !wasPublished {
			if err := s.posts.AdjustCommentCount(comment.PostID, 1); err != nil {
				s.logger.Error("increment post comment count", "error", err)
			}
		}
		if err := s.review.CancelPending("comment", commentID); err != nil {
			s.logger.Error("cancel comment review", "error", err)
		}
		return comment, nil, false, nil
	}
}

// Withdraw 撤回评论：仅作者可操作；公开评论会同步减少帖子评论数。
func (s *commentService) Withdraw(identityID, commentID uint) error {
	comment, err := s.comments.FindByID(commentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrCommentNotFound
		}
		return err
	}
	if comment.IdentityID != identityID {
		return ErrOperationForbidden
	}
	if comment.Status == constants.CommentStatusWithdrawn {
		return ErrCommentAlreadyDeleted
	}
	if err := s.comments.WithdrawCascade(commentID); err != nil {
		return fmt.Errorf("withdraw comment: %w", err)
	}
	return nil
}
