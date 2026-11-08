// Package middleware 提供 dockge 应用的 HTTP 中间件。
package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	v1 "dockge/api/v1"
	"dockge/pkg/jwt"
	"dockge/pkg/log"

	"github.com/gin-gonic/gin"
)

// setupChecker 是 SetupRequired 依赖的最小接口（避免耦合完整 AuthService）。
type setupChecker interface {
	CheckNeedSetup(ctx context.Context) (bool, error)
}

// sessionChecker 是 StrictAuth 逐请求会话校验依赖的最小接口
// （实现：authService.CheckSession——用户存在、启用且密码哈希未变）。
type sessionChecker interface {
	CheckSession(ctx context.Context, uid uint, h string) error
}

// StrictAuth 强制认证中间件：Authorization 头缺失或非法时直接 401。
// fetch SSE 同样携带 Authorization；不接收 URL token，避免访问日志泄漏。
// token 解析成功后逐请求校验会话（CheckSession）：账号被停用/删除或改密后，
// 旧 token 的下一次请求立即 401。
func StrictAuth(j *jwt.JWT, logger *log.Logger, sessions sessionChecker) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		tokenString := ctx.Request.Header.Get("Authorization")
		if tokenString == "" {
			v1.HandleError(ctx, http.StatusUnauthorized,
				fmt.Errorf("%w: 缺少 Authorization 请求头，需要提供 Bearer token", v1.ErrUnauthorized), nil)
			ctx.Abort()
			return
		}
		claims, err := j.ParseToken(tokenString)
		if err != nil {
			logger.WithContext(ctx).Warn().Err(err).Msg("token error")
			v1.HandleError(ctx, http.StatusUnauthorized,
				fmt.Errorf("%w: Authorization token 校验失败：%v", v1.ErrUnauthorized, err), nil)
			ctx.Abort()
			return
		}
		if sessions != nil {
			if err := sessions.CheckSession(ctx, claims.UserId, claims.H); err != nil {
				logger.WithContext(ctx).Warn().Uint("uid", claims.UserId).Err(err).Msg("session invalid")
				// 会话失效使用业务码；存储故障则为 500，不能误导用户反复登录。
				status := http.StatusInternalServerError
				var bizErr *v1.Error
				if errors.As(err, &bizErr) {
					status = bizErr.Code
				}
				v1.HandleError(ctx, status, fmt.Errorf("会话校验失败：%w", err), nil)
				ctx.Abort()
				return
			}
		}
		ctx.Set("claims", claims)
		ctx.Next()
	}
}

// SetupRequired 首次安装守卫：当尚未创建任何用户时，
// 除 setup 引导接口、健康检查和 robots.txt 外，全部返回 403。
// 由 http.go 在主路由组上统一挂载，无需重复挂载到每个子路由。
func SetupRequired(authService setupChecker) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		path := ctx.Request.URL.Path
		method := ctx.Request.Method
		// 放行：setup 引导、健康检查、robots.txt
		switch {
		case path == "/v1/auth/config", path == "/v1/auth/oidc/login", path == "/v1/auth/oidc/callback", path == "/v1/auth/oidc/session":
			ctx.Next()
			return
		case path == "/v1/setup/need":
			ctx.Next()
			return
		case path == "/v1/health":
			ctx.Next()
			return
		case path == "/v1/robots.txt":
			ctx.Next()
			return
		case path == "/v1/auto-login" && method == http.MethodPost:
			ctx.Next()
			return
		case path == "/v1/setup" && method == http.MethodPost:
			ctx.Next()
			return
		default:
			need, err := authService.CheckNeedSetup(ctx)
			if err != nil {
				v1.HandleError(ctx, http.StatusInternalServerError, &v1.Error{Code: 500, Message: "检查单用户初始化状态失败：" + err.Error()}, nil)
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
