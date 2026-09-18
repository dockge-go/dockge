package service

// 密码强度校验：Setup / ChangePassword / AdminCreateUser 共用的唯一入口。
// （原 pkg/totp 随 2FA 功能整体移除，此函数是其唯一存留消费者。）

// validatePasswordStrength 校验密码强度：≥6 位且含字母和数字。
func validatePasswordStrength(pw string) bool {
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
