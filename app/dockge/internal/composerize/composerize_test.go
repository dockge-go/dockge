package composerize

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestConvert 用表驱动覆盖 docker run 的常见形态。
func TestConvert(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		want    []string // 期望包含的语义行
		notWant []string // 期望不包含的语义行
		wantErr bool
	}{
		{
			name: "简单运行",
			cmd:  "docker run -d nginx:latest",
			want: []string{"services:", "  app:", "    image: nginx:latest"},
		},
		{
			name: "带 compose 前缀",
			cmd:  "docker compose run app redis:7",
			want: []string{"    image: redis:7"},
		},
		{
			name: "多端口不重复键",
			cmd:  "docker run -p 8080:80 -p 443:443 nginx",
			want: []string{
				"    ports:",
				"      - \"8080:80\"",
				"      - \"443:443\"",
			},
			notWant: []string{"image: \"nginx\""},
		},
		{
			name: "多卷与多环境变量",
			cmd:  `docker run -v /host/data:/data -v "cfg:/etc/cfg" -e A=1 -e "B=hello world" busybox`,
			want: []string{
				"    volumes:",
				"      - \"/host/data:/data\"",
				"      - \"cfg:/etc/cfg\"",
				"    environment:",
				"      - \"A=1\"",
				"      - \"B=hello world\"",
			},
		},
		{
			name: "等号形式选项",
			cmd:  "docker run --env=KEY=value --name=web --restart=always nginx",
			want: []string{
				"    container_name: web",
				"    restart: always",
				"      - \"KEY=value\"",
			},
		},
		{
			name: "常见运行选项",
			cmd:  "docker run -d --name db -w /work -u 1000 --network net1 -m 512m --cpus 1.5 postgres:16",
			want: []string{
				"    image: postgres:16",
				"    container_name: db",
				"    networks:",
				"      - net1",
				"    working_dir: /work",
				"    user: \"1000\"",
				"    mem_limit: 512m",
				"    cpus: \"1.5\"",
			},
		},
		{
			name: "镜像后命令",
			cmd:  "docker run alpine echo hello world",
			want: []string{
				"    image: alpine",
				"    command:",
				"      - echo",
				"      - hello",
				"      - world",
			},
		},
		{
			name: "带空格的参数被引号保护",
			cmd:  `docker run --entrypoint "/bin/sh -c" -l "app=name x=y" myimage`,
			want: []string{
				"    entrypoint: \"/bin/sh -c\"",
				"      - \"app=name x=y\"",
			},
		},
		{
			name: "布尔选项不吞值",
			cmd:  "docker run -d --privileged -p 5432:5432 postgres",
			want: []string{"      - \"5432:5432\"", "    image: postgres"},
		},
		{
			name:    "无镜像报错",
			cmd:     "docker run -d --name only-flags",
			wantErr: true,
		},
		{
			name:    "非 run 命令报错",
			cmd:     "docker ps -a",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Convert(tt.cmd)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Convert(%q) 期望报错，实际得到:\n%s", tt.cmd, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Convert(%q) 意外报错: %v", tt.cmd, err)
			}
			checkYAMLSemantic(t, got, tt.want)
			// notWant 检查：确保不期望的语义不存在
			for _, line := range tt.notWant {
				if strings.Contains(got, line) {
					t.Errorf("输出不应包含 %q，实际:\n%s", line, got)
				}
			}
		})
	}
}

// TestRenderYAMLValid 用 yaml 包验证多值场景产出合法 YAML。
func TestRenderYAMLValid(t *testing.T) {
	out, err := Convert("docker run -p 80:80 -p 443:443 -v a:b -v c:d -e X=1 -e Y=2 --name multi nginx")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("产出非法 YAML: %v\n%s", err, out)
	}
	services, _ := doc["services"].(map[string]any)
	app, _ := services["app"].(map[string]any)
	if app == nil {
		t.Fatalf("缺少 services.app:\n%s", out)
	}
	if ports, _ := app["ports"].([]any); len(ports) != 2 {
		t.Errorf("ports 应有 2 项，实际 %d:\n%s", len(ports), out)
	}
}
