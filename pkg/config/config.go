// Package config 加载默认配置与 DOCKGE_* 高级环境覆盖。容器部署保持内部路径固定。
package config

import (
	"fmt"
	"strconv"
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
	conf.SetDefault("oidc.enabled", false)
	for _, key := range []string{"issuer", "client_id", "client_secret", "redirect_url", "subject"} {
		conf.SetDefault("oidc."+key, "")
	}
	conf.SetDefault("http.host", "0.0.0.0")
	conf.SetDefault("http.port", 5001)
	// 未配置标记；仓储初始化时生成并持久化随机密钥，不用此字符串签名。
	conf.SetDefault("security.jwt.key", "change-me-in-production")
	conf.SetDefault("container.cli", "auto")
	conf.SetDefault("container.compose", "")
	conf.SetDefault("stacks_dir", "/opt/stacks")
	conf.SetDefault("data.db.user.dsn", "/app/data/dockge.db")
	conf.SetDefault("log.log_level", "info")
	conf.SetDefault("log.mode", "console")
	conf.SetDefault("log.encoding", "console")
	conf.SetDefault("log.log_file_name", "/app/data/logs/dockge.log")
	conf.SetDefault("log.max_backups", 7)
	conf.SetDefault("log.max_age", 7)
	conf.SetDefault("log.max_size", 10)
	conf.SetDefault("log.compress", true)
	for _, key := range conf.AllKeys() {
		if err := conf.BindEnv(key); err != nil {
			return nil, err
		}
	}
	if _, err := strconv.ParseBool(fmt.Sprint(conf.Get("oidc.enabled"))); err != nil {
		return nil, fmt.Errorf("DOCKGE_OIDC_ENABLED 必须是 true 或 false，实际为 %q：%w", conf.Get("oidc.enabled"), err)
	}
	if err := OIDCFrom(conf).Validate(); err != nil {
		return nil, err
	}
	return conf, nil
}
