package service

// 用户管理用例：挂在 authService 上的 admin 专用操作（角色守卫在 handler 层，
// 本层再兜底两条领域不变量——不许对本人执行削弱操作、至少保留一名活跃 admin）。

import (
	"context"
	"errors"
	"fmt"
	"strings"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/model"
	"dockge/app/dockge/internal/repository"
	"dockge/pkg/hash"
)

// AdminListUsers 返回全部账号（按 ID 升序）。
func (s *authService) AdminListUsers(ctx context.Context) ([]model.DockgeUser, error) {
	return s.repo.ListUsers(ctx)
}

// AdminCreateUser 由管理员创建账号（密码强度校验 + 用户名查重）。
func (s *authService) AdminCreateUser(ctx context.Context, req *v1.UserCreateRequest) (model.DockgeUser, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return model.DockgeUser{}, fmt.Errorf("%w: 用户名不能为空", v1.ErrBadRequest)
	}
	role := req.Role
	if role == "" {
		role = model.RoleMember // 自然默认：新建账号给 member，管理员显式提权
	}
	if role != model.RoleAdmin && role != model.RoleMember {
		return model.DockgeUser{}, fmt.Errorf("%w: 角色只能是 admin 或 member", v1.ErrBadRequest)
	}
	if !validatePasswordStrength(req.Password) {
		return model.DockgeUser{}, fmt.Errorf("%w: 密码至少 6 位且需包含字母和数字", v1.ErrBadRequest)
	}
	if _, err := s.repo.GetUserByUsername(ctx, username); err == nil {
		return model.DockgeUser{}, fmt.Errorf("%w: 用户名 %s 已存在", v1.ErrConflict, username)
	}
	hashed, err := hash.BcryptHash(req.Password)
	if err != nil {
		return model.DockgeUser{}, v1.ErrInternalServerError
	}
	user := &model.DockgeUser{
		Username: username, Nickname: username,
		Password: hashed, Role: role, Active: true, Source: model.SourceLocal,
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return model.DockgeUser{}, v1.ErrInternalServerError
	}
	return *user, nil
}

// guardWeakening 校验削弱类操作（降级/停用/删除）的两条不变量。
func (s *authService) guardWeakening(ctx context.Context, actorID, targetID uint) error {
	target, err := s.repo.GetUser(ctx, targetID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return v1.ErrNotFound
		}
		return v1.ErrInternalServerError
	}
	if actorID == targetID {
		return fmt.Errorf("%w: 不能对本人账号执行该操作", v1.ErrBadRequest)
	}
	if target.Role != model.RoleAdmin {
		return nil // member 的削弱操作不威胁管理面
	}
	// 目标是 admin：确保删/停/降之后仍有至少一名活跃 admin
	users, err := s.repo.ListUsers(ctx)
	if err != nil {
		return v1.ErrInternalServerError
	}
	activeAdmins := 0
	for _, u := range users {
		if u.ID != targetID && u.Role == model.RoleAdmin && u.Active {
			activeAdmins++
		}
	}
	if activeAdmins < 1 {
		return fmt.Errorf("%w: 至少保留一名活跃的管理员", v1.ErrConflict)
	}
	return nil
}

// AdminSetUserRole 变更账号角色。
func (s *authService) AdminSetUserRole(ctx context.Context, actorID, targetID uint, role string) error {
	if role != model.RoleAdmin && role != model.RoleMember {
		return fmt.Errorf("%w: 角色只能是 admin 或 member", v1.ErrBadRequest)
	}
	if err := s.guardWeakening(ctx, actorID, targetID); err != nil {
		return err
	}
	return s.repo.SetUserRole(ctx, targetID, role)
}

// AdminSetUserActive 启用或停用账号；停用即时生效（CheckSession 拒绝其全部会话）。
func (s *authService) AdminSetUserActive(ctx context.Context, actorID, targetID uint, active bool) error {
	if !active {
		if err := s.guardWeakening(ctx, actorID, targetID); err != nil {
			return err
		}
	} else if actorID == targetID {
		return fmt.Errorf("%w: 不能对本人账号执行该操作", v1.ErrBadRequest)
	}
	return s.repo.SetUserActive(ctx, targetID, active)
}

// AdminDeleteUser 删除账号（其全部会话随 CheckSession 即时失效）。
func (s *authService) AdminDeleteUser(ctx context.Context, actorID, targetID uint) error {
	if err := s.guardWeakening(ctx, actorID, targetID); err != nil {
		return err
	}
	return s.repo.DeleteUser(ctx, targetID)
}
