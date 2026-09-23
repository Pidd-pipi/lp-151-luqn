package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gbtreehole/backend/internal/constants"
	"github.com/gbtreehole/backend/internal/model"
	"github.com/gbtreehole/backend/internal/repository"
)

var (
	ErrPostNotFound       = errors.New("post not found")
	ErrPostNotVisible     = errors.New("post not visible")
	ErrOperationForbidden = errors.New("operation forbidden")
	ErrPostAlreadyDeleted = errors.New("post already withdrawn")
)

type PostService interface {
	Create(identityID uint, title, content string, images []string, tagNames []string) (*model.Post, []string, bool, error)
	GetVisibleByID(id, viewerID uint) (*model.Post, error)
	List(page, pageSize int, tagID uint, featured bool) ([]model.Post, int64, error)
	ListHot(limit int) ([]model.Post, error)
	ListFeatured(limit int) ([]model.Post, error)
	DailyFeatured(limit int) ([]model.Post, error)
	IncrementView(id uint) error
	SetFeatured(id uint, featured bool) error
	// Update 作者编辑帖子，重新检测敏感词，命中则转入审核。
	Update(identityID, id uint, title, content string, images []string) (*model.Post, []string, bool, error)
	// Withdraw 作者撤回帖子，级联撤下评论、点赞与标签关系。
	Withdraw(identityID, id uint) error
}

type postService struct {
	posts     repository.PostRepository
	tags      TagService
	sensitive SensitiveWordService
	review    ReviewService
	logger    *slog.Logger
}

func NewPostService(posts repository.PostRepository, tags TagService, sensitive SensitiveWordService, review ReviewService, logger *slog.Logger) PostService {
	return &postService{posts: posts, tags: tags, sensitive: sensitive, review: review, logger: logger}
}

func (s *postService) Create(identityID uint, title, content string, images []string, tagNames []string) (*model.Post, []string, bool, error) {
	hits, blocked := s.sensitive.Detect(title + " " + content)
	post := &model.Post{
		IdentityID: identityID,
		Title:      title,
		Content:    content,
		Status:     constants.PostStatusPublished,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if len(images) > 0 {
		imgBytes, err := json.Marshal(images)
		if err != nil {
			return nil, hits, blocked, fmt.Errorf("marshal images: %w", err)
		}
		post.Images = string(imgBytes)
	}
	if blocked {
		post.Status = constants.PostStatusPending
	}
	var tags []model.Tag
	if len(tagNames) > 0 {
		for _, name := range tagNames {
			tag, err := s.tags.GetOrCreate(name)
			if err != nil {
				return nil, hits, blocked, err
			}
			tags = append(tags, *tag)
		}
	}
	post.Tags = tags
	if err := s.posts.Create(post); err != nil {
		return nil, hits, blocked, err
	}
	for _, tag := range tags {
		if err := s.tags.IncCount(tag.ID); err != nil {
			s.logger.Error("increment tag count", "error", err)
		}
	}
	if blocked {
		if err := s.review.Enqueue("post", post.ID, title+" "+content, hits); err != nil {
			s.logger.Error("enqueue post review", "error", err)
		}
	}
	return post, hits, blocked, nil
}

// GetVisibleByID 详情可见性：撤回帖对所有人不可见；
// 审核中/被屏蔽的帖子仅作者本人可见。
func (s *postService) GetVisibleByID(id, viewerID uint) (*model.Post, error) {
	post, err := s.posts.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}
	switch {
	case post.Status == constants.PostStatusWithdrawn:
		return nil, ErrPostNotVisible
	case post.Status != constants.PostStatusPublished && post.IdentityID != viewerID:
		return nil, ErrPostNotVisible
	}
	return post, nil
}

func (s *postService) List(page, pageSize int, tagID uint, featured bool) ([]model.Post, int64, error) {
	return s.posts.List(page, pageSize, constants.PostStatusPublished, featured, tagID)
}

func (s *postService) ListHot(limit int) ([]model.Post, error) {
	return s.posts.ListHot(limit)
}

func (s *postService) ListFeatured(limit int) ([]model.Post, error) {
	return s.posts.ListFeatured(limit)
}

// DailyFeatured 结合手动精选与热度自动筛选今日内容。
func (s *postService) DailyFeatured(limit int) ([]model.Post, error) {
	featured, err := s.posts.ListFeatured(limit)
	if err != nil {
		return nil, err
	}
	if len(featured) >= limit {
		return featured, nil
	}
	hot, err := s.posts.ListHot(limit * 2)
	if err != nil {
		return nil, err
	}
	seen := make(map[uint]bool)
	for _, p := range featured {
		seen[p.ID] = true
	}
	result := featured
	for _, p := range hot {
		if seen[p.ID] {
			continue
		}
		if len(result) >= limit {
			break
		}
		result = append(result, p)
	}
	return result, nil
}

func (s *postService) IncrementView(id uint) error {
	return s.posts.IncrementView(id)
}

func (s *postService) SetFeatured(id uint, featured bool) error {
	post, err := s.posts.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrPostNotFound
		}
		return err
	}
	post.IsFeatured = featured
	if featured {
		now := time.Now()
		post.FeaturedAt = &now
	} else {
		post.FeaturedAt = nil
	}
	post.UpdatedAt = time.Now()
	return s.posts.Update(post)
}

// Update 编辑帖子：仅作者可操作；重新检测敏感词，
// 命中则进入审核队列（沿用或新建审核单），未命中则恢复公开并取消旧审核单。
func (s *postService) Update(identityID, id uint, title, content string, images []string) (*model.Post, []string, bool, error) {
	post, err := s.posts.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, false, ErrPostNotFound
		}
		return nil, nil, false, err
	}
	if post.IdentityID != identityID {
		return nil, nil, false, ErrOperationForbidden
	}
	if post.Status == constants.PostStatusWithdrawn {
		return nil, nil, false, ErrPostAlreadyDeleted
	}

	post.Title = title
	post.Content = content
	if len(images) > 0 {
		imgBytes, err := json.Marshal(images)
		if err != nil {
			return nil, nil, false, fmt.Errorf("marshal images: %w", err)
		}
		post.Images = string(imgBytes)
	} else {
		post.Images = ""
	}
	post.UpdatedAt = time.Now()

	hits, blocked := s.sensitive.Detect(title + " " + content)
	if blocked {
		post.Status = constants.PostStatusPending
		if err := s.posts.Update(post); err != nil {
			return nil, nil, false, err
		}
		if err := s.review.Resubmit("post", id, title+" "+content, hits); err != nil {
			s.logger.Error("resubmit post review", "error", err)
		}
		return post, hits, true, nil
	}

	post.Status = constants.PostStatusPublished
	post.ReviewRemark = ""
	if err := s.posts.Update(post); err != nil {
		return nil, nil, false, err
	}
	if err := s.review.CancelPending("post", id); err != nil {
		s.logger.Error("cancel post review", "error", err)
	}
	return post, nil, false, nil
}

// Withdraw 撤回帖子：仅作者可操作，级联撤下相关数据。
func (s *postService) Withdraw(identityID, id uint) error {
	post, err := s.posts.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrPostNotFound
		}
		return err
	}
	if post.IdentityID != identityID {
		return ErrOperationForbidden
	}
	if post.Status == constants.PostStatusWithdrawn {
		return ErrPostAlreadyDeleted
	}
	if err := s.posts.WithdrawCascade(id); err != nil {
		return err
	}
	return nil
}
