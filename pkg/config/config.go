// Package config 提供配置加载：无配置文件——一切来自内置默认值与
// DOCKGE_ 前缀的环境变量（部署侧由该服务自身 compose.yaml 的
// environment 段定义，键名 = 配置键大写并以 _ 连接，如 DOCKGE_HTTP_PORT）。
package config

import (
	"strings"

	"github.com/spf13/viper"
)

// Load 返回配置：内置默认值 + DOCKGE_* 环境变量覆盖。
func Load() (*viper.Viper, error) {
	conf := viper.New()
	conf.SetEnvPrefix("DOCKGE")
	conf.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	conf.AutomaticEnv()
	conf.SetDefault("env", "prod")
	conf.SetDefault("http.host", "0.0.0.0")
	conf.SetDefault("http.port", 5001)
	// 与镜像内默认钥匙一致：安全自检对默认值告警，不因零配置而放宽
	conf.SetDefault("security.jwt.key", "change-me-in-production")
	conf.SetDefault("container.cli", "auto")
	conf.SetDefault("container.compose", "")
	conf.SetDefault("stacks_dir", "storage/stacks")
	conf.SetDefault("data.db.user.dsn", "storage/dockge.db")
	conf.SetDefault("log.log_level", "info")
	conf.SetDefault("log.mode", "console")
	conf.SetDefault("log.encoding", "console")
	conf.SetDefault("log.log_file_name", "storage/logs/dockge.log")
	conf.SetDefault("log.max_backups", 7)
	conf.SetDefault("log.max_age", 7)
	conf.SetDefault("log.max_size", 10)
	conf.SetDefault("log.compress", true)
	for _, key := range conf.AllKeys() {
		if err := conf.BindEnv(key); err != nil {
			return nil, err
		}
	}
	return conf, nil
}
