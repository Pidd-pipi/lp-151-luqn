package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/gbtreehole/backend/internal/handler"
	"github.com/gbtreehole/backend/internal/middleware"
	"github.com/gbtreehole/backend/internal/model"
	"github.com/gbtreehole/backend/internal/repository"
	"github.com/gbtreehole/backend/internal/service"
)

type testEnv struct {
	engine *gin.Engine
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	identityRepo := repository.NewIdentityRepository(db)
	postRepo := repository.NewPostRepository(db)
	commentRepo := repository.NewCommentRepository(db)
	tagRepo := repository.NewTagRepository(db)
	likeRepo := repository.NewLikeRepository(db)
	sensitiveRepo := repository.NewSensitiveWordRepository(db)
	reviewRepo := repository.NewReviewQueueRepository(db)

	tokenService := service.NewTokenService("test-secret", 60)
	identityService := service.NewIdentityService(identityRepo, tokenService, logger)
	tagService := service.NewTagService(tagRepo)
	sensitiveService := service.NewSensitiveWordService(sensitiveRepo)
	if _, err := sensitiveService.Create("赌博"); err != nil {
		t.Fatalf("seed sensitive word: %v", err)
	}
	reviewService := service.NewReviewService(reviewRepo, postRepo, commentRepo, logger)
	postService := service.NewPostService(postRepo, commentRepo, likeRepo, tagService, sensitiveService, reviewService, logger)
	commentService := service.NewCommentService(commentRepo, postRepo, likeRepo, sensitiveService, reviewService, logger)
	likeService := service.NewLikeService(likeRepo, postRepo, commentRepo, logger)

	authHandler := handler.NewAuthHandler(identityService, logger)
	postHandler := handler.NewPostHandler(postService, likeService, logger)
	commentHandler := handler.NewCommentHandler(commentService, likeService, logger)
	tagHandler := handler.NewTagHandler(tagService, logger)
	likeHandler := handler.NewLikeHandler(likeService, logger)
	adminHandler := handler.NewAdminHandler(reviewService, postService, sensitiveService, tagService, logger)

	identityMW := middleware.NewIdentityAuthMiddleware(tokenService)
	sensitiveMW := middleware.NewSensitiveWordMiddleware(sensitiveService, logger)

	engine := New(logger, authHandler, postHandler, commentHandler, tagHandler, likeHandler, adminHandler, identityMW, sensitiveMW)
	return &testEnv{engine: engine}
}

func (e *testEnv) do(t *testing.T, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	return w.Code, resp
}

func dataMap(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing data object: %v", resp)
	}
	return data
}

func createIdentity(t *testing.T, env *testEnv) (float64, string) {
	t.Helper()
	code, resp := env.do(t, http.MethodPost, "/api/v1/auth/identities", "", map[string]any{})
	if code != http.StatusOK {
		t.Fatalf("create identity: code=%d resp=%v", code, resp)
	}
	data := dataMap(t, resp)
	identity := data["identity"].(map[string]any)
	return identity["id"].(float64), data["token"].(string)
}

func TestPostManageFlow(t *testing.T) {
	env := newTestEnv(t)
	_, token1 := createIdentity(t, env)
	_, token2 := createIdentity(t, env)

	// 身份1 发帖
	code, resp := env.do(t, http.MethodPost, "/api/v1/posts", token1, map[string]any{
		"title": "树洞一号", "content": "今天想吐槽一下", "tags": []string{"生活"},
	})
	if code != http.StatusOK {
		t.Fatalf("create post: code=%d resp=%v", code, resp)
	}
	postID := dataMap(t, resp)["post"].(map[string]any)["id"].(float64)

	// 非本人编辑/撤回被拒绝
	if code, _ := env.do(t, http.MethodPut, fmt.Sprintf("/api/v1/posts/%v", postID), token2, map[string]any{
		"content": "别人来改",
	}); code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner edit, got %d", code)
	}
	if code, _ := env.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/posts/%v", postID), token2, nil); code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner withdraw, got %d", code)
	}
	// 未登录请求被拒绝
	if code, _ := env.do(t, http.MethodPut, fmt.Sprintf("/api/v1/posts/%v", postID), "", map[string]any{
		"content": "匿名修改",
	}); code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous edit, got %d", code)
	}

	// 本人编辑，命中敏感词 → 进入审核
	code, resp = env.do(t, http.MethodPut, fmt.Sprintf("/api/v1/posts/%v", postID), token1, map[string]any{
		"title": "树洞一号", "content": "这里聊聊赌博的事",
	})
	if code != http.StatusOK {
		t.Fatalf("edit post: code=%d resp=%v", code, resp)
	}
	if blocked := dataMap(t, resp)["blocked"].(bool); !blocked {
		t.Fatal("expected blocked=true after sensitive edit")
	}
	// 待审中：匿名访问详情 404，作者可见状态
	if code, _ := env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/posts/%v", postID), "", nil); code != http.StatusNotFound {
		t.Fatalf("expected 404 for anonymous on pending post, got %d", code)
	}
	code, resp = env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/posts/%v", postID), token1, nil)
	if code != http.StatusOK {
		t.Fatalf("expected author can view pending post, got %d", code)
	}
	if status := dataMap(t, resp)["status"].(float64); status != 2 {
		t.Fatalf("expected status=2 (pending), got %v", status)
	}

	// 审核放行后公开可见
	code, resp = env.do(t, http.MethodGet, "/api/v1/admin/reviews?status=1", token1, nil)
	if code != http.StatusOK {
		t.Fatalf("list reviews: code=%d", code)
	}
	items := dataMap(t, resp)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 pending review, got %d", len(items))
	}
	queueID := items[0].(map[string]any)["id"].(float64)
	if code, _ := env.do(t, http.MethodPost, "/api/v1/admin/reviews/action", token1, map[string]any{
		"queueId": queueID, "action": "approve",
	}); code != http.StatusOK {
		t.Fatalf("approve review failed")
	}
	code, resp = env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/posts/%v", postID), "", nil)
	if code != http.StatusOK {
		t.Fatalf("expected approved post visible to anonymous, got %d", code)
	}
	if status := dataMap(t, resp)["status"].(float64); status != 1 {
		t.Fatalf("expected status=1 (published), got %v", status)
	}

	// 身份2 评论，评论数同步
	code, resp = env.do(t, http.MethodPost, "/api/v1/comments", token2, map[string]any{
		"postId": postID, "content": "沙发",
	})
	if code != http.StatusOK {
		t.Fatalf("create comment: code=%d resp=%v", code, resp)
	}
	commentID := dataMap(t, resp)["comment"].(map[string]any)["id"].(float64)
	_, resp = env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/posts/%v", postID), "", nil)
	if count := dataMap(t, resp)["commentCount"].(float64); count != 1 {
		t.Fatalf("expected commentCount=1, got %v", count)
	}

	// 非本人撤回评论被拒绝
	if code, _ := env.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/comments/%v", commentID), token1, nil); code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner comment withdraw, got %d", code)
	}
	// 本人撤回评论，评论数减少
	if code, _ := env.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/comments/%v", commentID), token2, nil); code != http.StatusOK {
		t.Fatalf("withdraw comment failed")
	}
	_, resp = env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/posts/%v", postID), "", nil)
	if count := dataMap(t, resp)["commentCount"].(float64); count != 0 {
		t.Fatalf("expected commentCount=0 after withdraw, got %v", count)
	}

	// 撤回帖子：详情 404，列表不再出现
	if code, _ := env.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/posts/%v", postID), token1, nil); code != http.StatusOK {
		t.Fatalf("withdraw post failed")
	}
	if code, _ := env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/posts/%v", postID), token1, nil); code != http.StatusNotFound {
		t.Fatalf("expected 404 for withdrawn post, got %d", code)
	}
	_, resp = env.do(t, http.MethodGet, "/api/v1/posts", "", nil)
	if total := dataMap(t, resp)["total"].(float64); total != 0 {
		t.Fatalf("expected 0 posts in list after withdraw, got %v", total)
	}
}

func TestCommentEditReviewFlow(t *testing.T) {
	env := newTestEnv(t)
	_, token1 := createIdentity(t, env)
	_, token2 := createIdentity(t, env)

	code, resp := env.do(t, http.MethodPost, "/api/v1/posts", token1, map[string]any{"content": "楼主的内容"})
	if code != http.StatusOK {
		t.Fatalf("create post: code=%d", code)
	}
	postID := dataMap(t, resp)["post"].(map[string]any)["id"].(float64)

	code, resp = env.do(t, http.MethodPost, "/api/v1/comments", token2, map[string]any{
		"postId": postID, "content": "一条正常评论",
	})
	if code != http.StatusOK {
		t.Fatalf("create comment: code=%d", code)
	}
	commentID := dataMap(t, resp)["comment"].(map[string]any)["id"].(float64)

	// 编辑命中敏感词 → 待审，评论数减少
	code, resp = env.do(t, http.MethodPut, fmt.Sprintf("/api/v1/comments/%v", commentID), token2, map[string]any{
		"content": "改成一个赌博话题",
	})
	if code != http.StatusOK || !dataMap(t, resp)["blocked"].(bool) {
		t.Fatalf("expected blocked comment edit, code=%d resp=%v", code, resp)
	}
	_, resp = env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/posts/%v", postID), "", nil)
	if count := dataMap(t, resp)["commentCount"].(float64); count != 0 {
		t.Fatalf("expected commentCount=0 while pending, got %v", count)
	}
	// 作者能在评论列表看到待审状态，匿名看不到
	_, resp = env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/posts/%v/comments", postID), token2, nil)
	items := dataMap(t, resp)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["status"].(float64) != 2 {
		t.Fatalf("expected author sees pending comment, got %v", items)
	}
	_, resp = env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/posts/%v/comments", postID), "", nil)
	if total := dataMap(t, resp)["total"].(float64); total != 0 {
		t.Fatalf("expected anonymous sees 0 comments, got %v", total)
	}

	// 审核拒绝 → 作者看到未通过状态
	code, resp = env.do(t, http.MethodGet, "/api/v1/admin/reviews?status=1", token1, nil)
	if code != http.StatusOK {
		t.Fatalf("list reviews: code=%d", code)
	}
	reviews := dataMap(t, resp)["items"].([]any)
	if len(reviews) != 1 {
		t.Fatalf("expected 1 pending review, got %d", len(reviews))
	}
	if code, _ := env.do(t, http.MethodPost, "/api/v1/admin/reviews/action", token1, map[string]any{
		"queueId": reviews[0].(map[string]any)["id"].(float64), "action": "reject", "note": "违规内容",
	}); code != http.StatusOK {
		t.Fatalf("reject review failed")
	}
	_, resp = env.do(t, http.MethodGet, fmt.Sprintf("/api/v1/posts/%v/comments", postID), token2, nil)
	items = dataMap(t, resp)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["status"].(float64) != 3 {
		t.Fatalf("expected author sees rejected comment, got %v", items)
	}
}
