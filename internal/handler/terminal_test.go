package handler

import (
	"bufio"
	"context"
	"dockge/pkg/jwt"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestTerminalValidationOverSSE(t *testing.T) {
	router := gin.New()
	h := &TerminalHandler{}
	router.GET("/terminal/:name/:type", h.Events)
	server := httptest.NewServer(router)
	defer server.Close()
	for _, tc := range []struct{ path, detail string }{
		{"/terminal/test/unknown", "未知终端类型: unknown"}, {"/terminal/test/exec?shell=zsh", "不支持的 shell: zsh"},
		{"/terminal/test/compose-logs?tail=-1", "tail=-1"}, {"/terminal/-invalid/container-exec", "非法容器标识符: -invalid"},
	} {
		response, err := http.Get(server.URL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != 400 || !strings.Contains(string(data), tc.detail) {
			t.Fatalf("status=%d body=%s", response.StatusCode, data)
		}
	}
}
func TestSessionSSEPreservesSplitBytes(t *testing.T) {
	router := gin.New()
	router.GET("/stream", func(c *gin.Context) {
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		cmd := exec.CommandContext(ctx, "sh", "-c", `printf '\344'; sleep 0.02; printf '\270\255\346\226\207'`)
		(&TerminalHandler{}).serveSession(c, cmd, false, cancel)
	})
	server := httptest.NewServer(router)
	defer server.Close()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(server.URL + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	event := ""
	var output []byte
	done := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			event = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if event == "bytes" {
				var b string
				if err := json.Unmarshal([]byte(data), &b); err != nil {
					t.Fatal(err)
				}
				bytes, err := base64.StdEncoding.DecodeString(b)
				if err != nil {
					t.Fatal(err)
				}
				output = append(output, bytes...)
			}
			if event == "done" {
				done = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if string(output) != "中文" || !done {
		t.Fatalf("output=%q done=%v", output, done)
	}
}

func TestPTYInputResizeOwnershipAndDisconnectCleanup(t *testing.T) {
	h := &TerminalHandler{}
	ended := make(chan struct{})
	router := gin.New()
	router.Use(func(c *gin.Context) {
		hash := "owner"
		if c.GetHeader("X-Test-Owner") == "other" {
			hash = "other"
		}
		c.Set("claims", &jwt.Claims{UserId: 1, H: hash})
	})
	router.GET("/stream", func(c *gin.Context) {
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		h.serveSession(c, exec.CommandContext(ctx, "sh"), true, cancel)
		close(ended)
	})
	router.POST("/input/:id", h.Input)
	server := httptest.NewServer(router)
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Get(server.URL + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	var id string
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "data: ") {
			if err := json.Unmarshal([]byte(strings.TrimPrefix(scanner.Text(), "data: ")), &id); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if id == "" {
		t.Fatal("PTY 无可写会话")
	}
	send := func(data, owner string, want int) {
		t.Helper()
		req, _ := http.NewRequest("POST", server.URL+"/input/"+id, strings.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Owner", owner)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("status=%d want=%d body=%s", resp.StatusCode, want, body)
		}
	}
	send(`{"type":"input","data":"echo forbidden\n"}`, "other", 403)
	send(`{"type":"resize","rows":0,"cols":80}`, "", 400)
	send(`{"type":"resize","rows":31,"cols":97}`, "", 200)
	send(`{"type":"input","data":"printf 'REAL-PTY-'; printf 'OK\\n'; stty size\n"}`, "", 200)
	var output []byte
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event:") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			var text string
			if json.Unmarshal([]byte(strings.TrimSpace(line[5:])), &text) == nil {
				if chunk, err := base64.StdEncoding.DecodeString(text); err == nil {
					output = append(output, chunk...)
				}
			}
		}
		if strings.Contains(string(output), "REAL-PTY-OK") && strings.Contains(string(output), "31 97") {
			break
		}
	}
	if !strings.Contains(string(output), "REAL-PTY-OK") || !strings.Contains(string(output), "31 97") {
		t.Fatalf("真实 PTY 输出/尺寸=%q", output)
	}
	response.Body.Close()
	select {
	case <-ended:
	case <-time.After(3 * time.Second):
		t.Fatal("断流后终端子进程未结束")
	}
	if _, ok := h.sessions.Load(id); ok {
		t.Fatal("断流后会话遗留")
	}
	send(`{"type":"input","data":"echo after\n"}`, "", 404)
}

func TestSSEJWTDeadline(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/events", nil)
	c.Set("claims", &jwt.Claims{RegisteredClaims: jwtv5.RegisteredClaims{ExpiresAt: &jwtv5.NumericDate{Time: time.Now().Add(30 * time.Millisecond)}}})
	ctx, cancel := streamContext(c, nil)
	defer cancel()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("JWT 到期未结束 SSE")
	}
}

// 只用本地子进程；事件通知避免依赖生产运行时或线上实例。
type terminalEventWriter struct {
	*httptest.ResponseRecorder
	frames    chan string
	failEvent string
	pending   string
}

func (w *terminalEventWriter) Write(p []byte) (int, error) {
	frame := string(p)
	if strings.HasPrefix(frame, "event: "+w.failEvent+"\n") {
		return 0, errors.New("测试客户端写入断开")
	}
	w.pending += frame
	for {
		end := strings.Index(w.pending, "\n\n")
		if end < 0 {
			break
		}
		select {
		case w.frames <- w.pending[:end+2]:
		default:
		}
		w.pending = w.pending[end+2:]
	}
	return w.ResponseRecorder.Write(p)
}

// 帧边界由 SSE 分隔符决定，与服务端选择一次或分段 Write 无关。
func (w *terminalEventWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

func awaitTerminalEvent(t *testing.T, frames <-chan string, event string) string {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case frame := <-frames:
			if strings.HasPrefix(frame, "event: "+event+"\n") {
				return strings.TrimSuffix(strings.TrimPrefix(frame, "event: "+event+"\ndata: "), "\n\n")
			}
		case <-timer.C:
			t.Fatalf("未收到终端事件 %s", event)
		}
	}
}

func TestPTYInputCancellationInterruptsBlockedWrite(t *testing.T) {
	h := &TerminalHandler{}
	streamCtx, cancelStream := context.WithCancel(context.Background())
	writer := &terminalEventWriter{ResponseRecorder: httptest.NewRecorder(), frames: make(chan string, 10)}
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest("GET", "/stream", nil).WithContext(streamCtx)
	c.Set("claims", &jwt.Claims{UserId: 1, H: "owner"})
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		h.serveSession(c, exec.CommandContext(streamCtx, "sh", "-c", "stty raw -echo; printf terminal-ready; exec sleep 30"), true, cancelStream)
	}()
	defer func() {
		cancelStream()
		select {
		case <-ended:
		case <-time.After(3 * time.Second):
			t.Error("终端清理未退出")
		}
	}()
	var id string
	if err := json.Unmarshal([]byte(awaitTerminalEvent(t, writer.frames, "ready")), &id); err != nil {
		t.Fatal(err)
	}
	awaitTerminalEvent(t, writer.frames, "bytes")
	inputCtx, cancelInput := context.WithCancel(context.Background())
	defer cancelInput()
	body, _ := json.Marshal(map[string]string{"type": "input", "data": strings.Repeat("x", 16384)})
	inputWriter := httptest.NewRecorder()
	input, _ := gin.CreateTestContext(inputWriter)
	input.Request = httptest.NewRequest("POST", "/input/"+id, strings.NewReader(string(body))).WithContext(inputCtx)
	input.Params = gin.Params{{Key: "id", Value: id}}
	input.Set("claims", &jwt.Claims{UserId: 1, H: "owner"})
	inputEnded := make(chan struct{})
	go func() { defer close(inputEnded); h.Input(input) }()
	select {
	case <-inputEnded:
		t.Fatal("未构造出 PTY stdin 背压")
	case <-time.After(50 * time.Millisecond):
	}
	cancelInput()
	select {
	case <-inputEnded:
		if inputWriter.Code != 500 || !strings.Contains(inputWriter.Body.String(), "终端") {
			t.Fatalf("输入错误明细丢失: %s", inputWriter.Body.String())
		}
	case <-time.After(time.Second):
		cancelStream()
		select {
		case <-inputEnded:
		case <-time.After(3 * time.Second):
			t.Error("阻塞输入 goroutine 清理失败")
		}
		t.Fatal("POST 取消后，阻塞 stdin 写入仍未退出")
	}
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("输入写入错误未清理终端")
	}
	if _, ok := h.sessions.Load(id); ok {
		t.Fatal("输入错误后会话遗留")
	}
}

func TestTerminalDirectExitBoundsInheritedOutputPipe(t *testing.T) {
	h := &TerminalHandler{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := &terminalEventWriter{ResponseRecorder: httptest.NewRecorder(), frames: make(chan string, 10)}
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest("GET", "/stream", nil).WithContext(ctx)
	pidPath := t.TempDir() + "/child.pid"
	cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("sleep 30 & echo $! > %q; exit 0", pidPath))
	ended := make(chan struct{})
	go func() { defer close(ended); h.serveSession(c, cmd, false, cancel) }()
	awaitTerminalEvent(t, writer.frames, "ready")
	select {
	case <-ended:
		if !strings.Contains(writer.Body.String(), "event: done") || strings.Contains(writer.Body.String(), "event: error") {
			t.Fatalf("正常退出未完成安全清理: %s", writer.Body.String())
		}
		if cmd.ProcessState == nil {
			t.Fatal("终端退出后组长未收尸")
		}
		if err := cmd.Cancel(); err != nil {
			t.Fatalf("完成后重复取消失败: %v", err)
		}
	case <-time.After(3 * time.Second):
		cancel()
		// 旧实现还没有 Wait；取消会释放本测试创建的整个进程组。
		select {
		case <-ended:
		case <-time.After(3 * time.Second):
			t.Error("终端管道清理未退出")
		}
		data, _ := os.ReadFile(pidPath)
		t.Fatalf("直接进程退出后，继承 stdout 的后代仍占住 SSE，child pid=%s", data)
	}
}

func TestTerminalReadAndWriteErrorsCleanUp(t *testing.T) {
	for _, tc := range []struct {
		name, command, failEvent, typ, detail string
	}{
		{name: "ready_write", command: "exec sleep 30", failEvent: "ready"},
		{name: "output_write", command: "printf output; exec sleep 30", failEvent: "bytes"},
		{name: "oversized_log", command: "printf '%01048576d\\n' 0; exec sleep 30", typ: "compose-logs", detail: "bufio.Scanner: token too long"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &TerminalHandler{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			writer := &terminalEventWriter{ResponseRecorder: httptest.NewRecorder(), frames: make(chan string, 10), failEvent: tc.failEvent}
			c, _ := gin.CreateTestContext(writer)
			c.Request = httptest.NewRequest("GET", "/stream", nil).WithContext(ctx)
			c.Params = gin.Params{{Key: "type", Value: tc.typ}}
			cmd := exec.CommandContext(ctx, "sh", "-c", tc.command)
			ended := make(chan struct{})
			go func() { defer close(ended); h.serveSession(c, cmd, tc.typ == "", cancel) }()
			select {
			case <-ended:
			case <-time.After(3 * time.Second):
				cancel()
				select {
				case <-ended:
				case <-time.After(3 * time.Second):
					t.Error("错误路径清理未退出")
				}
				t.Fatal("输出/读取失败后终端未及时回收")
			}
			if ctx.Err() == nil || cmd.ProcessState == nil {
				t.Fatal("错误后未取消会话并等待进程退出")
			}
			h.sessions.Range(func(_, _ any) bool { t.Error("错误后会话遗留"); return true })
			if tc.detail != "" && !strings.Contains(writer.Body.String(), tc.detail) {
				t.Fatalf("丢失读取错误明细: %s", writer.Body.String())
			}
		})
	}
}
