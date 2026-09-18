# Dockge Monorepo


Dockge Monorepo 是 [louislam/dockge](https://github.com/louislam/dockge) 的 Go 语言复刻版：一个"面向 docker compose 栈"的可视化管理器。它以 [nunu-monorepo](https://github.com/go-nunu/nunu) 的应用骨架为底座实现——Gin + bbolt + samber/do 的后端分层，SolidJS + Vite 的前端，`go:embed` 打包为单个二进制。

> 项目结构说明：`app/dockge` 是本项目唯一的应用（栈管理器）；模板中的 `app/admin`、`app/home`、`deploy/`、`pkg/sid`、`pkg/server/grpc` 等与 Dockge 无关的示例模块已删除，依赖树随之精简。

## 复刻范围

| 原版 Dockge 功能 | 本复刻 | 说明 |
| --- | --- | --- |
| compose 栈列表（含外部栈） | ✅ | 扫描 stacks 目录 + `docker compose ls --all` 合并 |
| 栈状态（running/exited/created/未部署） | ✅ | 状态码语义与原版一致（0-4，`StackStatusFromString`） |
| 创建/编辑 compose.yaml 与 .env | ✅ | YAML 语法校验、栈名规则与原版相同（`^[a-z0-9_-]+$`） |
| 启动 / 停止 / 重启 / Down / 更新（pull+up） | ✅ | 与原版相同的 docker compose 命令行（常规操作 3min 超时、update 10min） |
| 删除栈（down + 删目录） | ✅ | |
| 栈内容器列表（compose ps） | ✅ | |
| 组合日志 | ✅ 增强 | 终端面板 xterm 实时流（compose-logs WebSocket） |
| 操作输出展示（up/down 的 compose 输出） | ✅ | |
| 容器总览（docker/podman ps -a） | ✅ 增强 | REST 加载清单 + SSE 更新状态 + 过滤 + 分页（每页 15）+ 批量删除 |
| 镜像管理（列表/删除/拉取/清理未使用） | ✅ 增强 | 原版无此能力；size/created 为结构化字段（字节数 / unix 秒） |
| 网络管理（列表/创建/删除/清理） | ✅ 增强 | 原版无此能力；列表含 driver |
| 数据卷管理（列表/删除/清理未使用） | ✅ 增强 | 原版无此能力 |
| 容器 exec 终端 | ✅ 增强 | 容器行一键进入容器内 shell（bash→sh 探测回退，终端面板多标签） |
| 仪表盘（docker 版本 + 栈/容器统计 + 系统 CPU/内存） | ✅ 增强 | procfs 统计走 SSE 2 秒帧实时推送（数据源 `/proc` 仅 Linux 提供；其他平台显示「实时统计不可用」并保持重试） |
| 一键清理（prune） | ✅ 增强 | 已停止容器 / 未使用镜像 / 网络 / 卷 |
| 登录认证 | ✅ 增强 | JWT + bcrypt + shake256 密码绑定 + 按 IP+账号限流（10 次/分钟）；另有反代头认证、OIDC SSO 与多用户管理 admin/member（原版无；2FA/TOTP 已于 2026-09-12 评估为冗余移除，决策 Q7） |
| 交互式终端（PTY，xterm.js） | ✅ 增强 | 栈日志 / 容器 exec 两类；**出于安全不提供宿主 shell** |
| YAML 编辑器 | ✅ | CodeMirror 6：YAML 语法高亮、行号、当前行、undo、Tab 缩进 + 朴素 `services:` 校验（2026-09-12 引入，关闭 DESIGN.md 债务） |
| composerize（docker run → compose） | ✅ 增强 | 原生 Go 解析器（shell 风格分词 + 完整 flag 表），非正则 stub |
| 实时状态 | ✅ | 容器清单走 REST；后端监听 `docker events`，通过 SSE 推送容器 id/state/status 与全量镜像列表（500ms 防抖 + 30s 兜底，任一资源采集失败整帧不推） |

## 技术栈

| 层 | 选型 |
| --- | --- |
| 后端 | Go 1.26、Gin、bbolt、Viper、Zerolog、samber/do |
| 编排引擎 | Podman v6 bindings（`go.podman.io/podman/v6`，优先）+ docker CLI（降级），栈编排走 `docker compose` CLI |
| 认证 | 四模式 `security.auth.mode`：`jwt`（默认，HS256 + bcrypt + shake256 密码绑定）/ `proxy`（受信反代头 + CIDR 白名单 + 自动开户）/ `oidc`（多 Provider，Auth Code + PKCE）/ `disable` |
| 存储 | bbolt（账号/设置，bucket: users/settings）+ 文件系统（栈目录） |
| 前端 | SolidJS + @solidjs/router + Kobalte + xterm.js 6 + lucide-solid + CodeMirror 6 + Vite + TypeScript，zh-CN/en-US 双语（各 315 键，类型强制对齐）、明暗主题（无 FOUC），go:embed 内嵌 |
| 构建平台 | **Linux**（`pkg/pty` 依赖 `TIOCGPTN`/`/dev/pts` 等 Linux 专有 ioctl；macOS / Windows 构建不通过） |

## 与原版 Dockge 的架构对照

| 原版（Node.js/TS） | 本复刻（Go） |
| --- | --- |
| Express + Socket.IO（`backend/dockge-server.ts`） | Gin REST 路由 + 专用 SSE（`app/dockge/internal/server/http.go` + `api/v1/container_status.go`） |
| `Stack` 类（`backend/stack.ts`，583 行） | `repository/stack.go`（文件系统）+ `repository/compose.go`（compose CLI）+ `service/stack.go`（用例） |
| knex/redbean-node + SQLite（用户/设置表） | bbolt + JSON 序列化（`users` / `settings` bucket） |
| node-pty 终端 + xterm.js | `pkg/pty/pty.go`（手写 `/dev/ptmx` + ioctl，仅 Linux）+ `handler/terminal.go`（WebSocket）+ `components/Terminal.tsx` |
| Vue 3 + Vite + Bootstrap（`frontend/`） | SolidJS + Vite + 手写 CSS 设计系统（`app/dockge/web/`，见 `DESIGN.md`） |
| 栈状态：`docker compose ls` Status 字符串解析 | 完全一致的解析逻辑（`StackStatusFromString`） |

## 快速开始

依赖：**Linux**、Go ≥ 1.26、Node ≥ 20、pnpm ≥ 8、docker 守护进程。
（`pkg/pty` 使用 Linux 专有终端 ioctl，在 macOS 上 `go build` 直接失败；跨平台开发请在 Linux 虚拟机/容器内进行。）

```bash
# 1. 初始化数据库（破坏性重建，种子账号 admin/123456）并创建 stacks 目录
make migrate

# 2. 构建前端（tsc 类型检查 + vite，产物经 go:embed 内嵌）
make web-build

# 3. 启动
make run                 # 等价于 go run ./app/dockge/cmd/server -conf config/dockge/local.yml
```

一键构建单二进制：`make build`（产物 `bin/dockge-server`，版本号由 `git describe` 经 `-ldflags` 注入 `internal/version`）。

打开 <http://127.0.0.1:5001>，使用 `admin / 123456` 登录。

- 栈根目录：`storage/stacks/`（可在 `config/dockge/*.yml` 的 `dockge.stacks_dir` 修改，指向已有的 compose 目录即可纳管）
- 每个子目录一个栈：`compose.yaml`（或 `compose.yml` / `docker-compose.y(a)ml`）+ 可选 `.env`
- `docker compose ls` 中已注册但不在 stacks 目录的项目会作为"外部栈"展示，可执行生命周期操作但不可编辑文件

## 认证模式

`config/dockge/*.yml` 的 `security.auth.mode` 四选一：

| 模式 | 行为 |
| --- | --- |
| `jwt`（默认） | 本地用户名密码 → JWT（HS256，7 天 TTL，绑定密码哈希摘要，改密即失效）；登录限流 10 次/分钟（IP+账号键） |
| `proxy` | 受信反代（Authelia/traefik forwardAuth 等）注入身份头；仅当来源 IP 命中 `trusted_proxies` CIDR 时信任（未配置即拒绝，fail-closed），可自动开户，会话为 httpOnly cookie |
| `oidc` | 多 Provider，Authorization Code + PKCE；回调验签 ID Token 后映射/自动开户本地用户，`admin_groups` 命中授予 admin 角色 |
| `disable` | 免登录模式（`POST /v1/auto-login` 以首个活跃用户建立会话；开启需校验当前密码） |

任何模式下本地密码登录保留为兜底通道。token 传递顺序：`Authorization: Bearer` → `?token=`（SSE/WebSocket）→ `dockge_token` cookie（proxy/oidc 模式）。

OIDC 与 Traefik 反向代理的完整部署步骤（IdP 配置、forwardAuth、docker-compose、故障排查）见 [docs/AUTH-DEPLOY.md](docs/AUTH-DEPLOY.md)；认证闭环交互原型见 [docs/prototype/auth-flow.html](docs/prototype/auth-flow.html)。

## API 概览

公开端点（无需认证；未完成首启设置时另有 403 门禁，白名单仅 `/v1/setup*`、`/v1/health`、`/v1/robots.txt`、`/v1/auto-login`）：

```text
POST /v1/login                       登录
POST /v1/setup                       首次安装引导（已有用户时 409）
GET  /v1/setup/need                  是否需要初始化
POST /v1/auto-login                  免登录模式自动登录（disableAuth 开启时有效）
GET  /v1/auth/config                 认证配置探测 {mode, providers, disableAuth}
GET  /v1/oidc/providers              OIDC Provider 列表
GET  /v1/oidc/:provider/auth         302 跳转 IdP 授权页（state + PKCE）
GET  /v1/oidc/:provider/callback     OIDC 回调，种 httpOnly 会话 cookie 后 302 回首页
GET  /v1/health                      健康检查
GET  /v1/robots.txt                  爬虫协议
```

认证端点均需 `Authorization: Bearer <token>`（或 `?token=` / cookie）：

```text
GET  /v1/me                          当前用户
PUT  /v1/me/password                 修改密码
GET/POST /v1/me/disableauth          免登录开关（开启需当前密码；前端入口暂缓）

GET  /v1/users                       用户列表（admin；含角色/状态/来源）
POST /v1/users                       创建账号（admin；默认 member）
PUT  /v1/users/:id/role              变更角色（admin）
PUT  /v1/users/:id/active            启用/停用（admin；停用即时失效其全部会话）
DELETE /v1/users/:id                 删除账号（admin）

GET  /v1/stacks                      栈列表（含外部栈与状态）
POST /v1/stacks                      创建栈 {name, yaml, env}
GET  /v1/stacks/:name                栈详情（文件内容 + 容器 + 状态）
PUT  /v1/stacks/:name                保存文件
DELETE /v1/stacks/:name              删除栈（down + 删目录）
POST /v1/stacks/:name/:op            生命周期操作（start/stop/restart/down/update），返回 compose 输出

GET  /v1/docker/version              引擎版本摘要
GET  /v1/docker/info                 仪表盘汇总
GET  /v1/docker/containers           容器总览（含 compose 项目 label、结构化 ports）
GET  /v1/docker/containers/stream    容器状态 SSE（id/state/status + 全量镜像列表，15s 心跳）
GET  /v1/docker/containers/:id/inspect              容器详情
GET  /v1/docker/containers/:id/logs                 容器日志 SSE
POST /v1/docker/containers/:id/stop|start|restart   容器生命周期
DELETE /v1/docker/containers/:id     删除容器（-f）
POST /v1/docker/containers/prune     清理已停止容器
GET  /v1/docker/images               镜像列表（sizeBytes/createdAt 结构化字段）
DELETE /v1/docker/images/:id         删除镜像
POST /v1/docker/images/pull          拉取镜像 {reference}（同步阻塞）
POST /v1/docker/images/prune         清理未使用镜像
GET  /v1/docker/networks             网络列表（含 driver）
GET  /v1/docker/networks/:name       网络详情
POST /v1/docker/networks/create      创建网络 {name, driver, subnet}
DELETE /v1/docker/networks/:name     删除网络
POST /v1/docker/networks/prune       清理未使用网络
GET  /v1/docker/volumes              数据卷列表
DELETE /v1/docker/volumes/:name      删除数据卷
POST /v1/docker/volumes/prune        清理未使用数据卷
GET  /v1/docker/stats/stream         系统 CPU/内存 SSE 实时推送（2s 帧）
GET  /v1/docker/df                   磁盘占用（sizeBytes/reclaimableBytes 结构化字段）

GET/PUT /v1/settings/globalenv       全局环境变量
POST /v1/composerize                 docker run → compose（原生 Go 解析器）
GET  /v1/version/check               版本更新检查（对比上游 louislam/dockge releases，5min 缓存）
GET  /v1/terminal/:name/:type        终端 WebSocket（type=exec 容器 ID / compose-logs 栈名；不提供宿主 shell）
```

## 模块总结

完整模块分析与测试映射见 [docs/PROJECT_SPEC.md](docs/PROJECT_SPEC.md)（规格总入口）。

### 核心模块

| 模块 | 路径 | 职责 |
| --- | --- | --- |
| **Auth** | `handler/auth.go` + `service/auth.go` + `repository/user_bbolt.go` + `internal/security/` | 认证模式枚举、JWT 登录、改密、限流 |
| **Proxy 认证** | `internal/authproxy/` + `middleware/web.go`（ProxyAuth） | 受信 CIDR 校验、身份头换取本地会话 |
| **OIDC** | `internal/authoidc/` + `handler/oidc.go` | 多 Provider、Discovery、PKCE、token 验签、角色映射 |
| **Stack** | `handler/stack.go` + `service/stack.go` + `repository/{stack,compose}.go` | 栈 CRUD、生命周期操作（start/stop/restart/down/update） |
| **Docker 资源** | `handler/docker.go` + `service/docker.go` + `repository/{container,image,network,volume,stats,system,podman,cli}.go` | Podman v6 优先 + docker CLI 兜底：容器/镜像/网络/卷的列表、操作、清理与统计 |
| **Terminal** | `handler/terminal.go` + `pkg/pty` + `web/src/components/Terminal.tsx` | WebSocket 终端（容器 exec / 栈 compose-logs），PTY + xterm.js 全链路；出于安全不提供宿主 shell |
| **Composerize** | `internal/composerize/` | 原生 Go 的 docker run → compose 转换器 |
| **Middleware** | `internal/middleware/` | StrictAuth（Bearer/?token=/cookie）、ProxyAuth、CORS、SetupRequired、请求日志 |
| **API 契约** | `api/v1/` | DTO、`{code,message,data}` 响应封套、业务哨兵错误 |
| **Frontend** | `app/dockge/web/` | SolidJS SPA（11 个视图），go:embed 内嵌 |

### 共享基础设施（pkg/）

| 包 | 职责 |
| --- | --- |
| `pkg/app` | 应用生命周期容器（Run/Stop/Signal 处理） |
| `pkg/config` | Viper 配置加载 |
| `pkg/jwt` | JWT 签发/校验 |
| `pkg/hash` | shake256 摘要 |
| `pkg/rate` | 滑动窗口限流（键容量上限） |
| `pkg/pty` | 伪终端（`/dev/ptmx` + ioctl，仅 Linux） |
| `pkg/log` | Zerolog + Lumberjack 轮转日志 |
| `pkg/server` | HTTP Server 抽象（Start/Stop） |

### 当前状态

功能完成度、能力规格清单（含测试映射）与剩余工作以 [docs/PROJECT_SPEC.md](docs/PROJECT_SPEC.md) 为准；逐条需求与整改项追溯见 [docs/REQUIREMENTS.md](docs/REQUIREMENTS.md)。

## 项目目录

```text
.
├── app/
│   └── dockge/              # ★ 本项目唯一应用：Dockge 复刻
│       ├── api/v1/          # 请求/响应 DTO 与业务错误
│       ├── cmd/             # server / migration / reset-password 入口
│       ├── internal/        # handler → service → repository（podman/docker CLI + bbolt + 文件系统）
│       │   └── security / authproxy / authoidc / composerize / version / testutil
│       └── web/             # SolidJS 前端，go:embed 内嵌
├── config/dockge/           # local / prod 配置（security.auth.* 认证模式）
├── docs/                    # PROJECT_SPEC.md（规格总入口）/ REQUIREMENTS.md（需求事实来源）
├── pkg/                     # 共享基础设施（app/config/hash/jwt/log/pty/rate/server）
├── storage/                 # dockge.db（bbolt）与 stacks 栈目录
├── openspec/                # 规格流水线：specs/（当前有效规格）+ changes/（增量变更）
└── Makefile                 # bootstrap / run / migrate / web-build / build / test / verify
```

## 单机边界与安全提示

- 每个 Dockge 实例只管理部署它的主机上的 Docker/Podman 运行时，不保存或代理其他主机的凭据与操作；跨主机运维应在每台目标主机独立部署 Dockge，并通过 VPN、反向代理或零信任访问层远程访问。
- 生产部署请修改 `config/dockge/prod.yml` 的 `security.jwt.key`；
- 栈管理接口等同于授予宿主机 docker 权限，请勿将服务暴露到不可信网络；
- 迁移种子账号为弱口令 `admin/123456`，首启后请立即改密。

## 文档索引

| 文档 | 职责 |
| --- | --- |
| [docs/PROJECT_SPEC.md](docs/PROJECT_SPEC.md) | 规格总入口：As-Is 基线 + To-Be + 测试/工作流纪律 |
| [docs/REQUIREMENTS.md](docs/REQUIREMENTS.md) | 需求事实来源：R1–R4 逐条需求、问题/整改清单、决策记录 |
| [docs/VISION.md](docs/VISION.md) | 愿景导航：目标图景、差距地图与优先级路线（附 UI 原型 [docs/prototype/vision.html](docs/prototype/vision.html)） |
| [DESIGN.md](DESIGN.md) | Crate 设计系统：视觉与交互契约 |
| [openspec/specs/](openspec/specs/) | 经归档的当前有效能力规格 |
| [openspec/changes/](openspec/changes/) | 增量变更流水（含归档） |

## License

Apache 2.0（继承自 nunu-monorepo 模板；原版 Dockge 为 MIT）。
