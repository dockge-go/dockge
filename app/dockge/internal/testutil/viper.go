// Package testutil 提供跨包共享的测试辅助函数。
package testutil

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// ViperFromYAML 从 YAML 字符串构建 viper 实例（用于测试配置解析）。
func ViperFromYAML(t *testing.T, yaml string) *viper.Viper {
	t.Helper()
	conf := viper.New()
	conf.SetConfigType("yaml")
	if err := conf.ReadConfig(strings.NewReader(yaml)); err != nil {
		t.Fatalf("read config: %v", err)
	}
	return conf
}
