package repository

import (
	"errors"
	"fmt"

	"github.com/gbtreehole/backend/internal/constants"
	"github.com/gbtreehole/backend/internal/model"
	"gorm.io/gorm"
)

type ReviewQueueRepository interface {
	Create(item *model.ReviewQueue) error
	Update(item *model.ReviewQueue) error
	FindByID(id uint) (*model.ReviewQueue, error)
	// FindPending 查询某内容当前待审核的审核单，不存在时返回 ErrNotFound。
	FindPending(targetType string, targetID uint) (*model.ReviewQueue, error)
	// LatestByTargets 批量查询每个内容最新的一张审核单（含已取消），用于作者查看处理状态。
	LatestByTargets(targetType string, targetIDs []uint) (map[uint]model.ReviewQueue, error)
	List(page, pageSize int, status int) ([]model.ReviewQueue, int64, error)
}

type reviewQueueRepository struct {
	db *gorm.DB
}

func NewReviewQueueRepository(db *gorm.DB) ReviewQueueRepository {
	return &reviewQueueRepository{db: db}
}

func (r *reviewQueueRepository) Create(item *model.ReviewQueue) error {
	if err := r.db.Create(item).Error; err != nil {
		return fmt.Errorf("create review queue: %w", err)
	}
	return nil
}

func (r *reviewQueueRepository) Update(item *model.ReviewQueue) error {
	if err := r.db.Save(item).Error; err != nil {
		return fmt.Errorf("update review queue: %w", err)
	}
	return nil
}

func (r *reviewQueueRepository) FindByID(id uint) (*model.ReviewQueue, error) {
	var item model.ReviewQueue
	if err := r.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find review queue by id: %w", err)
	}
	return &item, nil
}

func (r *reviewQueueRepository) FindPending(targetType string, targetID uint) (*model.ReviewQueue, error) {
	var item model.ReviewQueue
	if err := r.db.Where("target_type = ? AND target_id = ? AND status = ?",
		targetType, targetID, constants.ReviewStatusPending).Order("id DESC").First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find pending review: %w", err)
	}
	return &item, nil
}

// LatestByTargets 取每个内容最新的审核单（含已取消），用于作者查看处理状态。
func (r *reviewQueueRepository) LatestByTargets(targetType string, targetIDs []uint) (map[uint]model.ReviewQueue, error) {
	result := make(map[uint]model.ReviewQueue)
	if len(targetIDs) == 0 {
		return result, nil
	}
	// 按 id 升序取出后在内存中覆盖，保证 MySQL / SQLite 行为一致。
	var items []model.ReviewQueue
	if err := r.db.Where("target_type = ? AND target_id IN ?", targetType, targetIDs).
		Order("id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("find latest reviews: %w", err)
	}
	for _, item := range items {
		result[item.TargetID] = item
	}
	return result, nil
}

func (r *reviewQueueRepository) List(page, pageSize int, status int) ([]model.ReviewQueue, int64, error) {
	var items []model.ReviewQueue
	var total int64
	q := r.db.Model(&model.ReviewQueue{})
	if status > 0 {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count review queue: %w", err)
	}
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list review queue: %w", err)
	}
	return items, total, nil
}
