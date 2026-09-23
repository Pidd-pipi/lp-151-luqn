package repository

import (
	"testing"
	"time"

	"github.com/gbtreehole/backend/internal/model"
)

func TestLikeRepository(t *testing.T) {
	db := newTestDB(t)
	repo := NewLikeRepository(db)
	like := &model.Like{IdentityID: 1, TargetType: "post", TargetID: 10, CreatedAt: time.Now()}
	if err := repo.Create(like); err != nil {
		t.Fatalf("create like: %v", err)
	}
	found, err := repo.Find(1, "post", 10)
	if err != nil {
		t.Fatalf("find like: %v", err)
	}
	if found.TargetID != 10 {
		t.Fatalf("target mismatch: %d", found.TargetID)
	}
	count, err := repo.CountByTarget("post", 10)
	if err != nil {
		t.Fatalf("count like: %v", err)
	}
	if count != 1 {
		t.Fatalf("count mismatch: %d", count)
	}
	liked, err := repo.IsLiked(1, "post", []uint{10})
	if err != nil {
		t.Fatalf("is liked: %v", err)
	}
	if !liked[10] {
		t.Fatal("expected liked")
	}
	if err := repo.Delete(found.ID); err != nil {
		t.Fatalf("delete like: %v", err)
	}
	if _, err := repo.Find(1, "post", 10); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
