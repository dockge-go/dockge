// Package service 是 dockge 应用的用例编排层。
package service

import (
	"context"

	"dockge/internal/repository"

	"github.com/samber/do/v2"
)

// Package 注册用例层依赖。
var Package = do.Package(
	do.Lazy(NewAuthService),
	do.Lazy(NewOIDCService),
	do.Lazy(NewStackService),
	do.Lazy(NewImageService),
	do.Lazy(NewResourceService),
	do.Lazy(NewSettingsService),
)

// SettingsService 提供全局环境变量与通用设置项读写用例。
type SettingsService interface {
	GetGlobalEnv(ctx context.Context) (string, error)
	SetGlobalEnv(ctx context.Context, content string) error
	GetPrimaryHostname(ctx context.Context) (string, error)
	SetPrimaryHostname(ctx context.Context, hostname string) error
}

type settingsService struct {
	repo *repository.Repository
}

// NewSettingsService 构造全局环境变量服务，由注入容器调用。
func NewSettingsService(i do.Injector) (SettingsService, error) {
	return &settingsService{repo: do.MustInvoke[*repository.Repository](i)}, nil
}

// GetGlobalEnv 返回已保存的环境变量内容，未配置时返回空字符串。
func (s *settingsService) GetGlobalEnv(ctx context.Context) (string, error) {
	data, err := s.repo.GetAllSettingsByType(ctx, "general")
	if err != nil {
		return "", err
	}
	return data["globalENV"], nil
}

// SetGlobalEnv 写入全局环境变量（存储于 settings 表）。
func (s *settingsService) SetGlobalEnv(ctx context.Context, content string) error {
	return s.repo.SetSetting(ctx, "globalENV", content, "general")
}

// GetPrimaryHostname 读取主主机名（空 = 前端回退 location.hostname）。
func (s *settingsService) GetPrimaryHostname(ctx context.Context) (string, error) {
	data, err := s.repo.GetAllSettingsByType(ctx, "general")
	if err != nil {
		return "", err
	}
	return data["primaryHostname"], nil
}

// SetPrimaryHostname 写入主主机名。
func (s *settingsService) SetPrimaryHostname(ctx context.Context, hostname string) error {
	return s.repo.SetSetting(ctx, "primaryHostname", hostname, "general")
}
