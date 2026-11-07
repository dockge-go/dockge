package service

// ValidatePasswordStrength 统一初始化账号与修改密码的规则：至少 6 位且含字母和数字。
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
