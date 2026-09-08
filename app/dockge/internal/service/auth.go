package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/model"
	"dockge/app/dockge/internal/repository"
	"dockge/pkg/hash"
	"dockge/pkg/rate"
	"dockge/pkg/totp"

	"github.com/samber/do/v2"
)

const tokenTTL = time.Hour * 24 * 7

// AuthService 提供登录、当前用户、改密用例。
type AuthService interface {
	Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginResponseData, error)
	Me(ctx context.Context, uid uint) (*v1.MeUserData, error)
	ChangePassword(ctx context.Context, uid uint, req *v1.ChangePasswordRequest) error
	Setup(ctx context.Context, req *v1.SetupRequest) (*v1.LoginResponseData, error)
	CheckNeedSetup(ctx context.Context) (bool, error)
	Check2FA(ctx context.Context, req *v1.TwoFARequest) (*v1.LoginResponseData, error)
	Enable2FA(ctx context.Context, uid uint) (string, error)
	Disable2FA(ctx context.Context, uid uint) error
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value, typ string) error
	GetAllSettings(ctx context.Context, typ string) (map[string]string, error)
	GetDisableAuth(ctx context.Context) bool
	ToggleDisableAuth(ctx context.Context, uid uint, enable bool, currentPassword string) error
	AutoLogin(ctx context.Context) (*v1.LoginResponseData, error)
	GetLatestVersion(ctx context.Context) (string, error)
}

type authService struct {
	*Service
	loginLimiter *rate.Limiter
}

// NewAuthService 构造认证服务（含登录限流器），由注入容器调用。

func NewAuthService(i do.Injector) (AuthService, error) {
	return &authService{
		Service:      do.MustInvoke[*Service](i),
		loginLimiter: rate.New(20, time.Minute),
	}, nil
}

// GetDisableAuth 读取免登录模式开关。

func (s *authService) GetDisableAuth(ctx context.Context) bool {
	v, err := s.repo.GetSetting(ctx, "disableAuth")
	if err != nil {
		return false
	}
	return v == "true"
}

// Login 校验用户名密码；开启 2FA 的账号返回中间令牌并要求提交验证码。

func (s *authService) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginResponseData, error) {
	if !s.loginLimiter.Allow() {
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
	if user.TwofaStatus {
		token, _ := s.jwt.GenToken(user.ID, user.Password, time.Now().Add(tokenTTL))
		return &v1.LoginResponseData{
			AccessToken:   token,
			TokenRequired: true,
			User:          v1.MeUserData{ID: user.ID, Username: user.Username, Nickname: user.Nickname},
		}, nil
	}
	token, err := s.jwt.GenToken(user.ID, user.Password, time.Now().Add(tokenTTL))
	if err != nil {
		return nil, v1.ErrInternalServerError
	}
	return &v1.LoginResponseData{
		AccessToken: token,
		User:        v1.MeUserData{ID: user.ID, Username: user.Username, Nickname: user.Nickname},
	}, nil
}

// Check2FA 校验 TOTP 验证码（防重放），通过后签发正式会话令牌。

func (s *authService) Check2FA(ctx context.Context, req *v1.TwoFARequest) (*v1.LoginResponseData, error) {
	user, err := s.repo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, v1.ErrUnauthorized
	}
	if !user.Active || !user.TwofaStatus {
		return nil, v1.ErrUnauthorized
	}
	if !totp.Verify(req.Token, user.TwofaSecret) {
		return nil, &v1.Error{Code: 401, Message: "authInvalidToken"}
	}
	if user.TwofaLastToken == req.Token {
		return nil, &v1.Error{Code: 401, Message: "authInvalidToken"}
	}
	s.repo.UpdateUserTwofa(ctx, user.ID, user.TwofaSecret, req.Token, user.TwofaStatus)
	token, err := s.jwt.GenToken(user.ID, user.Password, time.Now().Add(tokenTTL))
	if err != nil {
		return nil, v1.ErrInternalServerError
	}
	return &v1.LoginResponseData{
		AccessToken: token,
		User:        v1.MeUserData{ID: user.ID, Username: user.Username, Nickname: user.Nickname},
	}, nil
}

// Enable2FA 生成 TOTP 密钥并立即启用，返回 otpauth 二维码 URL。

func (s *authService) Enable2FA(ctx context.Context, uid uint) (string, error) {
	secret, err := totp.GenerateSecret()
	if err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}
	if err := s.repo.UpdateUserTwofa(ctx, uid, secret, "", true); err != nil {
		return "", err
	}
	return totp.QRCodeURL(secret), nil
}

// Disable2FA 清除 2FA 密钥并停用两步验证。

func (s *authService) Disable2FA(ctx context.Context, uid uint) error {
	_, err := s.repo.GetUser(ctx, uid)
	if err != nil {
		return err
	}
	return s.repo.UpdateUserTwofa(ctx, uid, "", "", false)
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
	if !totp.ValidatePasswordStrength(req.Password) {
		return nil, &v1.Error{Code: 400, Message: "Password is too weak. It should contain alphabetic and numeric characters. It must be at least 6 characters in length."}
	}
	hashed, err := hash.BcryptHash(req.Password)
	if err != nil {
		return nil, v1.ErrInternalServerError
	}
	user := &model.DockgeUser{Username: req.Username, Nickname: req.Username, Password: hashed}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, v1.ErrInternalServerError
	}
	token, err := s.jwt.GenToken(user.ID, hashed, time.Now().Add(tokenTTL))
	if err != nil {
		return nil, v1.ErrInternalServerError
	}
	return &v1.LoginResponseData{
		AccessToken: token,
		User:        v1.MeUserData{ID: user.ID, Username: user.Username, Nickname: user.Nickname},
	}, nil
}

// CheckNeedSetup 判断是否需要首次安装引导（用户数为 0）。

func (s *authService) CheckNeedSetup(ctx context.Context) (bool, error) {
	count, err := s.repo.CountUsers(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// Me 返回当前登录用户信息（含 2FA 状态）。

func (s *authService) Me(ctx context.Context, uid uint) (*v1.MeUserData, error) {
	user, err := s.repo.GetUser(ctx, uid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, v1.ErrNotFound
		}
		return nil, v1.ErrInternalServerError
	}
	return &v1.MeUserData{
		ID: user.ID, Username: user.Username, Nickname: user.Nickname,
		TwoFA: user.TwofaStatus,
	}, nil
}

// ChangePassword 校验旧密码与强度后更新密码，并踢出其他会话。

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
	if !totp.ValidatePasswordStrength(req.NewPassword) {
		return fmt.Errorf("%w: 密码至少6位且需包含字母和数字", v1.ErrBadRequest)
	}
	hashed, err := hash.BcryptHash(req.NewPassword)
	if err != nil {
		return v1.ErrInternalServerError
	}
	if err := s.repo.UpdatePassword(ctx, uid, hashed); err != nil {
		return err
	}
	return nil
}

// GetSetting 读取原始设置项（供认证相关逻辑使用）。

func (s *authService) GetSetting(ctx context.Context, key string) (string, error) {
	return s.repo.GetSetting(ctx, key)
}

// SetSetting 写入原始设置项。

func (s *authService) SetSetting(ctx context.Context, key, value, typ string) error {
	return s.repo.SetSetting(ctx, key, value, typ)
}

// GetAllSettings 返回指定分组的全部设置项。

func (s *authService) GetAllSettings(ctx context.Context, typ string) (map[string]string, error) {
	return s.repo.GetAllSettingsByType(ctx, typ)
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
			token, err := s.jwt.GenToken(u.ID, u.Password, time.Now().Add(tokenTTL))
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

// ToggleDisableAuth 切换免登录模式：切换到关闭认证时须校验当前密码。
func (s *authService) ToggleDisableAuth(ctx context.Context, uid uint, enable bool, currentPassword string) error {
	if enable {
		// 从已启用切换到免登录：须验证当前密码
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
