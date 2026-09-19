package repository

import (
	"context"
	"testing"
)

// DetectRuntime 的解析表：配置优先、自动探测回退、compose 前缀三种形态。
func TestDetectRuntime(t *testing.T) {
	cases := []struct {
		name        string
		cli         string
		compose     string
		wantBin     string
		wantCompose string
		wantSub     []string
	}{
		{"显式 docker + 默认 compose 前缀", "docker", "", "docker", "docker", []string{"compose"}},
		{"显式 podman + 默认 compose 前缀", "podman", "", "podman", "podman", []string{"compose"}},
		{"显式 nerdctl", "nerdctl", "", "nerdctl", "nerdctl", []string{"compose"}},
		{"独立 compose 命令（podman-compose）", "podman", "podman-compose", "podman", "podman-compose", nil},
		{"compose 命令带参数（docker compose）", "docker", "docker compose", "docker", "docker", []string{"compose"}},
		{"auto 显式等价于空", "auto", "", "docker", "docker", []string{"compose"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectRuntime(c.cli, c.compose)
			if c.cli == "podman" || c.cli == "nerdctl" || c.cli == "docker" || c.cli == "auto" {
				if got.bin != c.wantBin {
					t.Fatalf("bin = %q, want %q", got.bin, c.wantBin)
				}
			}
			if got.composeBin != c.wantCompose {
				t.Fatalf("composeBin = %q, want %q", got.composeBin, c.wantCompose)
			}
			if len(got.composeSub) != len(c.wantSub) {
				t.Fatalf("composeSub = %v, want %v", got.composeSub, c.wantSub)
			}
			for i := range c.wantSub {
				if got.composeSub[i] != c.wantSub[i] {
					t.Fatalf("composeSub = %v, want %v", got.composeSub, c.wantSub)
				}
			}
		})
	}
}

// auto 探测：本机存在 docker 时应解析为 docker（PATH 中无 podman/nerdctl 时）。
func TestDetectRuntimeAutoPrefersFirstAvailable(t *testing.T) {
	got := DetectRuntime("", "")
	if got.bin == "" {
		t.Fatal("bin 不应为空")
	}
	found := false
	for _, candidate := range runtimeCandidates {
		if got.bin == candidate {
			found = true
		}
	}
	if !found {
		t.Fatalf("bin = %q 不在候选列表 %v 中", got.bin, runtimeCandidates)
	}
}

// 命令构造：compose 前缀与子命令序列正确拼接；缺失命令给出可读错误。
func TestRuntimeCommandShapes(t *testing.T) {
	rt := Runtime{bin: "docker", composeBin: "docker", composeSub: []string{"compose"}}
	cmd, err := rt.ComposeCommand(context.Background(), "-p", "demo", "up", "-d")
	if err != nil {
		t.Fatalf("ComposeCommand: %v", err)
	}
	want := []string{"docker", "compose", "-p", "demo", "up", "-d"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("args = %v, want %v", cmd.Args, want)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Fatalf("args = %v, want %v", cmd.Args, want)
		}
	}

	// 独立 compose 命令：不追加子命令（用恒存在的 sh 验证参数形态，避免依赖已安装的 podman-compose）
	standalone := Runtime{bin: "podman", composeBin: "sh"}
	cmd2, err := standalone.ComposeCommand(context.Background(), "-p", "demo", "ps")
	if err != nil {
		t.Fatalf("standalone ComposeCommand: %v", err)
	}
	if cmd2.Args[0] != "sh" || cmd2.Args[1] != "-p" {
		t.Fatalf("standalone args = %v", cmd2.Args)
	}

	if _, err := (Runtime{bin: "definitely-not-a-runtime"}).Command(context.Background(), "ps"); err == nil {
		t.Fatal("缺失运行时命令应返回错误")
	}
}
