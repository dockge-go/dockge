package service

// 密码强度校验：Setup / ChangePassword / AdminCreateUser 共用的唯一入口。
// （原 pkg/totp 随 2FA 功能整体移除，此函数是其唯一存留消费者。）

// ValidatePasswordStrength 校验密码强度：≥6 位且含字母和数字。
// HTTP 侧（Setup/ChangePassword）与 CLI 侧（reset-password）共用此唯一规则，
// 避免两条路径判定不一致、用户被"网页能设命令行不能设"困惑。
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
