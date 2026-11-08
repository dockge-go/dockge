package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	v1 "dockge/api/v1"
	"dockge/internal/model"
	"dockge/internal/repository"
	"dockge/pkg/config"
	"dockge/pkg/hash"
	"dockge/pkg/jwt"
	"dockge/pkg/rate"
	"github.com/spf13/viper"

	"github.com/samber/do/v2"
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
	// CheckSession 供 StrictAuth 逐请求校验：用户存在、启用且密码哈希未变，改密后旧 token 立即失效。
	CheckSession(ctx context.Context, uid uint, h string) error
	// GetDisableAuth 读取免登录模式开关。
	GetDisableAuth(ctx context.Context) bool
	// ToggleDisableAuth 切换免登录模式。
	ToggleDisableAuth(ctx context.Context, uid uint, enable bool, currentPassword string) error
	// AutoLogin 免登录模式下以首个活跃用户自动登录。
	AutoLogin(ctx context.Context) (*v1.LoginResponseData, error)
}

type authService struct {
	repo         *repository.Repository
	jwt          *jwt.JWT
	loginLimiter *rate.KeyedLimiter
	oidc         config.OIDC
}

// NewAuthService 构造认证服务（含按 IP+账号的登录限流器），由注入容器调用。
func NewAuthService(i do.Injector) (AuthService, error) {
	return &authService{
		repo:         do.MustInvoke[*repository.Repository](i),
		jwt:          do.MustInvoke[*jwt.JWT](i),
		loginLimiter: rate.NewKeyed(loginRateLimit, loginRateWindow, loginMaxKeys),
		oidc:         config.OIDCFrom(do.MustInvoke[*viper.Viper](i)),
	}, nil
}

// loginRateKey 使用定长摘要，避免限流桶长期保留原始输入；长度前缀避免键拼接歧义。
func loginRateKey(clientIP, username string) string {
	sum := sha256.Sum256([]byte(strconv.Itoa(len(clientIP)) + ":" + clientIP + username))
	return fmt.Sprintf("%x", sum)
}

// dummyBcryptHash 是合法格式的 bcrypt 串：未知用户时也执行一次等成本校验，
// 消除「不存在用户立即返回」与「密码校验耗时」之间的时序侧信道（防账号存在性探测）。
const dummyBcryptHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// Login 仅校验唯一账号的用户名与密码；OIDC 模式禁止本地登录。
func (s *authService) Login(ctx context.Context, req *v1.LoginRequest, clientIP string) (*v1.LoginResponseData, error) {
	// 服务层也限制长度，避免内部调用绕过 DTO 后将异常输入送入限流或密码校验。
	if length := utf8.RuneCountInString(req.Username); length > 32 {
		return nil, &v1.Error{Code: http.StatusBadRequest, Message: fmt.Sprintf("username 过长：%d 个字符，最多允许 32 个字符", length)}
	}
	// 失败原因各自成文：都回「登录失效」会让用户把「密码错」「被限流」「账号停用」当成会话过期，
	// 无法判断该改什么（上游同样区分 authIncorrectCreds / authUserInactiveOrDeleted）。
	if !s.loginLimiter.Allow(loginRateKey(clientIP, req.Username)) {
		return nil, &v1.Error{Code: http.StatusUnauthorized, Message: "尝试过于频繁，请稍后再试"}
	}
	// 未知用户与密码错误使用相同文案，防止探测账号是否存在。
	const badCredentials = "用户名或密码错误"
	if s.oidc.Enabled {
		return nil, oidcOnlyError()
	}
	user, err := s.repo.SingleUser(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// 等成本：对 dummy 哈希跑一次 BcryptCheck（忽略结果），时序与真实校验一致。
			_ = hash.BcryptCheck(req.Password, dummyBcryptHash)
			return nil, &v1.Error{Code: http.StatusUnauthorized, Message: badCredentials}
		}
		return nil, fmt.Errorf("%w: 读取用户失败：%v", v1.ErrInternalServerError, err)
	}
	if user.Username != req.Username {
		_ = hash.BcryptCheck(req.Password, dummyBcryptHash)
		return nil, &v1.Error{Code: http.StatusUnauthorized, Message: badCredentials}
	}
	if err := hash.BcryptCheck(req.Password, user.Password); err != nil {
		return nil, &v1.Error{Code: http.StatusUnauthorized, Message: badCredentials}
	}
	// Active 检查必须在密码校验之后：放在前面会让密码错误时也能区分「账号存在但停用」，
	// 形成账号存在性探测通道。
	if !user.Active {
		return nil, &v1.Error{Code: http.StatusUnauthorized, Message: "该账号已被停用"}
	}
	return s.session(&user)
}

// Setup 创建首个管理员账号（仅当无任何用户时），成功即返回登录态。
func (s *authService) Setup(ctx context.Context, req *v1.SetupRequest) (*v1.LoginResponseData, error) {
	if s.oidc.Enabled {
		return nil, oidcOnlyError()
	}
	if !ValidateUsername(req.Username) {
		return nil, &v1.Error{Code: 400, Message: fmt.Sprintf("用户名不合法：%s（需 2-32 位字母、数字或 _ . -）", req.Username)}
	}
	count, err := s.repo.CountUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: 检查初始化状态失败：%v", v1.ErrInternalServerError, err)
	}
	if count > 0 {
		return nil, &v1.Error{Code: 409, Message: "Dockge 已完成初始化"}
	}
	if !ValidatePasswordStrength(req.Password) {
		return nil, &v1.Error{Code: 400, Message: "密码至少6位且需包含字母和数字"}
	}
	hashed, err := hash.BcryptHash(req.Password)
	if err != nil {
		return nil, fmt.Errorf("%w: 密码加密失败：%v", v1.ErrInternalServerError, err)
	}
	user := &model.DockgeUser{
		Username: req.Username, Nickname: req.Username,
		Password: hashed, Role: model.RoleAdmin, Active: true, Source: model.SourceLocal,
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, &v1.Error{Code: 409, Message: "Dockge 已完成初始化，单用户模式不允许新增账号"}
		}
		return nil, fmt.Errorf("%w: 创建管理员账号失败：%v", v1.ErrInternalServerError, err)
	}
	return s.session(user)
}

// CheckNeedSetup 判断是否需要首次安装引导（用户数为 0）。
func (s *authService) CheckNeedSetup(ctx context.Context) (bool, error) {
	if s.oidc.Enabled {
		return false, nil
	}
	count, err := s.repo.CountUsers(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// Me 仅返回唯一账号的身份，不暴露历史额外账号。
func (s *authService) Me(ctx context.Context, uid uint) (*v1.MeUserData, error) {
	user, err := s.repo.SingleUser(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("%w: 用户不存在", v1.ErrNotFound)
		}
		return nil, fmt.Errorf("%w: 读取用户失败：%v", v1.ErrInternalServerError, err)
	}
	if user.ID != uid {
		return nil, v1.ErrUnauthorized
	}
	mp := meData(&user)
	return &mp, nil
}

// ChangePassword 校验旧密码与强度后更新密码；
// 旧 token 因 claims 中的密码摘要绑定（CheckSession）自动失效。
func (s *authService) ChangePassword(ctx context.Context, uid uint, req *v1.ChangePasswordRequest) error {
	if s.oidc.Enabled {
		return oidcOnlyError()
	}
	user, err := s.repo.SingleUser(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("%w: 用户不存在", v1.ErrNotFound)
		}
		return fmt.Errorf("%w: 读取用户失败：%v", v1.ErrInternalServerError, err)
	}
	if user.ID != uid {
		return v1.ErrUnauthorized
	}
	if err := hash.BcryptCheck(req.OldPassword, user.Password); err != nil {
		// 上游此处有明确文案，不能落回笼统的「参数错误」
		return &v1.Error{Code: http.StatusBadRequest, Message: "当前密码不正确"}
	}
	if !ValidatePasswordStrength(req.NewPassword) {
		return &v1.Error{Code: http.StatusBadRequest, Message: "新密码至少6位且需包含字母和数字"}
	}
	hashed, err := hash.BcryptHash(req.NewPassword)
	if err != nil {
		return fmt.Errorf("%w: 密码加密失败：%v", v1.ErrInternalServerError, err)
	}
	return s.repo.UpdatePassword(ctx, uid, hashed)
}

// CheckSession 逐请求校验会话有效性：用户存在、启用，且密码哈希摘要与 token 一致。
func (s *authService) CheckSession(ctx context.Context, uid uint, h string) error {
	user, err := s.repo.SingleUser(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return v1.ErrUnauthorized
		}
		return err
	}
	if user.ID != uid || !user.Active {
		return v1.ErrUnauthorized
	}
	if hash.Shake256(sessionSecret(&user, s.oidc)) != h {
		return v1.ErrUnauthorized // 密码已修改，旧 token 失效
	}
	return nil
}

// GetDisableAuth 读取免登录模式开关。
func (s *authService) GetDisableAuth(ctx context.Context) bool {
	if s.oidc.Enabled {
		return false
	}
	v, err := s.repo.GetSetting(ctx, "disableAuth")
	if err != nil {
		return false
	}
	return v == "true"
}

// ToggleDisableAuth 切换免登录模式：切换到关闭认证时须校验当前密码。
func (s *authService) ToggleDisableAuth(ctx context.Context, uid uint, enable bool, currentPassword string) error {
	if s.oidc.Enabled {
		return oidcOnlyError()
	}
	if enable {
		user, err := s.repo.SingleUser(ctx)
		if err != nil {
			return err
		}
		if user.ID != uid {
			return v1.ErrUnauthorized
		}
		if err := hash.BcryptCheck(currentPassword, user.Password); err != nil {
			return &v1.Error{Code: http.StatusBadRequest, Message: "当前密码不正确"}
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
	user, err := s.repo.SingleUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: 读取单用户账号失败：%v", v1.ErrInternalServerError, err)
	}
	if !user.Active {
		return nil, v1.ErrUnauthorized
	}
	return s.session(&user)
}

// session 签发绑定密码哈希的本地 JWT。
func (s *authService) session(user *model.DockgeUser) (*v1.LoginResponseData, error) {
	expiry := time.Now().Add(SessionTTL).Truncate(time.Second)
	token, err := s.jwt.GenToken(user.ID, sessionSecret(user, s.oidc), expiry)
	if err != nil {
		return nil, fmt.Errorf("%w: 签发会话令牌失败：%v", v1.ErrInternalServerError, err)
	}
	return &v1.LoginResponseData{AccessToken: token, ExpiresAt: expiry.UnixMilli(), User: meData(user)}, nil
}

// meData 把用户实体转为对外视图（角色为空的历史用户按 admin 展示）。
func meData(u *model.DockgeUser) v1.MeUserData {
	return v1.MeUserData{
		ID: u.ID, Username: u.Username, Nickname: u.Nickname,
		Role: model.RoleAdmin,
	}
}

func oidcOnlyError() error {
	return &v1.Error{Code: 403, Message: "已启用 OIDC：本地初始化、密码登录、改密及免认证模式已禁用，请使用配置的唯一 OIDC 身份登录"}
}

func sessionSecret(u *model.DockgeUser, c config.OIDC) string {
	if !c.Enabled {
		return u.Password
	}
	return u.Password + "\x00oidc\x00" + c.Issuer + "\x00" + c.ClientID + "\x00" + c.Subject
}
