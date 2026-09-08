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

// Container 是栈内或全局的一个容器视图。
type Container struct {
	ID      string
	Name    string
	Service string `json:"service,omitempty"`
	Image   string
	State   string
	Status  string
	Ports   string
	Stack   string // 所属 compose 项目（来自容器 label，可空）
}

// ContainerStat 是单次容器资源采样（CPU/内存）。
type ContainerStat struct {
	ID         string
	Name       string
	CPUPercent float64
	MemPercent float64
	MemUsage   string // 人类可读用量，如 "120MiB / 3.8GiB"
}

// DfCategory 是一类 Docker 资源的磁盘占用汇总。
type DfCategory struct {
	Type        string
	Count       int
	Active      int
	Size        string
	Reclaimable string
}

// Volume 是本地存储的一个数据卷视图。
type Volume struct {
	Name   string
	Driver string
}

// Image 是本地存储的一个容器镜像视图。
type Image struct {
	ID      string
	Repo    string
	Tag     string
	Size    string // 人类可读大小，如 "52.2MB"
	Created string // 人类可读时间，如 "2 weeks ago"
}

// ---- 账号 ----

// DockgeUser 是 dockge 控制台的登录账号。
type DockgeUser struct {
	ID             uint
	Username       string
	Nickname       string
	Password       string // bcrypt 哈希
	Active         bool
	Timezone       string
	TwofaSecret    string // TOTP 密钥（base32）
	TwofaStatus    bool
	TwofaLastToken string // 上次 TOTP 令牌（防重放）
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ---- 设置 ----

// Setting 是 setting 表的一行。
type Setting struct {
	Key   string
	Value string // JSON 字符串或纯文本
	Type  string // 分组：general / security / ...
}
