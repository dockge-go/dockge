package service

import "testing"

// TestValidatePasswordStrength 守护唯一密码规则：Setup / ChangePassword / CLI 三处共用。
func TestValidatePasswordStrength(t *testing.T) {
	cases := []struct {
		name string
		pw   string
		want bool
	}{
		{"字母加数字且够长", "abc123", true},
		{"六位边界", "a1b2c3", true},
		{"纯数字被拒", "123456", false},
		{"纯字母被拒", "abcdef", false},
		{"过短被拒", "ab12", false},
		{"空串被拒", "", false},
		{"含符号的字母数字组合", "a1!b2@", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidatePasswordStrength(tc.pw); got != tc.want {
				t.Errorf("ValidatePasswordStrength(%q) = %v, want %v", tc.pw, got, tc.want)
			}
		})
	}
}
