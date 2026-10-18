// Package handler 是 dockge 应用的传输层：解析 HTTP 请求、调用 service、
// 统一写出响应；本层不包含业务规则。
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	v1 "dockge/api/v1"
	"dockge/internal/repository"
	"dockge/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/samber/do/v2"
)

// Package registers all handler-layer providers into the injector.
var Package = do.Package(
	do.Lazy(NewAuthHandler),
	do.Lazy(NewStackHandler),
	do.Lazy(NewImageHandler),
	do.Lazy(NewDockerHandler),
	do.Lazy(NewSettingsHandler),
	do.Lazy(NewComposerizeHandler),
	do.Lazy(NewTerminalHandler),
)

// GetUserIDFromCtx 从 gin 上下文取出 JWT 中间件写入的当前用户 ID。
func GetUserIDFromCtx(ctx *gin.Context) uint {
	v, exists := ctx.Get("claims")
	if !exists {
		return 0
	}
	return v.(*jwt.Claims).UserId
}

// handleServiceError 把 service 层错误映射为统一 HTTP 响应。
// 业务错误（*v1.Error：哨兵本身或被 fmt.Errorf 包装的哨兵）按其自带 code 与文案输出——
// 否则「同码但文案具体」的错误（如登录失败的各种原因）会被哨兵文案吞掉；
// 未归类的内部错误一律 500，但错误链原文直达用户（铁律：错误必须明细，
// 单用户自托管面板不存在「内部细节不可示人」的场景）。
func handleServiceError(ctx *gin.Context, err error) {
	if err == nil {
		return
	}
	var bizErr *v1.Error
	if errors.As(err, &bizErr) {
		// HandleError 统一展开错误链：%w 包装的细节（docker 真实报错等）直达用户
		v1.HandleError(ctx, bizErr.Code, err, nil)
		return
	}
	// 栈目录不可写：包装链自带真实原因与路径，用户才能判断是挂载问题
	if errors.Is(err, repository.ErrStacksNotWritable) {
		v1.HandleError(ctx, http.StatusInternalServerError,
			&v1.Error{Code: http.StatusInternalServerError, Message: err.Error()}, nil)
		return
	}
	v1.HandleError(ctx, http.StatusInternalServerError, err, nil)
}

// bindJSON 解析 JSON 请求体；失败时把绑定/校验错误翻译为指明字段与原因的
// 具体文案（铁律：杜绝裸「参数错误」——缺哪个字段、类型错在哪必须直达用户）。
func bindJSON(ctx *gin.Context, obj any) error {
	if err := ctx.ShouldBindJSON(obj); err != nil {
		return fmt.Errorf("%w: %s", v1.ErrBadRequest, bindErrDetail(reflect.TypeOf(obj), err))
	}
	return nil
}

// bindErrDetail 翻译 JSON 绑定错误：validator 校验失败给出字段与期望，
// 类型不匹配给出期望与实际类型，其余解析错误保留原文。
func bindErrDetail(t reflect.Type, err error) string {
	var valErrs validator.ValidationErrors
	if errors.As(err, &valErrs) {
		parts := make([]string, 0, len(valErrs))
		for _, fe := range valErrs {
			field := jsonFieldName(t, fe.Field())
			switch fe.Tag() {
			case "required":
				parts = append(parts, fmt.Sprintf("%s 不能为空", field))
			default:
				if p := fe.Param(); p != "" {
					parts = append(parts, fmt.Sprintf("%s 不满足要求 %s=%s", field, fe.Tag(), p))
				} else {
					parts = append(parts, fmt.Sprintf("%s 不满足要求 %s", field, fe.Tag()))
				}
			}
		}
		return strings.Join(parts, "；")
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		if typeErr.Field == "" {
			return fmt.Sprintf("请求体应为 %s 类型，收到 %s", typeErr.Type, typeErr.Value)
		}
		return fmt.Sprintf("字段 %s 应为 %s 类型，收到 %s", typeErr.Field, typeErr.Type, typeErr.Value)
	}
	if errors.Is(err, io.EOF) {
		return "请求体不能为空"
	}
	return fmt.Sprintf("请求体解析失败：%s", err.Error())
}

// jsonFieldName 从请求结构体的 json tag 取字段名（无 tag 时退回 Go 字段名），
// 保证校验错误文案与前端提交的字段名一致。
func jsonFieldName(t reflect.Type, field string) string {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if f, ok := t.FieldByName(field); ok {
		if name, _, _ := strings.Cut(f.Tag.Get("json"), ","); name != "" {
			return name
		}
	}
	return field
}
