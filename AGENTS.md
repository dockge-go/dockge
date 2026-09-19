# AGENTS.md — AI 代理工作指引

Dockge：[louislam/dockge](https://github.com/louislam/dockge) 的**一比一复刻**（Go 重写）。**单机版**：管理本机 Docker Compose 栈。

## 硬约束

- **技术栈**：Gin + bbolt + samber/do 后端；SolidJS + Vite 前端；go:embed 单二进制。不引入其他框架。
- **单机（硬性）**：不做多主机/多实例/Agent（上游「Dockge 代理 beta」联机方案已移除）；全部栈操作经容器 CLI 子进程完成。
- **运行时无关（硬性）**：不得硬编码 `docker` —— 一律经 `repository.Runtime`（`container.cli` 配置或 PATH 自动探测 docker→podman→nerdctl；`container.compose` 支持独立的 `podman-compose` 等命令）。OCI 低层运行时（youki/crun 等）经 podman/containerd 选用，本层无需感知。
- **编辑体验重中之重**：web 编辑 compose.yaml 的体验（编辑器、校验闭环、保存/部署反馈）是最高优先级，任何改动不得使其退化。
- **一比一基准 = 上游 master**：任何 UI/功能改动前 MUST 先查看上游对应实现（`frontend/src` 源码）再动手；完成后 MUST 与上游逐项复核（布局/交互/文案/视觉）。时时对比、持续对齐；发现偏差先修齐再继续。与上游一致是默认态：**多的不要，少的也不要**。有意偏差仅限：双语（zh/en）、自定义主题（不引 Bootstrap）、无宿主终端（/console 已移除；容器 exec 与栈合并日志保留）、部署自检（面板内展示运行时/栈目录状态，上游无）、无应用内更新检查（版本随镜像更新，上游的 check-version 与顶栏提示已移除；关于页仅保留版本号与前后端不一致告警）。
- **代码精简**：无冗余代码、无冗余功能。上游没有的功能不实现；复刻不再使用的代码与端点删除，不保留「以防万一」。

## 目录结构（nunu 风格，仓库根）

```
api/v1         API 契约（DTO / 错误码）
cmd/           入口（main.go 服务 + reset-password 子命令）
config/        环境配置（local / prod / docker）
deploy/        部署产物（Dockerfile）
internal/      handler → service → repository 分层 + middleware/model/server/version
pkg/           可复用基础包（app/config/jwt/log/rate/hash/server）
scripts/       运维脚本（smoke.sh 发布前冒烟）
web/           SolidJS 前端（构建产物 dist 由 go:embed 内嵌）
```

## 工程约定（最小集）

- 分层单向：handler → service → repository；哨兵错误集中在 `api/v1` 与 repository 层。
- 导出符号写中文注释（说「为什么」）；前端禁 `any`/`@ts-ignore`；i18n 文案进 `web/src/i18n/{zh-CN,en-US}.ts`（措辞对齐上游 locale）。
- 测试就近、表驱动；完成改动后跑 `make verify`。

## 常用命令

```bash
make web-build       # 前端 tsc + vite 构建
make run             # 启动（127.0.0.1:5001）
make build           # 单二进制 bin/dockge-server
make test            # go test + 前端测试
make verify          # build + test
make smoke           # 发布前冒烟：对运行中实例 + 真实容器运行时跑全链路接口检查
make reset-password  # 交互式重置指定用户密码（唯一入口 cmd/main.go 的子命令）

# 容器镜像（三阶段构建，仅含 docker CLI + compose 插件 + 二进制）
docker build -f deploy/Dockerfile -t dockge-go --build-arg VERSION=$(git describe --tags --always) .
# 受限网络（proxy.golang.org 不可达）时追加：--build-arg GOPROXY=https://goproxy.cn,direct
```
