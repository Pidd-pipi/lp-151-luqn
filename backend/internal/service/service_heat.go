package service

import (
	"log/slog"

	"github.com/gbtreehole/backend/internal/model"
	"github.com/gbtreehole/backend/internal/repository"
)

type HeatService interface {
	Score(post *model.Post) float64
	RankPosts(limit int) ([]model.Post, error)
}

type heatService struct {
	posts  repository.PostRepository
	logger *slog.Logger
}

func NewHeatService(posts repository.PostRepository, logger *slog.Logger) HeatService {
	return &heatService{posts: posts, logger: logger}
}

func (s *heatService) Score(post *model.Post) float64 {
	return float64(post.LikeCount)*10 + float64(post.CommentCount)*5
}

func (s *heatService) RankPosts(limit int) ([]model.Post, error) {
	return s.posts.ListHot(limit)
}
