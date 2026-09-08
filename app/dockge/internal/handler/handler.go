// Package handler 是 dockge 应用的传输层：解析 HTTP 请求、调用 service、
// 统一写出响应；本层不包含业务规则。
package handler

import (
	"errors"
	"net/http"

	v1 "dockge/app/dockge/api/v1"
	"dockge/pkg/jwt"
	"dockge/pkg/log"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// Handler 是 handler 层公共依赖。
type Handler struct {
	logger *log.Logger
}

// Package registers all handler-layer providers into the injector.
var Package = do.Package(
	do.Lazy(New),
	do.Lazy(NewAuthHandler),
	do.Lazy(NewStackHandler),
	do.Lazy(NewDockerHandler),
	do.Lazy(NewSettingsHandler),
	do.Lazy(NewComposerizeHandler),
	do.Lazy(NewTerminalHandler),
)

// New 构造 handler 公共依赖，由注入容器调用。
func New(i do.Injector) (*Handler, error) {
	return &Handler{logger: do.MustInvoke[*log.Logger](i)}, nil
}

// GetUserIdFromCtx 从 gin 上下文取出 JWT 中间件写入的当前用户 ID。
func GetUserIdFromCtx(ctx *gin.Context) uint {
	v, exists := ctx.Get("claims")
	if !exists {
		return 0
	}
	return v.(*jwt.Claims).UserId
}

// handleServiceError 把 service 层错误映射为统一 HTTP 响应：
// 参数类错误回 400、未找到回 404、冲突回 409、docker 失败回 500 并携带消息。
func handleServiceError(ctx *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, v1.ErrUnauthorized):
		v1.HandleError(ctx, http.StatusUnauthorized, v1.ErrUnauthorized, nil)
	case errors.Is(err, v1.ErrBadRequest):
		v1.HandleError(ctx, http.StatusBadRequest, withDetail(v1.ErrBadRequest, err), nil)
	case errors.Is(err, v1.ErrConflict):
		v1.HandleError(ctx, http.StatusConflict, withDetail(v1.ErrConflict, err), nil)
	case errors.Is(err, v1.ErrNotFound):
		v1.HandleError(ctx, http.StatusNotFound, v1.ErrNotFound, nil)
	case errors.Is(err, v1.ErrDockerError):
		v1.HandleError(ctx, http.StatusInternalServerError, withDetail(v1.ErrDockerError, err), nil)
	default:
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
	}
}

// withDetail 在错误链上带有附加说明（fmt.Errorf 包装）时，
// 用完整信息作为响应消息，避免细节被哨兵错误的通用文案吞掉。
func withDetail(sentinel *v1.Error, err error) error {
	if err.Error() == sentinel.Error() {
		return sentinel
	}
	return &v1.Error{Code: sentinel.Code, Message: err.Error()}
}
