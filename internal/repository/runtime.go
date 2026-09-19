package repository

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runtime 描述容器运行时命令行。支持任意提供 docker 兼容 CLI 的实现：
// docker、podman、nerdctl，以及自建的包装脚本；OCI 低层运行时（youki/crun 等）
// 通过 podman/containerd 配置选用，无需本层感知。
type Runtime struct {
	bin        string   // 容器 CLI（docker/podman/nerdctl/...）
	composeBin string   // compose 命令的可执行文件
	composeSub []string // compose 子命令（bin 模式下为 ["compose"]；独立命令为空）
}

// 自动探测顺序：docker → podman → nerdctl（配置 container.cli 可显式指定）。
var runtimeCandidates = []string{"docker", "podman", "nerdctl"}

// DetectRuntime 解析运行时配置：
//   - cli 为空或 "auto"：按 PATH 探测候选，全部缺失时回退 docker（错误在调用处显式暴露）
//   - compose 为空：使用 `<cli> compose`；否则按空格解析（如 "podman-compose"、"docker compose"）
func DetectRuntime(cli, compose string) Runtime {
	bin := strings.TrimSpace(cli)
	if bin == "" || bin == "auto" {
		bin = "docker"
		for _, candidate := range runtimeCandidates {
			if _, err := exec.LookPath(candidate); err == nil {
				bin = candidate
				break
			}
		}
	}

	compose = strings.TrimSpace(compose)
	if compose == "" {
		return Runtime{bin: bin, composeBin: bin, composeSub: []string{"compose"}}
	}
	fields := strings.Fields(compose)
	if len(fields) == 1 {
		// 独立 compose 命令（podman-compose 等）：子命令序列为空
		return Runtime{bin: bin, composeBin: fields[0]}
	}
	return Runtime{bin: bin, composeBin: fields[0], composeSub: fields[1:]}
}

// resolved 补齐零值：未显式配置时按自动探测处理（零值构造的 Repository 同样可用）。
func (r Runtime) resolved() Runtime {
	if r.bin == "" {
		return DetectRuntime("", "")
	}
	return r
}

// Command 构造容器命令（exec/stats/network/ps 等）。
func (r Runtime) Command(ctx context.Context, args ...string) (*exec.Cmd, error) {
	r = r.resolved()
	return r.command(ctx, r.bin, nil, args)
}

// ComposeCommand 构造 compose 命令（docker compose / podman compose / podman-compose）。
func (r Runtime) ComposeCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	r = r.resolved()
	return r.command(ctx, r.composeBin, r.composeSub, args)
}

// command 组装可执行命令；同一命令名只解析一次路径（缓存），避免重复 PATH 查找。
func (r Runtime) command(ctx context.Context, bin string, prefix, args []string) (*exec.Cmd, error) {
	if bin == "" {
		return nil, fmt.Errorf("容器运行时未配置")
	}
	if _, err := exec.LookPath(bin); err != nil {
		return nil, fmt.Errorf("找不到容器运行时命令 %q（请安装 docker/podman/nerdctl 或配置 container.cli）", bin)
	}
	full := make([]string, 0, len(prefix)+len(args))
	full = append(full, prefix...)
	full = append(full, args...)
	return exec.CommandContext(ctx, bin, full...), nil
}
