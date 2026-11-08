package handler

import (
	"bufio"
	"context"
	"crypto/rand"
	v1 "dockge/api/v1"
	"dockge/internal/repository"
	"dockge/internal/service"
	"dockge/pkg/jwt"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"golang.org/x/sys/unix"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// TerminalHandler 每个 SSE 连接独占进程，HTTP 上行只接受对应认证会话的输入。
type TerminalHandler struct {
	Repository *repository.Repository `do:""`
	Auth       service.AuthService    `do:""`
	sessions   sync.Map
}

// NewTerminalHandler 注入终端依赖。
func NewTerminalHandler(i do.Injector) (*TerminalHandler, error) {
	return do.MustInvokeStruct[*TerminalHandler](i), nil
}

type terminalSession struct {
	mu     sync.Mutex
	master *os.File
	owner  uint
	hash   string
	cancel context.CancelFunc
	closed atomic.Bool
}

// 关闭不获取输入锁，否则背压中的 Write 会阻止关闭自身。
func (s *terminalSession) close() {
	if !s.closed.Swap(true) {
		s.cancel()
		_ = s.master.Close()
	}
}

func resizeTerminal(master *os.File, rows, cols uint16) error {
	raw, err := master.SyscallConn()
	if err != nil {
		return err
	}
	var ioctlErr error
	err = raw.Control(func(fd uintptr) {
		// pty.Setsize 使用 Fd()，会重新启用阻塞 I/O；直接 ioctl 保持 poller 可取消。
		ioctlErr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, &unix.Winsize{Row: rows, Col: cols})
		if ioctlErr == nil {
			ioctlErr = syscall.SetNonblock(int(fd), true)
		}
	})
	return errors.Join(err, ioctlErr)
}

// Events 提供容器/栈日志与真实 PTY 输出，不把认证信息放入 URL。
func (h *TerminalHandler) Events(ctx *gin.Context) {
	streamCtx, cancel := streamContext(ctx, h.Auth)
	defer cancel()
	ctx.Request = ctx.Request.WithContext(streamCtx)
	name, typ := ctx.Param("name"), ctx.Param("type")
	var cmd *exec.Cmd
	var err error
	readOnly := typ == "compose-logs" || typ == "container-logs"
	fail := func(message string) { v1.HandleError(ctx, http.StatusBadRequest, fmt.Errorf("%s", message), nil) }
	if typ != "compose-logs" && typ != "exec" && typ != "container-logs" && typ != "container-exec" {
		fail("未知终端类型: " + typ)
		return
	}
	shell := ctx.DefaultQuery("shell", "sh")
	if !readOnly && shell != "bash" && shell != "sh" {
		fail("不支持的 shell: " + shell + "，仅支持 sh/bash")
		return
	}
	tail := ctx.DefaultQuery("tail", "100")
	since := ctx.Query("since")
	if readOnly && since != "" {
		if _, err := time.Parse(time.RFC3339Nano, since); err != nil {
			fail("since=" + since + "，必须是 RFC3339 时间戳")
			return
		}
	}
	if readOnly && !(tail == "all" && since != "") {
		if n, e := strconv.Atoi(tail); e != nil || n < 0 || n > 10000 {
			fail("tail=" + tail + "，必须是 0–10000 的整数；有效 since 游标允许 all")
			return
		}
	}
	if typ == "container-logs" || typ == "container-exec" {
		if !repository.ContainerIDPattern.MatchString(name) {
			fail("非法容器标识符: " + name)
			return
		}
		if readOnly {
			args := []string{"logs", "-f", "--timestamps", "--tail", tail}
			if since != "" {
				args = append(args, "--since", since)
			}
			args = append(args, name)
			cmd, err = h.Repository.ContainerCommand(ctx.Request.Context(), args...)
		} else {
			cs, e := h.Repository.ResourceContainers(ctx.Request.Context())
			if e != nil {
				fail(e.Error())
				return
			}
			found := false
			for _, c := range cs {
				if c.ID == name {
					found = true
					if c.System {
						fail("禁止进入面板自身容器终端: " + name)
						return
					}
				}
			}
			if !found {
				fail("容器不存在: " + name)
				return
			}
			cmd, err = h.Repository.ContainerCommand(ctx.Request.Context(), "exec", "-it", "-e", "TERM=xterm-256color", name, shell)
		}
	} else {
		stack := name
		if typ == "exec" {
			stack = ctx.Query("stack")
			if !serviceNamePattern.MatchString(name) {
				fail("非法服务名: " + name)
				return
			}
		}
		if !service.StackNamePattern.MatchString(stack) {
			fail("非法栈名: " + stack)
			return
		}
		args := []string{}
		if typ == "exec" {
			args = append(args, "exec", "-e", "TERM=xterm-256color", name, shell)
		} else {
			args = append(args, "logs", "-f", "--tail", tail, "--no-color", "--timestamps")
			if since != "" {
				args = append(args, "--since", since)
			}
			if svc := ctx.Query("service"); svc != "" {
				if !serviceNamePattern.MatchString(svc) {
					fail("非法服务名: " + svc)
					return
				}
				args = append(args, svc)
			}
		}
		cmd, err = h.Repository.StackComposeCommand(ctx.Request.Context(), stack, args...)
	}
	if err != nil {
		fail(err.Error())
		return
	}
	h.serveSession(ctx, cmd, !readOnly, cancel)
}

// Input 顺序输入与 resize 使用 JSON POST；只读日志不会创建可写会话。
func (h *TerminalHandler) Input(c *gin.Context) {
	var input struct {
		Type string `json:"type"`
		Data string `json:"data"`
		Rows uint16 `json:"rows"`
		Cols uint16 `json:"cols"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024)
	if err := bindJSON(c, &input); err != nil {
		handleServiceError(c, err)
		return
	}
	value, ok := h.sessions.Load(c.Param("id"))
	if !ok {
		v1.HandleError(c, 404, fmt.Errorf("终端会话 %q 已结束或不存在", c.Param("id")), nil)
		return
	}
	session := value.(*terminalSession)
	claimsValue, _ := c.Get("claims")
	claims, _ := claimsValue.(*jwt.Claims)
	if claims == nil || claims.UserId != session.owner || claims.H != session.hash {
		v1.HandleError(c, 403, fmt.Errorf("当前登录会话不能操作终端 %q", c.Param("id")), nil)
		return
	}
	stopInput := context.AfterFunc(c.Request.Context(), session.close)
	defer stopInput()
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed.Load() {
		v1.HandleError(c, 410, fmt.Errorf("终端会话 %q 已关闭", c.Param("id")), nil)
		return
	}
	var err error
	switch input.Type {
	case "input":
		if len(input.Data) > 16*1024 {
			v1.HandleError(c, 400, fmt.Errorf("data 超过终端单次输入限制 16384 字节"), nil)
			return
		}
		err = session.master.SetWriteDeadline(time.Now().Add(3 * time.Second))
		if err == nil {
			_, err = session.master.Write([]byte(input.Data))
		}
	case "resize":
		if input.Rows < 1 || input.Rows > 500 || input.Cols < 1 || input.Cols > 500 {
			v1.HandleError(c, 400, fmt.Errorf("rows=%d cols=%d，窗口尺寸必须为 1–500", input.Rows, input.Cols), nil)
			return
		}
		err = resizeTerminal(session.master, input.Rows, input.Cols)
	default:
		v1.HandleError(c, 400, fmt.Errorf("未知终端控制类型 %q（input/resize）", input.Type), nil)
		return
	}
	if err != nil {
		session.close()
		v1.HandleError(c, 500, fmt.Errorf("终端 %s %s 失败: %w", c.Param("id"), input.Type, err), nil)
		return
	}
	v1.HandleSuccess(c, nil)
}
func (h *TerminalHandler) serveSession(c *gin.Context, cmd *exec.Cmd, usePTY bool, cancel context.CancelFunc) {
	s := newSSE(c)
	ctx := c.Request.Context()
	repository.ConfigureStreamingCommand(cmd, usePTY)
	var reader *os.File
	var session *terminalSession
	var id string
	if usePTY {
		master, err := pty.Start(cmd)
		if err != nil {
			s.finish(fmt.Errorf("无法打开终端: %w", err))
			return
		}
		reader = master
		session = &terminalSession{master: master, cancel: cancel}
		if value, ok := c.Get("claims"); ok {
			claims := value.(*jwt.Claims)
			session.owner, session.hash = claims.UserId, claims.H
		}
		bytes := make([]byte, 24)
		if _, err := rand.Read(bytes); err != nil {
			_ = master.Close()
			cancel()
			_ = cmd.Wait()
			s.finish(err)
			return
		}
		id = hex.EncodeToString(bytes)
		if err := resizeTerminal(master, 24, 80); err != nil {
			session.close()
			_ = cmd.Wait()
			s.finish(fmt.Errorf("初始化终端窗口与非阻塞 I/O 失败: %w", err))
			return
		}
		h.sessions.Store(id, session)
	} else {
		pr, pw, err := os.Pipe()
		if err != nil {
			s.finish(err)
			return
		}
		cmd.Stdout, cmd.Stderr = pw, pw
		if err := cmd.Start(); err != nil {
			_ = pr.Close()
			_ = pw.Close()
			s.finish(err)
			return
		}
		_ = pw.Close()
		reader = pr
	}

	// Wait 与读输出并行：直接进程退出不等于后代已释放 stdout/PTY。
	waitDone := make(chan struct{})
	var waitErr error
	go func() { waitErr = cmd.Wait(); close(waitDone) }()
	var readerDone chan struct{}
	closeDone := make(chan struct{})
	closeReader := context.AfterFunc(ctx, func() {
		defer close(closeDone)
		if session != nil {
			session.close()
		} else {
			_ = reader.Close()
		}
	})
	defer func() {
		if session != nil {
			session.close()
		}
		h.sessions.Delete(id)
		cancel()
		// Wait 已返回时 CommandContext 不再监听取消，仍需回收持有输出端的后代。
		_ = cmd.Cancel()
		_ = reader.Close()
		if !closeReader() {
			<-closeDone
		}
		if readerDone != nil {
			<-readerDone
		}
		<-waitDone
	}()
	if err := s.event("ready", id); err != nil {
		return
	}
	chunks := make(chan []byte, 1)
	var readErr error
	isLog := c.Param("type") == "container-logs" || c.Param("type") == "compose-logs"
	readerDone = make(chan struct{})
	go func() {
		defer close(readerDone)
		defer close(chunks)
		if isLog {
			scanner := bufio.NewScanner(reader)
			scanner.Buffer(make([]byte, 8192), 1024*1024)
			for scanner.Scan() {
				select {
				case chunks <- []byte(scanner.Text() + "\n"):
				case <-ctx.Done():
					return
				}
			}
			readErr = scanner.Err()
			return
		}
		buf := make([]byte, 8192)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				data := append([]byte(nil), buf[:n]...)
				select {
				case chunks <- data:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) && !(usePTY && errors.Is(err, syscall.EIO)) {
					readErr = err
				}
				return
			}
		}
	}()
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	waitSignal := (<-chan struct{})(waitDone)
	var drainTimer *time.Timer
	var drainDeadline <-chan time.Time
	defer func() {
		if drainTimer != nil {
			drainTimer.Stop()
		}
	}()
	for {
		if chunks == nil && waitSignal == nil {
			if ctx.Err() != nil {
				return
			}
			err := errors.Join(waitErr, readErr)
			if err != nil {
				s.finish(fmt.Errorf("终端命令 %s 结束: %w", cmd.Path, err))
			} else {
				s.finish(nil)
			}
			return
		}
		select {
		case <-waitSignal:
			waitSignal = nil
			if chunks != nil {
				drainTimer = time.NewTimer(2 * time.Second)
				drainDeadline = drainTimer.C
			}
		case <-drainDeadline:
			_ = cmd.Cancel()
			_ = reader.Close()
			s.finish(fmt.Errorf("终端命令 %s 已退出，但输出管道在 2 秒内未关闭: %w", cmd.Path, exec.ErrWaitDelay))
			return
		case <-ctx.Done():
			return
		case <-tick.C:
			if s.event("ping", true) != nil {
				return
			}
		case data, ok := <-chunks:
			if !ok {
				chunks = nil
				if readErr != nil {
					// 本地读取失败不等于客户端取消；先收尸，随后发送具体 error 帧。
					_ = cmd.Cancel()
				}
				continue
			}
			// []byte 在 JSON 中编码为 base64，不会破坏跨块 UTF-8 或 ANSI 字节。
			event, payload := "bytes", any(data)
			if isLog {
				event, payload = "log", parseLogRecord(string(data))
			}
			if s.event(event, payload) != nil {
				return
			}
		}
	}
}
