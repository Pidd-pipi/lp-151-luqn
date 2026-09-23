package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/gbtreehole/backend/internal/config"
	"github.com/gbtreehole/backend/internal/model"
)

func NewMySQL(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}

func Seed(db *gorm.DB) error {
	words := []string{"暴力", "色情", "赌博", "诈骗", "违禁"}
	for _, w := range words {
		var count int64
		db.Model(&model.SensitiveWord{}).Where("word = ?", w).Count(&count)
		if count == 0 {
			if err := db.Create(&model.SensitiveWord{Word: w}).Error; err != nil {
				return fmt.Errorf("seed sensitive word %s: %w", w, err)
			}
		}
	}
	return nil
}

func NewRedis(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	return client, nil
}
