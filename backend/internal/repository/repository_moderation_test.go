package repository

import (
	"testing"
	"time"

	"github.com/gbtreehole/backend/internal/constants"
	"github.com/gbtreehole/backend/internal/model"
)

// TestPostWithdrawCascade 验证撤帖时评论、点赞、标签关系与待审核单一并撤下。
func TestPostWithdrawCascade(t *testing.T) {
	db := newTestDB(t)
	posts := NewPostRepository(db)
	comments := NewCommentRepository(db)
	likes := NewLikeRepository(db)
	tags := NewTagRepository(db)
	reviews := NewReviewQueueRepository(db)

	now := time.Now()
	identity := &model.UserIdentity{ID: 7, IdentityKey: "k7", Nickname: "甲", CreatedAt: now, UpdatedAt: now}
	db.Create(identity)
	tag := &model.Tag{ID: 1, Name: "吐槽", PostCount: 1, CreatedAt: now, UpdatedAt: now}
	db.Create(tag)
	post := &model.Post{
		ID: 100, IdentityID: 7, Title: "标题", Content: "正文",
		Status: constants.PostStatusPublished, LikeCount: 2, CommentCount: 2,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := posts.Create(post); err != nil {
		t.Fatalf("create post: %v", err)
	}
	if err := tags.LinkPostTag(100, 1); err != nil {
		t.Fatalf("link tag: %v", err)
	}
	published := &model.Comment{ID: 201, PostID: 100, IdentityID: 7, Content: "公开评论",
		Status: constants.CommentStatusPublished, LikeCount: 1, CreatedAt: now, UpdatedAt: now}
	pending := &model.Comment{ID: 202, PostID: 100, IdentityID: 8, Content: "待审评论",
		Status: constants.CommentStatusPending, CreatedAt: now, UpdatedAt: now}
	if err := comments.Create(published); err != nil {
		t.Fatalf("create published comment: %v", err)
	}
	if err := comments.Create(pending); err != nil {
		t.Fatalf("create pending comment: %v", err)
	}
	// 帖子点赞 2 条、评论点赞 1 条
	if err := likes.Create(&model.Like{IdentityID: 7, TargetType: "post", TargetID: 100, CreatedAt: now}); err != nil {
		t.Fatalf("create like: %v", err)
	}
	if err := likes.Create(&model.Like{IdentityID: 8, TargetType: "post", TargetID: 100, CreatedAt: now}); err != nil {
		t.Fatalf("create like: %v", err)
	}
	if err := likes.Create(&model.Like{IdentityID: 8, TargetType: "comment", TargetID: 201, CreatedAt: now}); err != nil {
		t.Fatalf("create like: %v", err)
	}
	// 待审核单各一张
	db.Create(&model.ReviewQueue{TargetType: "post", TargetID: 100, Content: "正文",
		Status: constants.ReviewStatusPending, CreatedAt: now, UpdatedAt: now})
	db.Create(&model.ReviewQueue{TargetType: "comment", TargetID: 202, Content: "待审评论",
		Status: constants.ReviewStatusPending, CreatedAt: now, UpdatedAt: now})

	if err := posts.WithdrawCascade(100); err != nil {
		t.Fatalf("withdraw cascade: %v", err)
	}

	gotPost, err := posts.FindByID(100)
	if err != nil {
		t.Fatalf("find post: %v", err)
	}
	if gotPost.Status != constants.PostStatusWithdrawn {
		t.Fatalf("post status = %d", gotPost.Status)
	}
	if gotPost.CommentCount != 0 || gotPost.LikeCount != 0 {
		t.Fatalf("counts not reset: like=%d comment=%d", gotPost.LikeCount, gotPost.CommentCount)
	}

	for _, id := range []uint{201, 202} {
		cm, err := comments.FindByID(id)
		if err != nil {
			t.Fatalf("find comment %d: %v", id, err)
		}
		if cm.Status != constants.CommentStatusWithdrawn {
			t.Fatalf("comment %d status = %d", id, cm.Status)
		}
	}

	var likeCount int64
	db.Model(&model.Like{}).Where("target_type = ? AND target_id = ?", "post", 100).Count(&likeCount)
	if likeCount != 0 {
		t.Fatalf("post likes remaining: %d", likeCount)
	}
	db.Model(&model.Like{}).Where("target_type = ? AND target_id = ?", "comment", 201).Count(&likeCount)
	if likeCount != 0 {
		t.Fatalf("comment likes remaining: %d", likeCount)
	}

	var linkCount int64
	db.Model(&model.PostTag{}).Where("post_id = ?", 100).Count(&linkCount)
	if linkCount != 0 {
		t.Fatalf("post_tags remaining: %d", linkCount)
	}
	var tagPostCount int
	db.Model(&model.Tag{}).Where("id = 1").Pluck("post_count", &tagPostCount)
	if tagPostCount != 0 {
		t.Fatalf("tag post_count = %d", tagPostCount)
	}

	latest, err := reviews.LatestByTargets("post", []uint{100})
	if err != nil {
		t.Fatalf("latest reviews: %v", err)
	}
	if latest[100].Status != constants.ReviewStatusCanceled {
		t.Fatalf("post review status = %d", latest[100].Status)
	}
	latestC, err := reviews.LatestByTargets("comment", []uint{202})
	if err != nil {
		t.Fatalf("latest comment reviews: %v", err)
	}
	if latestC[202].Status != constants.ReviewStatusCanceled {
		t.Fatalf("comment review status = %d", latestC[202].Status)
	}

	// 撤回幂等：再次执行不报错
	if err := posts.WithdrawCascade(100); err != nil {
		t.Fatalf("withdraw idempotent: %v", err)
	}
}

// TestCommentWithdrawCascade 验证撤评论时点赞删除、待审核单取消、评论数同步。
func TestCommentWithdrawCascade(t *testing.T) {
	db := newTestDB(t)
	posts := NewPostRepository(db)
	comments := NewCommentRepository(db)
	now := time.Now()

	post := &model.Post{ID: 300, IdentityID: 9, Content: "帖",
		Status: constants.PostStatusPublished, CommentCount: 1, CreatedAt: now, UpdatedAt: now}
	if err := posts.Create(post); err != nil {
		t.Fatalf("create post: %v", err)
	}
	cm := &model.Comment{ID: 401, PostID: 300, IdentityID: 9, Content: "评论",
		Status: constants.CommentStatusPublished, LikeCount: 1, CreatedAt: now, UpdatedAt: now}
	if err := comments.Create(cm); err != nil {
		t.Fatalf("create comment: %v", err)
	}
	db.Create(&model.Like{IdentityID: 7, TargetType: "comment", TargetID: 401, CreatedAt: now})
	db.Create(&model.ReviewQueue{TargetType: "comment", TargetID: 401, Content: "评论",
		Status: constants.ReviewStatusPending, CreatedAt: now, UpdatedAt: now})

	if err := comments.WithdrawCascade(401); err != nil {
		t.Fatalf("withdraw comment: %v", err)
	}
	got, err := comments.FindByID(401)
	if err != nil {
		t.Fatalf("find comment: %v", err)
	}
	if got.Status != constants.CommentStatusWithdrawn || got.LikeCount != 0 {
		t.Fatalf("comment after withdraw: status=%d like=%d", got.Status, got.LikeCount)
	}
	gotPost, _ := posts.FindByID(300)
	if gotPost.CommentCount != 0 {
		t.Fatalf("post comment_count = %d", gotPost.CommentCount)
	}
	var likeCount int64
	db.Model(&model.Like{}).Where("target_type = ? AND target_id = ?", "comment", 401).Count(&likeCount)
	if likeCount != 0 {
		t.Fatalf("likes remaining: %d", likeCount)
	}
	reviews := NewReviewQueueRepository(db)
	latest, _ := reviews.LatestByTargets("comment", []uint{401})
	if latest[401].Status != constants.ReviewStatusCanceled {
		t.Fatalf("review status = %d", latest[401].Status)
	}

	// 待审评论撤回不应减少帖子评论数
	pending := &model.Comment{ID: 402, PostID: 300, IdentityID: 9, Content: "待审",
		Status: constants.CommentStatusPending, CreatedAt: now, UpdatedAt: now}
	comments.Create(pending)
	if err := comments.WithdrawCascade(402); err != nil {
		t.Fatalf("withdraw pending comment: %v", err)
	}
	gotPost2, _ := posts.FindByID(300)
	if gotPost2.CommentCount != 0 {
		t.Fatalf("comment_count changed by pending withdraw: %d", gotPost2.CommentCount)
	}
}

// TestListVisibleComments 验证审核中/被屏蔽评论仅作者本人可见。
func TestListVisibleComments(t *testing.T) {
	db := newTestDB(t)
	comments := NewCommentRepository(db)
	now := time.Now()
	db.Create(&model.Comment{ID: 501, PostID: 600, IdentityID: 1, Content: "公开",
		Status: constants.CommentStatusPublished, CreatedAt: now, UpdatedAt: now})
	db.Create(&model.Comment{ID: 502, PostID: 600, IdentityID: 2, Content: "待审",
		Status: constants.CommentStatusPending, CreatedAt: now, UpdatedAt: now})
	db.Create(&model.Comment{ID: 503, PostID: 600, IdentityID: 2, Content: "被拒",
		Status: constants.CommentStatusRejected, CreatedAt: now, UpdatedAt: now})
	db.Create(&model.Comment{ID: 504, PostID: 600, IdentityID: 2, Content: "撤回",
		Status: constants.CommentStatusWithdrawn, CreatedAt: now, UpdatedAt: now})

	// 匿名访客只能看到公开评论
	items, total, err := comments.ListVisibleByPostID(600, 0, 1, 20)
	if err != nil {
		t.Fatalf("list as guest: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != 501 {
		t.Fatalf("guest sees %d items total=%d", len(items), total)
	}
	// 作者 2 能看到自己的待审/被拒评论，但看不到自己已撤回的
	items, total, err = comments.ListVisibleByPostID(600, 2, 1, 20)
	if err != nil {
		t.Fatalf("list as owner: %v", err)
	}
	if total != 3 {
		t.Fatalf("owner total = %d", total)
	}
	ids := map[uint]bool{}
	for _, item := range items {
		ids[item.ID] = true
	}
	if !ids[501] || !ids[502] || !ids[503] || ids[504] {
		t.Fatalf("owner visible set = %v", ids)
	}
	// 其他登录身份只能看到公开评论
	_, total, _ = comments.ListVisibleByPostID(600, 3, 1, 20)
	if total != 1 {
		t.Fatalf("other identity total = %d", total)
	}
}
