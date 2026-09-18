// Package api/v1 定义 dockge 应用的 API 契约。
package v1

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type Error struct {
	Code    int
	Message string
}

// Error 实现 error 接口，返回业务错误消息。
func (e *Error) Error() string { return e.Message }

var (
	ErrSuccess             = newError(0, "ok")
	ErrBadRequest          = newError(400, "参数错误")
	ErrUnauthorized        = newError(401, "登录失效，请重新登录~")
	ErrNotFound            = newError(404, "数据不存在")
	ErrConflict            = newError(409, "名称已存在")
	ErrDockerError         = newError(500, "docker 操作失败")
	ErrInternalServerError = newError(500, "服务器错误~")
)

func newError(code int, msg string) *Error { return &Error{Code: code, Message: msg} }

// HandleSuccess 以统一响应包（code=0）写出成功结果；data 为 nil 时填充空对象。
func HandleSuccess(ctx *gin.Context, data interface{}) {
	if data == nil {
		data = map[string]interface{}{}
	}
	ctx.JSON(http.StatusOK, Response{Code: ErrSuccess.Code, Message: ErrSuccess.Message, Data: data})
}

// HandleError 以统一响应包写出失败结果；无法归类的错误按未知错误处理。
func HandleError(ctx *gin.Context, httpCode int, err error, data interface{}) {
	if data == nil {
		data = map[string]string{}
	}
	var bizErr *Error
	if !errors.As(err, &bizErr) {
		bizErr = &Error{Code: http.StatusInternalServerError, Message: "unknown error"}
	}
	ctx.JSON(httpCode, Response{Code: bizErr.Code, Message: bizErr.Message, Data: data})
}

// -------- 认证 --------

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type MeUserData struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Role     string `json:"role,omitempty"` // admin / member
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

type SetupRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginResponseData struct {
	AccessToken string     `json:"accessToken"`
	User        MeUserData `json:"user"`
}

// -------- 设置 --------

type SetGlobalEnvRequest struct {
	Content string `json:"content"`
}

// -------- 栈 --------

type StackSaveRequest struct {
	Name string `json:"name"`
	Yaml string `json:"yaml" binding:"required"`
	Env  string `json:"env"`
}

type StackSummaryData struct {
	Name            string `json:"name"`
	Status          int    `json:"status"`
	StatusLabel     string `json:"statusLabel"`
	Managed         bool   `json:"managed"`
	ComposeFileName string `json:"composeFileName,omitempty"`
	ConfigFiles     string `json:"configFiles,omitempty"`
}

type StackListData struct {
	List []StackSummaryData `json:"list"`
}

type StackDetailData struct {
	Name            string           `json:"name"`
	Status          int              `json:"status"`
	StatusLabel     string           `json:"statusLabel"`
	Managed         bool             `json:"managed"`
	ComposeFileName string           `json:"composeFileName,omitempty"`
	ConfigFiles     string           `json:"configFiles,omitempty"`
	Yaml            string           `json:"yaml"`
	Env             string           `json:"env"`
	Containers      []StackContainer `json:"containers"`
	URLs            []string         `json:"urls,omitempty"`
}

type StackContainer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Service string `json:"service,omitempty"`
	State   string `json:"state"`
	Status  string `json:"status"`
}

type StackOpResponse struct {
	Output string `json:"output"`
}

type StackValidateRequest struct {
	Yaml string `json:"yaml" binding:"required"`
	Env  string `json:"env"`
}

// StackValidateError 是一条校验诊断；Line 为 0 表示无法定位行号，
// 前端应降级为列表呈现而非行内标记。
type StackValidateError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

type StackValidateResponse struct {
	Valid  bool                 `json:"valid"`
	Errors []StackValidateError `json:"errors"`
}

// -------- docker --------

type DockerVersionData struct {
	Version    string `json:"version"`
	APIVersion string `json:"apiVersion"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
}

type DockerContainersData struct {
	List []DockerContainerData `json:"list"`
}

// ContainerStatusFrame 是容器状态长连接的单帧数据。
// 容器状态、资源计数与镜像列表同帧同源：前端一次性原子写入快照，
// 徽标计数（imagesTotal/stacksTotal 等）与列表永远一致——要删一起删、要留一起留。
// 后端任一资源采集失败则本帧整帧不推（下一事件或 30s 兜底重试），前端保持旧值。
type ContainerStatusFrame struct {
	Containers []ContainerStatusData `json:"containers"`
	Counts     *ResourceCounts       `json:"counts,omitempty"`
	Images     []DockerImageData     `json:"images,omitempty"`
}

// ResourceCounts 是侧栏徽标与仪表盘卡片的实时计数（由 docker events
// container+image 事件驱动，与容器状态同帧推送）。
type ResourceCounts struct {
	ContainersTotal   int `json:"containersTotal"`
	ContainersRunning int `json:"containersRunning"`
	StacksTotal       int `json:"stacksTotal"`
	StacksRunning     int `json:"stacksRunning"`
	ImagesTotal       int `json:"imagesTotal"`
}

// ContainerStatusData 仅携带会实时变化的容器字段。
type ContainerStatusData struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Status string `json:"status"`
}

type DockerContainerData struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Image  string        `json:"image"`
	State  string        `json:"state"`
	Status string        `json:"status"`
	Ports  []PortMapping `json:"ports"`
	Stack  string        `json:"stack,omitempty"`
}

// PortMapping 是一条端口映射（hostIP:hostPort -> containerPort/proto）。
type PortMapping struct {
	HostIP        string `json:"hostIP,omitempty"`
	HostPort      int    `json:"hostPort,omitempty"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
}

type DockerInfoData struct {
	Version           string `json:"version"`
	OS                string `json:"os"`
	Arch              string `json:"arch"`
	StacksTotal       int    `json:"stacksTotal"`
	StacksRunning     int    `json:"stacksRunning"`
	ContainersTotal   int    `json:"containersTotal"`
	ContainersRunning int    `json:"containersRunning"`
	ImagesTotal       int    `json:"imagesTotal"`
}

type DockerStatsData struct {
	CPUUsage   float64 `json:"cpuUsage"`   // 系统 CPU 使用率（0-100 百分数）
	MemUsage   float64 `json:"memUsage"`   // 已用内存 MB
	MemTotalMB float64 `json:"memTotalMB"` // 内存总量 MB
	MemPercent float64 `json:"memPercent"` // 内存使用率（0-100 百分数）
	// Error 非空表示本帧为错误帧（码语义，前端映射文案）：
	// stats_unavailable = 数据源不可用（如非 Linux 平台无 /proc），流保持 2s 重试兼作心跳。
	Error string `json:"error,omitempty"`
}

type DockerDfCategory struct {
	Type             string `json:"type"`
	Count            int    `json:"count"`
	Active           int    `json:"active"`
	SizeBytes        int64  `json:"sizeBytes"`
	ReclaimableBytes int64  `json:"reclaimableBytes"`
}

// -------- composerize --------

type ComposerizeRequest struct {
	DockerRunCommand string `json:"dockerRunCommand" binding:"required"`
}

type ComposerizeResponse struct {
	ComposeTemplate string `json:"composeTemplate"`
}

// VersionCheckResponse 是版本检查响应。
type VersionCheckResponse struct {
	LatestVersion  string `json:"latestVersion"`
	CurrentVersion string `json:"currentVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
}

// -------- 镜像 --------

type DockerImageData struct {
	ID          string `json:"id"`
	Repo        string `json:"repo"`
	Tag         string `json:"tag"`
	SizeBytes   int64  `json:"sizeBytes"`
	CreatedUnix int64  `json:"createdAt"` // 构建时间 unix 秒
}

type DockerVolumeData struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
}

// DockerNetworkData 是网络列表的一行。
type DockerNetworkData struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
}

type PullImageRequest struct {
	Reference string `json:"reference" binding:"required"`
}

// NetworkCreateRequest 创建网络的请求体。
type NetworkCreateRequest struct {
	Name   string `json:"name" binding:"required"`
	Driver string `json:"driver"`
	Subnet string `json:"subnet"`
}

// -------- 用户管理（admin 专用） --------

type UserRow struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`   // admin / member
	Active   bool   `json:"active"` // 停用即时失效其全部会话（CheckSession）
	Source   string `json:"source"` // local / proxy / oidc
}

type UserCreateRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role"` // 空 = member
}

type UserRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

type UserActiveRequest struct {
	Active bool `json:"active"`
}
