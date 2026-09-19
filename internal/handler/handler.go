// Package handler 是 dockge 应用的传输层：解析 HTTP 请求、调用 service、
// 统一写出响应；本层不包含业务规则。
package handler

import (
	"errors"
	"net/http"

	v1 "dockge/api/v1"
	"dockge/internal/repository"
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

// handleServiceError 把 service 层错误映射为统一 HTTP 响应。
// 业务错误（*v1.Error：哨兵本身或被 fmt.Errorf 包装的哨兵）按其自带 code 与文案输出——
// 否则「同码但文案具体」的错误（如登录失败的各种原因）会被哨兵文案吞掉；
// 未归类的内部错误一律 500 兜底，不把内部细节透给用户。
func handleServiceError(ctx *gin.Context, err error) {
	if err == nil {
		return
	}
	var bizErr *v1.Error
	if errors.As(err, &bizErr) {
		v1.HandleError(ctx, bizErr.Code, withDetail(bizErr, err), nil)
		return
	}
	// 栈目录不可写：必须带真实原因与路径，用户才能判断是挂载问题
	if errors.Is(err, repository.ErrStacksNotWritable) {
		v1.HandleError(ctx, http.StatusInternalServerError, withDetail(&v1.Error{Code: http.StatusInternalServerError, Message: "写出失败"}, err), nil)
		return
	}
	v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
}

// withDetail 在错误链上带有附加说明（fmt.Errorf 包装）时，
// 用完整信息作为响应消息，避免细节被哨兵错误的通用文案吞掉。
func withDetail(sentinel *v1.Error, err error) error {
	if err.Error() == sentinel.Error() {
		return sentinel
	}
	return &v1.Error{Code: sentinel.Code, Message: err.Error()}
}
