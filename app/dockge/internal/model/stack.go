// Package model 定义 dockge 应用的领域实体：compose 栈、容器、账号与设置。
package model

import "time"

// ---- 栈与容器 ----

// StackStatus 表示 compose 栈的运行状态，状态码语义与 Dockge 对齐。
type StackStatus int

const (
	StatusUnknown     StackStatus = 0 // 未部署/未知
	StatusCreatedFile StackStatus = 1 // 仅有 compose 文件
	StatusCreated     StackStatus = 2 // compose 状态 created
	StatusRunning     StackStatus = 3 // 全部 running
	StatusExited      StackStatus = 4 // 存在 exited
)

// Label 返回状态的可读展示名。
func (s StackStatus) Label() string {
	switch s {
	case StatusCreatedFile:
		return "未部署"
	case StatusCreated:
		return "已创建"
	case StatusRunning:
		return "运行中"
	case StatusExited:
		return "已停止"
	default:
		return "未知"
	}
}

// Stack 是一个 compose 编排栈。
type Stack struct {
	Name            string
	Status          StackStatus
	Managed         bool // stacks 目录托管
	ComposeFileName string
	ConfigFiles     string
	Yaml            string
	Env             string
	Containers      []Container
}

// PortMapping 是容器的一条端口映射（hostIP: hostPort -> containerPort/proto）。
type PortMapping struct {
	HostIP        string
	HostPort      int
	ContainerPort int
	Protocol      string
}

// ContainerStat 是单个容器的即时资源占用（docker stats --no-stream 的一行，
// 字段名与 docker stats --format json 输出对齐）。
type ContainerStat struct {
	Name     string `json:"Name"`
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
}

// Container 是栈内或全局的一个容器视图。
type Container struct {
	ID      string
	Name    string
	Service string `json:"service,omitempty"`
	Image   string
	State   string
	Status  string
	Ports   []PortMapping
	Stack   string // 所属 compose 项目（来自容器 label，可空）
}

// Network 是本机一个 docker/podman 网络视图。
type Network struct {
	Name   string
	Driver string
}

// Image 是本地存储的一个容器镜像视图。
type Image struct {
	ID          string
	Repo        string
	Tag         string
	SizeBytes   int64
	CreatedUnix int64 // 镜像构建时间的 unix 秒
}

// ---- 账号 ----

// 用户角色取值。
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// 账号来源取值。
const (
	SourceLocal = "local" // 首启引导/管理员创建
	SourceProxy = "proxy" // 受信反代头自动开户
	SourceOIDC  = "oidc"  // OIDC SSO 自动开户
)

// DockgeUser 是 dockge 控制台的登录账号。
// （TOTP/2FA 字段已随功能移除：存量 bbolt 记录中的旧 JSON 字段在反序列化时被忽略。）
type DockgeUser struct {
	ID        uint
	Username  string
	Nickname  string
	Password  string // bcrypt 哈希
	Role      string // admin / member；空值兼容旧数据，按 admin 处理
	Active    bool
	Timezone  string
	Source    string // "local" | "oidc" | "proxy"，记录账号来源
	CreatedAt time.Time
	UpdatedAt time.Time
}
