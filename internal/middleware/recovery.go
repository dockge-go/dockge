package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	v1 "dockge/api/v1"
	"dockge/pkg/log"
	"github.com/gin-gonic/gin"
)

// Recovery 将未写响应的 panic 转为明细 JSON；已写流则中止连接，避免拼接错误响应。
func Recovery(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			cause := recover()
			if cause == nil {
				return
			}
			// 保留 net/http 的主动断流语义，不当作业务崩溃重新写响应。
			if cause == http.ErrAbortHandler {
				panic(cause)
			}
			logger.WithContext(c).Error().
				Str("method", c.Request.Method).Str("path", c.Request.URL.Path).
				Str("cause", fmt.Sprint(cause)).Bytes("stack", debug.Stack()).Msg("HTTP handler panic")
			if c.Writer.Written() {
				panic(http.ErrAbortHandler)
			}
			c.Abort()
			v1.HandleError(c, http.StatusInternalServerError, &v1.Error{
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("处理请求 %s %s 时发生 panic：%v", c.Request.Method, c.Request.URL.Path, cause),
			}, nil)
		}()
		c.Next()
	}
}
