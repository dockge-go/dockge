// Package totp 提供 TOTP 双因素认证工具。
package totp

import (
	"github.com/pquerna/otp/totp"
)

// GenerateSecret 生成标准 TOTP 密钥（base32，20字节）。
func GenerateSecret() (string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Dockge",
		AccountName: "user",
	})
	if err != nil {
		return "", err
	}
	return key.Secret(), nil
}

// QRCodeURL 返回 TOTP 配置的 QR 码扫码 URL。
func QRCodeURL(secret string) string {
	key, _ := totp.Generate(totp.GenerateOpts{
		Issuer:      "Dockge",
		AccountName: "user",
		Secret:      []byte(secret),
	})
	return key.URL()
}

// Verify 校验 TOTP 码值，允许前后各一个时窗容错（30s 周期）。
func Verify(code, secret string) bool {
	return totp.Validate(code, secret)
}

// ValidatePasswordStrength 校验密码强度：≥6位且含字母和数字。
func ValidatePasswordStrength(pw string) bool {
	if len(pw) < 6 {
		return false
	}
	var hasLetter, hasDigit bool
	for _, r := range pw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}
