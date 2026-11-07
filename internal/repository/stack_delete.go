package repository

import (
	"context"
	v1 "dockge/api/v1"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func projectOf(labels map[string]string) string { return labels["com.docker.compose.project"] }

// cleanupImageRef 只补 OCI 镜像省略的默认标签，不把注册表端口误判为标签，也不改 digest/镜像 ID。
func cleanupImageRef(ref string) string {
	if ref == "" || strings.Contains(ref, "@") || strings.HasPrefix(ref, "sha256:") {
		return ref
	}
	last := ref[strings.LastIndex(ref, "/")+1:]
	if !strings.Contains(last, ":") {
		return ref + ":latest"
	}
	return ref
}

type cleanupResource struct {
	Name     string
	External bool
}
type cleanupMount struct{ Type, Source, Target string }
type cleanupService struct {
	Image   string
	Volumes []cleanupMount
}
type cleanupConfig struct {
	Volumes  map[string]cleanupResource
	Networks map[string]cleanupResource
	Services map[string]cleanupService
}

func (r *Repository) cleanupConfig(ctx context.Context, name string) (cleanupConfig, error) {
	file, ok := r.findComposeFile(name)
	if !ok {
		return cleanupConfig{}, fmt.Errorf("栈 %q 缺少 compose 文件，无法确认资源归属；目录保留", name)
	}
	out, err := r.runComposeIn(ctx, r.StackPath(name), composeOpTimeout, "-p", name, "-f", file, "config", "--format", "json")
	if err != nil {
		return cleanupConfig{}, fmt.Errorf("栈 %q 清理范围校验失败，目录保留: %w", name, err)
	}
	docs, err := decodeRows[cleanupConfig](out)
	if err != nil {
		return cleanupConfig{}, fmt.Errorf("栈 %q 清理配置解析失败: %w", name, err)
	}
	if len(docs) != 1 || docs[0].Services == nil {
		return cleanupConfig{}, fmt.Errorf("栈 %q 清理配置必须包含一个 services 对象，实际返回 %d 个对象", name, len(docs))
	}
	return docs[0], nil
}
func resolvedResourceName(project, key string, resource cleanupResource) string {
	if resource.Name != "" {
		return resource.Name
	}
	if resource.External {
		return key
	}
	return project + "_" + key
}
func outsideDir(dir, source string) bool {
	rel, err := filepath.Rel(dir, source)
	return err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// PlanStackDelete 在清理前重新核对实际与未部署栈的引用，无法确认归属时禁止破坏数据。
func (r *Repository) PlanStackDelete(ctx context.Context, name string) (*v1.StackDeletePlan, error) {
	plan := &v1.StackDeletePlan{Containers: []string{}, Volumes: []string{}, Networks: []string{}, Images: []string{}, Retained: []string{}, Blockers: []string{}, Directory: r.StackPath(name)}
	if info, err := os.Lstat(plan.Directory); err != nil {
		return nil, fmt.Errorf("不能完整删除栈 %q：托管目录 %s 无法读取: %w", name, plan.Directory, err)
	} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("栈目录 %s 必须是托管目录而非符号链接", plan.Directory)
	}
	doc, err := r.cleanupConfig(ctx, name)
	if err != nil {
		return nil, err
	}
	cs, err := r.ResourceContainers(ctx)
	if err != nil {
		return nil, err
	}
	vs, err := r.ResourceVolumes(ctx, cs)
	if err != nil {
		return nil, err
	}
	ns, err := r.ResourceNetworks(ctx)
	if err != nil {
		return nil, err
	}
	imgs, err := r.ResourceImages(ctx, cs)
	if err != nil {
		return nil, err
	}
	externalVol, declaredVol, externalNet, declaredNet := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for k, v := range doc.Volumes {
		n := resolvedResourceName(name, k, v)
		if v.External {
			externalVol[n] = true
			plan.Retained = append(plan.Retained, "外部卷 "+n)
		} else {
			declaredVol[n] = true
		}
	}
	for k, v := range doc.Networks {
		n := resolvedResourceName(name, k, v)
		if v.External {
			externalNet[n] = true
			plan.Retained = append(plan.Retained, "外部网络 "+n)
		} else {
			declaredNet[n] = true
		}
	}
	candidateImage, anonymousVol := map[string]bool{}, map[string]bool{}
	for _, svc := range doc.Services {
		if svc.Image != "" {
			candidateImage[cleanupImageRef(svc.Image)] = true
		}
		for _, m := range svc.Volumes {
			if m.Type == "bind" && outsideDir(plan.Directory, m.Source) {
				plan.Retained = append(plan.Retained, "栈外宿主挂载 "+m.Source+" → "+m.Target)
			}
		}
	}
	for _, c := range cs {
		if projectOf(c.Labels) != name {
			continue
		}
		if c.System {
			plan.Blockers = append(plan.Blockers, "栈包含面板自身容器 "+strings.Join(c.Names, ","))
		}
		plan.Containers = append(plan.Containers, c.ID)
		candidateImage[c.ImageID] = true
		for _, m := range c.Mounts {
			if m.Type == "volume" && !declaredVol[m.Name] && !externalVol[m.Name] {
				anon := false
				for _, cm := range doc.Services[c.Labels["com.docker.compose.service"]].Volumes {
					anon = anon || cm.Type == "volume" && cm.Source == "" && cm.Target == m.Dest
				}
				if anon {
					anonymousVol[m.Name] = true
				} else {
					owned := false
					for _, v := range vs {
						owned = owned || v.Name == m.Name && projectOf(v.Labels) == name
					}
					if !owned {
						plan.Blockers = append(plan.Blockers, "容器 "+c.ID+" 的卷 "+m.Name+" 无法确认专属归属，请将其明确声明为命名卷或 external")
					}
				}
			}
			if m.Type == "bind" && outsideDir(plan.Directory, m.Source) {
				plan.Retained = append(plan.Retained, "栈外宿主挂载 "+m.Source+" → "+m.Dest)
			}
		}
	}
	// Compose config 会展开 .env，并核对未部署栈的卷、网络和镜像；有坏配置则不猜测安全性。
	otherImages, otherVols, otherNets, otherBinds := map[string]string{}, map[string]string{}, map[string]string{}, map[string]string{}
	entries, err := os.ReadDir(r.stacksDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == name {
			continue
		}
		if _, ok := r.findComposeFile(entry.Name()); !ok {
			continue
		}
		d, e := r.cleanupConfig(ctx, entry.Name())
		if e != nil {
			return nil, fmt.Errorf("不能核对其他栈 %q 的引用，删除未执行: %w", entry.Name(), e)
		}
		for k, v := range d.Volumes {
			otherVols[resolvedResourceName(entry.Name(), k, v)] = entry.Name()
		}
		for k, v := range d.Networks {
			otherNets[resolvedResourceName(entry.Name(), k, v)] = entry.Name()
		}
		for _, s := range d.Services {
			otherImages[cleanupImageRef(s.Image)] = entry.Name()
			for _, m := range s.Volumes {
				if m.Type == "bind" {
					otherBinds[m.Source] = entry.Name()
				}
			}
		}
	}
	for path, stack := range otherBinds {
		if !outsideDir(plan.Directory, path) {
			plan.Blockers = append(plan.Blockers, "栈目录内挂载 "+path+" 仍被其他栈 "+stack+" 引用")
		}
	}
	for _, v := range vs {
		if externalVol[v.Name] {
			continue
		}
		if projectOf(v.Labels) != name && !declaredVol[v.Name] && !anonymousVol[v.Name] {
			continue
		}
		plan.Volumes = append(plan.Volumes, v.Name)
		if p := projectOf(v.Labels); p != "" && p != name {
			plan.Blockers = append(plan.Blockers, "卷 "+v.Name+" 属于其他项目 "+p)
		}
		if s := otherVols[v.Name]; s != "" {
			plan.Blockers = append(plan.Blockers, "卷 "+v.Name+" 仍被其他栈 "+s+" 声明引用")
		}
		for _, u := range v.UsedBy {
			for _, c := range cs {
				if c.ID == u.ID && projectOf(c.Labels) != name {
					plan.Blockers = append(plan.Blockers, "卷 "+v.Name+" 仍被其他容器 "+u.Name+" 使用")
				}
			}
		}
	}
	for _, n := range ns {
		if externalNet[n.Name] || projectOf(n.Labels) != name && !declaredNet[n.Name] {
			continue
		}
		plan.Networks = append(plan.Networks, n.ID)
		if n.System {
			plan.Blockers = append(plan.Blockers, "禁止删除运行时内置网络 "+n.Name)
		}
		if p := projectOf(n.Labels); p != "" && p != name {
			plan.Blockers = append(plan.Blockers, "网络 "+n.Name+" 属于其他项目 "+p)
		}
		if s := otherNets[n.Name]; s != "" {
			plan.Blockers = append(plan.Blockers, "网络 "+n.Name+" 仍被其他栈 "+s+" 声明引用")
		}
		for _, e := range n.Containers {
			owned := false
			for _, c := range cs {
				owned = owned || c.ID == e.ID && projectOf(c.Labels) == name
			}
			if !owned {
				plan.Blockers = append(plan.Blockers, "网络 "+n.Name+" 仍连接其他容器 "+e.Name)
			}
		}
	}
	for _, image := range imgs {
		candidate, shared := candidateImage[image.ID], image.System || otherImages[cleanupImageRef(image.ID)] != ""
		for _, tag := range append(append([]string{}, image.RepoTags...), image.RepoDigests...) {
			candidate = candidate || candidateImage[cleanupImageRef(tag)]
			shared = shared || otherImages[cleanupImageRef(tag)] != ""
		}
		if !candidate {
			continue
		}
		for _, c := range cs {
			shared = shared || c.ImageID == image.ID && projectOf(c.Labels) != name
		}
		if shared {
			plan.Retained = append(plan.Retained, "共享镜像 "+image.ID+" "+strings.Join(image.RepoTags, ", "))
		} else {
			plan.Images = append(plan.Images, image.ID)
		}
	}
	plan.Containers = unique(plan.Containers)
	plan.Volumes = unique(plan.Volumes)
	plan.Networks = unique(plan.Networks)
	plan.Images = unique(plan.Images)
	plan.Retained = unique(plan.Retained)
	plan.Blockers = unique(plan.Blockers)
	return plan, nil
}

// CleanStackResources 先清运行时，再核验残留，最后才允许调用方删目录；不使用 force。
func (r *Repository) CleanStackResources(ctx context.Context, name string) (string, error) {
	plan, err := r.PlanStackDelete(ctx, name)
	if err != nil {
		return "", err
	}
	if len(plan.Blockers) > 0 {
		return "", fmt.Errorf("栈 %q 删除已阻止，未改动资源：%s", name, strings.Join(plan.Blockers, "；"))
	}
	file, _ := r.findComposeFile(name)
	output, err := r.runComposeIn(ctx, plan.Directory, composeOpTimeout, "-p", name, "-f", file, "down", "--remove-orphans", "--volumes")
	if err != nil {
		return output, fmt.Errorf("栈 %q 清理失败，目录 %s 已保留：%w", name, plan.Directory, err)
	}
	run := func(kind, id string) error {
		out, e := r.runDocker(ctx, composeOpTimeout, kind, "rm", id)
		output += "\n" + out
		if e != nil {
			return fmt.Errorf("%s %s 清理失败，目录保留: %w", kind, id, e)
		}
		return nil
	}
	cs, err := r.ResourceContainers(ctx)
	if err != nil {
		return output, err
	}
	for _, c := range cs {
		if projectOf(c.Labels) != name {
			continue
		}
		planned := false
		for _, id := range plan.Containers {
			planned = planned || id == c.ID
		}
		if !planned || c.System {
			return output, fmt.Errorf("栈 %q 清理期间出现未确认归属的容器 %s，目录保留，请重新核对删除范围", name, c.ID)
		}
		// Compose 不一定识别缺少完整元数据的孤立容器，只清理预检确认的项目成员。
		if c.State == "running" || c.State == "restarting" || c.State == "paused" {
			out, e := r.runDocker(ctx, composeOpTimeout, "stop", c.ID)
			output += "\n" + out
			if e != nil {
				return output, fmt.Errorf("孤立容器 %s 停止失败，目录保留: %w", c.ID, e)
			}
		}
		out, e := r.runDocker(ctx, composeOpTimeout, "rm", c.ID)
		output += "\n" + out
		if e != nil {
			return output, fmt.Errorf("孤立容器 %s 删除失败，目录保留: %w", c.ID, e)
		}
	}
	cs, err = r.ResourceContainers(ctx)
	if err != nil {
		return output, err
	}
	for _, c := range cs {
		if projectOf(c.Labels) == name {
			return output, fmt.Errorf("栈 %q 残留容器 %s，目录保留", name, c.ID)
		}
	}
	vs, err := r.ResourceVolumes(ctx, cs)
	if err != nil {
		return output, err
	}
	for _, v := range vs {
		for _, id := range plan.Volumes {
			if v.Name == id {
				if e := run("volume", id); e != nil {
					return output, e
				}
			}
		}
	}
	ns, err := r.ResourceNetworks(ctx)
	if err != nil {
		return output, err
	}
	for _, n := range ns {
		for _, id := range plan.Networks {
			if n.ID == id {
				if e := run("network", id); e != nil {
					return output, e
				}
			}
		}
	}
	imgs, err := r.ResourceImages(ctx, cs)
	if err != nil {
		return output, err
	}
	for _, image := range imgs {
		for _, id := range plan.Images {
			if image.ID == id {
				// 多标签镜像按标签逐个非强制删除，最后一个标签会删除镜像本体。
				refs := image.RepoTags
				if len(refs) == 0 {
					refs = []string{id}
				}
				for _, ref := range refs {
					if e := run("image", ref); e != nil {
						return output, e
					}
				}
			}
		}
	}
	cs, err = r.ResourceContainers(ctx)
	if err != nil {
		return output, err
	}
	for _, c := range cs {
		if projectOf(c.Labels) == name {
			return output, fmt.Errorf("栈 %q 删除后仍有容器 %s，目录保留", name, c.ID)
		}
	}
	vs, err = r.ResourceVolumes(ctx, cs)
	if err != nil {
		return output, err
	}
	for _, v := range vs {
		for _, id := range plan.Volumes {
			if v.Name == id {
				return output, fmt.Errorf("卷 %s 删除后仍存在，目录保留", id)
			}
		}
	}
	ns, err = r.ResourceNetworks(ctx)
	if err != nil {
		return output, err
	}
	for _, n := range ns {
		for _, id := range plan.Networks {
			if n.ID == id {
				return output, fmt.Errorf("网络 %s 删除后仍存在，目录保留", id)
			}
		}
	}
	imgs, err = r.ResourceImages(ctx, cs)
	if err != nil {
		return output, err
	}
	for _, image := range imgs {
		for _, id := range plan.Images {
			if image.ID == id {
				return output, fmt.Errorf("镜像 %s 删除后仍存在，目录保留", id)
			}
		}
	}
	if len(plan.Retained) > 0 {
		output += "\n已保留非专属引用：\n" + strings.Join(plan.Retained, "\n")
	}
	return output, nil
}
