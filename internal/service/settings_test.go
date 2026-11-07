package service

import (
	"context"
	"testing"
)

func TestGlobalEnvReturnsStoredContent(t *testing.T) {
	repo, _ := newTestAuthRepo(t)
	s := &settingsService{repo: repo}
	ctx := context.Background()
	if got, err := s.GetGlobalEnv(ctx); err != nil || got != "" {
		t.Fatalf("未配置时应为空，got=%q err=%v", got, err)
	}
	for _, content := range []string{"TZ=UTC\n", "# 保留用户注释\n", ""} {
		if err := s.SetGlobalEnv(ctx, content); err != nil {
			t.Fatal(err)
		}
		if got, err := s.GetGlobalEnv(ctx); err != nil || got != content {
			t.Fatalf("内容必须原样读回，got=%q want=%q err=%v", got, content, err)
		}
	}
}
