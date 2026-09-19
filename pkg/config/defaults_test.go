package config

import "testing"

// 内置默认配置：裸二进制零配置可跑，且 APP_* 环境变量可覆盖默认值。
func TestDefaults(t *testing.T) {
	cases := []struct {
		key, env, want string
	}{
		{"http.port", "", "5001"},
		{"env", "", "prod"},
		{"dockge.stacks_dir", "", "storage/stacks"},
		{"data.db.user.dsn", "", "storage/dockge.db"},
		{"log.mode", "", "console"},
		{"http.port", "APP_HTTP_PORT", "15099"}, // 环境变量覆盖默认值
	}
	for _, c := range cases {
		t.Run(c.key+"="+c.want, func(t *testing.T) {
			if c.env != "" {
				t.Setenv(c.env, c.want)
			}
			conf, err := Defaults()
			if err != nil {
				t.Fatal(err)
			}
			if got := conf.GetString(c.key); got != c.want {
				t.Fatalf("%s = %q, want %q", c.key, got, c.want)
			}
		})
	}
}
