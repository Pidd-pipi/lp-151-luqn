package handler

import (
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

type CommentHandler struct {
	comments service.CommentService
	likes    service.LikeService
	logger   *slog.Logger
}

func NewCommentHandler(comments service.CommentService, likes service.LikeService, logger *slog.Logger) *CommentHandler {
	return &CommentHandler{comments: comments, likes: likes, logger: logger}
}

// CreateComment 发表评论
// @Summary 发表评论
// @Tags comment
// @Accept json
// @Produce json
// @Param request body dto.CreateCommentRequest true "评论内容"
// @Success 200 {object} Response
// @Router /api/v1/comments [post]
func (h *CommentHandler) CreateComment(c *gin.Context) {
	identityID := c.GetUint("identityId")
	var req dto.CreateCommentRequest
	if !BindAndValidate(c, &req) {
		return
	}
	comment, hits, blocked, err := h.comments.Create(identityID, req.PostID, req.Content)
	if err != nil {
		h.logger.Error("create comment", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "create comment failed")
		return
	}
	OK(c, gin.H{"comment": toCommentResponse(comment, false), "blocked": blocked, "hitWords": hits})
}

// UpdateComment 编辑自己的评论
// @Summary 编辑评论（仅作者）
// @Tags comment
// @Accept json
// @Produce json
// @Param id path int true "评论ID"
// @Param request body dto.UpdateCommentRequest true "评论内容"
// @Success 200 {object} Response
// @Router /api/v1/comments/{id} [put]
func (h *CommentHandler) UpdateComment(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}
	identityID := c.GetUint("identityId")
	var req dto.UpdateCommentRequest
	if !BindAndValidate(c, &req) {
		return
	}
	comment, hits, blocked, err := h.comments.Update(identityID, id, req.Content)
	if err != nil {
		h.writeCommentManageError(c, err, "update comment")
		return
	}
	OK(c, gin.H{"comment": toCommentResponse(comment, false), "blocked": blocked, "hitWords": hits})
}

// WithdrawComment 撤回自己的评论
// @Summary 撤回评论（仅作者）
// @Tags comment
// @Produce json
// @Param id path int true "评论ID"
// @Success 200 {object} Response
// @Router /api/v1/comments/{id} [delete]
func (h *CommentHandler) WithdrawComment(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}
	if err := h.comments.Withdraw(c.GetUint("identityId"), id); err != nil {
		h.writeCommentManageError(c, err, "withdraw comment")
		return
	}
	OK(c, nil)
}

// writeCommentManageError 将编辑/撤回的业务错误映射为标准响应。
func (h *CommentHandler) writeCommentManageError(c *gin.Context, err error, op string) {
	switch {
	case errors.Is(err, service.ErrCommentNotFound):
		Fail(c, http.StatusNotFound, constants.CodeNotFound, "comment not found")
	case errors.Is(err, service.ErrForbidden):
		Fail(c, http.StatusForbidden, constants.CodeForbidden, "只能操作自己的评论")
	case errors.Is(err, service.ErrCommentWithdrawn):
		Fail(c, http.StatusConflict, constants.CodeConflict, "comment already withdrawn")
	default:
		h.logger.Error(op, "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, op+" failed")
	}
}

// ListComments 帖子评论列表
// @Summary 评论列表
// @Tags comment
// @Produce json
// @Param id path int true "帖子ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} Response
// @Router /api/v1/posts/{id}/comments [get]
func (h *CommentHandler) ListComments(c *gin.Context) {
	postID := parseID(c)
	if postID == 0 {
		return
	}
	var req dto.ListCommentRequest
	if !BindQuery(c, &req) {
		return
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	comments, total, err := h.comments.ListByPostID(postID, req.Page, req.PageSize, c.GetUint("identityId"))
	if err != nil {
		h.logger.Error("list comments", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "list comments failed")
		return
	}
	ids := make([]uint, 0, len(comments))
	for _, cm := range comments {
		ids = append(ids, cm.ID)
	}
	likedMap := map[uint]bool{}
	if identityID := c.GetUint("identityId"); identityID > 0 {
		if m, err := h.likes.IsLiked(identityID, "comment", ids); err == nil {
			likedMap = m
		}
	}
	items := make([]dto.CommentResponse, 0, len(comments))
	for _, cm := range comments {
		items = append(items, toCommentResponse(&cm, likedMap[cm.ID]))
	}
	OK(c, dto.PageResult{Items: items, Total: total, Page: req.Page, PageSize: req.PageSize})
}

func toCommentResponse(comment *model.Comment, liked bool) dto.CommentResponse {
	resp := dto.CommentResponse{
		ID:         comment.ID,
		PostID:     comment.PostID,
		IdentityID: comment.IdentityID,
		Content:    comment.Content,
		Status:     comment.Status,
		LikeCount:  comment.LikeCount,
		Liked:      liked,
		CreatedAt:  comment.CreatedAt.Format(time.RFC3339),
	}
	if comment.Identity != nil {
		resp.Nickname = comment.Identity.Nickname
		resp.Avatar = comment.Identity.Avatar
	}
	return resp
}
