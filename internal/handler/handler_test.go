package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type bindTestReq struct {
	Username string `json:"username" binding:"required"`
}

// TestBindJSON 表驱动验证请求体错误的明细文案：缺字段、类型错、空体、坏 JSON
// 必须各自给出可定位的原因（铁律：杜绝裸「参数错误」）。
func TestBindJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name    string
		body    string
		wantSub string // 期望 message 包含的明细片段；空串表示应成功
	}{
		{"正常请求", `{"username":"admin"}`, ""},
		{"缺必填字段", `{}`, "username 不能为空"},
		{"字段类型错误", `{"username":123}`, "字段 username 应为 string 类型"},
		{"请求体为空", ``, "请求体不能为空"},
		{"JSON 语法错误", `{"username":`, "请求体解析失败"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			err := bindJSON(c, &bindTestReq{})
			if tt.wantSub == "" {
				if err != nil {
					t.Fatalf("不应报错，得到: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("应报错（%s）", tt.name)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("错误明细应包含 %q，得到: %v", tt.wantSub, err)
			}
		})
	}
}
