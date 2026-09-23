package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gbtreehole/backend/internal/constants"
	"github.com/gbtreehole/backend/internal/service"
)

type IdentityAuthMiddleware struct {
	token service.TokenService
}

func NewIdentityAuthMiddleware(token service.TokenService) *IdentityAuthMiddleware {
	return &IdentityAuthMiddleware{token: token}
}

// RequireAuth 校验 Bearer JWT，将身份信息写入上下文。
func (m *IdentityAuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		tokenString := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    constants.CodeUnauthorized,
				"message": "missing bearer token",
				"data":    nil,
			})
			return
		}
		claims, err := m.token.Parse(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    constants.CodeUnauthorized,
				"message": "invalid token",
				"data":    nil,
			})
			return
		}
		c.Set("identityId", claims.IdentityID)
		c.Set("identityKey", claims.IdentityKey)
		c.Next()
	}
}

// OptionalAuth 允许匿名访问，若带有效 token 则写入身份上下文。
func (m *IdentityAuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		tokenString := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if tokenString == "" {
			c.Next()
			return
		}
		if claims, err := m.token.Parse(tokenString); err == nil {
			c.Set("identityId", claims.IdentityID)
			c.Set("identityKey", claims.IdentityKey)
		}
		c.Next()
	}
}
