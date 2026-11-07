package handler

import (
	"bufio"
	"context"
	"dockge/pkg/jwt"
	"encoding/base64"
	"encoding/json"
	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"io"
	"net/http"
	"net/http/httptest"
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
