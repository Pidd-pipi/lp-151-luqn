package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gbtreehole/backend/internal/model"
	"github.com/gbtreehole/backend/internal/repository"
)

var ErrWordExists = errors.New("sensitive word exists")

type SensitiveWordService interface {
	Create(word string) (*model.SensitiveWord, error)
	Delete(id uint) error
	List() ([]model.SensitiveWord, error)
	Detect(content string) ([]string, bool)
}

type sensitiveWordService struct {
	repo repository.SensitiveWordRepository
}

func NewSensitiveWordService(repo repository.SensitiveWordRepository) SensitiveWordService {
	return &sensitiveWordService{repo: repo}
}

func (s *sensitiveWordService) Create(word string) (*model.SensitiveWord, error) {
	word = strings.TrimSpace(word)
	if word == "" {
		return nil, fmt.Errorf("word is empty")
	}
	existing, err := s.repo.FindByWord(word)
	if err == nil && existing != nil {
		return nil, ErrWordExists
	}
	sw := &model.SensitiveWord{Word: word}
	if err := s.repo.Create(sw); err != nil {
		return nil, err
	}
	return sw, nil
}

func (s *sensitiveWordService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *sensitiveWordService) List() ([]model.SensitiveWord, error) {
	return s.repo.List()
}

// Detect 返回命中的敏感词以及是否命中。空词条跳过。
func (s *sensitiveWordService) Detect(content string) ([]string, bool) {
	words, err := s.repo.List()
	if err != nil {
		// 过滤失败不阻塞主流程，交由上层记录
		return nil, false
	}
	lower := strings.ToLower(content)
	var hits []string
	for _, w := range words {
		if w.Word == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(w.Word)) {
			hits = append(hits, w.Word)
		}
	}
	return hits, len(hits) > 0
}
