package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gbtreehole/backend/internal/constants"
	"github.com/gbtreehole/backend/internal/dto"
	"github.com/gbtreehole/backend/internal/service"
)

type LikeHandler struct {
	likes  service.LikeService
	logger *slog.Logger
}

func NewLikeHandler(likes service.LikeService, logger *slog.Logger) *LikeHandler {
	return &LikeHandler{likes: likes, logger: logger}
}

// ToggleLike 点赞/取消点赞
// @Summary 点赞/取消点赞
// @Tags like
// @Accept json
// @Produce json
// @Param request body dto.LikeRequest true "点赞目标"
// @Success 200 {object} Response
// @Router /api/v1/likes/toggle [post]
func (h *LikeHandler) ToggleLike(c *gin.Context) {
	identityID := c.GetUint("identityId")
	var req dto.LikeRequest
	if !BindAndValidate(c, &req) {
		return
	}
	liked, count, err := h.likes.Toggle(identityID, req.TargetType, req.TargetID)
	if err != nil {
		h.logger.Error("toggle like", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "toggle like failed")
		return
	}
	OK(c, dto.LikeResponse{Liked: liked, LikeCount: count})
}
