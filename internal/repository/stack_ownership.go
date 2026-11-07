package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// stackDirectoryAliases 将固定容器路径与真实宿主 bind 路径视为同一个托管目录。
func (r *Repository) stackDirectoryAliases(name string) []string {
	dir := r.StackPath(name)
	aliases := []string{dir}
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		aliases = append(aliases, real)
	}
	// bind 挂载在容器内和宿主上的路径不同，挂载表提供固定路径的真实别名。
	if data, err := os.ReadFile(mountinfoPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 6 {
				continue
			}
			root, point := unescapeMountPoint(fields[3]), unescapeMountPoint(fields[4])
			if point == "/" || root == "/" {
				continue
			}
			if rel, err := filepath.Rel(point, dir); err == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
				aliases = append(aliases, filepath.Join(root, rel))
			}
		}
	}
	return aliases
}

// configOwned 核对配置文件路径，而不是仅靠 Compose 项目名推断归属。
func (r *Repository) configOwned(name, configFiles string) bool {
	if strings.TrimSpace(configFiles) == "" {
		return false
	}
	aliases := r.stackDirectoryAliases(name)
	for _, path := range strings.Split(configFiles, ",") {
		path = strings.TrimSpace(path)
		matched := false
		for _, alias := range aliases {
			for _, file := range acceptedComposeFileNames {
				matched = matched || filepath.Clean(path) == filepath.Join(alias, file)
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
func (r *Repository) checkStackOwnership(ctx context.Context, name string) error {
	items, err := r.ComposeLs(ctx)
	if err != nil {
		return fmt.Errorf("无法核验栈 %q 的项目归属，操作未执行: %w", name, err)
	}
	trustedProject := false
	for _, item := range items {
		if item.Name == name && !r.configOwned(name, item.ConfigFiles) {
			return fmt.Errorf("栈 %q 与目录外 Compose 项目同名：运行时配置 %q，托管目录 %s；为避免误操作外部资源，操作已阻止", name, item.ConfigFiles, r.StackPath(name))
		}
		trustedProject = trustedProject || item.Name == name
	}
	containers, err := r.ResourceContainers(ctx)
	if err != nil {
		return fmt.Errorf("无法核验栈 %q 的容器归属，操作未执行: %w", name, err)
	}
	for _, c := range containers {
		if projectOf(c.Labels) != name {
			continue
		}
		files := c.Labels["com.docker.compose.project.config_files"]
		if (files != "" && !r.configOwned(name, files)) || (files == "" && !trustedProject) {
			return fmt.Errorf("同名项目容器 %s 的配置归属无法确认（config_files=%q）；栈 %q 操作已阻止", c.ID, files, name)
		}
		if c.System {
			return fmt.Errorf("栈 %q 包含面板自身容器 %s，操作已阻止", name, c.ID)
		}
	}
	return nil
}

func (r *Repository) stackSourceInside(name, source string) bool {
	paths := []string{source}
	if real, err := filepath.EvalSymlinks(source); err == nil {
		paths = append(paths, real)
	}
	for _, dir := range r.stackDirectoryAliases(name) {
		for _, path := range paths {
			if !outsideDir(dir, path) {
				return true
			}
		}
	}
	return false
}
