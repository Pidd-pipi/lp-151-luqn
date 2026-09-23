package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gbtreehole/backend/internal/model"
	"github.com/gbtreehole/backend/internal/repository"
)

var ErrTagExists = errors.New("tag exists")

type TagService interface {
	GetOrCreate(name string) (*model.Tag, error)
	List() ([]model.Tag, error)
	FindByID(id uint) (*model.Tag, error)
	IncCount(id uint) error
}

type tagService struct {
	repo repository.TagRepository
}

func NewTagService(repo repository.TagRepository) TagService {
	return &tagService{repo: repo}
}

func (s *tagService) GetOrCreate(name string) (*model.Tag, error) {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "#")
	if name == "" {
		return nil, fmt.Errorf("tag name is empty")
	}
	tag, err := s.repo.FindByName(name)
	if err == nil {
		return tag, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	tag = &model.Tag{Name: name}
	if err := s.repo.Create(tag); err != nil {
		return nil, err
	}
	return tag, nil
}

func (s *tagService) List() ([]model.Tag, error) {
	return s.repo.List()
}

func (s *tagService) FindByID(id uint) (*model.Tag, error) {
	tag, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return tag, nil
}

func (s *tagService) IncCount(id uint) error {
	return s.repo.IncrementPostCount(id)
}
