package repository

import (
	"bytes"
	"reflect"
	"testing"

	"dockge/internal/model"
)

// TestStackStatusFromString 校验 compose ls 状态串到枚举的映射（与 Dockge 的 statusConvert 一致）。
func TestStackStatusFromString(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   model.StackStatus
	}{
		{"empty", "", model.StatusUnknown},
		{"running", "running(3)", model.StatusRunning},
		{"mixed exits", "exited(1), running(2)", model.StatusExited},
		{"all exited", "exited(6)", model.StatusExited},
		{"created", "created", model.StatusCreated},
		{"garbage", "what-is-this", model.StatusUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StackStatusFromString(tt.status); got != tt.want {
				t.Errorf("StackStatusFromString(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

// TestStackOpPhases 校验栈操作到 compose 参数序列的映射（操作语义基线）：
// start=up -d、update=pull+up、stop/restart/down 原样；托管栈仅 -p 前缀。
func TestStackOpPhases(t *testing.T) {
	tests := []struct {
		op      string
		want    [][]string
		wantErr bool
	}{
		{"start", [][]string{{"-p", "demo", "up", "-d", "--remove-orphans"}}, false},
		{"stop", [][]string{{"-p", "demo", "stop"}}, false},
		{"restart", [][]string{{"-p", "demo", "restart"}}, false},
		{"down", [][]string{{"-p", "demo", "down", "--remove-orphans"}}, false},
		{"update", [][]string{
			{"-p", "demo", "pull"},
			{"-p", "demo", "up", "-d", "--remove-orphans"},
		}, false},
		{"pause", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			got, err := stackOpPhases("demo", nil, tt.op)
			if (err != nil) != tt.wantErr {
				t.Fatalf("stackOpPhases(%q) error = %v, wantErr %v", tt.op, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("stackOpPhases(%q) = %v, want %v", tt.op, got, tt.want)
			}
		})
	}

	// 外部栈：附带 -f 指向反查到的配置文件
	got, err := stackOpPhases("ext", []string{"/opt/x/compose.yaml"}, "stop")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"-p", "ext", "-f", "/opt/x/compose.yaml", "stop"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("external stack phases = %v, want %v", got, want)
	}
}

// TestProgressFilter 校验拉取进度 tick 的折叠：同层同动作原地覆盖，其余行原样通过。
func TestProgressFilter(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{
			name:  "普通行直接通过",
			lines: []string{"[+] Running 1/1", " Container nginx Started"},
			want:  "[+] Running 1/1\n Container nginx Started\n",
		},
		{
			name: "同层进度 tick 覆盖为单行",
			lines: []string{
				"3672748066c3 Downloading [=>  ] 1.2MB/111.8MB",
				"3672748066c3 Downloading [==> ] 2.4MB/111.8MB",
				"3672748066c3 Downloading [===>] 23.07MB/111.8MB",
			},
			want: "3672748066c3 Downloading [=>  ] 1.2MB/111.8MB" +
				"\r\x1b[2K3672748066c3 Downloading [==> ] 2.4MB/111.8MB" +
				"\r\x1b[2K3672748066c3 Downloading [===>] 23.07MB/111.8MB\n",
		},
		{
			name: "同层不同动作各占一行",
			lines: []string{
				"3672748066c3 Downloading [===>] 23.07MB/111.8MB",
				"3672748066c3 Extracting  1.1MB/3.4MB",
				"3672748066c3 Pull complete",
			},
			want: "3672748066c3 Downloading [===>] 23.07MB/111.8MB\n" +
				"3672748066c3 Extracting  1.1MB/3.4MB\n" +
				"3672748066c3 Pull complete\n",
		},
		{
			name: "不同层交替刷新各自独立",
			lines: []string{
				"aaaabbbbcccc Downloading [>] 1MB/10MB",
				"dddddddddddd Downloading [>] 2MB/20MB",
				"aaaabbbbcccc Downloading [>] 5MB/10MB",
			},
			want: "aaaabbbbcccc Downloading [>] 1MB/10MB\n" +
				"dddddddddddd Downloading [>] 2MB/20MB\n" +
				"aaaabbbbcccc Downloading [>] 5MB/10MB\n",
		},
		{
			name: "进度行后普通行闭合悬空行",
			lines: []string{
				"3672748066c3 Downloading [===>] 23.07MB/111.8MB",
				"3672748066c3 Downloading [====] 40MB/111.8MB",
				"nginx Pulled",
			},
			want: "3672748066c3 Downloading [===>] 23.07MB/111.8MB" +
				"\r\x1b[2K3672748066c3 Downloading [====] 40MB/111.8MB" +
				"\nnginx Pulled\n",
		},
		{
			name: "docker 直连格式（ID 后带冒号）同样折叠",
			lines: []string{
				"24c03149e0d9: Downloading [===> ] 1.2MB/45MB",
				"24c03149e0d9: Downloading [====> ] 23MB/45MB",
				"24c03149e0d9: Download complete",
			},
			want: "24c03149e0d9: Downloading [===> ] 1.2MB/45MB" +
				"\r\x1b[2K24c03149e0d9: Downloading [====> ] 23MB/45MB" +
				"\n24c03149e0d9: Download complete\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			p := &progressFilter{w: &buf}
			for _, line := range tt.lines {
				if err := p.line(line); err != nil {
					t.Fatalf("line(%q): %v", line, err)
				}
			}
			if err := p.flush(); err != nil {
				t.Fatalf("flush: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("output mismatch\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}
