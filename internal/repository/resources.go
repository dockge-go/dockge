package repository

import (
	"context"
	v1 "dockge/api/v1"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

type inspectedContainer struct {
	ID           string `json:"Id"`
	Name         string
	Image        string
	Created      time.Time
	RestartCount int
	Config       struct {
		Image      string
		Env        []string
		Labels     map[string]string
		Entrypoint []string
		Cmd        []string
		Hostname   string
	}
	State struct {
		Status     string
		ExitCode   int
		StartedAt  time.Time
		FinishedAt time.Time
		Health     *struct{ Status string }
	}
	HostConfig struct {
		NanoCPUs      int64
		Memory        int64
		RestartPolicy struct{ Name string }
	}
	Mounts []struct {
		Type        string
		Name        string
		Source      string
		Destination string
		RW          bool
	}
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string
		}
		Networks map[string]struct {
			IPAddress         string
			GlobalIPv6Address string
		}
	}
}

func decodeRows[T any](out string) ([]T, error) {
	rows := []T{}
	if strings.TrimSpace(out) == "" {
		return rows, nil
	}
	if strings.HasPrefix(strings.TrimSpace(out), "[") {
		err := json.Unmarshal([]byte(out), &rows)
		return rows, err
	}
	decoder := json.NewDecoder(strings.NewReader(out))
	for {
		var row T
		if err := decoder.Decode(&row); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("解析运行时 JSON（字节 %d）: %w", decoder.InputOffset(), err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// ResourceContainers 以一次批量 inspect 获取全部容器，包括已停止的占用者。
func (r *Repository) ResourceContainers(ctx context.Context) ([]v1.ResourceContainer, error) {
	out, err := r.runDocker(ctx, composeOpTimeout, "ps", "-aq", "--no-trunc")
	if err != nil {
		return nil, err
	}
	ids := strings.Fields(out)
	result := []v1.ResourceContainer{}
	if len(ids) == 0 {
		return result, nil
	}
	out, err = r.runDocker(ctx, composeOpTimeout, append([]string{"inspect", "--type", "container"}, ids...)...)
	if err != nil {
		return nil, err
	}
	rows, err := decodeRows[inspectedContainer](out)
	if err != nil {
		return nil, err
	}
	selfID := selfContainerIdentity()
	for _, c := range rows {
		v := v1.ResourceContainer{ID: c.ID, Names: []string{c.Name}, Image: c.Config.Image, ImageID: c.Image, State: c.State.Status, Created: c.Created.UnixMilli(), Ports: []v1.ResourcePort{}, Env: c.Config.Env, Mounts: []v1.ResourceMount{}, Networks: []string{}, NetworkIPs: map[string]string{}, Labels: c.Config.Labels, RestartPolicy: c.HostConfig.RestartPolicy.Name, RestartCount: c.RestartCount, CPUs: float64(c.HostConfig.NanoCPUs) / 1e9, MemoryMB: float64(c.HostConfig.Memory) / 1048576, Entrypoint: strings.Join(c.Config.Entrypoint, " "), Cmd: strings.Join(c.Config.Cmd, " ")}
		v.System = selfID != "" && strings.HasPrefix(c.ID, selfID)
		if c.State.Health != nil {
			v.Health = c.State.Health.Status
		}
		if c.State.Status == "running" && !c.State.StartedAt.IsZero() {
			n := int(time.Since(c.State.StartedAt).Hours())
			v.UpHours = &n
		}
		if c.State.Status == "exited" || c.State.Status == "dead" {
			n := c.State.ExitCode
			v.ExitCode = &n
			if !c.State.FinishedAt.IsZero() {
				m := int(time.Since(c.State.FinishedAt).Minutes())
				v.ExitMinAgo = &m
			}
		}
		for key, binds := range c.NetworkSettings.Ports {
			p, proto, _ := strings.Cut(key, "/")
			port, _ := strconv.Atoi(p)
			if len(binds) == 0 {
				v.Ports = append(v.Ports, v1.ResourcePort{PrivatePort: port, Proto: proto, Type: proto})
			}
			for _, b := range binds {
				pub, _ := strconv.Atoi(b.HostPort)
				v.Ports = append(v.Ports, v1.ResourcePort{PrivatePort: port, PublicPort: pub, IP: b.HostIP, Proto: proto, Type: proto})
			}
		}
		sort.Slice(v.Ports, func(i, j int) bool { return v.Ports[i].PrivatePort < v.Ports[j].PrivatePort })
		for _, m := range c.Mounts {
			mode := "ro"
			if m.RW {
				mode = "rw"
			}
			source := m.Source
			if m.Name != "" {
				source = m.Name
			}
			v.Mounts = append(v.Mounts, v1.ResourceMount{Source: source, Name: m.Name, Dest: m.Destination, Mode: mode, Type: m.Type})
		}
		for name, n := range c.NetworkSettings.Networks {
			v.Networks = append(v.Networks, name)
			v.NetworkIPs[name] = strings.Trim(strings.Join([]string{n.IPAddress, n.GlobalIPv6Address}, " "), " ")
		}
		sort.Strings(v.Networks)
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Created > result[j].Created })
	return result, nil
}

// ResourceImages 获取镜像真实大小、创建时间、标签及容器引用。
func (r *Repository) ResourceImages(ctx context.Context, containers []v1.ResourceContainer) ([]v1.ResourceImage, error) {
	out, err := r.runDocker(ctx, composeOpTimeout, "image", "ls", "-q", "--no-trunc")
	if err != nil {
		return nil, err
	}
	ids := unique(strings.Fields(out))
	result := []v1.ResourceImage{}
	if len(ids) == 0 {
		return result, nil
	}
	out, err = r.runDocker(ctx, composeOpTimeout, append([]string{"image", "inspect"}, ids...)...)
	if err != nil {
		return nil, err
	}
	rows, err := decodeRows[struct {
		ID          string `json:"Id"`
		RepoTags    []string
		RepoDigests []string
		Size        int64
		Created     time.Time
	}](out)
	if err != nil {
		return nil, err
	}
	for _, x := range rows {
		v := v1.ResourceImage{ID: x.ID, RepoTags: x.RepoTags, RepoDigests: x.RepoDigests, Size: x.Size, Created: x.Created.UnixMilli()}
		if v.RepoTags == nil {
			v.RepoTags = []string{}
		}
		if len(v.RepoTags) > 0 {
			v.Repository = v.RepoTags[0]
		}
		for _, c := range containers {
			if c.ImageID == x.ID {
				v.InUse = true
				v.UsedBy = append(v.UsedBy, v1.ResourceUse{ID: c.ID, Name: strings.TrimPrefix(c.Names[0], "/")})
				v.System = v.System || c.System
			}
		}
		result = append(result, v)
	}
	return result, nil
}

// ResourceVolumes 的引用包含已停止容器，不能把“非系统”当作“未使用”。
func (r *Repository) ResourceVolumes(ctx context.Context, containers []v1.ResourceContainer) ([]v1.ResourceVolume, error) {
	out, err := r.runDocker(ctx, composeOpTimeout, "volume", "ls", "-q")
	if err != nil {
		return nil, err
	}
	names := strings.Fields(out)
	if len(names) == 0 {
		return []v1.ResourceVolume{}, nil
	}
	out, err = r.runDocker(ctx, composeOpTimeout, append([]string{"volume", "inspect"}, names...)...)
	if err != nil {
		return nil, err
	}
	rows, err := decodeRows[v1.ResourceVolume](out)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		v := &rows[i]
		v.Usage = ""
		v.UsedBy = []v1.ResourceUse{}
		for _, c := range containers {
			for _, m := range c.Mounts {
				if m.Type == "volume" && m.Name == v.Name {
					v.UsedBy = append(v.UsedBy, v1.ResourceUse{ID: c.ID, Name: strings.TrimPrefix(c.Names[0], "/")})
					v.System = v.System || c.System
				}
			}
		}
	}
	return rows, nil
}

// ResourceNetworks 读取真实子网与端点，并保护运行时内置网络。
func (r *Repository) ResourceNetworks(ctx context.Context, containers []v1.ResourceContainer) ([]v1.ResourceNetwork, error) {
	out, err := r.runDocker(ctx, composeOpTimeout, "network", "ls", "-q")
	if err != nil {
		return nil, err
	}
	ids := strings.Fields(out)
	if len(ids) == 0 {
		return []v1.ResourceNetwork{}, nil
	}
	out, err = r.runDocker(ctx, composeOpTimeout, append([]string{"network", "inspect"}, ids...)...)
	if err != nil {
		return nil, err
	}
	rows, err := decodeRows[struct {
		ID         string `json:"Id"`
		Name       string
		Driver     string
		Labels     map[string]string
		IPAM       struct{ Config []struct{ Subnet string } }
		Containers map[string]struct {
			Name        string
			IPv4Address string
			IPv6Address string
		}
	}](out)
	if err != nil {
		return nil, err
	}
	result := []v1.ResourceNetwork{}
	for _, x := range rows {
		v := v1.ResourceNetwork{ID: x.ID, Name: x.Name, Driver: x.Driver, Labels: x.Labels, Containers: []v1.ResourceEndpoint{}}
		_, v.System = builtinNetworks[x.Name]
		sub := []string{}
		for _, c := range x.IPAM.Config {
			sub = append(sub, c.Subnet)
		}
		v.Subnet = strings.Join(sub, ", ")
		for id, c := range x.Containers {
			v.Containers = append(v.Containers, v1.ResourceEndpoint{ID: id, Name: c.Name, IP: strings.TrimSpace(c.IPv4Address + " " + c.IPv6Address)})
		}
		for _, c := range containers {
			for _, name := range c.Networks {
				if name != v.Name {
					continue
				}
				found := false
				for _, endpoint := range v.Containers {
					found = found || endpoint.ID == c.ID
				}
				if !found {
					v.Containers = append(v.Containers, v1.ResourceEndpoint{ID: c.ID, Name: strings.TrimPrefix(c.Names[0], "/"), IP: c.NetworkIPs[name]})
				}
				v.System = v.System || c.System
			}
		}
		sort.Slice(v.Containers, func(i, j int) bool { return v.Containers[i].ID < v.Containers[j].ID })
		result = append(result, v)
	}
	return result, nil
}

// ResourceStatistics 返回真实 stats 快照；不支持的运行时由调用方显示具体原因。
func (r *Repository) ResourceStatistics(ctx context.Context, containers []v1.ResourceContainer) error {
	ids := []string{}
	for _, c := range containers {
		if c.State == "running" {
			ids = append(ids, c.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	out, err := r.runDocker(ctx, composeOpTimeout, append([]string{"stats", "--no-stream", "--no-trunc", "--format", "json"}, ids...)...)
	if err != nil {
		return err
	}
	rows, err := decodeRows[struct {
		ID        string
		Container string
		CPUPerc   string
		MemUsage  string
		MemPerc   string
		NetIO     string
		BlockIO   string
	}](out)
	if err != nil {
		return err
	}
	for _, x := range rows {
		cpu, e := parsePercent(x.CPUPerc)
		if e != nil {
			return fmt.Errorf("无法解析 CPU 用量 %q: %w", x.CPUPerc, e)
		}
		mem, e := parsePercent(x.MemPerc)
		if e != nil {
			return fmt.Errorf("无法解析内存百分比 %q: %w", x.MemPerc, e)
		}
		usage, _, _ := strings.Cut(x.MemUsage, "/")
		b, e := parseBytes(usage)
		if e != nil {
			return e
		}
		for i := range containers {
			if strings.HasPrefix(containers[i].ID, x.ID) && x.ID != "" || containers[i].ID == x.Container {
				containers[i].Stats = &v1.ResourceStats{CPUPct: cpu, CPUBar: min(cpu, 100), MemMB: b / 1048576, MemBar: mem, NetIO: x.NetIO, BlockIO: x.BlockIO}
			}
		}
	}
	return nil
}
func parsePercent(s string) (float64, error) {
	n, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, "%")), 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
		return 0, fmt.Errorf("期望有限非负数，实际 %q", s)
	}
	return n, nil
}

func parseBytes(s string) (float64, error) {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		i++
	}
	n, e := strconv.ParseFloat(s[:i], 64)
	if e != nil {
		return 0, fmt.Errorf("无法解析字节用量 %q: %w", s, e)
	}
	unit := strings.TrimSpace(s[i:])
	factor, ok := map[string]float64{"B": 1, "kB": 1e3, "KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12, "KiB": 1024, "MiB": 1048576, "GiB": 1073741824, "TiB": 1099511627776}[unit]
	if !ok {
		return 0, fmt.Errorf("无法识别字节单位 %q（原值 %q）", unit, s)
	}
	bytes := n * factor
	if math.IsNaN(bytes) || math.IsInf(bytes, 0) {
		return 0, fmt.Errorf("字节用量 %q 单位换算溢出，期望有限非负数", s)
	}
	return bytes, nil
}
func unique(ss []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// ResourceSystem 返回 CLI 信息与 df 数据，原始错误不以固定版本或零用量替代。
func (r *Repository) ResourceSystem(ctx context.Context) (map[string]string, []v1.ResourceDiskRow, map[string]string) {
	info := map[string]string{}
	errs := map[string]string{}
	out, err := r.runDocker(ctx, composeOpTimeout, "info", "--format", "json")
	if err != nil {
		errs["info"] = err.Error()
	} else {
		var obj map[string]json.RawMessage
		if err = json.Unmarshal([]byte(out), &obj); err != nil {
			errs["info"] = err.Error()
		} else {
			for k, v := range obj {
				if len(v) > 0 && v[0] != '{' && v[0] != '[' {
					info[k] = strings.Trim(string(v), "\"")
				}
			}
		}
	}
	disk := []v1.ResourceDiskRow{}
	out, err = r.runDocker(ctx, composeOpTimeout, "system", "df", "--format", "json")
	if err != nil {
		errs["disk"] = err.Error()
		return info, disk, errs
	}
	disk, err = parseDiskRows(out)
	if err != nil {
		errs["disk"] = err.Error()
		return info, []v1.ResourceDiskRow{}, errs
	}

	return info, disk, errs
}

// parseDiskRows 兼容运行时的字符串与数字计数；未知空间不能伪装成零或可清理。
func parseDiskRows(out string) ([]v1.ResourceDiskRow, error) {
	rows, err := decodeRows[struct {
		Type, Size, Reclaimable string
		TotalCount              json.RawMessage
	}](out)
	if err != nil {
		return nil, fmt.Errorf("解析 system df: %w", err)
	}
	result := []v1.ResourceDiskRow{}
	for _, row := range rows {
		key := map[string]string{"Images": "images", "Containers": "containers", "Local Volumes": "volumes", "Build Cache": "buildCache"}[row.Type]
		if key == "" {
			return nil, fmt.Errorf("system df 返回未知资源类型 %q", row.Type)
		}
		countText := strings.Trim(string(row.TotalCount), "\"")
		count, e := strconv.Atoi(countText)
		if e != nil || count < 0 {
			return nil, fmt.Errorf("无法解析 %s 数量 %q，期望非负整数", row.Type, countText)
		}
		amount := strings.TrimSpace(strings.SplitN(row.Reclaimable, "(", 2)[0])
		reclaim, e := parseBytes(amount)
		result = append(result, v1.ResourceDiskRow{Key: key, Used: row.Size, Reclaim: row.Reclaimable, Count: count, ReclaimBytes: reclaim, ReclaimKnown: e == nil})
	}
	return result, nil
}

// ResourceVolumeUsage 读取运行时的真实卷占用；不支持详细统计时保留未知并返回明细。
func (r *Repository) ResourceVolumeUsage(ctx context.Context, volumes []v1.ResourceVolume) error {
	if len(volumes) == 0 {
		return nil
	}
	out, err := r.runDocker(ctx, composeOpTimeout, "system", "df", "-v", "--format", "{{json .Volumes}}")
	if err != nil {
		return err
	}
	rows, err := decodeRows[struct{ Name, Size string }](out)
	if err != nil {
		return fmt.Errorf("解析卷空间统计: %w", err)
	}
	sizes := map[string]string{}
	for _, row := range rows {
		sizes[row.Name] = row.Size
	}
	for i := range volumes {
		volumes[i].Usage = sizes[volumes[i].Name]
	}
	return nil
}

// ResourceAction 只执行已验证的专用动作；删除不强制绕过运行时占用保护。
func (r *Repository) ResourceAction(ctx context.Context, kind, id, op, value string) (string, error) {
	switch kind {
	case "buildCache":
		if op == "delete" && id == "unused" {
			return r.runDocker(ctx, composeUpdateTimeout, "builder", "prune", "--all", "--force")
		}
	case "containers":
		cs, err := r.ResourceContainers(ctx)
		if err != nil {
			return "", err
		}
		found := false
		running, paused := false, false
		for _, c := range cs {
			if c.ID == id {
				found = true
				running = c.State == "running" || c.State == "restarting" || c.State == "paused"
				paused = c.State == "paused"
				if c.System {
					return "", fmt.Errorf("不能操作面板自身容器 %s", id)
				}
			}
		}
		if !found {
			return "", fmt.Errorf("容器 %q 不存在，请刷新列表", id)
		}
		switch op {
		case "start":
			// 暂停中的进程已经启动，恢复执行不能调用运行时的 start。
			if paused {
				return r.runDocker(ctx, composeOpTimeout, "unpause", id)
			}
			return r.runDocker(ctx, composeOpTimeout, op, id)
		case "stop", "restart":
			return r.runDocker(ctx, composeOpTimeout, op, id)
		case "delete":
			if paused {
				if _, err := r.runDocker(ctx, composeOpTimeout, "unpause", id); err != nil {
					return "", fmt.Errorf("恢复暂停容器 %s 失败，未执行删除: %w", id, err)
				}
			}
			if running {
				if _, err := r.runDocker(ctx, composeOpTimeout, "stop", id); err != nil {
					return "", fmt.Errorf("停止容器 %s 失败，未执行删除: %w", id, err)
				}
			}
			return r.runDocker(ctx, composeOpTimeout, "rm", "-v", id)
		case "rename":
			return r.runDocker(ctx, composeOpTimeout, "rename", id, value)
		case "commit":
			return r.runDocker(ctx, composeOpTimeout, "commit", id, value)
		}
	case "images":
		switch op {
		case "delete":
			cs, err := r.ResourceContainers(ctx)
			if err != nil {
				return "", err
			}
			images, err := r.ResourceImages(ctx, cs)
			if err != nil {
				return "", err
			}
			for _, image := range images {
				if image.ID != id {
					continue
				}
				if image.System || image.InUse {
					uses := []string{}
					for _, c := range image.UsedBy {
						uses = append(uses, fmt.Sprintf("%s (%s)", c.Name, c.ID))
					}
					return "", fmt.Errorf("镜像 %s 仍被容器引用：%s；请先确认并删除这些容器，停止容器仍会保留引用", id, strings.Join(uses, ", "))
				}
				refs := image.RepoTags
				if len(refs) == 0 {
					refs = []string{id}
				}
				output := ""
				for _, ref := range refs {
					out, e := r.runDocker(ctx, composeOpTimeout, "image", "rm", ref)
					output += out + "\n"
					if e != nil {
						return output, fmt.Errorf("镜像 %s 标签 %q 删除失败: %w", id, ref, e)
					}
				}
				remaining, e := r.ResourceImages(ctx, cs)
				if e != nil {
					return output, fmt.Errorf("删除后无法核验镜像 %s: %w", id, e)
				}
				for _, left := range remaining {
					if left.ID == id {
						out, e := r.runDocker(ctx, composeOpTimeout, "image", "rm", id)
						output += out + "\n"
						if e != nil {
							return output, fmt.Errorf("清理镜像 %s 的剩余摘要失败: %w", id, e)
						}
						final, e := r.ResourceImages(ctx, cs)
						if e != nil {
							return output, e
						}
						for _, last := range final {
							if last.ID == id {
								return output, fmt.Errorf("镜像 %s 删除后仍存在，请刷新并检查运行时引用", id)
							}
						}
					}
				}
				return output, nil
			}
			return "", fmt.Errorf("镜像 %q 不存在，请刷新列表", id)
		case "tag":
			return r.runDocker(ctx, composeOpTimeout, "image", "tag", id, value)
		}
	case "volumes", "networks":
		if op == "delete" {
			cs, err := r.ResourceContainers(ctx)
			if err != nil {
				return "", fmt.Errorf("无法核对 %s 引用，删除未执行: %w", kind, err)
			}
			found := false
			if kind == "volumes" {
				volumes, err := r.ResourceVolumes(ctx, cs)
				if err != nil {
					return "", err
				}
				for _, v := range volumes {
					if v.Name != id {
						continue
					}
					found = true
					if v.System || len(v.UsedBy) > 0 {
						return "", fmt.Errorf("卷 %s 仍被容器引用（包括已停止容器）: %v", id, v.UsedBy)
					}
				}
			} else {
				networks, err := r.ResourceNetworks(ctx, cs)
				if err != nil {
					return "", err
				}
				for _, n := range networks {
					if n.ID != id && n.Name != id {
						continue
					}
					found = true
					if n.System || len(n.Containers) > 0 {
						return "", fmt.Errorf("网络 %s 为系统网络或仍被容器引用（包括已停止容器）: %v", n.Name, n.Containers)
					}
				}
			}
			if !found {
				return "", fmt.Errorf("%s %q 不存在，请刷新列表", kind, id)
			}
			command := "volume"
			if kind == "networks" {
				command = "network"
			}
			return r.runDocker(ctx, composeOpTimeout, command, "rm", id)
		}
	}
	return "", fmt.Errorf("不支持资源操作 %s/%s/%s", kind, id, op)
}

// ResourceCreate 按表单构造 argv，命令字符串经过词法拆分而不交给宿主 shell。
func (r *Repository) ResourceCreate(ctx context.Context, kind string, q v1.ResourceCreateRequest) (string, error) {
	switch kind {
	case "volumes":
		volumes, err := r.ResourceVolumes(ctx, nil)
		if err != nil {
			return "", fmt.Errorf("无法核对卷重名，创建未执行: %w", err)
		}
		for _, volume := range volumes {
			if volume.Name == q.Name {
				return "", fmt.Errorf("%w: 卷 %q 已存在，未新建或修改", ErrConflict, q.Name)
			}
		}
		return r.runDocker(ctx, composeOpTimeout, "volume", "create", q.Name)
	case "networks":
		driver := q.Driver
		if driver == "" {
			driver = "bridge"
		}
		return r.runDocker(ctx, composeOpTimeout, "network", "create", "--driver", driver, q.Name)
	case "containers":
		args := []string{"run", "-d", "--name", q.Name}
		if q.ContainerPort != "" {
			p := q.ContainerPort
			if q.HostPort != "" {
				p = q.HostPort + ":" + p
			}
			args = append(args, "-p", p)
		}
		args = append(args, q.Image)
		cmd, err := splitCommand(q.Command)
		if err != nil {
			return "", err
		}
		args = append(args, cmd...)
		return r.runDocker(ctx, composeUpdateTimeout, args...)
	}
	return "", fmt.Errorf("不支持创建资源类型 %q", kind)
}
func splitCommand(s string) ([]string, error) {
	args := []string{}
	var b strings.Builder
	quote := rune(0)
	escaped := false
	started := false
	for _, c := range s {
		if escaped {
			b.WriteRune(c)
			escaped = false
			started = true
			continue
		}
		if c == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				b.WriteRune(c)
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			started = true
			continue
		}
		if c == ' ' || c == '\t' {
			if started {
				args = append(args, b.String())
				b.Reset()
				started = false
			}
			continue
		}
		b.WriteRune(c)
		started = true
	}
	if quote != 0 || escaped {
		return nil, fmt.Errorf("命令 %q 的引号或转义未闭合", s)
	}
	if started {
		args = append(args, b.String())
	}
	return args, nil
}

// ResourceHistory 读取运行时返回的镜像层记录。
func (r *Repository) ResourceHistory(ctx context.Context, id string) ([]map[string]string, error) {
	out, err := r.runDocker(ctx, composeOpTimeout, "image", "history", "--no-trunc", "--format", "json", id)
	if err != nil {
		return nil, err
	}
	rows, err := decodeRows[struct {
		ID        string
		CreatedBy string
		Size      string
	}](out)
	if err != nil {
		return nil, err
	}
	result := []map[string]string{}
	for _, x := range rows {
		result = append(result, map[string]string{"id": x.ID, "cmd": x.CreatedBy, "size": x.Size})
	}
	return result, nil
}
