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

// List 仅发现 stacks 目录内的栈；运行时项目列表只补充已发现栈的真实状态。
func (r *Repository) List(ctx context.Context) ([]model.Stack, error) {
	stacks := make([]model.Stack, 0)
	byName := make(map[string]int)

	entries, err := os.ReadDir(r.stacksDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read stacks dir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
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
		// 文件仍可编辑，但无法查询运行时就不能断言栈未部署。
		for i := range stacks {
			stacks[i].Status = model.StatusUnknown
			stacks[i].RuntimeError = fmt.Sprintf("查询栈 %q 运行状态失败: %v", stacks[i].Name, err)
		}
		return stacks, nil
	}
	for _, item := range lsItems {
		if index, ok := byName[item.Name]; ok {
			stacks[index].Status = StackStatusFromString(item.Status)
		}
	}

	sort.Slice(stacks, func(i, j int) bool { return stacks[i].Name < stacks[j].Name })
	return stacks, nil
}

// Get 只读取托管目录的真实文件；运行时不可达或配置损坏也必须保留编辑入口。
func (r *Repository) Get(ctx context.Context, name string) (model.Stack, error) {
	dir, files, err := r.resolveStackExec(ctx, name)
	if err != nil {
		return model.Stack{}, err
	}
	stack := model.Stack{Name: name, Status: model.StatusCreatedFile, Managed: true,
		ComposeFileName: filepath.Base(files[0]), ConfigFiles: files[0]}
	content, err := os.ReadFile(files[0])
	if err != nil {
		return model.Stack{}, fmt.Errorf("读取栈 %q 配置 %s: %w", name, files[0], err)
	}
	stack.Yaml = string(content)
	envPath := filepath.Join(dir, ".env")
	if env, err := os.ReadFile(envPath); err == nil {
		stack.Env = string(env)
	} else if !errors.Is(err, os.ErrNotExist) {
		return model.Stack{}, fmt.Errorf("读取栈 %q 的环境文件 %s: %w", name, envPath, err)
	}
	if lsItems, err := r.ComposeLs(ctx); err == nil {
		for _, item := range lsItems {
			if item.Name == name {
				stack.Status = StackStatusFromString(item.Status)
				break
			}
		}
	} else {
		stack.Status = model.StatusUnknown
		stack.RuntimeError = fmt.Sprintf("查询栈 %q 运行状态失败: %v", name, err)
	}
	containers, err := r.StackPs(ctx, name)
	if err != nil {
		stack.Status = model.StatusUnknown
		stack.RuntimeError = strings.TrimSpace(stack.RuntimeError + "\n" + err.Error())
		stack.Containers = []model.Container{}
		return stack, nil
	}
	stack.Containers = containers
	return stack, nil
}

// Save 把 compose 文件写入栈目录；isAdd 时创建目录并拒绝重名，
// 否则要求目录已存在。栈目录其余文件（如 .env 之外的挂载卷）不受影响。
func (r *Repository) Save(ctx context.Context, stack *model.Stack, isAdd bool) error {
	dir := r.StackPath(stack.Name)
	if isAdd {
		if _, err := os.Lstat(dir); err == nil {
			return ErrConflict
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("%w：无法创建目录 %s：%v", ErrStacksNotWritable, dir, err)
		}
	} else {
		if info, err := os.Lstat(dir); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("%w: 读取栈目录 %s: %v", ErrNotFound, dir, err)
			}
			return fmt.Errorf("读取栈目录 %s: %w", dir, err)
		} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("栈目录 %s 必须是真实目录而非符号链接", dir)
		}
	}
	if !isAdd {
		if file, ok := r.findComposeFile(stack.Name); ok {
			stack.ComposeFileName = file
		}
	}
	file := stack.ComposeFileName
	if file == "" {
		file = "compose.yaml"
		stack.ComposeFileName = file
	}
	valid := false
	for _, accepted := range acceptedComposeFileNames {
		valid = valid || file == accepted
	}
	if !valid {
		return fmt.Errorf("非法 compose 文件名 %q：仅允许 %v", file, acceptedComposeFileNames)
	}
	if err := writeFileAtomic(filepath.Join(dir, file), []byte(stack.Yaml), 0o644); err != nil {
		return fmt.Errorf("%w：无法写入 %s：%v", ErrStacksNotWritable, filepath.Join(dir, file), err)
	}
	if stack.Env != "" {
		if err := writeFileAtomic(filepath.Join(dir, ".env"), []byte(stack.Env), 0o600); err != nil {
			return fmt.Errorf("%w：无法写入 %s：%v", ErrStacksNotWritable, filepath.Join(dir, ".env"), err)
		}
	} else if err := os.Remove(filepath.Join(dir, ".env")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale .env: %w", err)
	}
	return nil
}

// writeFileAtomic 原子写文件：先写同目录临时文件再 rename 替换，
// 中途崩溃（断电/被杀）不会留下半截文件——compose.yaml 是栈的唯一真相源。
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".dockge-tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(perm); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, path)
}

// Delete 删除整个栈目录，调用方须先完成运行时资源清理与核验。
func (r *Repository) Delete(ctx context.Context, name string) error {
	dir := r.StackPath(name)
	if info, err := os.Lstat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return fmt.Errorf("读取栈目录 %s: %w", dir, err)
	} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("栈目录 %s 必须是真实目录而非符号链接", dir)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove stack dir: %w", err)
	}
	return nil
}

// EnsureStacksDir 在启动时确保 stacks 目录存在。
func (r *Repository) EnsureStacksDir() error {
	if err := os.MkdirAll(r.stacksDir, 0o755); err != nil {
		return fmt.Errorf("ensure stacks dir: %w", err)
	}
	return nil
}

// findComposeFile 不跟随栈目录或配置文件符号链接，避免读取/执行目录外配置。
func (r *Repository) findComposeFile(name string) (string, bool) {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.HasPrefix(name, ".") {
		return "", false
	}
	dir := r.StackPath(name)
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", false
	}
	for _, filename := range acceptedComposeFileNames {
		info, err := os.Lstat(filepath.Join(dir, filename))
		if err == nil && info.Mode().IsRegular() {
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
