package repository

import (
	"errors"
	"fmt"

	"github.com/gbtreehole/backend/internal/model"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("not found")

type IdentityRepository interface {
	Create(identity *model.UserIdentity) error
	FindByID(id uint) (*model.UserIdentity, error)
	FindByKey(key string) (*model.UserIdentity, error)
	ListByKeys(keys []string) ([]model.UserIdentity, error)
}

type identityRepository struct {
	db *gorm.DB
}

func NewIdentityRepository(db *gorm.DB) IdentityRepository {
	return &identityRepository{db: db}
}

func (r *identityRepository) Create(identity *model.UserIdentity) error {
	if err := r.db.Create(identity).Error; err != nil {
		return fmt.Errorf("create identity: %w", err)
	}
	return nil
}

func (r *identityRepository) FindByID(id uint) (*model.UserIdentity, error) {
	var identity model.UserIdentity
	if err := r.db.First(&identity, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find identity by id: %w", err)
	}
	return &identity, nil
}

func (r *identityRepository) FindByKey(key string) (*model.UserIdentity, error) {
	var identity model.UserIdentity
	if err := r.db.Where("identity_key = ?", key).First(&identity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find identity by key: %w", err)
	}
	return &identity, nil
}

func (r *identityRepository) ListByKeys(keys []string) ([]model.UserIdentity, error) {
	var identities []model.UserIdentity
	if len(keys) == 0 {
		return identities, nil
	}
	if err := r.db.Where("identity_key IN ?", keys).Find(&identities).Error; err != nil {
		return nil, fmt.Errorf("list identities by keys: %w", err)
	}
	return identities, nil
}
