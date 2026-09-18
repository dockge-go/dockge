package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/authproxy"
	"dockge/app/dockge/internal/model"
	"dockge/app/dockge/internal/repository"
	"dockge/pkg/hash"
	"dockge/pkg/rate"

	"github.com/samber/do/v2"
	"github.com/spf13/viper"
)

// SessionTTL 是本地会话（JWT 与 dockge_token cookie）的有效期。
const SessionTTL = time.Hour * 24 * 7

// 登录限流：单个「IP+账号」键在窗口期内的最大尝试次数与键容量上限。
const (
	loginRateLimit  = 10
	loginRateWindow = time.Minute
	loginMaxKeys    = 4096
)

// AuthService 提供登录、当前用户、改密、外部身份映射与会话校验用例。
type AuthService interface {
	Login(ctx context.Context, req *v1.LoginRequest, clientIP string) (*v1.LoginResponseData, error)
	Me(ctx context.Context, uid uint) (*v1.MeUserData, error)
	ChangePassword(ctx context.Context, uid uint, req *v1.ChangePasswordRequest) error
	Setup(ctx context.Context, req *v1.SetupRequest) (*v1.LoginResponseData, error)
	CheckNeedSetup(ctx context.Context) (bool, error)
	// CheckSession 供 StrictAuth 逐请求校验：用户存在、启用且密码哈希未变
	// （已接线关闭债务 D12：停用/改密后旧 token 下一次请求即 401）。
	CheckSession(ctx context.Context, uid uint, h string) error
	// ProxyLogin 把受信反代注入的身份换成本地会话。
	ProxyLogin(ctx context.Context, username string, remoteAddr string) (*v1.LoginResponseData, error)
	// OIDCLogin 把验签后的 OIDC 身份换成本地会话。
	OIDCLogin(ctx context.Context, username string, admin bool) (*v1.LoginResponseData, error)
	// GetDisableAuth 读取免登录模式开关。
	GetDisableAuth(ctx context.Context) bool
	// ToggleDisableAuth 切换免登录模式。
	ToggleDisableAuth(ctx context.Context, uid uint, enable bool, currentPassword string) error
	// AutoLogin 免登录模式下以首个活跃用户自动登录。
	AutoLogin(ctx context.Context) (*v1.LoginResponseData, error)
	GetLatestVersion(ctx context.Context) (string, error)
	// ---- 用户管理（admin 专用；角色守卫在 handler 层，领域不变量在本层）----
	AdminListUsers(ctx context.Context) ([]model.DockgeUser, error)
	AdminCreateUser(ctx context.Context, req *v1.UserCreateRequest) (model.DockgeUser, error)
	AdminSetUserRole(ctx context.Context, actorID, targetID uint, role string) error
	AdminSetUserActive(ctx context.Context, actorID, targetID uint, active bool) error
	AdminDeleteUser(ctx context.Context, actorID, targetID uint) error
}

type authService struct {
	*Service
	loginLimiter *rate.KeyedLimiter
	proxyCfg     authproxy.Config
}

// NewAuthService 构造认证服务（含按 IP+账号的登录限流器），由注入容器调用。
func NewAuthService(i do.Injector) (AuthService, error) {
	conf := do.MustInvoke[*viper.Viper](i)
	return &authService{
		Service:      do.MustInvoke[*Service](i),
		loginLimiter: rate.NewKeyed(loginRateLimit, loginRateWindow, loginMaxKeys),
		proxyCfg:     authproxy.FromViper(conf),
	}, nil
}

// loginRateKey 组合限流键：IP 与账号双维度。
func loginRateKey(clientIP, username string) string {
	return clientIP + "|" + username
}

// Login 校验用户名密码；开启 2FA 的账号返回中间令牌并要求提交验证码。
func (s *authService) Login(ctx context.Context, req *v1.LoginRequest, clientIP string) (*v1.LoginResponseData, error) {
	if !s.loginLimiter.Allow(loginRateKey(clientIP, req.Username)) {
		return nil, v1.ErrUnauthorized
	}
	user, err := s.repo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, v1.ErrUnauthorized
		}
		return nil, v1.ErrInternalServerError
	}
	if !user.Active {
		return nil, v1.ErrUnauthorized
	}
	if err := hash.BcryptCheck(req.Password, user.Password); err != nil {
		return nil, v1.ErrUnauthorized
	}
	return s.session(&user)
}

// Setup 创建首个管理员账号（仅当无任何用户时），成功即返回登录态。
func (s *authService) Setup(ctx context.Context, req *v1.SetupRequest) (*v1.LoginResponseData, error) {
	count, err := s.repo.CountUsers(ctx)
	if err != nil {
		return nil, v1.ErrInternalServerError
	}
	if count > 0 {
		return nil, &v1.Error{Code: 409, Message: "Dockge has been initialized."}
	}
	if !validatePasswordStrength(req.Password) {
		return nil, &v1.Error{Code: 400, Message: "Password is too weak. It should contain alphabetic and numeric characters. It must be at least 6 characters in length."}
	}
	hashed, err := hash.BcryptHash(req.Password)
	if err != nil {
		return nil, v1.ErrInternalServerError
	}
	user := &model.DockgeUser{
		Username: req.Username, Nickname: req.Username,
		Password: hashed, Role: model.RoleAdmin, Active: true, Source: model.SourceLocal,
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, v1.ErrInternalServerError
	}
	return s.session(user)
}

// CheckNeedSetup 判断是否需要首次安装引导（用户数为 0）。
func (s *authService) CheckNeedSetup(ctx context.Context) (bool, error) {
	count, err := s.repo.CountUsers(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// Me 返回当前登录用户信息（含角色与 2FA 状态）。
func (s *authService) Me(ctx context.Context, uid uint) (*v1.MeUserData, error) {
	user, err := s.repo.GetUser(ctx, uid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, v1.ErrNotFound
		}
		return nil, v1.ErrInternalServerError
	}
	mp := meData(&user)
	return &mp, nil
}

// ChangePassword 校验旧密码与强度后更新密码；
// 旧 token 因 claims 中的密码摘要绑定（CheckSession）自动失效。
func (s *authService) ChangePassword(ctx context.Context, uid uint, req *v1.ChangePasswordRequest) error {
	user, err := s.repo.GetUser(ctx, uid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return v1.ErrNotFound
		}
		return v1.ErrInternalServerError
	}
	if err := hash.BcryptCheck(req.OldPassword, user.Password); err != nil {
		return v1.ErrBadRequest
	}
	if !validatePasswordStrength(req.NewPassword) {
		return fmt.Errorf("%w: 密码至少6位且需包含字母和数字", v1.ErrBadRequest)
	}
	hashed, err := hash.BcryptHash(req.NewPassword)
	if err != nil {
		return v1.ErrInternalServerError
	}
	return s.repo.UpdatePassword(ctx, uid, hashed)
}

// CheckSession 逐请求校验会话有效性：用户存在、启用，且密码哈希摘要与 token 一致。
func (s *authService) CheckSession(ctx context.Context, uid uint, h string) error {
	user, err := s.repo.GetUser(ctx, uid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return v1.ErrUnauthorized
		}
		return err
	}
	if !user.Active {
		return v1.ErrUnauthorized
	}
	if hash.Shake256(user.Password) != h {
		return v1.ErrUnauthorized // 密码已修改，旧 token 失效
	}
	return nil
}

// ProxyLogin 受信反代模式：来源命中受信网段时读取身份头，映射为本地会话。
// 伪造来源（非受信 IP 携带身份头）一律拒绝。
func (s *authService) ProxyLogin(ctx context.Context, username string, remoteAddr string) (*v1.LoginResponseData, error) {
	if !s.proxyCfg.TrustedIP(remoteAddr) {
		return nil, v1.ErrUnauthorized
	}
	if username == "" {
		return nil, v1.ErrUnauthorized
	}
	return s.externalLogin(ctx, username, model.SourceProxy, false, s.proxyCfg.AutoProvision)
}

// OIDCLogin 把验签后的 OIDC 身份映射为本地用户并签发会话（OIDC 身份始终自动开通）。
func (s *authService) OIDCLogin(ctx context.Context, username string, admin bool) (*v1.LoginResponseData, error) {
	return s.externalLogin(ctx, username, model.SourceOIDC, admin, true)
}

// externalLogin 是 ProxyLogin/OIDCLogin 的公共主体：
// 查找或自动开通本地用户（外部账号密码为随机值，不可用于密码登录），签发本地 JWT。
func (s *authService) externalLogin(ctx context.Context, username, source string, admin, autoProvision bool) (*v1.LoginResponseData, error) {
	if username == "" {
		return nil, v1.ErrUnauthorized
	}
	user, err := s.repo.GetUserByUsername(ctx, username)
	switch {
	case err == nil:
		if !user.Active {
			return nil, v1.ErrUnauthorized
		}
	case errors.Is(err, repository.ErrNotFound):
		if !autoProvision {
			return nil, v1.ErrUnauthorized
		}
		hashed, hashErr := hash.BcryptHash(unusablePassword())
		if hashErr != nil {
			return nil, v1.ErrInternalServerError
		}
		role := model.RoleMember
		if admin {
			role = model.RoleAdmin
		}
		newUser := model.DockgeUser{
			Username: username, Nickname: username, Password: hashed,
			Role: role, Active: true, Source: source,
		}
		if createErr := s.repo.CreateUser(ctx, &newUser); createErr != nil {
			return nil, v1.ErrInternalServerError
		}
		user = newUser
		s.logger.WithContext(ctx).Info().
			Str("username", username).Str("source", source).Str("role", role).
			Msg("auto-provision user from external auth")
	default:
		return nil, v1.ErrInternalServerError
	}
	return s.session(&user)
}

// GetDisableAuth 读取免登录模式开关。
func (s *authService) GetDisableAuth(ctx context.Context) bool {
	v, err := s.repo.GetSetting(ctx, "disableAuth")
	if err != nil {
		return false
	}
	return v == "true"
}

// ToggleDisableAuth 切换免登录模式：切换到关闭认证时须校验当前密码。
func (s *authService) ToggleDisableAuth(ctx context.Context, uid uint, enable bool, currentPassword string) error {
	if enable {
		user, err := s.repo.GetUser(ctx, uid)
		if err != nil {
			return err
		}
		if err := hash.BcryptCheck(currentPassword, user.Password); err != nil {
			return v1.ErrBadRequest
		}
	}
	return s.repo.SetSetting(ctx, "disableAuth", strconv.FormatBool(enable), "security")
}

// AutoLogin 免登录模式：仅当 disableAuth=true 时以首个活跃用户自动登录；
// 否则拒绝（该端点无需认证，必须防止无条件登录）。
func (s *authService) AutoLogin(ctx context.Context) (*v1.LoginResponseData, error) {
	if !s.GetDisableAuth(ctx) {
		return nil, v1.ErrUnauthorized
	}
	users, err := s.repo.ListUsers(ctx)
	if err != nil {
		return nil, v1.ErrInternalServerError
	}
	for _, u := range users {
		if u.Active {
			token, err := s.jwt.GenToken(u.ID, u.Password, time.Now().Add(SessionTTL))
			if err != nil {
				return nil, v1.ErrInternalServerError
			}
			return &v1.LoginResponseData{
				AccessToken: token,
				User:        v1.MeUserData{ID: u.ID, Username: u.Username, Nickname: u.Nickname},
			}, nil
		}
	}
	return nil, v1.ErrUnauthorized
}

// session 签发绑定密码哈希的本地 JWT。
func (s *authService) session(user *model.DockgeUser) (*v1.LoginResponseData, error) {
	token, err := s.jwt.GenToken(user.ID, user.Password, time.Now().Add(SessionTTL))
	if err != nil {
		return nil, v1.ErrInternalServerError
	}
	return &v1.LoginResponseData{AccessToken: token, User: meData(user)}, nil
}

// GetLatestVersion 返回 GitHub 最新 release tag。
func (s *authService) GetLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/louislam/dockge/releases/latest", nil)
	if err != nil {
		return "", nil
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil
	}
	defer resp.Body.Close()
	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", nil
	}
	return result.TagName, nil
}

// unusablePassword 生成外部身份账号的密码哈希原值：随机 256 位。
// 用户不可知，因此这类账号无法通过密码登录，只能走外部认证。
func unusablePassword() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}

// meData 把用户实体转为对外视图（角色为空的历史用户按 admin 展示）。
func meData(u *model.DockgeUser) v1.MeUserData {
	role := u.Role
	if role == "" {
		role = model.RoleAdmin
	}
	return v1.MeUserData{
		ID: u.ID, Username: u.Username, Nickname: u.Nickname,
		Role: role,
	}
}
