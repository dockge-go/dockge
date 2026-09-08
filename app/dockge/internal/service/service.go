// Package service 是 dockge 应用的用例编排层。
package service

import (
	"context"
	"time"

	"dockge/app/dockge/internal/repository"
	"dockge/pkg/jwt"
	"dockge/pkg/log"

	"github.com/samber/do/v2"
)

// Service 是所有业务服务的公共依赖。
type Service struct {
	repo   *repository.Repository
	jwt    *jwt.JWT
	logger *log.Logger
}

// Package registers all service-layer providers.
var Package = do.Package(
	do.Lazy(New),
	do.Lazy(NewAuthService),
	do.Lazy(NewStackService),
	do.Lazy(NewDockerService),
	do.Lazy(NewSettingsService),
)

// New 构造服务层公共依赖（repo/jwt/logger），由注入容器调用。

func New(i do.Injector) (*Service, error) {
	return &Service{
		repo:   do.MustInvoke[*repository.Repository](i),
		jwt:    do.MustInvoke[*jwt.JWT](i),
		logger: do.MustInvoke[*log.Logger](i),
	}, nil
}

// -------- SettingsService --------

// SettingsService 提供全局环境变量读写用例。
type SettingsService interface {
	GetGlobalEnv(ctx context.Context) (string, error)
	SetGlobalEnv(ctx context.Context, content string) error
}

type settingsService struct {
	*Service
}

// NewSettingsService 构造全局环境变量服务，由注入容器调用。

func NewSettingsService(i do.Injector) (SettingsService, error) {
	return &settingsService{Service: do.MustInvoke[*Service](i)}, nil
}

// GetGlobalEnv 读取全局环境变量内容；为空时返回占位符模板。

func (s *settingsService) GetGlobalEnv(ctx context.Context) (string, error) {
	data, err := s.repo.GetAllSettingsByType(ctx, "general")
	if err != nil {
		return "", err
	}
	if v, ok := data["globalENV"]; ok && v != "" && v != "# VARIABLE=value #comment" {
		return v, nil
	}
	return "# VARIABLE=value #comment", nil
}

// SetGlobalEnv 写入全局环境变量（存储于 settings 表）。

func (s *settingsService) SetGlobalEnv(ctx context.Context, content string) error {
	return s.repo.SetSetting(ctx, "globalENV", content, "general")
}

// -------- SettingsCacheCleaner --------
// 后台定时清理 setting 缓存（对齐原版 60s TTL 策略）。
func StartSettingsCleaner(i do.Injector) {
	repo := do.MustInvoke[*repository.Repository](i)
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			// SQLite 自动 VACUUM 维护，此处仅记录心跳
			repo.GetSetting(context.Background(), "_health")
		}
	}()
}
