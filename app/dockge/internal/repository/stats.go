// 主机统计域：系统级 CPU/内存使用率（procfs 数据源），
// 与原版 Dockge 的 systeminformation 语义对齐。
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/procfs"
)

// DockerStats 返回宿主机系统级 CPU/内存使用率（与原版 Dockge 的
// systeminformation 语义一致），数据源为 procfs。
// CPU 需两次采样（间隔 200ms）计算时间片增量。
func (r *Repository) DockerStats(ctx context.Context) (cpuPerc, memPerc, memUsedMB, memTotalMB float64, err error) {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("procfs: %w", err)
	}
	stat1, err := fs.Stat()
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("procfs stat: %w", err)
	}
	time.Sleep(200 * time.Millisecond)
	stat2, err := fs.Stat()
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("procfs stat: %w", err)
	}
	dTotal := cpuStatTotal(stat2.CPUTotal) - cpuStatTotal(stat1.CPUTotal)
	dIdle := (stat2.CPUTotal.Idle + stat2.CPUTotal.Iowait) - (stat1.CPUTotal.Idle + stat1.CPUTotal.Iowait)
	if dTotal > 0 {
		cpuPerc = (1 - dIdle/dTotal) * 100
		if cpuPerc < 0 {
			cpuPerc = 0
		}
		if cpuPerc > 100 {
			cpuPerc = 100
		}
	}
	meminfo, memErr := fs.Meminfo()
	if memErr != nil || meminfo.MemTotal == nil {
		return cpuPerc, 0, 0, 0, nil
	}
	// procfs 的 Meminfo 字段单位为 kB
	memTotalMB = float64(*meminfo.MemTotal) / 1024
	var availKB uint64
	if meminfo.MemAvailable != nil {
		availKB = *meminfo.MemAvailable
	}
	memUsedMB = float64(*meminfo.MemTotal-availKB) / 1024
	if *meminfo.MemTotal > 0 {
		memPerc = float64(*meminfo.MemTotal-availKB) / float64(*meminfo.MemTotal) * 100
	}
	return cpuPerc, memPerc, memUsedMB, memTotalMB, nil
}

// cpuStatTotal 汇总 CPU 全部时间片（procfs v0.22 的 CPUStat 无 Total 方法）。
func cpuStatTotal(c procfs.CPUStat) float64 {
	return c.User + c.Nice + c.System + c.Idle + c.Iowait + c.IRQ + c.SoftIRQ + c.Steal + c.Guest + c.GuestNice
}
