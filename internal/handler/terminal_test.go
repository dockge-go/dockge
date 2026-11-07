package handler

import (
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// TestTerminalValidationOverWebSocket 锁定浏览器能收到校验明细，而非只有不透明的升级失败。
func TestTerminalValidationOverWebSocket(t *testing.T) {
	router := gin.New()
	h := &TerminalHandler{}
	router.GET("/terminal/:name/:type", h.WebSocket)
	server := httptest.NewServer(router)
	defer server.Close()
	for _, tc := range []struct{ path, detail string }{
		{"/terminal/test/unknown", "未知终端类型: unknown"},
		{"/terminal/test/exec?shell=zsh", "不支持的 shell: zsh"},
		{"/terminal/test/compose-logs?tail=-1", "tail=-1"},
		{"/terminal/-invalid/container-exec", "非法容器标识符: -invalid"},
	} {
		t.Run(tc.detail, func(t *testing.T) {
			ws, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+tc.path, nil)
			if err != nil {
				t.Fatalf("upgrade failed: %v %v", err, response)
			}
			defer ws.Close()
			_, message, err := ws.ReadMessage()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(message), "[error] "+tc.detail) {
				t.Fatalf("missing detail: %s", message)
			}
		})
	}
}

// TestSessionUsesBinaryFrames 保证任意 UTF-8 分片在浏览器端仍能连续解码，不产生非法文本帧。
func TestSessionUsesBinaryFrames(t *testing.T) {
	router := gin.New()
	router.GET("/stream", func(ctx *gin.Context) {
		ws, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			return
		}
		cmd := exec.Command("sh", "-c", `printf '\344'; sleep 0.02; printf '\270\255\346\226\207'`)
		(&TerminalHandler{}).serveSession(ws, cmd, true, false)
	})
	server := httptest.NewServer(router)
	defer server.Close()
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	_ = ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	var output []byte
	for {
		kind, data, err := ws.ReadMessage()
		if err != nil {
			break
		}
		if kind != websocket.BinaryMessage {
			t.Fatalf("kind=%d", kind)
		}
		output = append(output, data...)
	}
	if string(output) != "中文" {
		t.Fatalf("输出损坏: %q", output)
	}
}
