package service

import (
	"testing"

	"github.com/gbtreehole/backend/internal/model"
	"github.com/gbtreehole/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSensitiveService(t *testing.T) SensitiveWordService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SensitiveWord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewSensitiveWordService(repository.NewSensitiveWordRepository(db))
}

func TestSensitiveWordDetect(t *testing.T) {
	svc := newSensitiveService(t)
	if _, err := svc.Create("赌博"); err != nil {
		t.Fatalf("create word: %v", err)
	}
	if _, err := svc.Create("诈骗"); err != nil {
		t.Fatalf("create word: %v", err)
	}
	tests := []struct {
		name    string
		content string
		blocked bool
	}{
		{"clean", "今天天气很好", false},
		{"single hit", "这里有人赌博", true},
		{"multiple hits", "赌博和诈骗都要禁止", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hits, blocked := svc.Detect(tt.content)
			if blocked != tt.blocked {
				t.Fatalf("blocked = %v, want %v, hits=%v", blocked, tt.blocked, hits)
			}
		})
	}
}

func TestSensitiveWordCreateDuplicate(t *testing.T) {
	svc := newSensitiveService(t)
	if _, err := svc.Create("暴力"); err != nil {
		t.Fatalf("create word: %v", err)
	}
	if _, err := svc.Create("暴力"); err != ErrWordExists {
		t.Fatalf("expected ErrWordExists, got %v", err)
	}
}
