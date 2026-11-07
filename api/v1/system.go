package v1

// HostSystem 明确采样范围；不可用的指标为 null，不能以容器统计代替服务器统计。
type HostSystem struct {
	Source     string            `json:"source"`
	SampledAt  string            `json:"sampledAt"`
	Hostname   string            `json:"hostname"`
	OS         string            `json:"os"`
	Kernel     string            `json:"kernel"`
	Arch       string            `json:"arch"`
	CPUModel   string            `json:"cpuModel"`
	CPUCount   *int              `json:"cpuCount"`
	CPUPercent *float64          `json:"cpuPercent"`
	Memory     *HostMemory       `json:"memory"`
	Swap       *HostMemory       `json:"swap"`
	Disk       *HostDisk         `json:"disk"`
	Load       []float64         `json:"load"`
	Uptime     *float64          `json:"uptime"`
	Errors     map[string]string `json:"errors"`
}

// HostMemory 使用 MemAvailable 而非 MemFree，避免把可回收缓存计为已用内存。
type HostMemory struct {
	Total     uint64  `json:"total"`
	Used      uint64  `json:"used"`
	Available uint64  `json:"available"`
	Percent   float64 `json:"percent"`
}

// HostDisk 只统计声明的文件系统，不以镜像逻辑大小冒充服务器磁盘占用。
type HostDisk struct {
	Path      string  `json:"path"`
	Total     uint64  `json:"total"`
	Used      uint64  `json:"used"`
	Available uint64  `json:"available"`
	Percent   float64 `json:"percent"`
}
