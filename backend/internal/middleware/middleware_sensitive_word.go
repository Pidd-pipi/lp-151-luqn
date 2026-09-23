package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/gbtreehole/backend/internal/service"
)

// SensitiveWordMiddleware 集成在请求链路上的敏感词检测。
// 只做探测并将命中结果写入上下文，不直接拦截：
// 被拦截内容由业务层进入审核队列，保证数据可追踪。
type SensitiveWordMiddleware struct {
	sensitive service.SensitiveWordService
	logger    *slog.Logger
}

func NewSensitiveWordMiddleware(sensitive service.SensitiveWordService, logger *slog.Logger) *SensitiveWordMiddleware {
	return &SensitiveWordMiddleware{sensitive: sensitive, logger: logger}
}

func (m *SensitiveWordMiddleware) Detect() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil {
			c.Next()
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err == nil {
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
			var payload map[string]any
			if json.Unmarshal(body, &payload) == nil {
				var text string
				for _, key := range []string{"content", "title", "note"} {
					if v, ok := payload[key].(string); ok {
						text += " " + v
					}
				}
				if text != "" {
					if hits, blocked := m.sensitive.Detect(text); blocked {
						c.Set("sensitiveHitWords", hits)
						c.Set("sensitiveBlocked", true)
						m.logger.Warn("sensitive words detected", "hits", hits, "path", c.Request.URL.Path)
					}
				}
			}
		}
		c.Next()
	}
}
