// Package v1 定义 API 响应结构与错误码。
package v1

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构。
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// Error 携带业务码的错误类型。
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

// newError 构造带业务码的错误。
func newError(code int, msg string) *Error { return &Error{Code: code, Message: msg} }

// HandleSuccess 以统一响应包（code=0）写出成功结果；data 为 nil 时填充空对象。
func HandleSuccess(ctx *gin.Context, data interface{}) {
	if data == nil {
		data = map[string]interface{}{}
	}
	ctx.JSON(http.StatusOK, Response{Code: ErrSuccess.Code, Message: ErrSuccess.Message, Data: data})
}

// HandleError 以统一响应包写出失败结果；无法归类的错误按未知错误处理。
// 错误链带附加说明（fmt.Errorf %w 包装）时用完整链文本作 message，
// 避免细节被哨兵错误的通用文案吞掉——docker 失败等原因必须到达用户。
func HandleError(ctx *gin.Context, httpCode int, err error, data interface{}) {
	if data == nil {
		data = map[string]string{}
	}
	var bizErr *Error
	if !errors.As(err, &bizErr) {
		bizErr = &Error{Code: http.StatusInternalServerError, Message: "unknown error"}
	}
	msg := bizErr.Message
	if err != nil && err.Error() != msg {
		msg = err.Error()
	}
	ctx.JSON(httpCode, Response{Code: bizErr.Code, Message: msg, Data: data})
}
