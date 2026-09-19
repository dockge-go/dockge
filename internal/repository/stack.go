package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockge/internal/model"
	"gopkg.in/yaml.v3"
)

// compose 文件的候选名（与 Dockge 的 acceptedComposeFileNames 一致）。
var acceptedComposeFileNames = []string{
	"compose.yaml",
	"compose.yml",
	"docker-compose.yml",
	"docker-compose.yaml",
}

// List 扫描 stacks 目录得到托管栈，再合并 `docker compose ls` 的
// 全部项目（含 stacks 目录之外的"外部栈"）与状态。
func (r *Repository) List(ctx context.Context) ([]model.Stack, error) {
	stacks := make([]model.Stack, 0)
	byName := make(map[string]int)

	entries, err := os.ReadDir(r.stacksDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read stacks dir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		composeFile, ok := r.findComposeFile(entry.Name())
		if !ok {
			continue
		}
		stacks = append(stacks, model.Stack{
			Name:            entry.Name(),
			Status:          model.StatusCreatedFile,
			Managed:         true,
			ComposeFileName: composeFile,
			ConfigFiles:     filepath.Join(r.stacksDir, entry.Name(), composeFile),
		})
		byName[entry.Name()] = len(stacks) - 1
	}

	lsItems, err := r.ComposeLs(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range lsItems {
		if item.Name == "" {
			continue
		}
		if index, ok := byName[item.Name]; ok {
			stacks[index].Status = StackStatusFromString(item.Status)
			stacks[index].ConfigFiles = item.ConfigFiles
			continue
		}
		// 外部栈：不在 stacks 目录，但由 docker compose 管理，仍展示可操作
		stacks = append(stacks, model.Stack{
			Name:        item.Name,
			Status:      StackStatusFromString(item.Status),
			Managed:     false,
			ConfigFiles: item.ConfigFiles,
		})
		byName[item.Name] = len(stacks) - 1
	}

	sort.Slice(stacks, func(i, j int) bool { return stacks[i].Name < stacks[j].Name })
	return stacks, nil
}

// Get 读取单个栈：compose 文件与 .env 内容 + 状态 + 容器列表。
// stacks 目录中不存在但 compose 已注册的（外部栈）也能取到状态，只是无文件内容。
func (r *Repository) Get(ctx context.Context, name string) (model.Stack, error) {
	stack := model.Stack{Name: name, Status: model.StatusUnknown}
	external := false

	if composeFile, ok := r.findComposeFile(name); ok {
		stack.Managed = true
		stack.ComposeFileName = composeFile
		stack.ConfigFiles = filepath.Join(r.stacksDir, name, composeFile)
		yaml, err := os.ReadFile(filepath.Join(r.stacksDir, name, composeFile))
		if err != nil {
			return model.Stack{}, fmt.Errorf("read compose file: %w", err)
		}
		stack.Yaml = string(yaml)
		if env, err := os.ReadFile(filepath.Join(r.stacksDir, name, ".env")); err == nil {
			stack.Env = string(env)
		}
	} else {
		// 目录不存在：仍可能是由 compose 管理的外部栈
		external = true
		items, err := r.ComposeLs(ctx)
		if err != nil {
			return model.Stack{}, err
		}
		found := false
		for _, item := range items {
			if item.Name == name {
				stack.Status = StackStatusFromString(item.Status)
				stack.ConfigFiles = item.ConfigFiles
				found = true
				break
			}
		}
		if !found {
			return model.Stack{}, ErrNotFound
		}
		// 外部栈：读取其真实 compose 文件展示（只读，不可通过本程序保存）
		if first := firstConfigFile(stack.ConfigFiles); first != "" {
			if yaml, err := os.ReadFile(first); err == nil {
				stack.Yaml = string(yaml)
			}
		}
	}

	// 托管栈的状态在上方文件读取时未知，需查询 compose ls；
	// 外部栈分支刚刚查过并已设置状态，跳过以免重复调用 CLI。
	if !external {
		if lsItems, err := r.ComposeLs(ctx); err == nil {
			for _, item := range lsItems {
				if item.Name == name {
					stack.Status = StackStatusFromString(item.Status)
					stack.ConfigFiles = item.ConfigFiles
					break
				}
			}
		}
	}

	containers, err := r.StackPs(ctx, name)
	if err != nil {
		return model.Stack{}, err
	}
	stack.Containers = containers
	return stack, nil
}

// Save 把 compose 文件写入栈目录；isAdd 时创建目录并拒绝重名，
// 否则要求目录已存在。栈目录其余文件（如 .env 之外的挂载卷）不受影响。
func (r *Repository) Save(ctx context.Context, stack *model.Stack, isAdd bool) error {
	dir := r.StackPath(stack.Name)
	if isAdd {
		if _, err := os.Stat(dir); err == nil {
			return ErrConflict
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("%w：无法创建目录 %s：%v", ErrStacksNotWritable, dir, err)
		}
	} else {
		if _, err := os.Stat(dir); err != nil {
			return ErrNotFound
		}
	}
	file := stack.ComposeFileName
	if file == "" {
		file = "compose.yaml"
		stack.ComposeFileName = file
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(stack.Yaml), 0o644); err != nil {
		return fmt.Errorf("%w：无法写入 %s：%v", ErrStacksNotWritable, filepath.Join(dir, file), err)
	}
	if stack.Env != "" {
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(stack.Env), 0o600); err != nil {
			return fmt.Errorf("%w：无法写入 %s：%v", ErrStacksNotWritable, filepath.Join(dir, ".env"), err)
		}
	} else if err := os.Remove(filepath.Join(dir, ".env")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale .env: %w", err)
	}
	return nil
}

// Delete 删除整个栈目录（调用方须先执行 compose down）。
func (r *Repository) Delete(ctx context.Context, name string) error {
	dir := r.StackPath(name)
	if _, err := os.Stat(dir); err != nil {
		return ErrNotFound
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove stack dir: %w", err)
	}
	return nil
}

// EnsureStacksDir 确保 stacks 目录存在（迁移与启动时调用）。
func (r *Repository) EnsureStacksDir() error {
	if err := os.MkdirAll(r.stacksDir, 0o755); err != nil {
		return fmt.Errorf("ensure stacks dir: %w", err)
	}
	return nil
}

// firstConfigFile 取 ConfigFiles（可能是逗号分隔的多文件）中的第一个。
func firstConfigFile(configFiles string) string {
	for _, f := range strings.Split(configFiles, ",") {
		if f = strings.TrimSpace(f); f != "" {
			return f
		}
	}
	return ""
}

// findComposeFile 返回栈目录内第一个存在的 compose 文件名。
func (r *Repository) findComposeFile(name string) (string, bool) {
	for _, filename := range acceptedComposeFileNames {
		info, err := os.Stat(filepath.Join(r.stacksDir, name, filename))
		if err == nil && !info.IsDir() {
			return filename, true
		}
	}
	return "", false
}

// TrimStackOutput 规整 compose 输出（去掉首尾空白行）。
func TrimStackOutput(out string) string {
	return strings.TrimSpace(out)
}

// ParseXDockgeURLs 从 compose YAML 中解析 x-dockge.urls 扩展字段，
// 支持 ${VAR} 替换（从 .env 内容中提取变量）。
func ParseXDockgeURLs(yamlContent, envContent string) []string {
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		return nil
	}
	xDockge, ok := doc["x-dockge"].(map[string]any)
	if !ok {
		return nil
	}
	urlsRaw, ok := xDockge["urls"].([]any)
	if !ok {
		return nil
	}
	envVars := parseEnvVars(envContent)
	result := make([]string, 0, len(urlsRaw))
	for _, u := range urlsRaw {
		s, ok := u.(string)
		if !ok {
			continue
		}
		for k, v := range envVars {
			s = strings.ReplaceAll(s, "${"+k+"}", v)
			s = strings.ReplaceAll(s, "${"+strings.ToUpper(k)+"}", v)
		}
		if strings.TrimSpace(s) != "" {
			result = append(result, s)
		}
	}
	return result
}

func parseEnvVars(content string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}
