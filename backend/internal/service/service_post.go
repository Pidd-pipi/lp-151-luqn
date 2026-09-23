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
	ErrPostNotFound  = errors.New("post not found")
	ErrForbidden     = errors.New("forbidden: not the owner")
	ErrPostWithdrawn = errors.New("post has been withdrawn")
)

type PostService interface {
	Create(identityID uint, title, content string, images []string, tagNames []string) (*model.Post, []string, bool, error)
	Update(identityID, postID uint, title, content string, images []string, tagNames []string) (*model.Post, []string, bool, error)
	Withdraw(identityID, postID uint) error
	GetByID(id uint) (*model.Post, error)
	List(page, pageSize int, tagID uint, featured bool) ([]model.Post, int64, error)
	ListHot(limit int) ([]model.Post, error)
	ListFeatured(limit int) ([]model.Post, error)
	DailyFeatured(limit int) ([]model.Post, error)
	IncrementView(id uint) error
	SetFeatured(id uint, featured bool) error
}

type postService struct {
	posts     repository.PostRepository
	comments  repository.CommentRepository
	likes     repository.LikeRepository
	tags      TagService
	sensitive SensitiveWordService
	review    ReviewService
	logger    *slog.Logger
}

func NewPostService(posts repository.PostRepository, comments repository.CommentRepository, likes repository.LikeRepository, tags TagService, sensitive SensitiveWordService, review ReviewService, logger *slog.Logger) PostService {
	return &postService{posts: posts, comments: comments, likes: likes, tags: tags, sensitive: sensitive, review: review, logger: logger}
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
	imagesJSON, err := marshalImages(images)
	if err != nil {
		return nil, hits, blocked, err
	}
	post.Images = imagesJSON
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

// Update 编辑自己的帖子：仅作者可操作；内容重新检测敏感词，命中则转入待审核。
func (s *postService) Update(identityID, postID uint, title, content string, images []string, tagNames []string) (*model.Post, []string, bool, error) {
	post, err := s.posts.FindByID(postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, false, ErrPostNotFound
		}
		return nil, nil, false, err
	}
	if post.IdentityID != identityID {
		return nil, nil, false, ErrForbidden
	}
	if post.Status == constants.PostStatusWithdrawn {
		return nil, nil, false, ErrPostWithdrawn
	}
	hits, blocked := s.sensitive.Detect(title + " " + content)
	imagesJSON, err := marshalImages(images)
	if err != nil {
		return nil, hits, blocked, err
	}
	post.Title = title
	post.Content = content
	post.Images = imagesJSON
	if err := s.syncTags(post, tagNames); err != nil {
		return nil, hits, blocked, err
	}
	if blocked {
		post.Status = constants.PostStatusPending
	} else {
		post.Status = constants.PostStatusPublished
	}
	post.UpdatedAt = time.Now()
	post.Tags = nil // 标签关系已显式同步，避免 Save 重复写入关联表
	if err := s.posts.Update(post); err != nil {
		return nil, hits, blocked, err
	}
	if blocked {
		if err := s.review.Enqueue("post", post.ID, title+" "+content, hits); err != nil {
			s.logger.Error("enqueue post review", "error", err)
		}
	}
	updated, err := s.posts.FindByID(postID)
	if err != nil {
		return nil, hits, blocked, err
	}
	return updated, hits, blocked, nil
}

// Withdraw 撤回自己的帖子：评论、点赞与标签关系一并撤下。
func (s *postService) Withdraw(identityID, postID uint) error {
	post, err := s.posts.FindByID(postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrPostNotFound
		}
		return err
	}
	if post.IdentityID != identityID {
		return ErrForbidden
	}
	if post.Status == constants.PostStatusWithdrawn {
		return nil
	}
	commentIDs, err := s.comments.ListIDsByPostID(postID)
	if err != nil {
		return err
	}
	if err := s.likes.DeleteByTargets("comment", commentIDs); err != nil {
		return err
	}
	if err := s.comments.WithdrawByPostID(postID, constants.CommentStatusWithdrawn); err != nil {
		return err
	}
	if err := s.likes.DeleteByTargets("post", []uint{postID}); err != nil {
		return err
	}
	for _, tag := range post.Tags {
		if err := s.tags.UnlinkPost(postID, tag.ID); err != nil {
			return err
		}
		if err := s.tags.DecCount(tag.ID); err != nil {
			s.logger.Error("decrement tag count", "error", err)
		}
	}
	post.Status = constants.PostStatusWithdrawn
	post.IsFeatured = false
	post.FeaturedAt = nil
	post.UpdatedAt = time.Now()
	post.Tags = nil
	return s.posts.Update(post)
}

// syncTags 将帖子的标签关系调整为 tagNames 指定的集合，并同步标签帖子数。
func (s *postService) syncTags(post *model.Post, tagNames []string) error {
	oldSet := make(map[uint]bool, len(post.Tags))
	for _, tag := range post.Tags {
		oldSet[tag.ID] = true
	}
	newSet := make(map[uint]bool, len(tagNames))
	for _, name := range tagNames {
		tag, err := s.tags.GetOrCreate(name)
		if err != nil {
			return err
		}
		if newSet[tag.ID] {
			continue
		}
		newSet[tag.ID] = true
		if !oldSet[tag.ID] {
			if err := s.tags.LinkPost(post.ID, tag.ID); err != nil {
				return err
			}
			if err := s.tags.IncCount(tag.ID); err != nil {
				s.logger.Error("increment tag count", "error", err)
			}
		}
	}
	for _, tag := range post.Tags {
		if newSet[tag.ID] {
			continue
		}
		if err := s.tags.UnlinkPost(post.ID, tag.ID); err != nil {
			return err
		}
		if err := s.tags.DecCount(tag.ID); err != nil {
			s.logger.Error("decrement tag count", "error", err)
		}
	}
	return nil
}

func marshalImages(images []string) (string, error) {
	if len(images) == 0 {
		return "", nil
	}
	imgBytes, err := json.Marshal(images)
	if err != nil {
		return "", fmt.Errorf("marshal images: %w", err)
	}
	return string(imgBytes), nil
}

func (s *postService) GetByID(id uint) (*model.Post, error) {
	post, err := s.posts.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
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
