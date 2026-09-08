# Dockge Monorepo


Dockge Monorepo 是 [louislam/dockge](https://github.com/louislam/dockge) 的 Go 语言复刻版：一个"面向 docker compose 栈"的可视化管理器。它以 [nunu-monorepo](https://github.com/go-nunu/nunu) 的应用骨架为底座实现——Gin + bbolt + samber/do 的后端分层，VanJS + Vite 的零 CSS 前端，`go:embed` 打包为单个二进制。

> 项目结构说明：`app/dockge` 是本项目唯一的应用（栈管理器）；模板中的 `app/admin`、`app/home`、`deploy/`、`pkg/sid`、`pkg/server/grpc` 等与 Dockge 无关的示例模块已删除，依赖树随之精简（go.mod 直接依赖 30+ → 11）。

## 复刻范围

| 原版 Dockge 功能 | 本复刻 | 说明 |
| --- | --- | --- |
| compose 栈列表（含外部栈） | ✅ | 扫描 stacks 目录 + `docker compose ls --all` 合并 |
| 栈状态（running/exited/created/未部署） | ✅ | 状态码语义与原版一致（0-4） |
| 创建/编辑 compose.yaml 与 .env | ✅ | YAML 语法校验、栈名规则与原版相同（`^[a-z0-9_-]+$`） |
| 启动 / 停止 / 重启 / Down / 更新（pull+up） | ✅ | 与原版相同的 docker compose 命令行 |
| 删除栈（down + 删目录） | ✅ | |
| 栈内容器列表（compose ps） | ✅ | |
| 组合日志 | ✅ 增强 | 终端面板 xterm 实时流（compose-logs WebSocket） |
| 操作输出展示（up/down 的 compose 输出） | ✅ | |
| 容器总览（docker/podman ps -a） | ✅ 增强 | REST 加载清单 + SSE 更新状态 + 过滤 + 分页（每页 15）+ 批量删除 |
| 镜像管理（列表/删除/拉取/清理未使用） | ✅ 增强 | 原版无此能力 |
| 网络管理（列表/删除/清理未使用） | ✅ 增强 | 原版无此能力 |
| 数据卷管理（列表/删除/清理未使用） | ✅ 增强 | 原版无此能力 |
| 容器 exec 终端 | ✅ 增强 | 容器行一键进入容器内 shell（终端面板多标签） |
| 仪表盘（docker 版本 + 栈/容器统计 + 系统 CPU/内存） | ✅ 增强 | procfs 统计走 SSE 2 秒帧实时推送 |
| 一键清理（prune） | ✅ 增强 | 已停止容器 / 未使用镜像 / 网络 / 卷 |
| 登录认证 | ✅ | JWT + bcrypt + shake256 密码绑定 + 登录限流（2FA/免登录后端就绪，入口按需求暂缓） |
| 交互式终端（PTY，xterm.js） | ✅ 增强 | 宿主 shell / 栈日志 / 容器 exec，多标签终端面板 |
| YAML 编辑器 | ✅ 增强 | CodeMirror 6：高亮、自动缩进、格式化（js-yaml）、保存前自动排版 |
| composerize（docker run → compose） | ✅ | 新建栈页内一键转换并填入编辑器 |
| 实时状态 | ✅ | 容器清单走 REST；后端监听 `docker events`，仅通过 SSE 推送容器 id/state/status（500ms 防抖 + 30s 兜底） |

## 技术栈

| 层 | 选型 |
| --- | --- |
| 后端 | Go 1.26、Gin、bbolt、Viper、Zerolog、samber/do |
| 编排引擎 | Podman v6 bindings（`go.podman.io/podman/v6`，优先）+ docker CLI（降级），栈编排走 `docker compose` CLI |
| 认证 | JWT（HS256）+ bcrypt + shake256 密码绑定 |
| 存储 | bbolt（账号/设置，bucket: users/settings）+ 文件系统（栈目录） |
| 前端 | VanJS + VanUI + Vite + TypeScript（无 CSS 文件，全内联样式），go:embed 内嵌 |

## 与原版 Dockge 的架构对照

| 原版（Node.js/TS） | 本复刻（Go） |
| --- | --- |
| Express + Socket.IO（`backend/dockge-server.ts`） | Gin REST 路由 + 专用 SSE（`app/dockge/internal/server/http.go` + `container_status.go`） |
| `Stack` 类（`backend/stack.ts`，583 行） | `repository/stack.go`（文件系统）+ `repository/docker.go`（compose CLI）+ `service/stack.go`（用例） |
| knex/redbean-node + SQLite（用户/设置表） | bbolt + JSON 序列化（`users` / `settings` bucket） |
| node-pty 终端 + xterm.js | `pkg/pty/pty.go`（PTY 机制）+ `handler/terminal.go`（WebSocket）+ `components/Terminal.ts`（npm 版 xterm.js） |
| Vue 3 + Vite + Bootstrap（`frontend/`） | VanJS + Vite + 内联样式（`app/dockge/web/`） |
| 栈状态：`docker compose ls` Status 字符串解析 | 完全一致的解析逻辑（`StackStatusFromString`） |

## 快速开始

依赖：Go ≥ 1.26、Node ≥ 20、pnpm ≥ 8、docker 守护进程。

```bash
# 1. 初始化数据库（破坏性重建，种子账号 admin/123456）并创建 stacks 目录
make migrate

# 2. 构建前端（Go 二进制通过 go:embed 内嵌 dist）
make web-build

# 3. 启动
make run                 # 等价于 go run ./app/dockge/cmd/server -conf config/dockge/local.yml
```

一键构建单二进制：`make build`（产物 `bin/dockge-server`）。

打开 <http://127.0.0.1:5001>，使用 `admin / 123456` 登录。

- 栈根目录：`storage/stacks/`（可在 `config/dockge/*.yml` 的 `dockge.stacks_dir` 修改，指向已有的 compose 目录即可纳管）
- 每个子目录一个栈：`compose.yaml`（或 `compose.yml` / `docker-compose.y(a)ml`）+ 可选 `.env`
- `docker compose ls` 中已注册但不在 stacks 目录的项目会作为"外部栈"展示，可执行生命周期操作但不可编辑文件

## API 概览

认证后接口均需 `Authorization: Bearer <token>`。

```text
POST /v1/login                       登录
GET  /v1/me                          当前用户
PUT  /v1/me/password                 修改密码
GET  /v1/stacks                      栈列表（含外部栈与状态）
POST /v1/stacks                      创建栈 {name, yaml, env}
GET  /v1/stacks/:name                栈详情（文件内容 + 容器 + 状态）
PUT  /v1/stacks/:name                保存文件
DELETE /v1/stacks/:name              删除栈（down + 删目录）
POST /v1/stacks/:name/start|stop|restart|down|update   生命周期操作，返回 compose 输出
GET  /v1/stacks/:name/logs?tail=200  栈日志
GET  /v1/stacks/:name/logs/stream    栈日志 SSE 实时流
GET  /v1/docker/containers/stream    容器状态 SSE（仅 id/state/status）
GET  /v1/docker/version              引擎版本摘要
GET  /v1/docker/containers           容器总览（含 compose 项目 label）
GET  /v1/docker/info                 仪表盘汇总
GET  /v1/docker/stats                系统 CPU/内存采样
GET  /v1/docker/stats/stream         系统 CPU/内存 SSE 实时推送（2s 帧）
GET  /v1/docker/images               镜像列表
DELETE /v1/docker/images/:id         删除镜像
POST /v1/docker/images/pull          拉取镜像 {reference}
POST /v1/docker/images/prune         清理未使用镜像
GET  /v1/docker/networks             网络列表
GET  /v1/docker/networks/:name       网络详情
DELETE /v1/docker/networks/:name     删除网络
POST /v1/docker/networks/prune       清理未使用网络
GET  /v1/docker/volumes              数据卷列表
DELETE /v1/docker/volumes/:name      删除数据卷
POST /v1/docker/volumes/prune        清理未使用数据卷
POST /v1/docker/containers/:id/stop|start|restart   容器生命周期
DELETE /v1/docker/containers/:id     删除容器（-f）
POST /v1/docker/containers/prune     清理已停止容器
GET  /v1/terminal/:name/:type        终端 WebSocket（host / compose-logs / exec）
GET/PUT /v1/settings/globalenv       全局环境变量
POST /v1/composerize                 docker run → compose
GET  /v1/version/check               版本更新检查
GET  /v1/setup/need | POST /v1/setup 首次安装引导
GET  /v1/me/2fa/enable | DELETE /v1/me/2fa         两步验证（前端入口暂缓）
GET/POST /v1/me/disableauth          免登录模式（前端入口暂缓）
```

## 模块总结

完整模块分析见 [ARCHITECTURE.md](ARCHITECTURE.md)（Mermaid 逻辑图）与 [openspec/project.md](openspec/project.md)（功能规格）。

### 核心模块

| 模块 | 路径 | 职责 |
| --- | --- | --- |
| **Auth** | `app/dockge/internal/handler/auth.go` + `service/auth.go` + `repository/user_bbolt.go` | JWT 登录、用户信息管理、密码修改 |
| **Stack** | `app/dockge/internal/handler/stack.go` + `service/stack.go` + `repository/stack.go` | 栈 CRUD、生命周期操作（start/stop/restart/down/update）、日志查询 |
| **Docker 资源** | `handler/docker.go` + `service/docker.go` + `repository/{container,image,network,volume,stats,system,podman}.go` | Podman v6（`go.podman.io/podman/v6`）优先 + docker CLI 兜底：容器/镜像/网络/卷的列表、操作、清理与统计 |
| **Terminal** | `handler/terminal.go` + `pkg/pty` + `web/src/components/Terminal.ts` | WebSocket 终端（宿主 shell / 栈日志 / 容器 exec），PTY + xterm.js 全链路 |
| **Middleware** | `app/dockge/internal/middleware/` | JWT 鉴权、CORS、请求日志 |
| **API 契约** | `app/dockge/api/v1/` | DTO、响应包裹、业务哨兵错误 |
| **Frontend** | `app/dockge/web/` | VanJS SPA，go:embed 内嵌 |

### 共享基础设施（pkg/）

| 包 | 职责 |
| --- | --- |
| `pkg/app` | 应用生命周期容器（Run/Stop/Signal 处理） |
| `pkg/config` | Viper 配置加载 |
| `pkg/jwt` | JWT 签发/校验 |
| `pkg/log` | Zerolog + Lumberjack 轮转日志 |
| `pkg/server` | HTTP Server 抽象（Start/Stop） |

### 当前状态

功能完成度与剩余工作以 [TODO.md](TODO.md) 为准；与原版 Dockge 的逐项对照见 [DOCKGE_COMPARISON.md](DOCKGE_COMPARISON.md)。

> 详见 [openspec/README.md](openspec/README.md)

## 项目目录

```text
.
├── app/
│   └── dockge/              # ★ 本项目唯一应用：Dockge 复刻
│       ├── api/v1/          # 请求/响应 DTO 与业务错误
│       ├── cmd/             # server / migration 入口
│       ├── internal/        # handler → service → repository（docker/podman CLI + bbolt + 文件系统）
│       └── web/             # VanJS 前端，go:embed 内嵌
├── config/dockge/           # local / prod 配置
├── pkg/                     # 共享基础设施（app/config/jwt/log/server/http）
├── storage/                 # dockge.db（bbolt）与 stacks 栈目录
├── openspec/                # 原版 Dockge 需求基线（69 条需求 / 126 个场景）
└── Makefile                 # bootstrap / run / migrate / web-build / build / test
```

## 单机边界与安全提示

- 每个 Dockge 实例只管理部署它的主机上的 Docker/Podman 运行时，不保存或代理其他主机的凭据与操作；跨主机运维应在每台目标主机独立部署 Dockge，并通过 VPN、反向代理或零信任访问层远程访问。
- 生产部署请修改 `config/dockge/prod.yml` 的 `security.jwt.key`；
- 栈管理接口等同于授予宿主机 docker 权限，请勿将服务暴露到不可信网络。

## License

Apache 2.0（继承自 nunu-monorepo 模板；原版 Dockge 为 MIT）。
