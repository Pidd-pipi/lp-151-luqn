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

type AdminHandler struct {
	review    service.ReviewService
	posts     service.PostService
	sensitive service.SensitiveWordService
	tags      service.TagService
	logger    *slog.Logger
}

func NewAdminHandler(review service.ReviewService, posts service.PostService, sensitive service.SensitiveWordService, tags service.TagService, logger *slog.Logger) *AdminHandler {
	return &AdminHandler{review: review, posts: posts, sensitive: sensitive, tags: tags, logger: logger}
}

// ListReviews 审核队列
// @Summary 审核队列
// @Tags admin
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param status query int false "状态 1待审 2通过 3拒绝"
// @Success 200 {object} Response
// @Router /api/v1/admin/reviews [get]
func (h *AdminHandler) ListReviews(c *gin.Context) {
	var req dto.ListReviewRequest
	if !BindQuery(c, &req) {
		return
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	items, total, err := h.review.List(req.Page, req.PageSize, req.Status)
	if err != nil {
		h.logger.Error("list reviews", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "list reviews failed")
		return
	}
	responses := make([]dto.ReviewItemResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toReviewItemResponse(&item))
	}
	OK(c, dto.PageResult{Items: responses, Total: total, Page: req.Page, PageSize: req.PageSize})
}

// ReviewItem 审核放行/屏蔽
// @Summary 审核放行/屏蔽
// @Tags admin
// @Accept json
// @Produce json
// @Param request body dto.ReviewRequest true "审核操作"
// @Success 200 {object} Response
// @Router /api/v1/admin/reviews/action [post]
func (h *AdminHandler) ReviewItem(c *gin.Context) {
	var req dto.ReviewRequest
	if !BindAndValidate(c, &req) {
		return
	}
	adminID := c.GetUint("identityId")
	var err error
	switch req.Action {
	case "approve":
		err = h.review.Approve(req.QueueID, adminID, req.Note)
	case "reject":
		err = h.review.Reject(req.QueueID, adminID, req.Note)
	default:
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "invalid action")
		return
	}
	if err != nil {
		h.logger.Error("review action", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "review action failed")
		return
	}
	OK(c, gin.H{"action": req.Action})
}

// ListSensitiveWords 敏感词列表
// @Summary 敏感词列表
// @Tags admin
// @Produce json
// @Success 200 {object} Response
// @Router /api/v1/admin/sensitive-words [get]
func (h *AdminHandler) ListSensitiveWords(c *gin.Context) {
	words, err := h.sensitive.List()
	if err != nil {
		h.logger.Error("list sensitive words", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "list sensitive words failed")
		return
	}
	OK(c, words)
}

// CreateSensitiveWord 新增敏感词
// @Summary 新增敏感词
// @Tags admin
// @Accept json
// @Produce json
// @Param request body dto.CreateSensitiveWordRequest true "敏感词"
// @Success 200 {object} Response
// @Router /api/v1/admin/sensitive-words [post]
func (h *AdminHandler) CreateSensitiveWord(c *gin.Context) {
	var req dto.CreateSensitiveWordRequest
	if !BindAndValidate(c, &req) {
		return
	}
	word, err := h.sensitive.Create(req.Word)
	if err != nil {
		if errors.Is(err, service.ErrWordExists) {
			Fail(c, http.StatusConflict, constants.CodeConflict, "sensitive word exists")
			return
		}
		h.logger.Error("create sensitive word", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "create sensitive word failed")
		return
	}
	OK(c, word)
}

// DeleteSensitiveWord 删除敏感词
// @Summary 删除敏感词
// @Tags admin
// @Produce json
// @Param id path int true "敏感词ID"
// @Success 200 {object} Response
// @Router /api/v1/admin/sensitive-words/{id} [delete]
func (h *AdminHandler) DeleteSensitiveWord(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}
	if err := h.sensitive.Delete(id); err != nil {
		h.logger.Error("delete sensitive word", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "delete sensitive word failed")
		return
	}
	OK(c, nil)
}

// SetFeatured 手动推荐/取消推荐
// @Summary 手动推荐
// @Tags admin
// @Accept json
// @Produce json
// @Param request body dto.FeatureRequest true "推荐操作"
// @Success 200 {object} Response
// @Router /api/v1/admin/feature [post]
func (h *AdminHandler) SetFeatured(c *gin.Context) {
	var req dto.FeatureRequest
	if !BindAndValidate(c, &req) {
		return
	}
	if err := h.posts.SetFeatured(req.PostID, req.Featured); err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			Fail(c, http.StatusNotFound, constants.CodeNotFound, "post not found")
			return
		}
		h.logger.Error("set featured", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "set featured failed")
		return
	}
	OK(c, nil)
}

// ListAllTags 标签维护（管理员）
// @Summary 标签维护
// @Tags admin
// @Produce json
// @Success 200 {object} Response
// @Router /api/v1/admin/tags [get]
func (h *AdminHandler) ListAllTags(c *gin.Context) {
	tags, err := h.tags.List()
	if err != nil {
		h.logger.Error("list tags", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "list tags failed")
		return
	}
	OK(c, tags)
}

func toReviewItemResponse(item *model.ReviewQueue) dto.ReviewItemResponse {
	return dto.ReviewItemResponse{
		ID:         item.ID,
		TargetType: item.TargetType,
		TargetID:   item.TargetID,
		Content:    item.Content,
		Status:     item.Status,
		HitWords:   item.HitWords,
		ReviewNote: item.ReviewNote,
		CreatedAt:  item.CreatedAt.Format(time.RFC3339),
	}
}
