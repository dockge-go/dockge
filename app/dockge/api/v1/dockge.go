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
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Service string        `json:"service,omitempty"`
	Image   string        `json:"image,omitempty"`
	State   string        `json:"state"`
	Status  string        `json:"status"`
	Ports   []PortMapping `json:"ports,omitempty"`
}

// ContainerStat 是单个容器的即时资源占用（对齐 docker stats --format json）。
type ContainerStat struct {
	Name     string `json:"name"`
	CPUPerc  string `json:"cpuPerc"`
	MemUsage string `json:"memUsage"`
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

// PortMapping 是一条端口映射（hostIP:hostPort -> containerPort/proto）。
type PortMapping struct {
	HostIP        string `json:"hostIP,omitempty"`
	HostPort      int    `json:"hostPort,omitempty"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
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

// -------- 用户管理（admin 专用） --------
