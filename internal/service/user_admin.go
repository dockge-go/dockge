package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	v1 "dockge/api/v1"
	"dockge/internal/model"
	"dockge/internal/repository"
	"dockge/pkg/hash"
)

// usernamePattern 与前端 auth.ts 冻结契约同款：2-32 位字母、数字或 _ . -。
// 后端必须等价校验——信任边界在服务端，前端拦截只是体验。
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{2,32}$`)

// ValidateUsername 校验用户名格式（与前端 passwordProblem 同级的唯一入口）。
func ValidateUsername(username string) bool {
	return usernamePattern.MatchString(username)
}

// effectiveRole 返回用户的实际角色（空 role 的历史数据按 admin 兼容，对齐 meData）。
func effectiveRole(u model.DockgeUser) string {
	if u.Role == "" {
		return model.RoleAdmin
	}
	return u.Role
}

// userAdminService 承载管理员用户管理用例；全部方法先 requireAdmin——
// 权限边界在服务端强制，前端隐藏入口不构成安全（D-02）。
type userAdminService struct {
	*authService
}

func requireAdmin(s *authService, ctx context.Context, uid uint) (model.DockgeUser, error) {
	actor, err := s.repo.GetUser(ctx, uid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return actor, v1.ErrUnauthorized // 会话指向已删除用户，按未授权处理
		}
		return actor, fmt.Errorf("%w: 读取操作者失败：%v", v1.ErrInternalServerError, err)
	}
	if effectiveRole(actor) != model.RoleAdmin {
		return actor, &v1.Error{Code: http.StatusForbidden, Message: "需要管理员权限"}
	}
	return actor, nil
}

// toPublic 把用户实体转为列表视图（Created 毫秒；CreatedAt 零值时回退 UpdatedAt，
// 两者皆零取 0，兼容历史迁移数据）。
func toPublic(u model.DockgeUser) v1.UserPublicData {
	created := u.CreatedAt.UnixMilli()
	if u.CreatedAt.IsZero() {
		created = u.UpdatedAt.UnixMilli()
		if u.UpdatedAt.IsZero() {
			created = 0
		}
	}
	return v1.UserPublicData{Username: u.Username, Role: effectiveRole(u), Created: created}
}

// ListUsers 管理员列出全部用户。
func (s *authService) ListUsers(ctx context.Context, actingUID uint) ([]v1.UserPublicData, error) {
	if _, err := requireAdmin(s, ctx, actingUID); err != nil {
		return nil, err
	}
	users, err := s.repo.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: 读取用户列表失败：%v", v1.ErrInternalServerError, err)
	}
	list := make([]v1.UserPublicData, 0, len(users))
	for _, u := range users {
		list = append(list, toPublic(u))
	}
	return list, nil
}

// CreateUser 管理员创建 admin/member 用户；用户名/角色/密码强度三类 400 文案各带实际值。
func (s *authService) CreateUser(ctx context.Context, actingUID uint, req *v1.CreateUserRequest) error {
	if _, err := requireAdmin(s, ctx, actingUID); err != nil {
		return err
	}
	if !ValidateUsername(req.Username) {
		return &v1.Error{Code: http.StatusBadRequest, Message: fmt.Sprintf("用户名不合法：%s（需 2-32 位字母、数字或 _ . -）", req.Username)}
	}
	if req.Role != model.RoleAdmin && req.Role != model.RoleMember {
		return &v1.Error{Code: http.StatusBadRequest, Message: fmt.Sprintf("角色不合法：%s（仅 admin 或 member）", req.Role)}
	}
	if !ValidatePasswordStrength(req.Password) {
		return &v1.Error{Code: http.StatusBadRequest, Message: "密码至少6位且需包含字母和数字"}
	}
	hashed, err := hash.BcryptHash(req.Password)
	if err != nil {
		return fmt.Errorf("%w: 密码加密失败：%v", v1.ErrInternalServerError, err)
	}
	user := &model.DockgeUser{
		Username: req.Username, Nickname: req.Username,
		Password: hashed, Role: req.Role, Active: true, Source: model.SourceLocal,
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return &v1.Error{Code: http.StatusConflict, Message: fmt.Sprintf("用户名已存在：%s", req.Username)}
		}
		return fmt.Errorf("%w: 创建用户失败：%v", v1.ErrInternalServerError, err)
	}
	return nil
}

// ResetUserPassword 管理员重置他人密码；JWT 摘要绑定使目标用户旧会话即时失效。
func (s *authService) ResetUserPassword(ctx context.Context, actingUID uint, username, newPassword string) error {
	if _, err := requireAdmin(s, ctx, actingUID); err != nil {
		return err
	}
	target, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return &v1.Error{Code: http.StatusNotFound, Message: fmt.Sprintf("用户不存在：%s", username)}
		}
		return fmt.Errorf("%w: 读取目标用户失败：%v", v1.ErrInternalServerError, err)
	}
	if !ValidatePasswordStrength(newPassword) {
		return &v1.Error{Code: http.StatusBadRequest, Message: "密码至少6位且需包含字母和数字"}
	}
	hashed, err := hash.BcryptHash(newPassword)
	if err != nil {
		return fmt.Errorf("%w: 密码加密失败：%v", v1.ErrInternalServerError, err)
	}
	return s.repo.UpdatePassword(ctx, target.ID, hashed)
}

// DeleteUser 管理员删除用户。检查顺序固定：先查最后一名 admin（409），再查自删（403）——
// target≠actor 蕴含 admin 数 ≥2，唯一 admin 场景只可能在自删时成立，反序会使 409 成为死分支。
func (s *authService) DeleteUser(ctx context.Context, actingUID uint, username string) error {
	if _, err := requireAdmin(s, ctx, actingUID); err != nil {
		return err
	}
	target, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return &v1.Error{Code: http.StatusNotFound, Message: fmt.Sprintf("用户不存在：%s", username)}
		}
		return fmt.Errorf("%w: 读取目标用户失败：%v", v1.ErrInternalServerError, err)
	}
	if effectiveRole(target) == model.RoleAdmin {
		users, err := s.repo.ListUsers(ctx)
		if err != nil {
			return fmt.Errorf("%w: 统计管理员失败：%v", v1.ErrInternalServerError, err)
		}
		admins := 0
		for _, u := range users {
			if effectiveRole(u) == model.RoleAdmin {
				admins++
			}
		}
		if admins <= 1 {
			return &v1.Error{Code: http.StatusConflict, Message: "不能删除最后一名管理员"}
		}
	}
	if target.ID == actingUID {
		return &v1.Error{Code: http.StatusForbidden, Message: "不能删除当前登录账号"}
	}
	return s.repo.DeleteUser(ctx, username)
}
