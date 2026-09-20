package config

import "testing"

// 零配置加载：裸二进制默认值可跑，DOCKGE_* 环境变量可覆盖默认值。
func TestLoad(t *testing.T) {
	cases := []struct {
		key, env, want string
	}{
		{"http.port", "", "5001"},
		{"env", "", "prod"},
		{"stacks_dir", "", "storage/stacks"},
		{"data.db.user.dsn", "", "storage/dockge.db"},
		{"log.mode", "", "console"},
		{"http.port", "DOCKGE_HTTP_PORT", "15099"}, // 环境变量覆盖默认值
		{"stacks_dir", "DOCKGE_STACKS_DIR", "/opt/stacks"},
	}
	for _, c := range cases {
		t.Run(c.key+"="+c.want, func(t *testing.T) {
			if c.env != "" {
				t.Setenv(c.env, c.want)
			}
			conf, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if got := conf.GetString(c.key); got != c.want {
				t.Fatalf("%s = %q, want %q", c.key, got, c.want)
			}
		})
	}
}
