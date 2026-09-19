// Package composerize 把 docker run 命令转换为等价的 compose 服务片段。
// 采用 shell 风格分词 + 完整的取值选项表，正确处理引号、= 形式与多值选项。
package composerize

import (
	"fmt"
	"strings"
)

// valueFlags 是需要消费一个参数值的选项（长选项与短选项等价形式同列）。
// 未列入的 - 开头 token 视为布尔开关，不吞并下一个 token。
var valueFlags = map[string]bool{
	"-p": true, "--publish": true,
	"-v": true, "--volume": true,
	"-e": true, "--env": true,
	"--env-file": true,
	"-l":         true, "--label": true,
	"--name": true, "--network": true,
	"--entrypoint": true, "--hostname": true, "-h": true,
	"--user": true, "-u": true, "--workdir": true, "-w": true,
	"--restart": true, "--memory": true, "-m": true, "--cpus": true,
	"--platform": true, "--pull": true, "--ip": true,
	"--cap-add": true, "--cap-drop": true, "--device": true,
	"--dns": true, "--add-host": true, "--sysctl": true,
	"--log-driver": true, "--health-cmd": true,
	"--stop-signal": true, "--stop-timeout": true, "--pid": true,
	"--ipc": true, "--uts": true, "--userns": true, "--group-add": true,
	"--tmpfs": true, "--shm-size": true, "--gpus": true,
	"--mac-address": true, "--runtime": true, "--attach": true, "-a": true,
}

// service 是转换产物：compose 服务片段的结构化表示。
type service struct {
	image         string
	containerName string
	restart       string
	workingDir    string
	user          string
	hostname      string
	entrypoint    string
	network       string
	memory        string
	cpus          string
	ports         []string
	volumes       []string
	environment   []string
	labels        []string
	devices       []string
	command       []string
}

// Convert 把 docker run 命令转换为 compose.yaml 片段。
// docker 与 docker compose 两个前缀均可；找不到镜像时报错。
func Convert(cmd string) (string, error) {
	tokens, err := tokenize(cmd)
	if err != nil {
		return "", err
	}
	svc, err := parse(tokens)
	if err != nil {
		return "", err
	}
	return svc.render(), nil
}

// parse 遍历 token 流提取选项与镜像/命令。
func parse(tokens []string) (*service, error) {
	// 定位 run 子命令（允许 "docker run" / "docker container run"）
	start := -1
	for i, t := range tokens {
		if t == "run" {
			start = i + 1
			break
		}
	}
	if start <= 0 {
		return nil, fmt.Errorf("命令中找不到 docker run 子命令")
	}

	svc := &service{}
	rest := tokens[start:]
	imageIdx := -1
	for i := 0; i < len(rest); i++ {
		t := rest[i]
		if !strings.HasPrefix(t, "-") {
			// 第一个非选项 token 是镜像，其后是容器命令
			imageIdx = i
			break
		}
		// 支持 --opt=value 形式：值内联在同一个 token 中
		flag, inlineValue, hasInline := strings.Cut(t, "=")
		var value string
		if hasInline && valueFlags[flag] {
			value = inlineValue
		} else if valueFlags[t] {
			if i+1 >= len(rest) {
				return nil, fmt.Errorf("选项 %s 缺少参数值", t)
			}
			i++
			value = rest[i]
		} else {
			continue // 布尔开关（-d、--rm、--privileged 等），忽略
		}
		switch flag {
		case "-p", "--publish":
			svc.ports = append(svc.ports, value)
		case "-v", "--volume":
			svc.volumes = append(svc.volumes, value)
		case "-e", "--env":
			svc.environment = append(svc.environment, value)
		case "-l", "--label":
			svc.labels = append(svc.labels, value)
		case "--device":
			svc.devices = append(svc.devices, value)
		case "--name":
			svc.containerName = value
		case "--network":
			svc.network = value
		case "--restart":
			svc.restart = value
		case "-w", "--workdir":
			svc.workingDir = value
		case "-u", "--user":
			svc.user = value
		case "-h", "--hostname":
			svc.hostname = value
		case "--entrypoint":
			svc.entrypoint = value
		case "-m", "--memory":
			svc.memory = value
		case "--cpus":
			svc.cpus = value
		}
	}
	if imageIdx < 0 {
		return nil, fmt.Errorf("命令中找不到镜像名")
	}
	svc.image = rest[imageIdx]
	if imageIdx+1 < len(rest) {
		svc.command = rest[imageIdx+1:]
	}
	return svc, nil
}

// tokenize 按 shell 规则分词：支持单双引号与反斜杠转义。
func tokenize(s string) ([]string, error) {
	tokens := make([]string, 0, 8)
	var cur strings.Builder
	started := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'' || c == '"':
			started = true
			quote := c
			for i++; i < len(s); i++ {
				if s[i] == '\\' && quote == '"' && i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\\') {
					cur.WriteByte(s[i+1])
					i++
					continue
				}
				if s[i] == quote {
					break
				}
				cur.WriteByte(s[i])
			}
			if i >= len(s) {
				return nil, fmt.Errorf("引号未闭合")
			}
		case c == '\\' && i+1 < len(s):
			started = true
			cur.WriteByte(s[i+1])
			i++
		case c == ' ' || c == '\t':
			if started || cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
				started = false
			}
		default:
			started = true
			cur.WriteByte(c)
		}
	}
	if started || cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens, nil
}

// render 按 compose 惯用键序输出 YAML 片段。
func (s *service) render() string {
	var b strings.Builder
	b.WriteString("services:\n  app:\n")
	b.WriteString("    image: " + yamlScalar(s.image) + "\n")
	if s.containerName != "" {
		b.WriteString("    container_name: " + yamlScalar(s.containerName) + "\n")
	}
	if s.restart != "" {
		b.WriteString("    restart: " + yamlScalar(s.restart) + "\n")
	}
	writeList(&b, "ports", s.ports)
	writeList(&b, "volumes", s.volumes)
	writeList(&b, "environment", s.environment)
	writeList(&b, "labels", s.labels)
	writeList(&b, "devices", s.devices)
	if s.network != "" {
		b.WriteString("    networks:\n      - " + yamlScalar(s.network) + "\n")
	}
	if s.workingDir != "" {
		b.WriteString("    working_dir: " + yamlScalar(s.workingDir) + "\n")
	}
	if s.user != "" {
		b.WriteString("    user: " + yamlScalar(s.user) + "\n")
	}
	if s.hostname != "" {
		b.WriteString("    hostname: " + yamlScalar(s.hostname) + "\n")
	}
	if s.entrypoint != "" {
		b.WriteString("    entrypoint: " + yamlScalar(s.entrypoint) + "\n")
	}
	if s.memory != "" {
		b.WriteString("    mem_limit: " + yamlScalar(s.memory) + "\n")
	}
	if s.cpus != "" {
		b.WriteString("    cpus: " + yamlScalar(s.cpus) + "\n")
	}
	if len(s.command) > 0 {
		writeList(&b, "command", s.command)
	}
	return strings.TrimRight(b.String(), "\n")
}

// writeList 输出一个列表键；列表为空时跳过。
func writeList(b *strings.Builder, key string, values []string) {
	if len(values) == 0 {
		return
	}
	b.WriteString("    " + key + ":\n")
	for _, v := range values {
		b.WriteString("      - " + yamlScalar(v) + "\n")
	}
}

// yamlScalar 为含 YAML 特殊字符的标量加双引号，避免产出非法 YAML。
func yamlScalar(s string) string {
	if s == "" || strings.ContainsAny(s, ":#{}[]&*!|>'\"%@`") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		return quoteYAML(s)
	}
	// 形如 "KEY=VALUE"、路径、"a:b" 混合场景以保守判断为准
	if strings.Contains(s, ": ") || strings.Contains(s, " #") {
		return quoteYAML(s)
	}
	return s
}

// quoteYAML 以双引号包裹并转义反斜杠与双引号。
func quoteYAML(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return "\"" + s + "\""
}
