package repository

import (
	"fmt"

	"github.com/gbtreehole/backend/internal/model"
	"gorm.io/gorm"
)

type SensitiveWordRepository interface {
	Create(word *model.SensitiveWord) error
	FindByID(id uint) (*model.SensitiveWord, error)
	FindByWord(word string) (*model.SensitiveWord, error)
	List() ([]model.SensitiveWord, error)
	Delete(id uint) error
}

type sensitiveWordRepository struct {
	db *gorm.DB
}

func NewSensitiveWordRepository(db *gorm.DB) SensitiveWordRepository {
	return &sensitiveWordRepository{db: db}
}

func (r *sensitiveWordRepository) Create(word *model.SensitiveWord) error {
	if err := r.db.Create(word).Error; err != nil {
		return fmt.Errorf("create sensitive word: %w", err)
	}
	return nil
}

func (r *sensitiveWordRepository) FindByID(id uint) (*model.SensitiveWord, error) {
	var word model.SensitiveWord
	if err := r.db.First(&word, id).Error; err != nil {
		return nil, fmt.Errorf("find sensitive word by id: %w", err)
	}
	return &word, nil
}

func (r *sensitiveWordRepository) FindByWord(word string) (*model.SensitiveWord, error) {
	var sw model.SensitiveWord
	if err := r.db.Where("word = ?", word).First(&sw).Error; err != nil {
		return nil, fmt.Errorf("find sensitive word: %w", err)
	}
	return &sw, nil
}

func (r *sensitiveWordRepository) List() ([]model.SensitiveWord, error) {
	var words []model.SensitiveWord
	if err := r.db.Find(&words).Error; err != nil {
		return nil, fmt.Errorf("list sensitive words: %w", err)
	}
	return words, nil
}

func (r *sensitiveWordRepository) Delete(id uint) error {
	if err := r.db.Delete(&model.SensitiveWord{}, id).Error; err != nil {
		return fmt.Errorf("delete sensitive word: %w", err)
	}
	return nil
}
