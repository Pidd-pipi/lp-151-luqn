package repository

import (
	"testing"
	"time"

	"github.com/gbtreehole/backend/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.UserIdentity{}, &model.Post{}, &model.Tag{}, &model.PostTag{}, &model.Comment{}, &model.Like{}, &model.ReviewQueue{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestIdentityRepository(t *testing.T) {
	db := newTestDB(t)
	repo := NewIdentityRepository(db)
	identity := &model.UserIdentity{IdentityKey: "key-1", Nickname: "匿名", Avatar: "a.png", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := repo.Create(identity); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.FindByKey("key-1")
	if err != nil {
		t.Fatalf("find by key: %v", err)
	}
	if got.Nickname != "匿名" {
		t.Fatalf("nickname mismatch: %s", got.Nickname)
	}
	if _, err := repo.FindByKey("missing"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPostRepository(t *testing.T) {
	db := newTestDB(t)
	identityRepo := NewIdentityRepository(db)
	postRepo := NewPostRepository(db)
	identity := &model.UserIdentity{IdentityKey: "key-1", Nickname: "匿名", Avatar: "a.png", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := identityRepo.Create(identity); err != nil {
		t.Fatalf("create identity: %v", err)
	}
	post := &model.Post{IdentityID: identity.ID, Content: "hello", Status: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := postRepo.Create(post); err != nil {
		t.Fatalf("create post: %v", err)
	}
	got, err := postRepo.FindByID(post.ID)
	if err != nil {
		t.Fatalf("find post: %v", err)
	}
	if got.Content != "hello" {
		t.Fatalf("content mismatch: %s", got.Content)
	}
	posts, total, err := postRepo.List(1, 10, 1, false, 0)
	if err != nil {
		t.Fatalf("list posts: %v", err)
	}
	if total != 1 || len(posts) != 1 {
		t.Fatalf("expected 1 post, got total=%d len=%d", total, len(posts))
	}
	if _, err := postRepo.FindByID(999); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
