package repository

import (
	"testing"

	"dockge/app/dockge/internal/model"
)

// TestStackStatusFromString 校验 compose ls 状态串到枚举的映射（与 Dockge 的 statusConvert 一致）。
func TestStackStatusFromString(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   model.StackStatus
	}{
		{"empty", "", model.StatusUnknown},
		{"running", "running(3)", model.StatusRunning},
		{"mixed exits", "exited(1), running(2)", model.StatusExited},
		{"all exited", "exited(6)", model.StatusExited},
		{"created", "created", model.StatusCreated},
		{"garbage", "what-is-this", model.StatusUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StackStatusFromString(tt.status); got != tt.want {
				t.Errorf("StackStatusFromString(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

// TestStackFromLabels 校验 compose 项目 label 的提取。
func TestStackFromLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels string
		want   string
	}{
		{"has project", "com.docker.compose.project=uim,other=1", "uim"},
		{"no project", "foo=bar", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stackFromLabels(tt.labels); got != tt.want {
				t.Errorf("stackFromLabels(%q) = %q, want %q", tt.labels, got, tt.want)
			}
		})
	}
}
