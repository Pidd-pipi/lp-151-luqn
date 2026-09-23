package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gbtreehole/backend/internal/constants"
	"github.com/gbtreehole/backend/internal/dto"
	"github.com/gbtreehole/backend/internal/model"
	"github.com/gbtreehole/backend/internal/service"
)

type PostHandler struct {
	posts  service.PostService
	likes  service.LikeService
	logger *slog.Logger
}

func NewPostHandler(posts service.PostService, likes service.LikeService, logger *slog.Logger) *PostHandler {
	return &PostHandler{posts: posts, likes: likes, logger: logger}
}

// CreatePost 发布帖子
// @Summary 发布帖子
// @Tags post
// @Accept json
// @Produce json
// @Param request body dto.CreatePostRequest true "帖子内容"
// @Success 200 {object} Response
// @Router /api/v1/posts [post]
func (h *PostHandler) CreatePost(c *gin.Context) {
	identityID := c.GetUint("identityId")
	var req dto.CreatePostRequest
	if !BindAndValidate(c, &req) {
		return
	}
	post, hits, blocked, err := h.posts.Create(identityID, req.Title, req.Content, req.Images, req.Tags)
	if err != nil {
		h.logger.Error("create post", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "create post failed")
		return
	}
	resp := toPostResponse(post, false)
	OK(c, gin.H{"post": resp, "blocked": blocked, "hitWords": hits})
}

// UpdatePost 编辑自己的帖子
// @Summary 编辑帖子（仅作者）
// @Tags post
// @Accept json
// @Produce json
// @Param id path int true "帖子ID"
// @Param request body dto.UpdatePostRequest true "帖子内容"
// @Success 200 {object} Response
// @Router /api/v1/posts/{id} [put]
func (h *PostHandler) UpdatePost(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}
	identityID := c.GetUint("identityId")
	var req dto.UpdatePostRequest
	if !BindAndValidate(c, &req) {
		return
	}
	post, hits, blocked, err := h.posts.Update(identityID, id, req.Title, req.Content, req.Images, req.Tags)
	if err != nil {
		h.writePostManageError(c, err, "update post")
		return
	}
	OK(c, gin.H{"post": toPostResponse(post, false), "blocked": blocked, "hitWords": hits})
}

// WithdrawPost 撤回自己的帖子
// @Summary 撤回帖子（仅作者）
// @Tags post
// @Produce json
// @Param id path int true "帖子ID"
// @Success 200 {object} Response
// @Router /api/v1/posts/{id} [delete]
func (h *PostHandler) WithdrawPost(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}
	if err := h.posts.Withdraw(c.GetUint("identityId"), id); err != nil {
		h.writePostManageError(c, err, "withdraw post")
		return
	}
	OK(c, nil)
}

// writePostManageError 将编辑/撤回的业务错误映射为标准响应。
func (h *PostHandler) writePostManageError(c *gin.Context, err error, op string) {
	switch {
	case errors.Is(err, service.ErrPostNotFound):
		Fail(c, http.StatusNotFound, constants.CodeNotFound, "post not found")
	case errors.Is(err, service.ErrForbidden):
		Fail(c, http.StatusForbidden, constants.CodeForbidden, "只能操作自己的帖子")
	case errors.Is(err, service.ErrPostWithdrawn):
		Fail(c, http.StatusConflict, constants.CodeConflict, "post already withdrawn")
	default:
		h.logger.Error(op, "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, op+" failed")
	}
}

// ListPosts 最新/标签/精选帖子列表
// @Summary 帖子列表
// @Tags post
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param tag_id query int false "标签ID"
// @Param featured query bool false "是否精选"
// @Success 200 {object} Response
// @Router /api/v1/posts [get]
func (h *PostHandler) ListPosts(c *gin.Context) {
	var req dto.ListPostRequest
	if !BindQuery(c, &req) {
		return
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	posts, total, err := h.posts.List(req.Page, req.PageSize, req.TagID, req.Featured)
	if err != nil {
		h.logger.Error("list posts", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "list posts failed")
		return
	}
	items := h.buildPostResponses(posts, c.GetUint("identityId"))
	OK(c, dto.PageResult{Items: items, Total: total, Page: req.Page, PageSize: req.PageSize})
}

// GetPost 帖子详情
// @Summary 帖子详情
// @Tags post
// @Produce json
// @Param id path int true "帖子ID"
// @Success 200 {object} Response
// @Router /api/v1/posts/{id} [get]
func (h *PostHandler) GetPost(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}
	post, err := h.posts.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			Fail(c, http.StatusNotFound, constants.CodeNotFound, "post not found")
			return
		}
		h.logger.Error("get post", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "get post failed")
		return
	}
	identityID := c.GetUint("identityId")
	// 已撤回的帖子对所有人不可见；待审/未通过的帖子仅作者本人可见（用于查看处理状态）
	if post.Status == constants.PostStatusWithdrawn ||
		(post.Status != constants.PostStatusPublished && post.IdentityID != identityID) {
		Fail(c, http.StatusNotFound, constants.CodeNotFound, "post not found")
		return
	}
	_ = h.posts.IncrementView(id)
	resp := toPostResponse(post, identityID > 0 && h.isLiked(identityID, "post", id))
	OK(c, resp)
}

// HotPosts 热门帖子
// @Summary 热门帖子
// @Tags post
// @Produce json
// @Success 200 {object} Response
// @Router /api/v1/posts/hot [get]
func (h *PostHandler) HotPosts(c *gin.Context) {
	posts, err := h.posts.ListHot(50)
	if err != nil {
		h.logger.Error("hot posts", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "hot posts failed")
		return
	}
	OK(c, h.buildPostResponses(posts, c.GetUint("identityId")))
}

// FeaturedPosts 每日精选
// @Summary 每日精选
// @Tags post
// @Produce json
// @Success 200 {object} Response
// @Router /api/v1/posts/featured [get]
func (h *PostHandler) FeaturedPosts(c *gin.Context) {
	posts, err := h.posts.DailyFeatured(10)
	if err != nil {
		h.logger.Error("featured posts", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "featured posts failed")
		return
	}
	OK(c, h.buildPostResponses(posts, c.GetUint("identityId")))
}

func (h *PostHandler) buildPostResponses(posts []model.Post, identityID uint) []dto.PostResponse {
	ids := make([]uint, 0, len(posts))
	for _, p := range posts {
		ids = append(ids, p.ID)
	}
	likedMap := map[uint]bool{}
	if identityID > 0 {
		if m, err := h.likes.IsLiked(identityID, "post", ids); err == nil {
			likedMap = m
		}
	}
	items := make([]dto.PostResponse, 0, len(posts))
	for _, p := range posts {
		items = append(items, toPostResponse(&p, likedMap[p.ID]))
	}
	return items
}

func (h *PostHandler) isLiked(identityID uint, targetType string, targetID uint) bool {
	m, err := h.likes.IsLiked(identityID, targetType, []uint{targetID})
	if err != nil {
		return false
	}
	return m[targetID]
}

func toPostResponse(post *model.Post, liked bool) dto.PostResponse {
	resp := dto.PostResponse{
		ID:           post.ID,
		IdentityID:   post.IdentityID,
		Title:        post.Title,
		Content:      post.Content,
		Status:       post.Status,
		LikeCount:    post.LikeCount,
		CommentCount: post.CommentCount,
		ViewCount:    post.ViewCount,
		IsFeatured:   post.IsFeatured,
		Liked:        liked,
		CreatedAt:    post.CreatedAt.Format(time.RFC3339),
	}
	if post.Identity != nil {
		resp.Nickname = post.Identity.Nickname
		resp.Avatar = post.Identity.Avatar
	}
	if post.Images != "" {
		var images []string
		if err := json.Unmarshal([]byte(post.Images), &images); err == nil {
			resp.Images = images
		}
	}
	resp.Tags = make([]dto.TagResponse, 0, len(post.Tags))
	for _, tag := range post.Tags {
		resp.Tags = append(resp.Tags, dto.TagResponse{ID: tag.ID, Name: tag.Name, PostCount: tag.PostCount})
	}
	return resp
}

func parseID(c *gin.Context) uint {
	var uri struct {
		ID uint `uri:"id" binding:"required,min=1"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "invalid id")
		return 0
	}
	return uri.ID
}
