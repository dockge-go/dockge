package service

import "regexp"

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{2,32}$`)

// ValidateUsername 将初始化账号的格式约束放在服务端，避免绕过浏览器校验。
func ValidateUsername(username string) bool { return usernamePattern.MatchString(username) }
