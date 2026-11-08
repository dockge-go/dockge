package handler

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"

	v1 "dockge/api/v1"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
		{"尾随空白", "{\"username\":\"admin\"} \n\t", ""},
		{"非JSON空白", "\v{\"username\":\"admin\"}\u00a0", "请求体解析失败"},
		{"第二个JSON对象", `{"username":"admin"}{"username":"other"}`, "请求体解析失败"},
		{"尾随垃圾", `{"username":"admin"}garbage`, "请求体解析失败"},
		{"超限字段", `{"username":"` + strings.Repeat("a", 4<<20) + `"}`, "4194304"},
		{"超限尾随空白", `{"username":"admin"}` + strings.Repeat(" ", 4<<20), "4194304"},
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

// TestBindJSONNull 不带 required 的请求也必须提供对象，不能把 null 当作合法清空操作。
func TestBindJSONNull(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader("null"))
	err := bindJSON(c, &struct {
		Enable bool `json:"enable"`
	}{})
	if err == nil || !strings.Contains(err.Error(), "JSON 对象") {
		t.Fatalf("null 应拒绝且说明需要 JSON 对象，实际 %v", err)
	}
}

// bodyDeadlineWriter 模拟真实 HTTP writer 的读取期限接口，避免全局超时影响 SSE。
type bodyDeadlineWriter struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
	setErr    error
}

func (w *bodyDeadlineWriter) SetReadDeadline(d time.Time) error {
	w.deadlines = append(w.deadlines, d)
	return w.setErr
}
func TestBindJSONReadDeadline(t *testing.T) {
	for _, body := range []string{`{"username":"admin"}`, `{"username":`} {
		w := &bodyDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))
		before := time.Now()
		_ = bindJSON(c, &bindTestReq{})
		if len(w.deadlines) != 2 || !w.deadlines[0].After(before) || !w.deadlines[1].IsZero() {
			t.Fatalf("读取前应设置期限、读完后应清除期限，实际 %v", w.deadlines)
		}
	}
}
func TestBindJSONReadDeadlineFailure(t *testing.T) {
	cause := errors.New("read deadline unavailable")
	w := &bodyDeadlineWriter{ResponseRecorder: httptest.NewRecorder(), setErr: cause}
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"username":"admin"}`))
	err := bindJSON(c, &bindTestReq{})
	if err == nil || !strings.Contains(err.Error(), cause.Error()) {
		t.Fatalf("设置期限失败应停止读取并提供原因，实际 %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatal("绑定不应自行写出响应")
	}
}

type failedBodyReader struct{}

func (failedBodyReader) Read([]byte) (int, error) { return 0, os.ErrDeadlineExceeded }
func TestBindJSONBodyTimeoutClosesConnection(t *testing.T) {
	w := &bodyDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)
	c.Request.Body = io.NopCloser(failedBodyReader{})
	err := bindJSON(c, &bindTestReq{})
	if err == nil || !strings.Contains(err.Error(), "请求体读取超时（最大 10s）") {
		t.Fatalf("应保留超时明细，实际 %v", err)
	}
	if len(w.deadlines) != 1 || w.Header().Get("Connection") != "close" {
		t.Fatal("读取失败应关闭连接，不能清除期限后继续排空正文")
	}
}

func TestJSONSlowBodyReturnsAndReleasesHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	finished := make(chan struct{})
	router := gin.New()
	router.POST("/", func(c *gin.Context) {
		defer close(finished)
		_, err := readJSONBody(c, 50*time.Millisecond)
		if err != nil {
			handleServiceError(c, fmt.Errorf("%w: %s", v1.ErrBadRequest, err))
			return
		}
		c.Status(http.StatusNoContent)
	})
	server := httptest.NewServer(router)
	defer server.Close()
	conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	// 完整头部、未完成正文，不主动关闭发送端。
	_, err = fmt.Fprint(conn, "POST / HTTP/1.1\r\nHost: localhost\r\nContent-Length: 100\r\nContent-Type: application/json\r\n\r\n{\"username\":")
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusBadRequest || !response.Close || !strings.Contains(string(body), "请求体读取超时") {
		t.Fatalf("慢正文应具体报错并关闭连接，实际 %d %s", response.StatusCode, body)
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("正文超时后处理器仍未释放")
	}
}

func TestJSONBodyDeadlineDoesNotLimitFollowingSSE(t *testing.T) {
	router := gin.New()
	router.POST("/", func(c *gin.Context) {
		_, err := readJSONBody(c, 25*time.Millisecond)
		if err != nil {
			t.Errorf("正常正文读取失败：%v", err)
			c.Status(400)
			return
		}
		time.Sleep(75 * time.Millisecond)
		if err := newSSE(c).event("output", "real output"); err != nil {
			t.Errorf("读取后的 SSE 不应超时：%v", err)
		}
	})
	server := httptest.NewServer(router)
	defer server.Close()
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Post(server.URL, "application/json", strings.NewReader(`{"username":"admin"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != 200 || !strings.Contains(string(body), `data: "real output"`) {
		t.Fatalf("正常读取后应允许长响应，实际 %s %v", body, err)
	}
}
