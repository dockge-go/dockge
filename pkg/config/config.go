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
	conf.SetEnvPrefix("APP")
	conf.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	conf.AutomaticEnv()
	if err := conf.ReadInConfig(); err != nil {
		return nil, err
	}
	return conf, nil
}
