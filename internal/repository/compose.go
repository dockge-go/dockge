// compose 栈机制：栈的发现（compose ls）、状态（compose ps）、
// 生命周期操作（up/stop/restart/down/pull）与日志，全部通过 docker compose CLI
// 完成，与原版 Dockge（node-pty 执行相同命令）保持一致。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"dockge/internal/model"
)

// composeOpTimeout 是单次 compose 生命周期操作的默认超时；
// 含镜像拉取的操作（update，及首次部署内联拉镜像的 start）用 composeUpdateTimeout 放宽。
const composeOpTimeout = 3 * time.Minute

// composeUpdateTimeout 是 update（pull + up）的超时，镜像拉取可能较慢。
const composeUpdateTimeout = 10 * time.Minute

// composeLsItem 映射 `docker compose ls --all --format json` 的一行。
type composeLsItem struct {
	Name        string `json:"Name"`
	Status      string `json:"Status"`
	ConfigFiles string `json:"ConfigFiles"`
}

// composePsItem 映射 `docker compose ps --format json` 的一行（NDJSON）。
type composePsItem struct {
	ID         string `json:"ID"`
	Name       string `json:"Name"`
	Service    string `json:"Service"`
	State      string `json:"State"`
	Health     string `json:"Health"`
	ExitCode   int    `json:"ExitCode"`
	Image      string `json:"Image"`
	Publishers []composePsPublisher
}

// composePsPublisher 映射 ps 输出的端口发布条目。
type composePsPublisher struct {
	URL           string `json:"URL"`
	TargetPort    int    `json:"TargetPort"`
	PublishedPort int    `json:"PublishedPort"`
	Protocol      string `json:"Protocol"`
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
	out, err := r.runComposeIn(ctx, "", composeOpTimeout, "ls", "--all", "--format", "json")
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
// 不含 "compose" 子命令本身——由 Runtime.ComposeCommand 按运行时补全。
func composeArgs(name string, files []string, extra ...string) []string {
	args := []string{"-p", name}
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
	out, err := r.runComposeIn(ctx, stackDir, composeOpTimeout,
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
		ports := make([]model.PortMapping, 0, len(item.Publishers))
		for _, p := range item.Publishers {
			ports = append(ports, model.PortMapping{
				HostIP: p.URL, HostPort: p.PublishedPort,
				ContainerPort: p.TargetPort, Protocol: p.Protocol,
			})
		}
		containers = append(containers, model.Container{
			ID:      shortID(item.ID),
			Name:    item.Name,
			Service: item.Service,
			Image:   item.Image,
			State:   item.State,
			Status:  status,
			Ports:   ports,
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
		return r.runComposeIn(ctx, stackDir, timeout, composeArgs(name, files, args...)...)
	}
	// up -d 在镜像缺失时内联拉取，与 pull 同用放宽的超时
	upTimeout := composeUpdateTimeout
	switch op {
	case "start":
		return runCompose(upTimeout, "up", "-d", "--remove-orphans")
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
		fallback, fbErr := r.runComposeIn(ctx, r.stacksDir, composeOpTimeout,
			"-p", name, "down", "--remove-orphans")
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

// ServiceOp 对栈内单个服务执行 compose 操作（up/stop/restart <service>），
// 与上游 dockge 的服务级操作对齐。
func (r *Repository) ServiceOp(ctx context.Context, name, service, op string) (string, error) {
	stackDir, files, err := r.resolveStackExec(ctx, name)
	if err != nil {
		return "", fmt.Errorf("找不到栈 %s 的 compose 文件，无法执行 %s", name, op)
	}
	var args []string
	switch op {
	case "start":
		args = []string{"up", "-d", service}
	case "stop":
		args = []string{"stop", service}
	case "restart":
		args = []string{"restart", service}
	default:
		return "", fmt.Errorf("unknown service op: %s", op)
	}
	return r.runComposeIn(ctx, stackDir, composeOpTimeout, composeArgs(name, files, args...)...)
}

// StackStats 返回栈内容器的即时资源占用（docker stats --no-stream）。
// 栈未部署（无容器 ID）时返回空切片。
func (r *Repository) StackStats(ctx context.Context, name string) ([]model.ContainerStat, error) {
	idsOut, err := r.runComposeIn(ctx, "", composeOpTimeout, "-p", name, "ps", "--all", "-q")
	if err != nil {
		return []model.ContainerStat{}, nil
	}
	ids := strings.Fields(idsOut)
	if len(ids) == 0 {
		return []model.ContainerStat{}, nil
	}
	statsArgs := append([]string{"stats", "--no-stream", "--format", "json"}, ids...)
	out, err := r.runDocker(ctx, composeOpTimeout, statsArgs...)
	if err != nil {
		return nil, fmt.Errorf("docker stats: %w", err)
	}
	stats := make([]model.ContainerStat, 0)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		var stat model.ContainerStat
		if err := json.Unmarshal([]byte(line), &stat); err != nil {
			continue // 跳过无法解析的行（容器刚退出等）
		}
		stats = append(stats, stat)
	}
	return stats, nil
}

// DockerNetworkNames 返回本机网络名列表（编辑器 Networks 字段的建议来源）。
func (r *Repository) DockerNetworkNames(ctx context.Context) ([]string, error) {
	out, err := r.runDocker(ctx, composeOpTimeout, "network", "ls", "--format", "{{.Name}}")
	if err != nil {
		return nil, fmt.Errorf("docker network ls: %w", err)
	}
	return strings.Fields(strings.TrimSpace(out)), nil
}

// stackOpPhases 把栈操作映射为依次执行的 compose 参数序列（不含 "compose"
// 子命令本身），供 StackOpStream 流式执行；与 StackOp 的语义保持一致。
func stackOpPhases(name string, files []string, op string) ([][]string, error) {
	switch op {
	case "start":
		return [][]string{composeArgs(name, files, "up", "-d", "--remove-orphans")}, nil
	case "stop":
		return [][]string{composeArgs(name, files, "stop")}, nil
	case "restart":
		return [][]string{composeArgs(name, files, "restart")}, nil
	case "down":
		return [][]string{composeArgs(name, files, "down", "--remove-orphans")}, nil
	case "update":
		return [][]string{
			composeArgs(name, files, "pull"),
			composeArgs(name, files, "up", "-d", "--remove-orphans"),
		}, nil
	default:
		return nil, fmt.Errorf("unknown stack op: %s", op)
	}
}

// StackOpStream 以流式执行栈生命周期操作：输出逐行写入 w（调用方负责 flush
// 语义），供前端进度终端实时呈现。op 语义与 StackOp 一致。
func (r *Repository) StackOpStream(ctx context.Context, name, op string, w io.Writer) error {
	stackDir, files, err := r.resolveStackExec(ctx, name)
	if err != nil {
		return fmt.Errorf("找不到栈 %s 的 compose 文件，无法执行 %s", name, op)
	}
	phases, err := stackOpPhases(name, files, op)
	if err != nil {
		return err
	}
	timeout := composeOpTimeout
	if op == "start" || op == "update" {
		// up -d 在镜像缺失时内联拉取，与 pull 同用放宽的超时
		timeout = composeUpdateTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	filter := &progressFilter{w: w}
	for _, args := range phases {
		cmd, cmdErr := r.runtime.ComposeCommand(ctx, args...)
		if cmdErr != nil {
			return cmdErr
		}
		cmd.Dir = stackDir
		ch, err := streamCmd(ctx, cmd)
		if err != nil {
			return err
		}
		for line := range ch {
			if err := filter.line(strings.TrimSuffix(line, "\n")); err != nil {
				return err
			}
		}
		if err := filter.flush(); err != nil {
			return err
		}
		if code := cmd.ProcessState.ExitCode(); code != 0 {
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("compose %s 超时（%s）", op, timeout)
			}
			return fmt.Errorf("compose %s 失败（退出码 %d）", op, code)
		}
	}
	return nil
}

// composeProgressRe 匹配 docker 镜像层进度行（plain 管道输出），兼容两种格式：
// compose 输出 "3672748066c3 Downloading [=> ] 1MB/2MB"；docker 直连输出 "3672748066c3: Downloading ..."。
var composeProgressRe = regexp.MustCompile(
	`^\s*([0-9a-f]{12}):?\s+(Downloading|Extracting|VerifyingChecksum|Download complete|Pull complete|Pulling fs layer|Waiting|Already exists|Retrying)`)

// progressFilter 折叠镜像拉取的进度 tick：管道（非 PTY）下 docker 每个进度
// tick 输出一整行，同一层同一动作的后续 tick 以 "\r\x1b[2K"（回行首+清行）
// 覆盖前一个，前端 xterm 重放后等效 PTY 终端的原地刷新，避免刷屏。
type progressFilter struct {
	w    io.Writer
	open string // 当前未换行进度行的 "层ID 动作"，空表示上一行已闭合
}

func (p *progressFilter) line(s string) error {
	id := ""
	if m := composeProgressRe.FindStringSubmatch(s); m != nil {
		id = m[1] + " " + m[2]
	}
	switch {
	case id != "" && id == p.open:
		_, err := fmt.Fprintf(p.w, "\r\x1b[2K%s", s)
		return err
	case id != "":
		if err := p.closeOpen(); err != nil {
			return err
		}
		_, err := fmt.Fprint(p.w, s)
		if err == nil {
			p.open = id
		}
		return err
	default:
		if err := p.closeOpen(); err != nil {
			return err
		}
		_, err := fmt.Fprintf(p.w, "%s\n", s)
		return err
	}
}

// flush 结束时闭合悬空的进度行（流终止后该行即为最终状态）。
func (p *progressFilter) flush() error {
	return p.closeOpen()
}

func (p *progressFilter) closeOpen() error {
	if p.open == "" {
		return nil
	}
	_, err := fmt.Fprint(p.w, "\n")
	p.open = ""
	return err
}

// StackExecDir 返回栈 compose 操作的工作目录：
// 托管栈为栈目录，外部栈为 compose 配置文件所在目录。
func (r *Repository) StackExecDir(ctx context.Context, name string) (string, error) {
	dir, _, err := r.resolveStackExec(ctx, name)
	return dir, err
}
