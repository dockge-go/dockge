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
	TwoFA    bool   `json:"twoFA,omitempty"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

type SetupRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

type TwoFARequest struct {
	Username string `json:"username" binding:"required"`
	Token    string `json:"token" binding:"required"`
}

type LoginResponseData struct {
	AccessToken   string     `json:"accessToken"`
	User          MeUserData `json:"user"`
	TokenRequired bool       `json:"tokenRequired,omitempty"`
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
type ContainerStatusFrame struct {
	Containers []ContainerStatusData `json:"containers"`
}

// ContainerStatusData 仅携带会实时变化的容器字段。
type ContainerStatusData struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Status string `json:"status"`
}

type DockerContainerData struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	State  string `json:"state"`
	Status string `json:"status"`
	Ports  string `json:"ports"`
	Stack  string `json:"stack,omitempty"`
}

type DockerInfoData struct {
	Version           string `json:"version"`
	OS                string `json:"os"`
	Arch              string `json:"arch"`
	StacksTotal       int    `json:"stacksTotal"`
	StacksRunning     int    `json:"stacksRunning"`
	ContainersTotal   int    `json:"containersTotal"`
	ContainersRunning int    `json:"containersRunning"`
}

type ContainerStatData struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	CPU        float64 `json:"cpu"`
	MemPercent float64 `json:"memPercent"`
	MemUsage   string  `json:"memUsage"`
}

type DockerStatsData struct {
	CPUUsage   float64             `json:"cpuUsage"`             // 系统 CPU 使用率（0-100 百分数）
	MemUsage   float64             `json:"memUsage"`             // 已用内存 MB
	MemTotalMB float64             `json:"memTotalMB"`           // 内存总量 MB
	MemPercent float64             `json:"memPercent"`           // 内存使用率（0-100 百分数）
	Containers []ContainerStatData `json:"containers,omitempty"` // 运行中容器的单次采样
}

type DockerDfCategory struct {
	Type        string `json:"type"`
	Count       int    `json:"count"`
	Active      int    `json:"active"`
	Size        string `json:"size"`
	Reclaimable string `json:"reclaimable"`
}

type DockerDfData struct {
	List []DockerDfCategory `json:"list"`
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
	ID      string `json:"id"`
	Repo    string `json:"repo"`
	Tag     string `json:"tag"`
	Size    string `json:"size"`
	Created string `json:"created"`
}

type DockerImagesData struct {
	List []DockerImageData `json:"list"`
}

type DockerVolumeData struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
}

type DockerVolumesData struct {
	List []DockerVolumeData `json:"list"`
}

type PullImageRequest struct {
	Reference string `json:"reference" binding:"required"`
}
