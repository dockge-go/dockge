package handler

// 用户管理端点（5 个 /v1/users* 路由，全部要求 admin 角色）。
// 削弱操作（降级/停用/删除）的不变量由 service 层兜底：不许对本人、
// 至少保留一名活跃 admin。

import (
	"net/http"
	"strconv"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/model"
	"dockge/app/dockge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// UsersHandler 承载账号管理接口。
type UsersHandler struct {
	*Handler
	authService service.AuthService
}

// NewUsersHandler 构造用户管理处理器，由注入容器调用。
func NewUsersHandler(i do.Injector) (*UsersHandler, error) {
	return &UsersHandler{
		Handler:     do.MustInvoke[*Handler](i),
		authService: do.MustInvoke[service.AuthService](i),
	}, nil
}

// RequireAdmin 是用户管理路由的角色守卫：非 admin 一律 403。
func (h *UsersHandler) RequireAdmin(ctx *gin.Context) {
	me, err := h.authService.Me(ctx, GetUserIdFromCtx(ctx))
	if err != nil || me.Role != model.RoleAdmin {
		v1.HandleError(ctx, http.StatusForbidden, &v1.Error{Code: 403, Message: "仅管理员可管理用户"}, nil)
		ctx.Abort()
		return
	}
	ctx.Next()
}

func userRow(u model.DockgeUser) v1.UserRow {
	return v1.UserRow{
		ID: u.ID, Username: u.Username, Nickname: u.Nickname,
		Role: u.Role, Active: u.Active, Source: u.Source,
	}
}

// List 返回全部账号。
func (h *UsersHandler) List(ctx *gin.Context) {
	users, err := h.authService.AdminListUsers(ctx)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	rows := make([]v1.UserRow, 0, len(users))
	for _, u := range users {
		rows = append(rows, userRow(u))
	}
	v1.HandleSuccess(ctx, gin.H{"list": rows})
}

// Create 创建账号（默认 member，密码需满足强度）。
func (h *UsersHandler) Create(ctx *gin.Context) {
	var req v1.UserCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	user, err := h.authService.AdminCreateUser(ctx, &req)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, userRow(user))
}

// parseID 解析 :id 路由参数。
func parseID(ctx *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return 0, false
	}
	return uint(id), true
}

// SetRole 变更账号角色。
func (h *UsersHandler) SetRole(ctx *gin.Context) {
	var req v1.UserRoleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	id, ok := parseID(ctx)
	if !ok {
		return
	}
	if err := h.authService.AdminSetUserRole(ctx, GetUserIdFromCtx(ctx), id, req.Role); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// SetActive 启用或停用账号。
func (h *UsersHandler) SetActive(ctx *gin.Context) {
	var req v1.UserActiveRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	id, ok := parseID(ctx)
	if !ok {
		return
	}
	if err := h.authService.AdminSetUserActive(ctx, GetUserIdFromCtx(ctx), id, req.Active); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// Delete 删除账号。
func (h *UsersHandler) Delete(ctx *gin.Context) {
	id, ok := parseID(ctx)
	if !ok {
		return
	}
	if err := h.authService.AdminDeleteUser(ctx, GetUserIdFromCtx(ctx), id); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}
