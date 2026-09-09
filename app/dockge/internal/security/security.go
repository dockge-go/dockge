// Package security 定义认证配置的唯一命名空间 security.auth.*。
// 所有认证模式（jwt/proxy/oidc/disable）的取值与解析集中在此，
// 避免各层各自读取字符串字面量导致配置漂移。
package security

import "github.com/spf13/viper"

// Mode 是 security.auth.mode 的取值。
type Mode string

const (
	ModeJWT     Mode = "jwt"     // 默认：本地 JWT 登录
	ModeProxy   Mode = "proxy"   // 受信反向代理头认证
	ModeOIDC    Mode = "oidc"    // OIDC 授权码 + PKCE
	ModeDisable Mode = "disable" // 免登录（配合 disableAuth 设置项）
)

// AuthMode 返回 security.auth.mode 的当前值；未配置或非法值按 jwt 处理。
func AuthMode(conf *viper.Viper) Mode {
	m := Mode(conf.GetString("security.auth.mode"))
	switch m {
	case ModeProxy, ModeOIDC, ModeDisable:
		return m
	default:
		return ModeJWT
	}
}
