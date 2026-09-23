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
	review   service.ReviewService
	logger   *slog.Logger
}

func NewCommentHandler(comments service.CommentService, likes service.LikeService, review service.ReviewService, logger *slog.Logger) *CommentHandler {
	return &CommentHandler{comments: comments, likes: likes, review: review, logger: logger}
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
		h.writeCommentError(c, "create comment", err)
		return
	}
	items := []dto.CommentResponse{toCommentResponse(comment, false)}
	attachCommentReviewInfo(items, identityID, h.review)
	OK(c, gin.H{"comment": items[0], "blocked": blocked, "hitWords": hits})
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
	viewerID := c.GetUint("identityId")
	comments, total, err := h.comments.ListVisibleByPostID(postID, viewerID, req.Page, req.PageSize)
	if err != nil {
		h.writeCommentError(c, "list comments", err)
		return
	}
	ids := make([]uint, 0, len(comments))
	for _, cm := range comments {
		ids = append(ids, cm.ID)
	}
	likedMap := map[uint]bool{}
	if viewerID > 0 {
		if m, err := h.likes.IsLiked(viewerID, "comment", ids); err == nil {
			likedMap = m
		}
	}
	items := make([]dto.CommentResponse, 0, len(comments))
	for _, cm := range comments {
		items = append(items, toCommentResponse(&cm, likedMap[cm.ID]))
	}
	attachCommentReviewInfo(items, viewerID, h.review)
	OK(c, dto.PageResult{Items: items, Total: total, Page: req.Page, PageSize: req.PageSize})
}

// UpdateComment 作者编辑评论
// @Summary 编辑评论
// @Tags comment
// @Accept json
// @Produce json
// @Param id path int true "评论ID"
// @Param request body dto.UpdateCommentRequest true "评论内容"
// @Success 200 {object} Response
// @Router /api/v1/comments/{id} [put]
func (h *CommentHandler) UpdateComment(c *gin.Context) {
	identityID := c.GetUint("identityId")
	id := parseID(c)
	if id == 0 {
		return
	}
	var req dto.UpdateCommentRequest
	if !BindAndValidate(c, &req) {
		return
	}
	comment, hits, blocked, err := h.comments.Update(identityID, id, req.Content)
	if err != nil {
		h.writeCommentError(c, "update comment", err)
		return
	}
	items := []dto.CommentResponse{toCommentResponse(comment, false)}
	attachCommentReviewInfo(items, identityID, h.review)
	OK(c, gin.H{"comment": items[0], "blocked": blocked, "hitWords": hits})
}

// WithdrawComment 作者撤回评论
// @Summary 撤回评论
// @Tags comment
// @Produce json
// @Param id path int true "评论ID"
// @Success 200 {object} Response
// @Router /api/v1/comments/{id}/withdraw [post]
func (h *CommentHandler) WithdrawComment(c *gin.Context) {
	identityID := c.GetUint("identityId")
	id := parseID(c)
	if id == 0 {
		return
	}
	if err := h.comments.Withdraw(identityID, id); err != nil {
		h.writeCommentError(c, "withdraw comment", err)
		return
	}
	OK(c, gin.H{"id": id, "status": constants.CommentStatusWithdrawn})
}

func (h *CommentHandler) writeCommentError(c *gin.Context, action string, err error) {
	switch {
	case errors.Is(err, service.ErrCommentNotFound) || errors.Is(err, service.ErrCommentAlreadyDeleted):
		Fail(c, http.StatusNotFound, constants.CodeNotFound, "comment not found")
	case errors.Is(err, service.ErrPostNotFound) || errors.Is(err, service.ErrPostNotVisible):
		Fail(c, http.StatusNotFound, constants.CodeNotFound, "post not found")
	case errors.Is(err, service.ErrOperationForbidden):
		Fail(c, http.StatusForbidden, constants.CodeForbidden, "operation forbidden")
	default:
		h.logger.Error(action, "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, action+" failed")
	}
}

// attachCommentReviewInfo 仅向评论作者本人附加审核处理状态。
func attachCommentReviewInfo(items []dto.CommentResponse, viewerID uint, review service.ReviewService) {
	if viewerID == 0 || len(items) == 0 {
		return
	}
	ids := make([]uint, 0, len(items))
	for _, item := range items {
		if item.IdentityID == viewerID {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	latest, err := review.LatestReviews("comment", ids)
	if err != nil {
		return
	}
	for i := range items {
		if items[i].IdentityID != viewerID {
			continue
		}
		item := latest[items[i].ID]
		if item.ID == 0 {
			continue
		}
		items[i].ReviewStatus = item.Status
		items[i].ReviewNote = item.ReviewNote
		items[i].HitWords = item.HitWords
	}
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
		UpdatedAt:  comment.UpdatedAt.Format(time.RFC3339),
	}
	if comment.Identity != nil {
		resp.Nickname = comment.Identity.Nickname
		resp.Avatar = comment.Identity.Avatar
	}
	return resp
}
