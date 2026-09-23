package service

import (
	"io"
	"log/slog"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/gbtreehole/backend/internal/constants"
	"github.com/gbtreehole/backend/internal/model"
	"github.com/gbtreehole/backend/internal/repository"
)

// manageTestEnv 组装一套基于内存 sqlite 的服务，用于帖子/评论管理流程测试。
type manageTestEnv struct {
	posts       PostService
	comments    CommentService
	review      ReviewService
	likes       LikeService
	tags        TagService
	postRepo    repository.PostRepository
	commentRepo repository.CommentRepository
	tagRepo     repository.TagRepository
	likeRepo    repository.LikeRepository
}

func newManageTestEnv(t *testing.T) *manageTestEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	postRepo := repository.NewPostRepository(db)
	commentRepo := repository.NewCommentRepository(db)
	tagRepo := repository.NewTagRepository(db)
	likeRepo := repository.NewLikeRepository(db)
	sensitiveRepo := repository.NewSensitiveWordRepository(db)
	reviewRepo := repository.NewReviewQueueRepository(db)

	sensitive := NewSensitiveWordService(sensitiveRepo)
	if _, err := sensitive.Create("赌博"); err != nil {
		t.Fatalf("seed sensitive word: %v", err)
	}
	tags := NewTagService(tagRepo)
	review := NewReviewService(reviewRepo, postRepo, commentRepo, logger)
	posts := NewPostService(postRepo, commentRepo, likeRepo, tags, sensitive, review, logger)
	comments := NewCommentService(commentRepo, postRepo, likeRepo, sensitive, review, logger)
	likes := NewLikeService(likeRepo, postRepo, commentRepo, logger)
	return &manageTestEnv{
		posts:       posts,
		comments:    comments,
		review:      review,
		likes:       likes,
		tags:        tags,
		postRepo:    postRepo,
		commentRepo: commentRepo,
		tagRepo:     tagRepo,
		likeRepo:    likeRepo,
	}
}

func mustCreatePost(t *testing.T, env *manageTestEnv, identityID uint, content string, tags []string) *model.Post {
	t.Helper()
	post, _, _, err := env.posts.Create(identityID, "标题", content, nil, tags)
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	return post
}

func TestPostUpdateRequiresOwner(t *testing.T) {
	env := newManageTestEnv(t)
	post := mustCreatePost(t, env, 1, "今天天气不错", nil)

	if _, _, _, err := env.posts.Update(2, post.ID, "标题", "他人修改", nil, nil); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	if err := env.posts.Withdraw(2, post.ID); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	if _, _, _, err := env.posts.Update(1, 9999, "标题", "不存在", nil, nil); err != ErrPostNotFound {
		t.Fatalf("expected ErrPostNotFound, got %v", err)
	}
}

func TestPostUpdateSensitiveGoesPending(t *testing.T) {
	env := newManageTestEnv(t)
	post := mustCreatePost(t, env, 1, "今天天气不错", nil)

	updated, hits, blocked, err := env.posts.Update(1, post.ID, "新标题", "这里有人赌博", nil, nil)
	if err != nil {
		t.Fatalf("update post: %v", err)
	}
	if !blocked || len(hits) == 0 {
		t.Fatalf("expected blocked with hits, got blocked=%v hits=%v", blocked, hits)
	}
	if updated.Status != constants.PostStatusPending {
		t.Fatalf("expected pending status, got %d", updated.Status)
	}
	// 命中后进入审核队列
	items, total, err := env.review.List(1, 10, constants.ReviewStatusPending)
	if err != nil {
		t.Fatalf("list review: %v", err)
	}
	if total != 1 || items[0].TargetType != "post" || items[0].TargetID != post.ID {
		t.Fatalf("expected pending review for post %d, got total=%d", post.ID, total)
	}
	// 待审帖子不出现在公开列表
	_, listed, err := env.posts.List(1, 10, 0, false)
	if err != nil {
		t.Fatalf("list posts: %v", err)
	}
	if listed != 0 {
		t.Fatalf("expected 0 published posts, got %d", listed)
	}
	// 再次编辑为干净内容后恢复发布
	updated, _, blocked, err = env.posts.Update(1, post.ID, "新标题", "改成干净内容", nil, nil)
	if err != nil {
		t.Fatalf("update post clean: %v", err)
	}
	if blocked || updated.Status != constants.PostStatusPublished {
		t.Fatalf("expected published clean post, got blocked=%v status=%d", blocked, updated.Status)
	}
}

func TestPostWithdrawCascade(t *testing.T) {
	env := newManageTestEnv(t)
	post := mustCreatePost(t, env, 1, "带着标签的帖子", []string{"树洞"})

	comment, _, _, err := env.comments.Create(2, post.ID, "一楼评论")
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if _, _, err := env.likes.Toggle(2, "post", post.ID); err != nil {
		t.Fatalf("like post: %v", err)
	}
	if _, _, err := env.likes.Toggle(3, "comment", comment.ID); err != nil {
		t.Fatalf("like comment: %v", err)
	}
	tag, err := env.tags.GetOrCreate("树洞")
	if err != nil {
		t.Fatalf("get tag: %v", err)
	}
	if tag.PostCount != 1 {
		t.Fatalf("expected tag count 1, got %d", tag.PostCount)
	}

	if err := env.posts.Withdraw(1, post.ID); err != nil {
		t.Fatalf("withdraw post: %v", err)
	}

	got, err := env.postRepo.FindByID(post.ID)
	if err != nil {
		t.Fatalf("find post: %v", err)
	}
	if got.Status != constants.PostStatusWithdrawn {
		t.Fatalf("expected withdrawn post, got %d", got.Status)
	}
	if len(got.Tags) != 0 {
		t.Fatalf("expected tag relations removed, got %d", len(got.Tags))
	}
	// 评论一并撤回
	gotComment, err := env.commentRepo.FindByID(comment.ID)
	if err != nil {
		t.Fatalf("find comment: %v", err)
	}
	if gotComment.Status != constants.CommentStatusWithdrawn {
		t.Fatalf("expected withdrawn comment, got %d", gotComment.Status)
	}
	// 点赞一并撤下
	if count, _ := env.likeRepo.CountByTarget("post", post.ID); count != 0 {
		t.Fatalf("expected 0 post likes, got %d", count)
	}
	if count, _ := env.likeRepo.CountByTarget("comment", comment.ID); count != 0 {
		t.Fatalf("expected 0 comment likes, got %d", count)
	}
	// 标签计数回落
	tag, err = env.tags.GetOrCreate("树洞")
	if err != nil {
		t.Fatalf("get tag: %v", err)
	}
	if tag.PostCount != 0 {
		t.Fatalf("expected tag count 0, got %d", tag.PostCount)
	}
	// 列表不再出现
	_, listed, err := env.posts.List(1, 10, 0, false)
	if err != nil {
		t.Fatalf("list posts: %v", err)
	}
	if listed != 0 {
		t.Fatalf("expected 0 published posts, got %d", listed)
	}
	// 已撤回帖子不能再编辑
	if _, _, _, err := env.posts.Update(1, post.ID, "标题", "再改", nil, nil); err != ErrPostWithdrawn {
		t.Fatalf("expected ErrPostWithdrawn, got %v", err)
	}
}

func TestCommentWithdrawSyncsCount(t *testing.T) {
	env := newManageTestEnv(t)
	post := mustCreatePost(t, env, 1, "楼主的内容", nil)

	comment, _, _, err := env.comments.Create(2, post.ID, "一条评论")
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	got, _ := env.postRepo.FindByID(post.ID)
	if got.CommentCount != 1 {
		t.Fatalf("expected comment count 1, got %d", got.CommentCount)
	}
	// 非本人撤回被拒绝
	if err := env.comments.Withdraw(1, comment.ID); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	if err := env.comments.Withdraw(2, comment.ID); err != nil {
		t.Fatalf("withdraw comment: %v", err)
	}
	got, _ = env.postRepo.FindByID(post.ID)
	if got.CommentCount != 0 {
		t.Fatalf("expected comment count 0, got %d", got.CommentCount)
	}
	gotComment, _ := env.commentRepo.FindByID(comment.ID)
	if gotComment.Status != constants.CommentStatusWithdrawn {
		t.Fatalf("expected withdrawn comment, got %d", gotComment.Status)
	}
	// 已撤回评论不能再编辑
	if _, _, _, err := env.comments.Update(2, comment.ID, "再改"); err != ErrCommentWithdrawn {
		t.Fatalf("expected ErrCommentWithdrawn, got %v", err)
	}
}

func TestCommentEditAndReviewFlow(t *testing.T) {
	env := newManageTestEnv(t)
	post := mustCreatePost(t, env, 1, "楼主的内容", nil)

	comment, _, _, err := env.comments.Create(2, post.ID, "正常评论")
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	// 编辑命中敏感词：转入待审，帖子评论数减少
	updated, _, blocked, err := env.comments.Update(2, comment.ID, "这里有赌博内容")
	if err != nil {
		t.Fatalf("update comment: %v", err)
	}
	if !blocked || updated.Status != constants.CommentStatusPending {
		t.Fatalf("expected pending comment, got blocked=%v status=%d", blocked, updated.Status)
	}
	got, _ := env.postRepo.FindByID(post.ID)
	if got.CommentCount != 0 {
		t.Fatalf("expected comment count 0, got %d", got.CommentCount)
	}
	// 审核放行：评论可见，帖子评论数恢复
	items, total, err := env.review.List(1, 10, constants.ReviewStatusPending)
	if err != nil || total != 1 {
		t.Fatalf("expected 1 pending review, got total=%d err=%v", total, err)
	}
	if err := env.review.Approve(items[0].ID, 99, "放行"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	gotComment, _ := env.commentRepo.FindByID(comment.ID)
	if gotComment.Status != constants.CommentStatusPublished {
		t.Fatalf("expected published comment, got %d", gotComment.Status)
	}
	got, _ = env.postRepo.FindByID(post.ID)
	if got.CommentCount != 1 {
		t.Fatalf("expected comment count 1 after approve, got %d", got.CommentCount)
	}

	// 再次编辑命中后被拒绝：评论不可见，评论数不恢复
	if _, _, _, err := env.comments.Update(2, comment.ID, "还是赌博"); err != nil {
		t.Fatalf("update comment again: %v", err)
	}
	items, total, err = env.review.List(1, 10, constants.ReviewStatusPending)
	if err != nil || total != 1 {
		t.Fatalf("expected 1 pending review, got total=%d err=%v", total, err)
	}
	if err := env.review.Reject(items[0].ID, 99, "违规"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	gotComment, _ = env.commentRepo.FindByID(comment.ID)
	if gotComment.Status != constants.CommentStatusRejected {
		t.Fatalf("expected rejected comment, got %d", gotComment.Status)
	}
	got, _ = env.postRepo.FindByID(post.ID)
	if got.CommentCount != 0 {
		t.Fatalf("expected comment count 0 after reject, got %d", got.CommentCount)
	}
}

func TestReviewDoesNotReviveWithdrawnPost(t *testing.T) {
	env := newManageTestEnv(t)
	// 发帖即命中敏感词，进入待审
	post, _, blocked, err := env.posts.Create(1, "标题", "赌博内容", nil, nil)
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if !blocked {
		t.Fatal("expected blocked post")
	}
	// 作者在审核前撤回
	if err := env.posts.Withdraw(1, post.ID); err != nil {
		t.Fatalf("withdraw post: %v", err)
	}
	// 审核放行不应复活已撤回的帖子
	items, total, err := env.review.List(1, 10, constants.ReviewStatusPending)
	if err != nil || total != 1 {
		t.Fatalf("expected 1 pending review, got total=%d err=%v", total, err)
	}
	if err := env.review.Approve(items[0].ID, 99, "放行"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	got, _ := env.postRepo.FindByID(post.ID)
	if got.Status != constants.PostStatusWithdrawn {
		t.Fatalf("expected post to stay withdrawn, got %d", got.Status)
	}
}
