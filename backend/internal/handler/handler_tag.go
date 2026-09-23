package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gbtreehole/backend/internal/constants"
	"github.com/gbtreehole/backend/internal/dto"
	"github.com/gbtreehole/backend/internal/service"
)

type TagHandler struct {
	tags   service.TagService
	logger *slog.Logger
}

func NewTagHandler(tags service.TagService, logger *slog.Logger) *TagHandler {
	return &TagHandler{tags: tags, logger: logger}
}

// ListTags 标签列表
// @Summary 标签列表
// @Tags tag
// @Produce json
// @Success 200 {object} Response
// @Router /api/v1/tags [get]
func (h *TagHandler) ListTags(c *gin.Context) {
	tags, err := h.tags.List()
	if err != nil {
		h.logger.Error("list tags", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "list tags failed")
		return
	}
	items := make([]dto.TagResponse, 0, len(tags))
	for _, tag := range tags {
		items = append(items, dto.TagResponse{ID: tag.ID, Name: tag.Name, PostCount: tag.PostCount})
	}
	OK(c, items)
}

// CreateTag 创建标签
// @Summary 创建标签
// @Tags tag
// @Accept json
// @Produce json
// @Param request body dto.CreateTagRequest true "标签"
// @Success 200 {object} Response
// @Router /api/v1/tags [post]
func (h *TagHandler) CreateTag(c *gin.Context) {
	var req dto.CreateTagRequest
	if !BindAndValidate(c, &req) {
		return
	}
	tag, err := h.tags.GetOrCreate(req.Name)
	if err != nil {
		h.logger.Error("create tag", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "create tag failed")
		return
	}
	OK(c, dto.TagResponse{ID: tag.ID, Name: tag.Name, PostCount: tag.PostCount})
}

func formatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}
