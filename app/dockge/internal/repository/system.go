// 引擎信息域：docker/podman 服务端版本摘要。
// podman client 优先，不可用时降级 docker CLI。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.podman.io/podman/v6/pkg/bindings/system"

	"dockge/app/dockge/internal/model"
)

// DockerVersion 返回 docker/podman 服务端版本摘要；守护进程不可用时返回错误。
// 优先使用 podman Go client，不可用时降级到 docker CLI。
func (r *Repository) DockerVersion(ctx context.Context) (map[string]any, error) {
	if r.podman != nil && r.podman.IsAvailable() {
		info, err := system.Info(ctx, &system.InfoOptions{})
		if err == nil && info != nil {
			return map[string]any{
				"Version": info.Version.Version,
				"Os":      info.Host.OS,
				"Arch":    info.Host.Arch,
			}, nil
		}
		r.logger.Warn().Err(err).Msg("podman info failed, falling back to docker CLI")
	}
	out, err := runDocker(ctx, 30*time.Second, "version", "--format", "{{json .}}")
	if err != nil {
		return nil, fmt.Errorf("docker version: %w", err)
	}
	var full struct {
		Client map[string]any `json:"Client"`
		Server map[string]any `json:"Server"`
	}
	if err := json.Unmarshal([]byte(out), &full); err != nil {
		return nil, fmt.Errorf("parse docker version output: %w", err)
	}
	server := full.Server
	if server == nil {
		server = map[string]any{}
	}
	return server, nil
}

// DockerDf 返回镜像/容器/卷三类资源的磁盘占用汇总。
// podman bindings 优先（类型化数值，逐项汇总）；不可用时降级 docker system df CLI。
func (r *Repository) DockerDf(ctx context.Context) ([]model.DfCategory, error) {
	if r.podman != nil && r.podman.IsAvailable() {
		df, err := system.DiskUsage(ctx, &system.DiskOptions{})
		if err == nil {
			imgSize, imgUnused := 0.0, 0.0
			imgActive := 0
			for _, i := range df.Images {
				imgSize += float64(i.Size)
				if i.Containers > 0 {
					imgActive++
				} else {
					imgUnused += float64(i.Size)
				}
			}
			ctrSize, ctrActive := 0.0, 0
			for _, c := range df.Containers {
				ctrSize += float64(c.Size)
				if strings.Contains(c.Status, "Up") {
					ctrActive++
				}
			}
			volSize, volActive, volReclaim := 0.0, 0, 0.0
			for _, v := range df.Volumes {
				volSize += float64(v.Size)
				if v.Links > 0 {
					volActive++
				}
				volReclaim += float64(v.ReclaimableSize)
			}
			return []model.DfCategory{
				{Type: "镜像", Count: len(df.Images), Active: imgActive, SizeBytes: int64(imgSize), ReclaimableBytes: int64(imgUnused)},
				{Type: "容器", Count: len(df.Containers), Active: ctrActive, SizeBytes: int64(ctrSize)},
				{Type: "卷", Count: len(df.Volumes), Active: volActive, SizeBytes: int64(volSize), ReclaimableBytes: int64(volReclaim)},
			}, nil
		}
		r.logger.Warn().Err(err).Msg("podman df failed, falling back to docker CLI")
	}
	out, err := runDocker(ctx, 60*time.Second, "system", "df", "--format", "{{json .}}")
	if err != nil {
		return nil, fmt.Errorf("docker system df: %w", err)
	}
	result := make([]model.DfCategory, 0)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// TotalCount/Active 在不同 docker 版本中可能是数字或字符串，用 json.Number 兼容
		var item struct {
			Type        string      `json:"Type"`
			TotalCount  json.Number `json:"TotalCount"`
			Active      json.Number `json:"Active"`
			Size        string      `json:"Size"`
			Reclaimable string      `json:"Reclaimable"`
		}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("parse docker system df output: %w", err)
		}
		count, _ := item.TotalCount.Int64()
		active, _ := item.Active.Int64()
		// Reclaimable 形如 "500MB (40%)"，取括号前的数值部分
		reclaimable, _, _ := strings.Cut(item.Reclaimable, " (")
		result = append(result, model.DfCategory{
			Type: item.Type, Count: int(count), Active: int(active),
			SizeBytes: parseDockerSize(item.Size), ReclaimableBytes: parseDockerSize(reclaimable),
		})
	}
	return result, nil
}
