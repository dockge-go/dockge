// Package middleware 提供 dockge 应用的 HTTP 中间件。
package middleware

import (
	"net/http"
	"strings"
	"time"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/service"
	"dockge/pkg/jwt"
	"dockge/pkg/log"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// ctxProxyAuthDone 标记 proxy auth 已完成，StrictAuth 据此跳过 JWT 校验。
const ctxProxyAuthDone = "proxyAuthDone"

// proxyTokenTTL 是 proxy 模式下签发的 JWT 有效期。
const proxyTokenTTL = time.Hour * 24 * 7

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
// 若 ctxProxyAuthDone 已设置（proxy/oidc 模式由上游中间件完成认证），则跳过校验。
func StrictAuth(j *jwt.JWT, logger *log.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.GetBool(ctxProxyAuthDone) {
			ctx.Next()
			return
		}
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

// ProxyAuth 反向代理认证中间件：从指定请求头读取用户名，自动 provision 本地账号后签发 JWT。
// 当 security.auth.mode != "proxy" 时不做任何拦截（调用方负责路由挂载条件）。
// 认证成功后在 ctx 写入 claims 并设置 httpOnly JWT cookie；StrictAuth 据此跳过 JWT 校验。
func ProxyAuth(authService service.AuthService, logger *log.Logger, conf *viper.Viper) gin.HandlerFunc {
	headerKey := conf.GetString("security.auth.proxy.username_header")
	if headerKey == "" {
		headerKey = "X-Forwarded-User"
	}

	return func(ctx *gin.Context) {
		if conf.GetString("security.auth.mode") != "proxy" {
			ctx.Next()
			return
		}
		username := ctx.Request.Header.Get(headerKey)
		if username == "" {
			v1.HandleError(ctx, http.StatusUnauthorized, v1.ErrUnauthorized, nil)
			ctx.Abort()
			return
		}
		username = strings.TrimSpace(username)
		headers := make(map[string]string)
		for k, v := range ctx.Request.Header {
			headers[k] = v[0]
		}
		data, err := authService.ProxyLogin(ctx, headers, ctx.Request.RemoteAddr)
		if err != nil {
			logger.WithContext(ctx).Warn().Str("username", username).Err(err).Msg("proxy auth failed")
			v1.HandleError(ctx, http.StatusUnauthorized, v1.ErrUnauthorized, nil)
			ctx.Abort()
			return
		}
		// 将 JWT 写入 httpOnly cookie，同时写入 ctx claims 供后续 handler 使用。
		ctx.SetSameSite(http.SameSiteLaxMode)
		ctx.SetCookie("dockge_token", data.AccessToken, int(proxyTokenTTL.Seconds()), "/", "", false, true)
		ctx.Set("claims", &jwt.Claims{UserId: data.User.ID})
		ctx.Set(ctxProxyAuthDone, true)
		ctx.Next()
	}
}

// SetupRequired 首次安装守卫：当尚未创建任何用户时，
// 除 setup 引导接口、健康检查和 robots.txt 外，全部返回 403。
// 由 http.go 在主路由组上统一挂载，无需重复挂载到每个子路由。
func SetupRequired(authService service.AuthService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		path := ctx.Request.URL.Path
		// 放行：setup 引导、健康检查、robots.txt
		switch {
		case path == "/v1/setup" || path == "/v1/setup/need":
		case path == "/v1/health":
		case path == "/robots.txt":
		case path == "/v1/auto-login": // 免登录模式下的自动登录也需要在此阶段可用
		default:
			need, err := authService.CheckNeedSetup(ctx)
			if err != nil {
				v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
				ctx.Abort()
				return
			}
			if need {
				v1.HandleError(ctx, http.StatusForbidden, &v1.Error{Code: 403, Message: "需要先完成初始化设置"}, nil)
				ctx.Abort()
				return
			}
		}
		ctx.Next()
	}
}
