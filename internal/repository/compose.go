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

// resolveStackExec 只接受 stacks 中有真实配置文件的目录，不反查或操作外部项目。
func (r *Repository) resolveStackExec(ctx context.Context, name string) (dir string, files []string, err error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	if name == "" || filepath.Base(name) != name || strings.HasPrefix(name, ".") {
		return "", nil, fmt.Errorf("%w：栈名 %q 不属于可管理的 stacks 子目录", ErrNotFound, name)
	}
	if info, err := os.Lstat(r.StackPath(name)); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", nil, fmt.Errorf("栈目录 %s 必须是真实目录而非符号链接", r.StackPath(name))
		}
	} else if !os.IsNotExist(err) {
		return "", nil, fmt.Errorf("读取栈 %q 目录: %w", name, err)
	}
	if file, ok := r.findComposeFile(name); ok {
		dir := r.StackPath(name)
		return dir, []string{filepath.Join(dir, file)}, nil
	}
	return "", nil, fmt.Errorf("%w：栈 %q 必须位于 %s 的真实子目录且包含 compose 配置文件", ErrNotFound, name, r.stacksDir)
}

// composeArgs 显式指定托管栈的项目名与配置文件，不加载当前目录的其他配置。
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
// 配置只来自托管目录，不允许按项目名反查外部配置。
func (r *Repository) StackPs(ctx context.Context, name string) ([]model.Container, error) {
	stackDir, files, err := r.resolveStackExec(ctx, name)
	if err != nil {
		return nil, err
	}
	if err := r.checkStackOwnership(ctx, name); err != nil {
		return nil, err
	}
	out, err := r.runComposeIn(ctx, stackDir, composeOpTimeout,
		composeArgs(name, files, "ps", "--all", "--format", "json")...)
	if err != nil {
		return nil, fmt.Errorf("查询栈 %q 容器失败: %w", name, err)
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

// ServiceOp 对栈内单个服务执行 compose 操作（up/stop/restart <service>），
// 与上游 dockge 的服务级操作对齐。服务名作 CLI 参数，须校验防 flag 注入。
func (r *Repository) ServiceOp(ctx context.Context, name, service, op string) (string, error) {
	if !ContainerIDPattern.MatchString(service) {
		return "", fmt.Errorf("非法服务名: %s", service)
	}
	stackDir, files, err := r.resolveStackExec(ctx, name)
	if err != nil {
		return "", fmt.Errorf("栈 %q 无法执行 %s: %w", name, op, err)
	}
	if err := r.checkStackOwnership(ctx, name); err != nil {
		return "", err
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
	dir, files, err := r.resolveStackExec(ctx, name)
	if err != nil {
		return nil, err
	}
	if err := r.checkStackOwnership(ctx, name); err != nil {
		return nil, err
	}
	idsOut, err := r.runComposeIn(ctx, dir, composeOpTimeout, composeArgs(name, files, "ps", "--all", "-q")...)
	if err != nil {
		return nil, fmt.Errorf("查询栈 %q 统计对象失败: %w", name, err)
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
			return nil, fmt.Errorf("解析统计行 %q: %w", line, err)
		}
		stats = append(stats, stat)
	}
	return stats, nil
}

// docker 预设网络：services.networks 引用它们无意义（宿主网络等要走 network_mode），
// 出现在编辑器建议列表只会误导，过滤掉。
var builtinNetworks = map[string]struct{}{"bridge": {}, "host": {}, "none": {}}

// DockerNetworkNames 返回本机网络名列表（编辑器 Networks 字段的建议来源）。
func (r *Repository) DockerNetworkNames(ctx context.Context) ([]string, error) {
	out, err := r.runDocker(ctx, composeOpTimeout, "network", "ls", "--format", "{{.Name}}")
	if err != nil {
		return nil, fmt.Errorf("docker network ls: %w", err)
	}
	names := make([]string, 0)
	for _, name := range strings.Fields(strings.TrimSpace(out)) {
		if _, builtin := builtinNetworks[name]; builtin {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

// stackOpPhases 把栈操作映射为依次执行的 compose 参数序列（不含 "compose"
// 子命令本身），供 StackOpStream 流式执行；作为唯一生命周期实现。
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
// 语义），供前端进度终端实时呈现。与服务端操作白名单一致。
func (r *Repository) StackOpStream(ctx context.Context, name, op string, w io.Writer) error {
	stackDir, files, err := r.resolveStackExec(ctx, name)
	if err != nil {
		return fmt.Errorf("栈 %q 无法执行 %s: %w", name, op, err)
	}
	if err := r.checkStackOwnership(ctx, name); err != nil {
		return err
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
		for line := range ch.lines {
			if err := filter.line(strings.TrimSuffix(line, "\n")); err != nil {
				return err
			}
		}
		if err := filter.flush(); err != nil {
			return err
		}
		if ch.err != nil {
			code := cmd.ProcessState.ExitCode()
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("compose %s 超时（%s）", op, timeout)
			}
			return fmt.Errorf("compose %s 失败（退出码 %d）: %w", op, code, ch.err)
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
