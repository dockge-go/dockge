// Package middleware 提供 dockge 应用的 HTTP 中间件。
package middleware

import (
	"net/http"
	"time"

	v1 "dockge/app/dockge/api/v1"
	"dockge/pkg/jwt"
	"dockge/pkg/log"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware 回显请求 Origin 并放行跨域；OPTIONS 预检直接短路。

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		c.Header("Access-Control-Allow-Origin", c.GetHeader("Origin"))
		c.Header("Access-Control-Allow-Credentials", "true")
		if method == "OPTIONS" {
			c.Header("Access-Control-Allow-Methods", c.GetHeader("Access-Control-Request-Method"))
			c.Header("Access-Control-Allow-Headers", c.GetHeader("Access-Control-Request-Headers"))
			c.Header("Access-Control-Max-Age", "7200")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// RequestLog 记录每个请求的方法、路径、状态码与耗时。

func RequestLog(logger *log.Logger) gin.HandlerFunc {
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

// StrictAuth 强制认证中间件：Authorization 头缺失或非法时直接 401。
// WebSocket/EventSource 无法自定义请求头，额外接受 ?token= 查询参数。
func StrictAuth(j *jwt.JWT, logger *log.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		tokenString := ctx.Request.Header.Get("Authorization")
		if tokenString == "" {
			tokenString = ctx.Query("token")
		}
		if tokenString == "" {
			v1.HandleError(ctx, http.StatusUnauthorized, v1.ErrUnauthorized, nil)
			ctx.Abort()
			return
		}
		claims, err := j.ParseToken(tokenString)
		if err != nil {
			logger.WithContext(ctx).Warn().Err(err).Msg("token error")
			v1.HandleError(ctx, http.StatusUnauthorized, v1.ErrUnauthorized, nil)
			ctx.Abort()
			return
		}
		ctx.Set("claims", claims)
		ctx.Next()
	}
}
