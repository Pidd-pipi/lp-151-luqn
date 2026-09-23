package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheService interface {
	Get(ctx context.Context, key string, out any) error
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

type cacheService struct {
	client *redis.Client
}

func NewCacheService(client *redis.Client) CacheService {
	return &cacheService{client: client}
}

func (s *cacheService) Get(ctx context.Context, key string, out any) error {
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		return fmt.Errorf("cache get: %w", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("cache unmarshal: %w", err)
	}
	return nil
}

func (s *cacheService) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}
	if err := s.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("cache set: %w", err)
	}
	return nil
}

func (s *cacheService) Delete(ctx context.Context, keys ...string) error {
	if err := s.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("cache delete: %w", err)
	}
	return nil
}
