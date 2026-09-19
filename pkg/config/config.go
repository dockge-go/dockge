// Package config 提供基于 viper 的配置加载：支持文件路径与 APP_CONF
// 环境变量两种来源，并自动映射 APP_ 前缀的环境变量覆盖。
package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

// New loads the app config from the given path, or from the APP_CONF
// environment variable when set.
func New(p string) (*viper.Viper, error) {
	envConf := os.Getenv("APP_CONF")
	if envConf == "" {
		envConf = p
	}
	conf := viper.New()
	conf.SetConfigFile(envConf)
	if err := conf.ReadInConfig(); err != nil {
		return nil, err
	}
	if err := bindEnvKeys(conf); err != nil {
		return nil, err
	}
	return conf, nil
}

// bindEnvKeys 显式为每个已知键绑定环境变量：AutomaticEnv 只对「未设值」的键生效，
// 显式绑定才能保证 APP_* 覆盖配置文件里的值（如 APP_CONTAINER_CLI=podman）。
func bindEnvKeys(conf *viper.Viper) error {
	for _, key := range conf.AllKeys() {
		if err := conf.BindEnv(key); err != nil {
			return err
		}
	}
	return nil
}

// Defaults 返回内置默认配置：配置文件与 APP_CONF 均不存在时兜底，
// 使裸二进制下载即可运行；内容对齐 prod，仅路径改为可移植的相对路径。
func Defaults() (*viper.Viper, error) {
	conf := viper.New()
	conf.SetEnvPrefix("APP")
	conf.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	conf.AutomaticEnv()
	conf.SetDefault("env", "prod")
	conf.SetDefault("http.host", "0.0.0.0")
	conf.SetDefault("http.port", 5001)
	// 与镜像内默认钥匙一致：安全自检对默认值告警，不因零配置而放宽
	conf.SetDefault("security.jwt.key", "change-me-in-production")
	conf.SetDefault("container.cli", "auto")
	conf.SetDefault("container.compose", "")
	conf.SetDefault("dockge.stacks_dir", "storage/stacks")
	conf.SetDefault("data.db.user.dsn", "storage/dockge.db")
	conf.SetDefault("log.log_level", "info")
	conf.SetDefault("log.mode", "console")
	conf.SetDefault("log.encoding", "console")
	conf.SetDefault("log.log_file_name", "storage/logs/dockge.log")
	conf.SetDefault("log.max_backups", 7)
	conf.SetDefault("log.max_age", 7)
	conf.SetDefault("log.max_size", 10)
	conf.SetDefault("log.compress", true)
	if err := bindEnvKeys(conf); err != nil {
		return nil, err
	}
	return conf, nil
}
