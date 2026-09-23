package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/gbtreehole/backend/internal/constants"
	"github.com/gbtreehole/backend/internal/handler"
	"github.com/gbtreehole/backend/internal/middleware"
	"github.com/gbtreehole/backend/internal/model"
	"github.com/gbtreehole/backend/internal/repository"
	"github.com/gbtreehole/backend/internal/router"
	"github.com/gbtreehole/backend/internal/service"
)

type moderationStack struct {
	engine    *gin.Engine
	db        *gorm.DB
	ownerTok  string
	otherTok  string
	postSvc   service.PostService
	reviewSvc service.ReviewService
}

// stubSensitive 命中「违禁」即拦截。
type stubSensitive struct{}

func (stubSensitive) Create(string) (*model.SensitiveWord, error) { return &model.SensitiveWord{}, nil }
func (stubSensitive) Delete(uint) error                           { return nil }
func (stubSensitive) List() ([]model.SensitiveWord, error) {
	return []model.SensitiveWord{{ID: 1, Word: "违禁"}}, nil
}
func (stubSensitive) Detect(content string) ([]string, bool) {
	if bytes.Contains([]byte(content), []byte("违禁")) {
		return []string{"违禁"}, true
	}
	return nil, false
}

func setupModerationStack(t *testing.T) *moderationStack {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	db.Create(&model.UserIdentity{ID: 1, IdentityKey: "k1", Nickname: "作者", CreatedAt: now, UpdatedAt: now})
	db.Create(&model.UserIdentity{ID: 2, IdentityKey: "k2", Nickname: "路人", CreatedAt: now, UpdatedAt: now})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	postRepo := repository.NewPostRepository(db)
	commentRepo := repository.NewCommentRepository(db)
	tagRepo := repository.NewTagRepository(db)
	likeRepo := repository.NewLikeRepository(db)
	reviewRepo := repository.NewReviewQueueRepository(db)

	tokenSvc := service.NewTokenService("test-secret", 60)
	identitySvc := service.NewIdentityService(repository.NewIdentityRepository(db), tokenSvc, logger)
	tagSvc := service.NewTagService(tagRepo)
	var sens service.SensitiveWordService = stubSensitive{}
	reviewSvc := service.NewReviewService(reviewRepo, postRepo, commentRepo, logger)
	postSvc := service.NewPostService(postRepo, tagSvc, sens, reviewSvc, logger)
	commentSvc := service.NewCommentService(commentRepo, postRepo, sens, reviewSvc, logger)
	likeSvc := service.NewLikeService(likeRepo, postRepo, commentRepo, logger)

	authH := handler.NewAuthHandler(identitySvc, logger)
	postH := handler.NewPostHandler(postSvc, likeSvc, reviewSvc, logger)
	commentH := handler.NewCommentHandler(commentSvc, likeSvc, reviewSvc, logger)
	likeH := handler.NewLikeHandler(likeSvc, logger)
	tagH := handler.NewTagHandler(tagSvc, logger)
	adminH := handler.NewAdminHandler(reviewSvc, postSvc, sens, tagSvc, logger)

	identityMW := middleware.NewIdentityAuthMiddleware(tokenSvc)
	sensitiveMW := middleware.NewSensitiveWordMiddleware(sens, logger)

	engine := router.New(logger, authH, postH, commentH, tagH, likeH, adminH, identityMW, sensitiveMW)

	ownerTok, err := tokenSvc.Sign(1, "k1")
	if err != nil {
		t.Fatalf("sign owner token: %v", err)
	}
	otherTok, err := tokenSvc.Sign(2, "k2")
	if err != nil {
		t.Fatalf("sign other token: %v", err)
	}
	return &moderationStack{
		engine: engine, db: db, ownerTok: ownerTok, otherTok: otherTok,
		postSvc: postSvc, reviewSvc: reviewSvc,
	}
}

func (s *moderationStack) do(method, path, token string, body any) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var resp struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return map[string]any{"code": float64(resp.Code), "message": resp.Message, "data": resp.Data}
}

// TestEndToEndOwnershipAndModeration 完整验证权限拒绝、编辑重审、审核联动与撤回落链。
func TestEndToEndOwnershipAndModeration(t *testing.T) {
	s := setupModerationStack(t)

	// 1. 作者发帖
	w := s.do(http.MethodPost, "/api/v1/posts", s.ownerTok, map[string]any{"content": "今天好累", "tags": []string{"吐槽"}})
	if w.Code != http.StatusOK {
		t.Fatalf("create post: %d %s", w.Code, w.Body.String())
	}
	var created struct {
		Post struct {
			ID uint `json:"id"`
		} `json:"post"`
	}
	json.Unmarshal(decode(t, w)["data"].(json.RawMessage), &created)
	postID := created.Post.ID
	if postID == 0 {
		t.Fatal("missing post id")
	}

	// 2. 路人评论
	w = s.do(http.MethodPost, "/api/v1/comments", s.otherTok, map[string]any{"postId": postID, "content": "抱抱"})
	if w.Code != http.StatusOK {
		t.Fatalf("create comment: %d %s", w.Code, w.Body.String())
	}
	var commentCreated struct {
		Comment struct {
			ID uint `json:"id"`
		} `json:"comment"`
	}
	json.Unmarshal(decode(t, w)["data"].(json.RawMessage), &commentCreated)
	commentID := commentCreated.Comment.ID

	// 3. 非作者编辑/撤回帖子 -> 403
	w = s.do(http.MethodPut, "/api/v1/posts/"+itoa(postID), s.otherTok, map[string]any{"content": "篡改"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("other edit post status = %d", w.Code)
	}
	w = s.do(http.MethodPost, "/api/v1/posts/"+itoa(postID)+"/withdraw", s.otherTok, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("other withdraw post status = %d", w.Code)
	}
	// 未登录 -> 401
	w = s.do(http.MethodPut, "/api/v1/posts/"+itoa(postID), "", map[string]any{"content": "x"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anon edit post status = %d", w.Code)
	}

	// 4. 非作者编辑/撤回评论 -> 403
	w = s.do(http.MethodPut, "/api/v1/comments/"+itoa(commentID), s.ownerTok, map[string]any{"content": "改评论"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("owner edits other comment status = %d", w.Code)
	}
	w = s.do(http.MethodPost, "/api/v1/comments/"+itoa(commentID)+"/withdraw", s.ownerTok, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("owner withdraws other comment status = %d", w.Code)
	}

	// 5. 作者编辑帖子命中敏感词 -> 审核中，列表隐藏、详情仅作者可见
	w = s.do(http.MethodPut, "/api/v1/posts/"+itoa(postID), s.ownerTok, map[string]any{"content": "好累，违禁东西真多"})
	if w.Code != http.StatusOK {
		t.Fatalf("owner edit post: %d %s", w.Code, w.Body.String())
	}
	var editResp struct {
		Blocked bool `json:"blocked"`
		Post    struct {
			Status       int `json:"status"`
			ReviewStatus int `json:"reviewStatus"`
		} `json:"post"`
	}
	json.Unmarshal(decode(t, w)["data"].(json.RawMessage), &editResp)
	if !editResp.Blocked || editResp.Post.Status != constants.PostStatusPending || editResp.Post.ReviewStatus != constants.ReviewStatusPending {
		t.Fatalf("edit not pending: %+v", editResp)
	}
	// 列表为空
	w = s.do(http.MethodGet, "/api/v1/posts", "", nil)
	var list struct {
		Total int `json:"total"`
		Items []struct {
			ID uint `json:"id"`
		} `json:"items"`
	}
	json.Unmarshal(decode(t, w)["data"].(json.RawMessage), &list)
	if list.Total != 0 || len(list.Items) != 0 {
		t.Fatalf("pending post in feed: total=%d", list.Total)
	}
	// 路人看详情 404
	if w = s.do(http.MethodGet, "/api/v1/posts/"+itoa(postID), s.otherTok, nil); w.Code != http.StatusNotFound {
		t.Fatalf("other views pending post status = %d", w.Code)
	}
	// 作者看详情 200 且带审核状态
	w = s.do(http.MethodGet, "/api/v1/posts/"+itoa(postID), s.ownerTok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("owner views pending post status = %d", w.Code)
	}
	var detail struct {
		Status       int    `json:"status"`
		ReviewStatus int    `json:"reviewStatus"`
		HitWords     string `json:"hitWords"`
	}
	json.Unmarshal(decode(t, w)["data"].(json.RawMessage), &detail)
	if detail.ReviewStatus != constants.ReviewStatusPending || detail.HitWords != "违禁" {
		t.Fatalf("detail review info: %+v", detail)
	}

	// 6. 管理员放行后：列表/路人详情恢复
	var queue struct {
		Items []struct {
			ID uint `json:"id"`
		} `json:"items"`
	}
	w = s.do(http.MethodGet, "/api/v1/admin/reviews?status=1", s.ownerTok, nil)
	json.Unmarshal(decode(t, w)["data"].(json.RawMessage), &queue)
	if len(queue.Items) != 1 {
		t.Fatalf("review queue len = %d", len(queue.Items))
	}
	w = s.do(http.MethodPost, "/api/v1/admin/reviews/action", s.ownerTok,
		map[string]any{"queueId": queue.Items[0].ID, "action": "approve", "note": "没问题"})
	if w.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", w.Code, w.Body.String())
	}
	w = s.do(http.MethodGet, "/api/v1/posts/"+itoa(postID), s.otherTok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("other views approved post status = %d", w.Code)
	}
	w = s.do(http.MethodGet, "/api/v1/posts/"+itoa(postID), s.ownerTok, nil)
	json.Unmarshal(decode(t, w)["data"].(json.RawMessage), &detail)
	if detail.Status != constants.PostStatusPublished || detail.ReviewStatus != constants.ReviewStatusApproved {
		t.Fatalf("post after approve: status=%d review=%d", detail.Status, detail.ReviewStatus)
	}

	// 7. 评论编辑命中敏感词：从列表隐藏、评论数减少；放行后恢复
	w = s.do(http.MethodPut, "/api/v1/comments/"+itoa(commentID), s.otherTok, map[string]any{"content": "违禁抱抱"})
	if w.Code != http.StatusOK {
		t.Fatalf("edit comment: %d %s", w.Code, w.Body.String())
	}
	w = s.do(http.MethodGet, "/api/v1/posts/"+itoa(postID), s.ownerTok, nil)
	json.Unmarshal(decode(t, w)["data"].(json.RawMessage), &detail)
	// 帖子 commentCount 应降为 0
	var postDetail struct {
		CommentCount int `json:"commentCount"`
	}
	json.Unmarshal(decode(t, w)["data"].(json.RawMessage), &postDetail)
	if postDetail.CommentCount != 0 {
		t.Fatalf("commentCount after pending edit = %d", postDetail.CommentCount)
	}
	// 路人（评论作者）自己仍能看到该评论
	w = s.do(http.MethodGet, "/api/v1/posts/"+itoa(postID)+"/comments", s.otherTok, nil)
	var commentList struct {
		Total int `json:"total"`
	}
	json.Unmarshal(decode(t, w)["data"].(json.RawMessage), &commentList)
	if commentList.Total != 1 {
		t.Fatalf("owner sees own pending comment total = %d", commentList.Total)
	}
	// 其他访客看不到
	w = s.do(http.MethodGet, "/api/v1/posts/"+itoa(postID)+"/comments", s.ownerTok, nil)
	json.Unmarshal(decode(t, w)["data"].(json.RawMessage), &commentList)
	if commentList.Total != 0 {
		t.Fatalf("guest sees pending comment total = %d", commentList.Total)
	}

	// 8. 审核拒绝后：评论保持隐藏，作者仍能看到被屏蔽状态
	w = s.do(http.MethodGet, "/api/v1/admin/reviews?status=1", s.ownerTok, nil)
	json.Unmarshal(decode(t, w)["data"].(json.RawMessage), &queue)
	w = s.do(http.MethodPost, "/api/v1/admin/reviews/action", s.ownerTok,
		map[string]any{"queueId": queue.Items[0].ID, "action": "reject", "note": "违规"})
	if w.Code != http.StatusOK {
		t.Fatalf("reject: %d %s", w.Code, w.Body.String())
	}
	w = s.do(http.MethodGet, "/api/v1/posts/"+itoa(postID)+"/comments", s.otherTok, nil)
	var ownComments struct {
		Items []struct {
			Status       int    `json:"status"`
			ReviewStatus int    `json:"reviewStatus"`
			ReviewNote   string `json:"reviewNote"`
		} `json:"items"`
	}
	json.Unmarshal(decode(t, w)["data"].(json.RawMessage), &ownComments)
	if len(ownComments.Items) != 1 || ownComments.Items[0].Status != constants.CommentStatusRejected ||
		ownComments.Items[0].ReviewStatus != constants.ReviewStatusRejected || ownComments.Items[0].ReviewNote != "违规" {
		t.Fatalf("rejected comment view: %+v", ownComments.Items)
	}

	// 9. 作者撤回评论：评论数同步减少（此时本就为 0，幂等不报错）
	w = s.do(http.MethodPost, "/api/v1/comments/"+itoa(commentID)+"/withdraw", s.otherTok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("withdraw comment: %d %s", w.Code, w.Body.String())
	}
	// 再撤回 -> 404
	w = s.do(http.MethodPost, "/api/v1/comments/"+itoa(commentID)+"/withdraw", s.otherTok, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("double withdraw comment status = %d", w.Code)
	}

	// 10. 作者撤回帖子：级联落链，详情/列表 404
	w = s.do(http.MethodPost, "/api/v1/posts/"+itoa(postID)+"/withdraw", s.ownerTok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("withdraw post: %d %s", w.Code, w.Body.String())
	}
	if w = s.do(http.MethodGet, "/api/v1/posts/"+itoa(postID), s.ownerTok, nil); w.Code != http.StatusNotFound {
		t.Fatalf("owner views withdrawn post status = %d", w.Code)
	}
	if w = s.do(http.MethodGet, "/api/v1/posts/"+itoa(postID)+"/comments", s.ownerTok, nil); w.Code != http.StatusNotFound {
		t.Fatalf("comments of withdrawn post status = %d", w.Code)
	}
	var withdrawnRows, commentLeft, likeLeft, tagLink int64
	dbModel := s.db
	dbModel.Model(&model.Post{}).Where("id = ? AND status = ?", postID, constants.PostStatusWithdrawn).Count(&withdrawnRows)
	dbModel.Model(&model.Comment{}).Where("post_id = ? AND status <> ?", postID, constants.CommentStatusWithdrawn).Count(&commentLeft)
	dbModel.Model(&model.Like{}).Where("target_type = ? AND target_id = ?", "post", postID).Count(&likeLeft)
	dbModel.Model(&model.PostTag{}).Where("post_id = ?", postID).Count(&tagLink)
	if withdrawnRows != 1 {
		t.Fatalf("withdrawn post row missing: %d", withdrawnRows)
	}
	if commentLeft != 0 || tagLink != 0 || likeLeft != 0 {
		t.Fatalf("cascade left over: comments=%d likes=%d tagLinks=%d", commentLeft, likeLeft, tagLink)
	}

	// 对已撤回内容点赞应被拒绝
	if w = s.do(http.MethodPost, "/api/v1/likes/toggle", s.ownerTok,
		map[string]any{"targetType": "post", "targetId": postID}); w.Code != http.StatusForbidden {
		t.Fatalf("like withdrawn post status = %d", w.Code)
	}
}

func itoa(id uint) string {
	if id == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = byte('0' + id%10)
		id /= 10
	}
	return string(buf[i:])
}
