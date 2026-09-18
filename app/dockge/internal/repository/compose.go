// compose 栈机制：栈的发现（compose ls）、状态（compose ps）、
// 生命周期操作（up/stop/restart/down/pull）与日志，全部通过 docker compose CLI
// 完成，与原版 Dockge（node-pty 执行相同命令）保持一致。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dockge/app/dockge/internal/model"
)

// composeOpTimeout 是单次 compose 生命周期操作的默认超时（拉镜像的 update 单独放宽）。
const composeOpTimeout = 3 * time.Minute

// composeUpdateTimeout 是 update（pull + up）的超时，镜像拉取可能较慢。
const composeUpdateTimeout = 10 * time.Minute

// composeValidateTimeout 是草稿校验（docker compose config）的超时；
// config 只做插值与结构渲染，正常毫秒级完成。
const composeValidateTimeout = 15 * time.Second

// composeLsItem 映射 `docker compose ls --all --format json` 的一行。
type composeLsItem struct {
	Name        string `json:"Name"`
	Status      string `json:"Status"`
	ConfigFiles string `json:"ConfigFiles"`
}

// composePsItem 映射 `docker compose ps --format json` 的一行（NDJSON）。
type composePsItem struct {
	ID       string `json:"ID"`
	Name     string `json:"Name"`
	Service  string `json:"Service"`
	State    string `json:"State"`
	Health   string `json:"Health"`
	ExitCode int    `json:"ExitCode"`
}

// StackStatusFromString 把 `docker compose ls` 的 Status 字符串转换为状态枚举。
// 输入形如 "running(3)"、"exited(6), running(5)"、"created"。
func StackStatusFromString(status string) model.StackStatus {
	switch {
	case strings.HasPrefix(status, "created"):
		return model.StatusCreated
	case strings.Contains(status, "exited"):
		// 任一服务退出即视为栈已停止（与 Dockge 的 statusConvert 一致）
		return model.StatusExited
	case strings.HasPrefix(status, "running"):
		return model.StatusRunning
	default:
		return model.StatusUnknown
	}
}

// ComposeLs 返回 docker compose 已注册的全部项目（含非本程序托管的）。
func (r *Repository) ComposeLs(ctx context.Context) ([]composeLsItem, error) {
	out, err := runDocker(ctx, composeOpTimeout, "compose", "ls", "--all", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("docker compose ls: %w", err)
	}
	items := make([]composeLsItem, 0)
	if strings.TrimSpace(out) == "" {
		return items, nil
	}
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("parse compose ls output: %w", err)
	}
	return items, nil
}

// resolveStackExec 解析栈的执行上下文：
// 托管栈（stacks 目录存在）返回其目录；外部栈从 compose ls 的 ConfigFiles
// 反查 compose 文件路径（已不存在的文件被过滤）。返回空 files 时
// down 仍可按项目名 label 执行，其余操作将由 compose 报缺文件错误。
func (r *Repository) resolveStackExec(ctx context.Context, name string) (dir string, files []string, err error) {
	stackDir := r.StackPath(name)
	if info, statErr := os.Stat(stackDir); statErr == nil && info.IsDir() {
		return stackDir, nil, nil
	}
	items, err := r.ComposeLs(ctx)
	if err != nil {
		return "", nil, err
	}
	for _, item := range items {
		if item.Name != name || item.ConfigFiles == "" {
			continue
		}
		for _, f := range strings.Split(item.ConfigFiles, ",") {
			if f = strings.TrimSpace(f); f != "" && fileExists(f) {
				files = append(files, f)
			}
		}
		if len(files) > 0 {
			return filepath.Dir(files[0]), files, nil
		}
	}
	return r.stacksDir, nil, nil
}

// fileExists 判断常规文件是否存在。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// composeArgs 组装栈操作的 compose 参数：托管栈仅 -p；外部栈附带 -f 指向真实配置文件。
func composeArgs(name string, files []string, extra ...string) []string {
	args := []string{"compose", "-p", name}
	if len(files) > 0 {
		for _, f := range files {
			args = append(args, "-f", f)
		}
	}
	return append(args, extra...)
}

// StackPs 返回指定栈的容器列表（docker compose ps）。
// 栈从未部署时 compose ps 返回空输出，视为无容器；
// 外部栈按 compose ls 反查的配置文件执行，工作目录不存在也不受影响。
func (r *Repository) StackPs(ctx context.Context, name string) ([]model.Container, error) {
	stackDir, files, err := r.resolveStackExec(ctx, name)
	if err != nil {
		return nil, err
	}
	out, err := runDockerIn(ctx, stackDir, composeOpTimeout,
		composeArgs(name, files, "ps", "--all", "--format", "json")...)
	if err != nil {
		// compose v2 在项目不存在时可能报错，统一按无容器处理
		return []model.Container{}, nil
	}
	items := make([]composePsItem, 0)
	if strings.TrimSpace(out) == "" {
		return []model.Container{}, nil
	}
	// 兼容两种输出：JSON 数组或 NDJSON（每行一个对象）
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		items = items[:0]
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var item composePsItem
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				return nil, fmt.Errorf("parse compose ps output: %w", err)
			}
			items = append(items, item)
		}
	}
	containers := make([]model.Container, 0, len(items))
	for _, item := range items {
		status := strings.TrimSpace(item.Health)
		if status == "" || status == "none" {
			status = fmt.Sprintf("exit(%d)", item.ExitCode)
			if item.State == "running" {
				status = "running"
			}
		}
		containers = append(containers, model.Container{
			ID:      shortID(item.ID),
			Name:    item.Name,
			Service: item.Service,
			State:   item.State,
			Status:  status,
		})
	}
	return containers, nil
}

// StackOp 执行栈生命周期操作，返回 compose 命令的组合输出供前端展示。
// op 取值：start（up -d）/ stop / restart / down / update（pull + up -d）。
func (r *Repository) StackOp(ctx context.Context, name, op string) (string, error) {
	stackDir, files, err := r.resolveStackExec(ctx, name)
	if err != nil {
		return "", fmt.Errorf("找不到栈 %s 的 compose 文件，无法执行 %s", name, op)
	}
	runCompose := func(timeout time.Duration, args ...string) (string, error) {
		return runDockerIn(ctx, stackDir, timeout, composeArgs(name, files, args...)...)
	}
	switch op {
	case "start":
		return runCompose(composeOpTimeout, "up", "-d", "--remove-orphans")
	case "stop":
		return runCompose(composeOpTimeout, "stop")
	case "restart":
		return runCompose(composeOpTimeout, "restart")
	case "down":
		out, err := runCompose(composeOpTimeout, "down", "--remove-orphans")
		if err == nil {
			return out, nil
		}
		// 回退：不加载 compose 文件、不做变量插值，按项目名 label 匹配清理容器。
		// 覆盖两类场景：compose 文件已被删除；文件要求的环境变量在当前进程缺失。
		fallback, fbErr := runDockerIn(ctx, r.stacksDir, composeOpTimeout,
			"compose", "-p", name, "down", "--remove-orphans")
		if fbErr != nil {
			return out + "\n" + fallback, fmt.Errorf("stack down: %w", err)
		}
		return out + "\n" + fallback + "\n（compose 文件不可用，已按项目名清理容器与网络）", nil
	case "update":
		pullOut, err := runCompose(composeUpdateTimeout, "pull")
		if err != nil {
			return pullOut, fmt.Errorf("stack pull: %w", err)
		}
		upOut, err := runCompose(composeOpTimeout, "up", "-d", "--remove-orphans")
		return pullOut + upOut, err
	default:
		return "", fmt.Errorf("unknown stack op: %s", op)
	}
}

// ValidateCompose 对草稿执行 docker compose config：写入临时目录后以该目录
// 为工作目录运行，零副作用（不落盘栈目录、不创建 docker 资源），defer 清理。
// env 非空时一并写入 .env，使变量插值与真实部署一致。
func (r *Repository) ValidateCompose(ctx context.Context, yaml, env string) (string, error) {
	dir, err := os.MkdirTemp("", "dockge-validate-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(yaml), 0o600); err != nil {
		return "", fmt.Errorf("write draft compose: %w", err)
	}
	if env != "" {
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o600); err != nil {
			return "", fmt.Errorf("write draft env: %w", err)
		}
	}
	return runDockerIn(ctx, dir, composeValidateTimeout,
		"compose", "-p", "dockge-validate", "-f", "compose.yaml", "config")
}

// StackExecDir 返回栈 compose 操作的工作目录：
// 托管栈为栈目录，外部栈为 compose 配置文件所在目录。
func (r *Repository) StackExecDir(ctx context.Context, name string) (string, error) {
	dir, _, err := r.resolveStackExec(ctx, name)
	return dir, err
}
