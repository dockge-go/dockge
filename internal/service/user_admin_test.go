package service

import (
	"context"
	"strings"
	"testing"

	v1 "dockge/api/v1"
	"dockge/internal/model"
	"dockge/internal/repository"
	"dockge/pkg/rate"
)

// newTestAuthService 构造带临时库的 authService（绕过 do.Injector；限流器用真实实例）。
func newTestAuthService(t *testing.T) (*authService, uint) {
	t.Helper()
	repo, adminID := newTestUserRepo(t)
	return &authService{
		repo:         repo,
		loginLimiter: rate.NewKeyed(loginRateLimit, loginRateWindow, loginMaxKeys),
	}, adminID
}

// TestValidateUsername 表驱动：与前端 auth.ts 同款规则。
func TestValidateUsername(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"最小合法", "ab", true},
		{"含点横线下划线", "a.b_c-d", true},
		{"数字", "user123", true},
		{"过短", "a", false},
		{"过长33位", strings.Repeat("a", 33), false},
		{"含空格", "a b", false},
		{"含中文", "用户名", false},
		{"含冒号", "a:b", false},
		{"空串", "", false},
		{"32位边界", strings.Repeat("a", 32), true},
	}
	for _, c := range cases {
		if got := ValidateUsername(c.in); got != c.want {
			t.Errorf("ValidateUsername(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestListUsersRequiresAdmin member 角色被 403 拒绝，admin 可列出。
func TestListUsersRequiresAdmin(t *testing.T) {
	s, adminID := newTestAuthService(t)
	ctx := context.Background()

	if err := s.repo.CreateUser(ctx, &model.DockgeUser{Username: "bob", Password: "x", Role: model.RoleMember, Active: true}); err != nil {
		t.Fatal(err)
	}
	bob, err := s.repo.GetUserByUsername(ctx, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListUsers(ctx, bob.ID); err == nil {
		t.Fatal("ListUsers as member: want 403 error, got nil")
	}
	list, err := s.ListUsers(ctx, adminID)
	if err != nil {
		t.Fatalf("ListUsers as admin: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListUsers = %d users, want 2", len(list))
	}
	if list[0].Role != model.RoleAdmin || list[1].Role != model.RoleMember {
		t.Fatalf("roles = %q,%q, want admin,member", list[0].Role, list[1].Role)
	}
}

// TestCreateUserValidation 覆盖非法用户名/角色/密码强度与重复创建。
func TestCreateUserValidation(t *testing.T) {
	s, adminID := newTestAuthService(t)
	ctx := context.Background()

	cases := []struct {
		name     string
		username string
		password string
		role     string
		wantCode int
		wantIn   string
	}{
		{"非法用户名", "a b", "Pass123", model.RoleMember, 400, "用户名不合法"},
		{"非法角色", "bob", "Pass123", "superuser", 400, "角色不合法"},
		{"弱密码", "bob", "short", model.RoleMember, 400, "至少6位"},
		{"纯数字密码", "bob", "123456", model.RoleMember, 400, "至少6位"},
	}
	for _, c := range cases {
		err := s.CreateUser(ctx, adminID, &v1.CreateUserRequest{Username: c.username, Password: c.password, Role: c.role})
		ve, ok := err.(*v1.Error)
		if !ok || ve.Code != c.wantCode || !strings.Contains(ve.Message, c.wantIn) {
			t.Errorf("%s: err = %v, want code %d containing %q", c.name, err, c.wantCode, c.wantIn)
		}
	}

	if err := s.CreateUser(ctx, adminID, &v1.CreateUserRequest{Username: "bob", Password: "Pass123", Role: model.RoleMember}); err != nil {
		t.Fatalf("CreateUser(bob): %v", err)
	}
	err := s.CreateUser(ctx, adminID, &v1.CreateUserRequest{Username: "bob", Password: "Pass123", Role: model.RoleMember})
	if ve, ok := err.(*v1.Error); !ok || ve.Code != 409 {
		t.Errorf("duplicate CreateUser = %v, want 409", err)
	}
}

// TestDeleteUserProtection 覆盖检查顺序：先 last-admin 409，后自删 403。
func TestDeleteUserProtection(t *testing.T) {
	s, adminID := newTestAuthService(t)
	ctx := context.Background()
	admin, err := s.repo.GetUser(ctx, adminID)
	if err != nil {
		t.Fatal(err)
	}

	// 双 admin 时：自删 403（last-admin 检查通过后落自删分支）
	if err := s.repo.CreateUser(ctx, &model.DockgeUser{Username: "e2", Password: "x", Role: model.RoleAdmin, Active: true}); err != nil {
		t.Fatal(err)
	}
	err = s.DeleteUser(ctx, adminID, admin.Username)
	if ve, ok := err.(*v1.Error); !ok || ve.Code != 403 {
		t.Errorf("self-delete with 2 admins = %v, want 403", err)
	}

	// member 目标可删；随后删除另一名 admin e2，使 admin 成为唯一 admin
	if err := s.repo.CreateUser(ctx, &model.DockgeUser{Username: "bob", Password: "x", Role: model.RoleMember, Active: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(ctx, adminID, "bob"); err != nil {
		t.Fatalf("DeleteUser(bob): %v", err)
	}
	if err := s.DeleteUser(ctx, adminID, "e2"); err != nil {
		t.Fatalf("DeleteUser(e2): %v", err)
	}

	// 唯一 admin 自删 → 409（last-admin 分支在自删之前命中）
	err = s.DeleteUser(ctx, adminID, admin.Username)
	if ve, ok := err.(*v1.Error); !ok || ve.Code != 409 {
		t.Errorf("self-delete sole admin = %v, want 409", err)
	}

	// 不存在用户 → 404
	err = s.DeleteUser(ctx, adminID, "ghost")
	if ve, ok := err.(*v1.Error); !ok || ve.Code != 404 {
		t.Errorf("DeleteUser(ghost) = %v, want 404", err)
	}
}

// TestResetUserPassword 覆盖重置成功路径与目标不存在。
func TestResetUserPassword(t *testing.T) {
	s, adminID := newTestAuthService(t)
	ctx := context.Background()

	if err := s.repo.CreateUser(ctx, &model.DockgeUser{Username: "bob", Password: "oldhash", Role: model.RoleMember, Active: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetUserPassword(ctx, adminID, "bob", "NewPass123"); err != nil {
		t.Fatalf("ResetUserPassword: %v", err)
	}
	got, _ := s.repo.GetUserByUsername(ctx, "bob")
	if got.Password == "oldhash" {
		t.Fatal("password not changed")
	}
	err := s.ResetUserPassword(ctx, adminID, "ghost", "NewPass123")
	if ve, ok := err.(*v1.Error); !ok || ve.Code != 404 {
		t.Errorf("ResetUserPassword(ghost) = %v, want 404", err)
	}
}

// TestLoginNoEnumeration 未知用户与密码错误的响应逐字一致，停用账号在密码错误时不可区分。
func TestLoginNoEnumeration(t *testing.T) {
	s, _ := newTestAuthService(t)
	ctx := context.Background()
	req := &v1.LoginRequest{Username: "ghost", Password: "Whatever1"}

	unknown, err1 := s.Login(ctx, req, "9.9.9.9")
	_ = unknown
	ve1, ok := err1.(*v1.Error)
	if !ok || ve1.Code != 401 {
		t.Fatalf("unknown user = %v, want 401 v1.Error", err1)
	}

	// 已知用户 + 错误密码
	if err := s.repo.CreateUser(ctx, &model.DockgeUser{Username: "ops2", Password: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", Role: model.RoleAdmin, Active: true}); err != nil {
		t.Fatal(err)
	}
	_, err2 := s.Login(ctx, &v1.LoginRequest{Username: "ops2", Password: "WrongPass1"}, "9.9.9.9")
	ve2, ok := err2.(*v1.Error)
	if !ok || ve2.Code != 401 {
		t.Fatalf("wrong password = %v, want 401 v1.Error", err2)
	}
	if ve1.Message != ve2.Message {
		t.Fatalf("enumeration leak: unknown=%q wrongpw=%q", ve1.Message, ve2.Message)
	}
	if !strings.Contains(ve1.Message, "用户名或密码错误") {
		t.Fatalf("unexpected message %q", ve1.Message)
	}
}

// TestLoginActiveAfterPassword 停用账号在密码错误时返回统一文案（不可探测），
// 密码正确时才提示停用。
func TestLoginActiveAfterPassword(t *testing.T) {
	s, _ := newTestAuthService(t)
	ctx := context.Background()

	inactive := &model.DockgeUser{Username: "off", Password: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", Role: model.RoleAdmin, Active: false}
	if err := s.repo.CreateUser(ctx, inactive); err != nil {
		t.Fatal(err)
	}
	// 错误密码 + 停用 → 统一「用户名或密码错误」（不泄露存在性）
	_, err := s.Login(ctx, &v1.LoginRequest{Username: "off", Password: "WrongPass1"}, "8.8.8.8")
	if ve, ok := err.(*v1.Error); !ok || !strings.Contains(ve.Message, "用户名或密码错误") {
		t.Fatalf("inactive+wrongpw = %v, want unified 401 message", err)
	}
	// 正确密码 + 停用 → 明确「已停用」文案（此时已知凭据有效，可安全区分）
	// dummyBcryptHash 对应的明文未知，这里改用真实哈希验证停用分支可达。
	hashed := hashBcryptForTest("RealPass1")
	if err := s.repo.UpdatePassword(ctx, inactive.ID, hashed); err != nil {
		t.Fatal(err)
	}
	_, err = s.Login(ctx, &v1.LoginRequest{Username: "off", Password: "RealPass1"}, "8.8.8.8")
	if ve, ok := err.(*v1.Error); !ok || !strings.Contains(ve.Message, "停用") {
		t.Fatalf("inactive+correctpw = %v, want 停用 message", err)
	}
}

// newTestUserRepo 是 user_admin_test.go 内的 repo 夹具别名（避免 import cycle）。
func newTestUserRepo(t *testing.T) (*repository.Repository, uint) {
	return newTestAuthRepo(t)
}
