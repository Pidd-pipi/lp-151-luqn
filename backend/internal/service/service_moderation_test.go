package service

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/gbtreehole/backend/internal/constants"
	"github.com/gbtreehole/backend/internal/model"
	"github.com/gbtreehole/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubSensitiveService struct {
	words []string
}

func (s *stubSensitiveService) Create(word string) (*model.SensitiveWord, error) {
	s.words = append(s.words, word)
	return &model.SensitiveWord{Word: word}, nil
}

func (s *stubSensitiveService) Delete(uint) error { return nil }
func (s *stubSensitiveService) List() ([]model.SensitiveWord, error) {
	out := make([]model.SensitiveWord, 0, len(s.words))
	for _, w := range s.words {
		out = append(out, model.SensitiveWord{Word: w})
	}
	return out, nil
}

func (s *stubSensitiveService) Detect(content string) ([]string, bool) {
	var hits []string
	for _, w := range s.words {
		if len(w) > 0 && contains(content, w) {
			hits = append(hits, w)
		}
	}
	return hits, len(hits) > 0
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func newModerationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.UserIdentity{}, &model.Post{}, &model.Tag{}, &model.PostTag{},
		&model.Comment{}, &model.Like{}, &model.SensitiveWord{}, &model.ReviewQueue{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	db.Create(&model.UserIdentity{ID: 1, IdentityKey: "k1", Nickname: "作者", CreatedAt: now, UpdatedAt: now})
	db.Create(&model.UserIdentity{ID: 2, IdentityKey: "k2", Nickname: "路人", CreatedAt: now, UpdatedAt: now})
	return db
}

func newModerationServices(t *testing.T) (PostService, CommentService, ReviewService, *gorm.DB) {
	t.Helper()
	db := newModerationTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	postRepo := repository.NewPostRepository(db)
	commentRepo := repository.NewCommentRepository(db)
	reviewRepo := repository.NewReviewQueueRepository(db)
	tagRepo := repository.NewTagRepository(db)

	sensitive := &stubSensitiveService{words: []string{"违禁"}}
	tagSvc := NewTagService(tagRepo)
	reviewSvc := NewReviewService(reviewRepo, postRepo, commentRepo, logger)
	postSvc := NewPostService(postRepo, tagSvc, sensitive, reviewSvc, logger)
	commentSvc := NewCommentService(commentRepo, postRepo, sensitive, reviewSvc, logger)
	return postSvc, commentSvc, reviewSvc, db
}

// TestPostOwnership 非作者不能编辑/撤回他人的帖子。
func TestPostOwnership(t *testing.T) {
	posts, _, _, _ := newModerationServices(t)
	post, _, blocked, err := posts.Create(1, "标题", "正常内容", nil, nil)
	if err != nil || blocked {
		t.Fatalf("create post: err=%v blocked=%v", err, blocked)
	}

	if _, _, _, err := posts.Update(2, post.ID, "新", "路人改的", nil); err != ErrOperationForbidden {
		t.Fatalf("update by other: want ErrOperationForbidden, got %v", err)
	}
	if err := posts.Withdraw(2, post.ID); err != ErrOperationForbidden {
		t.Fatalf("withdraw by other: want ErrOperationForbidden, got %v", err)
	}
	// 作者本人仍可编辑
	if _, _, blocked, err := posts.Update(1, post.ID, "标题", "作者改的", nil); err != nil || blocked {
		t.Fatalf("owner update: err=%v blocked=%v", err, blocked)
	}
}

// TestPostEditResubmit 编辑后命中敏感词进入审核，改回正常内容自动恢复公开。
func TestPostEditResubmit(t *testing.T) {
	posts, _, reviews, db := newModerationServices(t)
	post, _, _, err := posts.Create(1, "标题", "正常内容", nil, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, hits, blocked, err := posts.Update(1, post.ID, "标题", "包含违禁词", nil)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !blocked || len(hits) != 1 || updated.Status != constants.PostStatusPending {
		t.Fatalf("expected pending, blocked=%v hits=%v status=%d", blocked, hits, updated.Status)
	}
	// 列表不再出现
	list, total, err := posts.List(1, 20, 0, false)
	if err != nil || total != 0 || len(list) != 0 {
		t.Fatalf("pending post should be hidden: total=%d err=%v", total, err)
	}
	// 审核单待审
	var queue model.ReviewQueue
	db.Where("target_type = ? AND target_id = ?", "post", post.ID).First(&queue)
	if queue.Status != constants.ReviewStatusPending {
		t.Fatalf("queue status = %d", queue.Status)
	}

	// 改回干净内容：审核单取消，帖子恢复公开
	updated, _, blocked, err = posts.Update(1, post.ID, "标题", "正常内容了", nil)
	if err != nil || blocked || updated.Status != constants.PostStatusPublished {
		t.Fatalf("expected published: err=%v blocked=%v status=%d", err, blocked, updated.Status)
	}
	db.First(&queue, queue.ID)
	if queue.Status != constants.ReviewStatusCanceled {
		t.Fatalf("queue should be canceled, got %d", queue.Status)
	}
	_ = reviews
}

// TestPostWithdrawCascades 撤帖后详情消失，关联数据全部撤下。
func TestPostWithdrawCascades(t *testing.T) {
	posts, comments, _, db := newModerationServices(t)
	post, _, _, _ := posts.Create(1, "标题", "正文", nil, nil)
	// 路人一条公开评论 + 一条待审评论
	if _, _, blocked, err := comments.Create(2, post.ID, "评论1"); err != nil || blocked {
		t.Fatalf("create comment1: err=%v blocked=%v", err, blocked)
	}
	if _, _, blocked, err := comments.Create(2, post.ID, "评论违禁"); err != nil || !blocked {
		t.Fatalf("create comment2: err=%v blocked=%v", err, blocked)
	}

	if err := posts.Withdraw(1, post.ID); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if _, err := posts.GetVisibleByID(post.ID, 1); err != ErrPostNotVisible {
		t.Fatalf("withdrawn post detail: want ErrPostNotVisible, got %v", err)
	}
	if _, err := posts.GetVisibleByID(post.ID, 2); err != ErrPostNotVisible {
		t.Fatalf("withdrawn post for other: %v", err)
	}
	var visibleComments int64
	db.Model(&model.Comment{}).Where("post_id = ? AND status <> ?", post.ID, constants.CommentStatusWithdrawn).Count(&visibleComments)
	if visibleComments != 0 {
		t.Fatalf("comments not withdrawn: %d", visibleComments)
	}
	var likes int64
	db.Model(&model.Like{}).Count(&likes)
	_ = likes

	// 已撤回不能再编辑
	if _, _, _, err := posts.Update(1, post.ID, "标题", "再改", nil); err != ErrPostAlreadyDeleted {
		t.Fatalf("edit withdrawn: want ErrPostAlreadyDeleted, got %v", err)
	}
	// 重复撤回应报错
	if err := posts.Withdraw(1, post.ID); err != ErrPostAlreadyDeleted {
		t.Fatalf("double withdraw: want ErrPostAlreadyDeleted, got %v", err)
	}
}

// TestCommentLifecycle 非作者拒绝；撤评论同步评论数；编辑命中敏感词先下架。
func TestCommentLifecycle(t *testing.T) {
	posts, comments, reviewSvc, db := newModerationServices(t)
	post, _, _, _ := posts.Create(1, "标题", "正文", nil, nil)
	comment, _, _, _ := comments.Create(2, post.ID, "评论内容")

	// 路人 1 不能操作路人 2 的评论
	if _, _, _, err := comments.Update(1, comment.ID, "改"); err != ErrOperationForbidden {
		t.Fatalf("update other comment: %v", err)
	}
	if err := comments.Withdraw(1, comment.ID); err != ErrOperationForbidden {
		t.Fatalf("withdraw other comment: %v", err)
	}

	// 作者编辑命中敏感词：下架待审，帖子评论数归零
	updated, _, blocked, err := comments.Update(2, comment.ID, "违禁内容")
	if err != nil || !blocked || updated.Status != constants.CommentStatusPending {
		t.Fatalf("edit comment: err=%v blocked=%v status=%d", err, blocked, updated.Status)
	}
	p, _ := posts.GetVisibleByID(post.ID, 1)
	if p.CommentCount != 0 {
		t.Fatalf("comment_count after pending edit = %d", p.CommentCount)
	}

	// 管理员放行：评论恢复公开，评论数补回
	var queue model.ReviewQueue
	db.Where("target_type = ? AND target_id = ?", "comment", comment.ID).First(&queue)
	if err := reviewSvc.Approve(queue.ID, 99, ""); err != nil {
		t.Fatalf("approve: %v", err)
	}
	p, _ = posts.GetVisibleByID(post.ID, 1)
	if p.CommentCount != 1 {
		t.Fatalf("comment_count after approve = %d", p.CommentCount)
	}

	// 作者撤回：评论数减少
	if err := comments.Withdraw(2, comment.ID); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	p, _ = posts.GetVisibleByID(post.ID, 1)
	if p.CommentCount != 0 {
		t.Fatalf("comment_count after withdraw = %d", p.CommentCount)
	}
}

// TestReviewRejectUpdatesStatus 审核拒绝后作者看到被屏蔽状态，内容仍可自查。
func TestReviewRejectUpdatesStatus(t *testing.T) {
	posts, _, reviewSvc, db := newModerationServices(t)
	post, _, _, _ := posts.Create(1, "标题", "违禁内容", nil, nil)

	var queue model.ReviewQueue
	db.Where("target_type = ? AND target_id = ?", "post", post.ID).First(&queue)
	if err := reviewSvc.Reject(queue.ID, 99, "违规"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	// 路人不可见
	if _, err := posts.GetVisibleByID(post.ID, 2); err != ErrPostNotVisible {
		t.Fatalf("rejected post should hide from others: %v", err)
	}
	// 作者可见且状态为拒绝
	visible, err := posts.GetVisibleByID(post.ID, 1)
	if err != nil {
		t.Fatalf("owner view rejected: %v", err)
	}
	if visible.Status != constants.PostStatusRejected {
		t.Fatalf("status = %d", visible.Status)
	}
}
