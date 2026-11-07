package v1

// ResourceContainer 保留运行时 inspect 的事实，缺少统计时省略而不伪造零值。
type ResourceContainer struct {
	ID            string            `json:"Id"`
	Names         []string          `json:"Names"`
	Image         string            `json:"Image"`
	ImageID       string            `json:"ImageID"`
	State         string            `json:"State"`
	Created       int64             `json:"Created"`
	Ports         []ResourcePort    `json:"Ports"`
	System        bool              `json:"system"`
	Env           []string          `json:"Env"`
	Mounts        []ResourceMount   `json:"Mounts"`
	Networks      []string          `json:"Networks"`
	NetworkIPs    map[string]string `json:"NetworkIPs"`
	Labels        map[string]string `json:"Labels"`
	RestartPolicy string            `json:"RestartPolicy"`
	RestartCount  int               `json:"RestartCount"`
	Health        string            `json:"Health,omitempty"`
	CPUs          float64           `json:"Cpus"`
	MemoryMB      float64           `json:"MemoryMB"`
	Entrypoint    string            `json:"Entrypoint"`
	Cmd           string            `json:"Cmd"`
	UpHours       *int              `json:"upHours,omitempty"`
	ExitCode      *int              `json:"exitCode,omitempty"`
	ExitMinAgo    *int              `json:"exitMinAgo,omitempty"`
	Stats         *ResourceStats    `json:"_stats,omitempty"`
}

// ResourcePort 表达 inspect 中的发布与未发布端口。
type ResourcePort struct {
	PrivatePort int
	PublicPort  int
	IP          string
	Proto       string
	Type        string
}

// ResourceMount 同时记录卷名和宿主源路径，以区分命名卷与 bind。
type ResourceMount struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
	Mode   string `json:"mode"`
	Type   string `json:"type"`
	Name   string `json:"name"`
}

// ResourceStats 的字节用量与 IO 原文都来自 stats 命令。
type ResourceStats struct {
	CPUPct  float64 `json:"cpuPct"`
	CPUBar  float64 `json:"cpuBar"`
	MemMB   float64 `json:"memMB"`
	MemBar  float64 `json:"memBar"`
	NetIO   string  `json:"netIO"`
	BlockIO string  `json:"blockIO"`
}

// ResourceImage 按镜像 ID 聚合全部标签，避免多个标签导致重复计数。
type ResourceImage struct {
	ID          string `json:"Id"`
	RepoTags    []string
	RepoDigests []string
	Repository  string
	Size        int64
	Created     int64
	System      bool `json:"system"`
	InUse       bool
	UsedBy      []ResourceUse
}

// ResourceUse 提供具体占用者，删除确认与错误不能只有模糊的“使用中”。
type ResourceUse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ResourceVolume 的 Usage 无法获得时明确标记未知。
type ResourceVolume struct {
	Name       string
	Driver     string
	Mountpoint string
	Usage      string
	System     bool `json:"system"`
	UsedBy     []ResourceUse
	Labels     map[string]string
}

// ResourceNetwork 提供真实子网、端点及项目归属。
type ResourceNetwork struct {
	ID         string
	Name       string
	Driver     string
	Subnet     string
	Containers []ResourceEndpoint
	System     bool `json:"system"`
	Labels     map[string]string
}

// ResourceEndpoint 提供网络端点的容器名及真实地址。
type ResourceEndpoint struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// ResourceDiskRow 的空间统计由 system df 提供，不用镜像虚拟大小相加猜测。
type ResourceDiskRow struct {
	Key          string  `json:"key"`
	Used         string  `json:"used"`
	Reclaim      string  `json:"reclaim"`
	Count        int     `json:"count"`
	ReclaimBytes float64 `json:"reclaimBytes"`
	ReclaimKnown bool    `json:"reclaimKnown"`
}

// ResourceSnapshot 各资源独立报错，避免一个不支持的命令遮蔽其他真实数据。
type ResourceSnapshot struct {
	Containers []ResourceContainer `json:"containers"`
	Images     []ResourceImage     `json:"images"`
	Volumes    []ResourceVolume    `json:"volumes"`
	Networks   []ResourceNetwork   `json:"networks"`
	Info       map[string]string   `json:"info"`
	Disk       []ResourceDiskRow   `json:"disk"`
	Errors     map[string]string   `json:"errors"`
	Runtime    string              `json:"runtime"`
}

// ResourceCreateRequest 限定创建参数，禁止传任意 CLI 选项。
type ResourceCreateRequest struct {
	Name          string `json:"name"`
	Image         string `json:"image"`
	HostPort      string `json:"hostPort"`
	ContainerPort string `json:"containerPort"`
	Command       string `json:"command"`
	Driver        string `json:"driver"`
}

// ResourceActionRequest 的 Value 用于重命名、镜像标签和提交镜像。
type ResourceActionRequest struct {
	Value string `json:"value"`
}

// StackDeletePlan 在任何破坏性操作前列出清理范围和不能误删的引用。
type StackDeletePlan struct {
	Containers []string `json:"containers"`
	Volumes    []string `json:"volumes"`
	Networks   []string `json:"networks"`
	Images     []string `json:"images"`
	Retained   []string `json:"retained"`
	Directory  string   `json:"directory"`
	Blockers   []string `json:"blockers"`
}
