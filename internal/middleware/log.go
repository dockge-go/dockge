package middleware

import (
	"time"

	"dockge/pkg/log"

	"github.com/gin-gonic/gin"
)

// RequestLogger 记录每个请求的方法、路径、状态码与耗时。
func RequestLogger(logger *log.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		ctx.Next()
		logger.WithContext(ctx).Info().
			Str("method", ctx.Request.Method).
			Str("path", ctx.Request.URL.Path).
			Int("status", ctx.Writer.Status()).
			Dur("latency", time.Since(start)).
			Msg("dockge request")
	}
}
