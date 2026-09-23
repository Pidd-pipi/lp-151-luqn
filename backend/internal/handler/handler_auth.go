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

type AuthHandler struct {
	identities service.IdentityService
	logger     *slog.Logger
}

func NewAuthHandler(identities service.IdentityService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{identities: identities, logger: logger}
}

// CreateIdentity 创建匿名身份，返回身份信息和 JWT
// @Summary 创建匿名身份
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.CreateIdentityRequest false "昵称与头像"
// @Success 200 {object} Response
// @Router /api/v1/auth/identities [post]
func (h *AuthHandler) CreateIdentity(c *gin.Context) {
	var req dto.CreateIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "invalid request body")
		return
	}
	identity, err := h.identities.Create(req.Nickname, req.Avatar)
	if err != nil {
		h.logger.Error("create identity", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "create identity failed")
		return
	}
	token, err := h.identities.GetToken(identity)
	if err != nil {
		h.logger.Error("sign token", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "sign token failed")
		return
	}
	OK(c, gin.H{"identity": toIdentityResponse(identity), "token": token})
}

// LoginIdentity 使用身份密钥换取 JWT
// @Summary 使用身份密钥登录
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.BindIdentityRequest true "身份密钥"
// @Success 200 {object} Response
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) LoginIdentity(c *gin.Context) {
	var req dto.BindIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "invalid request body")
		return
	}
	identity, err := h.identities.GetByKey(req.IdentityKey)
	if err != nil {
		if errors.Is(err, service.ErrIdentityNotFound) {
			Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, "identity not found")
			return
		}
		h.logger.Error("login identity", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "login failed")
		return
	}
	token, err := h.identities.GetToken(identity)
	if err != nil {
		h.logger.Error("sign token", "error", err)
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, "sign token failed")
		return
	}
	OK(c, gin.H{"identity": toIdentityResponse(identity), "token": token})
}

func toIdentityResponse(identity *model.UserIdentity) dto.IdentityResponse {
	return dto.IdentityResponse{
		ID:          identity.ID,
		IdentityKey: identity.IdentityKey,
		Nickname:    identity.Nickname,
		Avatar:      identity.Avatar,
		CreatedAt:   identity.CreatedAt.Format(time.RFC3339),
	}
}
