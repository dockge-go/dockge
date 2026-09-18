# Dockge 需求文档

| 项 | 值 |
| --- | --- |
| 版本 | v1.2（2026-09-12 对齐代码基线；v2 落地文档已并入本文） |
| 日期 | 2026-09-12 |
| 来源 | 需求人口述（4 条），整理人结合全量代码巡视扩写 |
| 代码基线 | `main @ 3885e0d` + 未提交清理改动（死代码/死防御/冗余透传清理、游离注释修复、过度导出收敛、Get 重复 CLI 调用修复；净 -3,802 行含删除的原型 HTML） |
| 语言约定 | 正文 zh-CN；需求条目保留英文 SHALL / MUST / SHOULD 关键字（与 OpenSpec 约定一致） |

> 本文档是需求的事实来源（Source of Truth）。口述原文逐条收录于各需求章首；需求条目描述**目标**，「落地状态」小节描述**代码现状**，两者不混淆。原 `docs/requirements-v2.md`（认证子系统重构落地记录）已并入 R1/R2 章并删除。

---

## 目录

1. [产品定位与总体目标](#1-产品定位与总体目标)
2. [现状巡视结论（代码基线）](#2-现状巡视结论代码基线)
3. [R1 注册登录：首次启动强制设置密码](#3-r1-注册登录首次启动强制设置密码)
4. [R2 反向代理中间件免登录方案（Traefik / Authelia / OIDC）](#4-r2-反向代理中间件免登录方案traefik--authelia--oidc)
5. [R3 后端接口全面整理（对齐前端原型）](#5-r3-后端接口全面整理对齐前端原型)
6. [R4 前端界面需求（原风格 · Unix 工具向 · Apple System）](#6-r4-前端界面需求原风格--unix-工具向--apple-system)
7. [工程规范：Go Style（强制）](#7-工程规范go-style强制)
8. [需求追溯矩阵与决策记录](#8-需求追溯矩阵与决策记录)

---

## 1. 产品定位与总体目标

Dockge（Go 复刻版）是**单机容器编排控制台**：以 docker compose 栈为中心，覆盖栈的创建/编辑/部署/日志/终端，以及容器、镜像、网络、卷、主机资源的日常运维。部署形态为**单二进制**（go:embed 内嵌前端），Podman bindings 优先、docker CLI 降级的双引擎策略。

总体目标（继承自口述，全局约束所有需求）：

- **G-1 单机边界**：不做多 Agent / 远程主机管理（OpenSpec `remove-remote-agent` 已实施）。
- **G-2 前后端契约唯一**：前端（`app/dockge/web/src`）即接口契约基准；后端 MUST 与之完全对齐（见 R3）。
- **G-3 视觉基调不变**：界面基于现有原风格（`DESIGN.md` 的 Apple 式 token 体系）演进，强化「Unix 工具向 + Apple System」气质（见 R4）。
- **G-4 代码风格**：所有 Go 代码 MUST 符合 Go 官方与 Uber Go Style Guide（见第 7 章）。

---

## 2. 现状巡视结论（代码基线）

### 2.1 架构与技术栈

| 层 | 现状（2026-09-12 核实） |
| --- | --- |
| 后端骨架 | Go 1.26 + Gin + samber/do（DI）+ bbolt（`users`/`settings` bucket）+ Viper + zerolog + lumberjack |
| 分层 | `cmd/{server,reset-password,migration}` → `internal/handler` → `internal/service` → `internal/repository`（栈文件系统 + compose CLI + podman bindings + docker CLI + bbolt）；另有 `internal/{security,authproxy,authoidc,composerize,version,testutil}` |
| 编排引擎 | `go.podman.io/podman/v6` bindings 优先，docker CLI 降级；栈编排统一走 `docker compose` CLI（常规操作 3 分钟超时 / update 10 分钟） |
| 认证 | 四模式 `security.auth.mode`：`jwt`（默认）/ `proxy`（受信 CIDR + 身份头 + 自动开户）/ `oidc`（多 Provider，Auth Code + PKCE，httpOnly cookie 会话）/ `disable`（auto-login）；JWT HS256 7 天 TTL，claims 绑定 `shake256(passwordHash)`；登录限流 **10 次/分钟（IP+账号键滑动窗口，容量 4096）** |
| 实时通道 | SSE ×3：容器状态流（docker events 驱动，500ms 防抖 + 30s 兜底，15s 心跳 + 末帧回放）、主机 stats 流（2s 帧）、容器日志流；WebSocket ×1：终端（容器 exec / 栈 compose-logs，PTY 仅 Linux） |
| 前端 | SolidJS 1.9 + @solidjs/router + Kobalte + xterm.js 6 + lucide-solid；手写 CSS 设计系统（`DESIGN.md`）；zh-CN/en-US 双语各 315 键（`Record<MsgKey,string>` 类型强制对齐）；明暗主题无 FOUC；Vite 8；构建产物 go:embed |
| 版本 | `internal/version.Version`（默认 `dev`，`make build` 经 `-ldflags` 注入 `git describe`） |
| 规模 | Go 非测试 59 文件 / 7,072 行 + 测试 10 文件 / 26 用例；前端 src ~4,337 行 + 测试 3 文件 / 7 用例；go.mod 直接依赖 14（Q7 移除 pquerna/otp）；前端 CodeMirror 6（qrcode 已随 Q7 卸载） |
| 平台限制 | `pkg/pty` 使用 Linux 专有 ioctl（`TIOCGPTN`/`TIOCSPTLCK`）与 `/dev/pts`，**macOS/Windows 构建不通过**；`make test` 中的 `go test ./...` 与服务器构建须在 Linux 执行 |

### 2.2 已实现能力地图

首启 Setup 门禁（SetupRequired 中间件 + `/setup` 引导页）、栈全生命周期（扫描 + 外部栈合并、CRUD、start/stop/restart/down/update、状态码 0-4 对齐上游）、容器总览（REST + SSE 增量状态 + 客户端分页 15/页 + 批量清理）、镜像/网络/卷管理（结构化 sizeBytes/createdAt/ports 字段）、容器 exec 终端与栈聚合日志（WebSocket）、主机 CPU/内存实时流、磁盘用量与分类 prune、composerize（原生 Go 解析器）、全局环境变量（bbolt）、版本检查（对比上游 louislam/dockge releases，5min 缓存）、四模式认证（含 OIDC SSO 登录页与反代头认证）、i18n 双语、明暗主题。

### 2.3 问题清单（P 编号；状态以 2026-09-12 代码为准）

| 编号 | 问题 | 位置 | 状态 |
| --- | --- | --- | --- |
| P1 | 前端 LogStream 栈模式引用的 `GET /v1/stacks/:name/logs/stream` 后端无此路由，栈日志仅由 WS 终端（compose-logs）承载 | `internal/server/http.go` | **已关闭**（2026-09-12 决策 Q4 维持 WS 终端；前端孤儿分支与后端死链已删，LogStream 收敛为容器日志单一用途） |
| P2 | `POST /v1/auto-login` 注册在公开路由组；`disableAuth` 开启后任何能触达端口者获得首个活跃用户会话 | `internal/server/http.go:95`、`service/auth.go AutoLogin` | **仍在**（仅 disableAuth=true 时生效，默认关闭；R1-7 建议最终下线） |
| P3 | CORS 中间件回显任意 `Origin` 且 `Allow-Credentials: true` | `internal/middleware/web.go:37-38` | **未修**（R2 原计划随 P0 收敛，未执行） |
| P4 | WebSocket `CheckOrigin` 全放行（注释自述依赖 JWT 兜底） | `internal/handler/terminal.go:21` | **未修** |
| P5 | composerize 曾为正则 stub，多端口/卷/-env 产出非法 YAML | `internal/handler/composerize.go` | ✅ **已修**（`internal/composerize` 原生分词解析 + 表驱动测试） |
| P6 | `GET /v1/docker/networks` 曾仅返回名字数组 | `api/v1/dockge.go DockerNetworkData` | ✅ **已修**（列表含 `driver`） |
| P7 | 2FA 后端就绪但前端无入口 | — | ⛔ **已移除**（2026-09-12 决策 Q7：2FA/TOTP 整体移除，功能冗余；端点/服务/模型字段/pkg-totp/前端步骤全部删除） |
| P8 | 版本号曾硬编码 `"v1.0.0"` | `internal/handler/docker.go` | ✅ **已修**（`internal/version` + ldflags 注入） |
| P9 | 登录限流曾为全局桶 | `service/auth.go:31-33` | ✅ **已修**（IP+账号键 10/min） |
| P10 | 死代码/未接线（ContainerExecStart、文件版 global env、SettingsCleaner、seedJWTSecret、httpx.Package 等） | 多处 | ✅ **已修**（已全部移除；工作区清理又移除 AuthService 的 settings 透传与 IsAdmin） |
| P11 | 配置键不一致（local 用 `data.db.dsn`，代码读 `data.db.user.dsn`） | `config/dockge/local.yml` | ✅ **已修**（统一 `data.db.user.dsn`） |
| P12 | 前端 i18n 模块与 formatBytes/timeAgo 等声明未消费 | `web/src` | ✅ **已修**（i18n 260 键全量接入；`api/format.ts` 收敛为实际消费的 shortId/errText/stateLabel/statusClass） |
| P13 | 镜像拉取、prune 为同步阻塞请求，无进度流 | `handler/docker.go` | **未改**（F9 保留，体验级） |
| P14 | 终端 `host` 类型注释与代码不符 | `handler/terminal.go:54-56` | ✅ **已修**（类型收敛为 `exec`/`compose-logs`，未知类型 400） |
| P15 | `PUT /v1/stacks/:name` 请求体冗余携带 `name` | `api/v1/dockge.go`、`web/src/api/api.ts:220` | **未变**（容忍项） |

> 补充发现（2026-09-12）：`AuthService.CheckSession`（逐请求校验用户存在/启用/哈希未变）已实现但**未接线**到 StrictAuth——被停用用户的既有 token 在 7 天 TTL 内仍有效，待 F11 一并接线。

---

## 3. R1 注册登录：首次启动强制设置密码

### 3.1 口述原文

> 要求注册登录。第一次启动时可以强制设置密码，如果设置密码后才能继续访问。

### 3.2 需求条目

**R1-1 首启引导（强制）**
系统首次启动（用户存储为空）时 SHALL 进入初始化状态：任何未认证的业务访问 MUST 被拦截；初始化引导页 SHALL 强制设置管理员用户名与密码；密码强度 MUST 满足现有规则（≥6 位且含字母与数字，前后端双重校验）。

**R1-2 引导完成即门禁生效**
初始化完成后，系统 SHALL 立即进入受保护状态；`POST /v1/setup` 在已有用户时 MUST 返回 409 且不再可用。

**R1-3 会话与凭证**
登录成功 SHALL 签发 JWT（HS256，TTL 7 天，绑定密码哈希 shake256 摘要——改密后旧 token 全部失效）。REST 用 `Authorization: Bearer`，SSE/WebSocket 因浏览器限制 MUST 同时接受 `?token=` 查询参数；proxy/oidc 模式另接受 `dockge_token` httpOnly cookie。

**R1-4 账号模型（决策 Q1：支持多用户）**
系统 SHALL 支持多用户账号：首用户为 admin；后续账号由 admin 创建，不开放自助注册；角色两档 admin/member；admin 不可删除自己或最后一名 admin；被停用用户的既有 token MUST 立即失效。

**R1-5 2FA（可选增强）**
TOTP 两步验证 SHALL 在前端补齐入口：登录流程遇 `tokenRequired: true` 时进入验证码步骤，设置页提供启用（二维码）与停用。

**R1-6 防爆破**
登录限流 SHOULD 按 IP + 账号双维度限流，失败响应保持统一 401 文案，不泄露用户是否存在。

**R1-7 免登录开关的处置**
外部认证模式落地后 SHALL 废弃公开的 auto-login 端点；「免登录」诉求由受信反代 / OIDC 模式承接，本地密码登录 MUST 保留为兜底。

### 3.3 验收场景

- 首次启动：清空 bbolt → `GET /v1/stacks` → 403（需初始化）；`POST /v1/setup` 成功返回 accessToken；再次 `POST /v1/setup` → 409。
- 密码门禁：错误/过期 token → 401；改密后旧 token → 401。
- 2FA：启用 2FA 的账号登录 → `tokenRequired`；提交验证码 → 正式 token；同一验证码重放 → 401。

### 3.4 落地状态（2026-09-12）

| 条目 | 状态 | 证据 |
| --- | --- | --- |
| R1-1 / R1-2 | ✅ | `middleware/web.go SetupRequired`：无用户时白名单仅 `/v1/setup*`、`/v1/health`、`/v1/robots.txt`、`POST /v1/auto-login`，其余 403；`views/Setup.tsx` 引导页 |
| R1-3 | ✅ | `pkg/jwt`（shake256 绑定）；StrictAuth 依次读 Bearer → `?token=` → cookie |
| R1-4 | 🟡 部分 | `model.DockgeUser` 已有 Role（admin/member，空值兼容旧数据）/ Active / Source（local/proxy/oidc）字段；`CheckSession` 已实现（校验存在+启用+哈希未变）但**未接线** StrictAuth；用户管理端点与 UI 未实现（F11） |
| R1-5 | ❌ 前端 | 后端 `POST /v1/2fa`、`POST /v1/me/2fa/enable`、`DELETE /v1/me/2fa` 就绪（含防重放）；前端登录遇 `tokenRequired` 抛 `toast.2faNotSupported` |
| R1-6 | ✅ | `pkg/rate.NewKeyed(10, time.Minute, 4096)`，IP+账号键 |
| R1-7 | 🟡 部分 | auto-login 仍在公开组（P2），仅 disableAuth=true 生效；proxy/oidc 模式已可承接免登录诉求 |

---

## 4. R2 反向代理中间件免登录方案（Traefik / Authelia / OIDC）

### 4.1 口述原文

> 如果使用中间件拦截实现免登录，比如 traefik、authelia 等等，你出个方案。要求支持 oidc 等鉴权。

### 4.2 设计原则（P-1 ~ P-4）

本地会话不变（外部身份只参与换取本地 JWT 的瞬间）/ 本地密码永远兜底 / 显式开启默认关闭（fail-closed）/ 外部身份映射到本地用户（自动开户为无密码账号）。

### 4.3 落地方案（已实施）

认证模式由 `security.auth.mode` 统一控制，取值 `jwt`（默认）| `proxy` | `oidc` | `disable`，非法值回退 `jwt`（`internal/security/security.go`）。

#### 模式 A：`proxy` —— 受信反向代理头（已实施，形态为中间件）

请求链路：`浏览器 → Traefik(forwardAuth→Authelia) → 注入身份头 → Dockge`。

- `ProxyAuth` 中间件仅当 `mode=proxy` 时挂载到受保护路由组（先于 StrictAuth）：读取 `security.auth.proxy.username_header`（默认 `X-Forwarded-User`）→ `authService.ProxyLogin` 校验来源 IP 命中 `trusted_proxies` CIDR → 映射或按 `auto_provision` 自动开户 → 签发本地 JWT 写入 httpOnly `dockge_token` cookie 并放行。
- **安全模型（fail-closed）**：`trusted_proxies` 为空时拒绝一切代理认证；非法 CIDR 静默忽略；伪造来源（非受信 IP 携带身份头）一律 401（`authproxy.TrustedIP`，有测试覆盖）。
- 与原方案的偏差：未实现独立的 `GET /v1/auth/proxy` 端点——前端无需探测，受保护 API 请求本身即可无感通过。

#### 模式 B：`oidc` —— Dockge 作为 OIDC Relying Party（已实施）

配置命名空间 `security.auth.oidc.providers.<id>.*`（label/issuer/client_id/client_secret/scopes/username_claim/groups_claim/admin_groups），支持多 Provider。

流程：`GET /v1/oidc/:provider/auth`（生成 state + PKCE S256 verifier，`oidc_state` httpOnly cookie 绑定浏览器，302 到 IdP）→ `GET /v1/oidc/:provider/callback`（校验 code/state，换 token 并验签 ID Token，提取 username + admin 标志）→ `OIDCLogin` 映射或自动开户（OIDC 始终 provision，`admin_groups` 命中授予 admin）→ 清 state cookie、写 `dockge_token` cookie、302 回 `/`。

- `authoidc.NewManager` 仅当 `mode=oidc` 时执行 Discovery 网络请求，其他模式返回空管理器（避免启动时无谓外呼）。
- 前端：`GET /v1/auth/config`（公开）返回 `{mode, providers, disableAuth}`，`Login.tsx` 据此渲染 SSO 按钮；`api.ts` 请求自动 `credentials: 'include'`。
- 与原方案的偏差（**Q2 决策修订**，见 §8.2）：会话交付采用 httpOnly cookie（同 proxy 模式），未实现 `POST /v1/auth/oidc/exchange` 一次性 code 换 JWT。

#### 模式 C：ForwardAuth 端点 —— **未实现**

原方案的 `GET /v1/auth/forwardauth`（让 Dockge 自身会话充当 Traefik 认证器）未实施；如需可后续增量补齐。

#### 模式 D：`disable` —— 免登录（已实施）

`GET/POST /v1/me/disableauth` 读写开关（开启需校验当前密码）；`POST /v1/auto-login` 仅当 `disableAuth=true` 时以首个活跃用户建立会话，否则 401。前端入口暂缓（P2 风险因此可控：默认关闭）。

### 4.4 配置 schema（实际生效，`config/dockge/*.yml`）

```yaml
security:
  jwt:
    key: change-me-for-production
  auth:
    mode: jwt                       # jwt | proxy | oidc | disable
    proxy:
      trusted_proxies: []           # 受信 CIDR 列表；为空即拒绝一切代理认证（fail-closed）
      username_header: X-Forwarded-User
      auto_provision: true
    oidc:
      providers:                    # 多 Provider，key 为内部标识
        enterprise:
          label: "企业 SSO"
          issuer: "https://sso.example.com"
          client_id: ""
          client_secret: ""
          scopes: ["openid", "profile"]
          username_claim: email     # 默认 preferred_username，回退 email
          groups_claim: groups
          admin_groups: ["admin", "ops"]
```

### 4.5 与 R1 的关系

首启强制设密在任何模式下依然生效（本地管理员密码是逃生通道）。R1-7 的 auto-login 下线尚未执行（P2）。

### 4.6 落地状态（2026-09-12）

| 项 | 状态 | 证据 |
| --- | --- | --- |
| 模式 A（proxy 中间件） | ✅ | `middleware/web.go ProxyAuth`、`internal/authproxy/`（4 测试）、`service/auth.go ProxyLogin/externalLogin` |
| 模式 B（OIDC + PKCE） | ✅ | `internal/authoidc/`（manager/provider/state，4 测试）、`handler/oidc.go`、`Login.tsx:53-137` SSO 按钮；回调流端到端无测试 |
| 模式 C（forwardauth） | ❌ 未实现 | — |
| 模式 D（disable + auto-login） | ✅ 后端 | `service/auth.go AutoLogin/ToggleDisableAuth`；前端入口暂缓 |
| P3/P4 安全收敛（CORS 白名单、WS CheckOrigin） | ❌ 未修 | 见 §2.3 |

---

## 5. R3 后端接口全面整理（对齐前端原型）

### 5.1 口述原文

> 全面的整理后端接口和需求。要求完全对齐前端的原型。

### 5.2 对齐原则

- **A-1** 前端为契约基准；后端 MUST 逐端点对齐路径、方法、请求字段、响应字段与流语义。
- **A-2** 统一响应包络 `{code, message, data}`，`code=0` 成功；错误哨兵沿用 `api/v1`，`handleServiceError` 映射 HTTP 状态。
- **A-3** 后端就绪但前端未接的端点不算「对齐」。
- **A-4** 预格式化字符串契约迁移为结构化数据（决策 Q3）。

### 5.3 接口总表（= `internal/server/http.go` 实际注册，2026-09-12）

状态：✅ 已对齐闭环 · 🟠 后端就绪前端无 UI · ❌ 缺失

#### 公开端点（noAuthRouter）

| 方法与路径 | 说明 | 状态 |
| --- | --- | --- |
| `POST /v1/login` | 登录 | ✅（2FA 分支已随 Q7 移除） |
| `POST /v1/setup` / `GET /v1/setup/need` | 首启引导 | ✅ |
| `POST /v1/auto-login` | 免登录自动登录（仅 disableAuth） | 🟠 后端就绪，前端无入口 |
| `GET /v1/auth/config` | 认证配置探测 | ✅（Login.tsx） |
| `GET /v1/oidc/providers` | Provider 列表 | ✅ |
| `GET /v1/oidc/:provider/auth` / `callback` | OIDC 授权与回调 | ✅ |
| `GET /v1/health` / `GET /v1/robots.txt` | 健康/爬虫 | ✅ |

#### 认证与账号（strictAuthRouter）

| 方法与路径 | 说明 | 状态 |
| --- | --- | --- |
| `GET /v1/me` / `PUT /v1/me/password` | 当前用户 / 改密 | ✅ |
| `GET/POST /v1/me/disableauth` | 免登录开关 | 🟠 前端无 UI |

#### 编排栈

| 方法与路径 | 说明 | 状态 |
| --- | --- | --- |
| `GET /v1/stacks` | 列表（含外部栈、状态 0-4） | ✅ |
| `POST /v1/stacks` / `PUT /v1/stacks/:name` | 创建/保存（body 冗余 `name`，容忍 P15） | ✅ |
| `GET /v1/stacks/:name` | 详情（文件 + 容器 + 状态） | ✅ |
| `DELETE /v1/stacks/:name` | 删除（down + 删目录） | ✅ |
| `POST /v1/stacks/:name/:op` | start/stop/restart/down/update → `{output}` | ✅ |
| `GET/POST /v1/users`、`PUT /v1/users/:id/role\|active`、`DELETE /v1/users/:id` | 用户管理 5 端点（admin；2026-09-12 F11 落地，含 CheckSession 接线关闭 D12） | ✅ |
| `GET /v1/stacks/:name/logs/stream` | 栈日志 SSE | ⛔ **决策不实施**（Q4：栈日志由 `/v1/terminal/:stack/compose-logs` WS 承载） |

#### 容器 / 镜像 / 网络 / 卷 / 系统

| 方法与路径 | 说明 | 状态 |
| --- | --- | --- |
| `GET /v1/docker/version` / `info` | 引擎版本 / 仪表盘汇总 | ✅ |
| `GET /v1/docker/containers` | 容器总览（`ports` 为 `[]PortMapping` 结构化数组） | ✅ |
| `GET /v1/docker/containers/stream` | 状态 SSE（仅 id/state/status） | ✅ |
| `GET /v1/docker/containers/:id/inspect` / `logs` | 详情 / 日志 SSE | ✅ |
| `POST /v1/docker/containers/:id/{start,stop,restart}` / `DELETE /v1/docker/containers/:id` | 容器生命周期 | ✅ |
| `POST /v1/docker/containers/prune` | 清理已停止容器 | ✅ |
| `GET /v1/docker/images` | 镜像列表（`sizeBytes`/`createdAt` unix 秒） | ✅ |
| `DELETE /v1/docker/images/:id` / `POST /v1/docker/images/pull` / `prune` | 镜像操作（pull 同步阻塞，P13） | ✅ |
| `GET /v1/docker/networks`（含 driver）/ `:name` / `create` / `DELETE :name` / `prune` | 网络管理 | ✅ |
| `GET /v1/docker/volumes` / `DELETE /v1/volumes/:name` / `prune` | 卷管理 | ✅ |
| `GET /v1/docker/stats/stream` | 主机 CPU/内存 SSE（2s 帧） | ✅ |
| `GET /v1/docker/df` | 磁盘占用（`sizeBytes`/`reclaimableBytes`） | ✅ |

#### 设置 / 工具 / 终端

| 方法与路径 | 说明 | 状态 |
| --- | --- | --- |
| `GET/PUT /v1/settings/globalenv` | 全局环境变量 | ✅ |
| `POST /v1/composerize` | docker run → compose（原生解析器） | ✅ |
| `GET /v1/version/check` | 对比上游 releases（5min 缓存） | ✅ |
| `GET /v1/terminal/:name/:type` | WS 终端：`exec`（容器 ID）/ `compose-logs`（栈名）；上行 JSON `{"type":"resize",...}`；**不提供宿主 shell** | ✅ |

### 5.4 整改清单（F 编号）状态

| 编号 | 整改项 | 状态（2026-09-12） |
| --- | --- | --- |
| F1 | ~~补齐栈日志 SSE `GET /v1/stacks/:name/logs/stream`~~ | ✅ **已关闭**（2026-09-12 决策维持 WS 终端；前后端死链与孤儿分支已清除） |
| F2 | composerize 重写为可靠解析 | ✅ 已修（`internal/composerize` + 测试） |
| F3 | networks 返回 driver | ✅ 已修 |
| F4 | ~~前端补 2FA 登录步骤与设置页管理~~ | ⛔ **已移除**（2026-09-12 决策 Q7：2FA 功能冗余，前后端整体移除；同日曾短暂落地后按用户决策撤销） |
| F5 | 版本常量统一（ldflags 注入） | ✅ 已修（`internal/version` + `make build` git describe） |
| F6 | 死代码清理（P10 全项） | ✅ 已修 |
| F7 | 配置键统一（`data.db.user.dsn`） | ✅ 已修 |
| F8 | 契约清债（i18n 接入、format 辅助收敛） | ✅ 已修（260 键全量接入；`format.ts` 仅保留实际消费的导出） |
| F9 | 拉取/prune 流式进度与回收统计 | ❌ 未实施（体验级，可后置） |
| F10 | 不实现宿主 shell，删除 host 分支 | ✅ 已修（类型收敛 `exec`/`compose-logs`） |
| F11 | 多用户管理（5 个 `/v1/users*` 端点 + Settings 用户管理 UI + CheckSession 接线） | ✅ **已修**（2026-09-12：5 端点 + UsersCard + StrictAuth 接线 CheckSession 关闭 D12；守卫：不许操作本人、至少保留一名活跃 admin） |
| F12 | 结构化字段迁移（sizeBytes/createdAt/ports/df） | ✅ 已修（双端同版本切换完成） |

### 5.5 验收场景

- 逐条核对 §5.3：✅ 项在接口测试中有用例（当前 handler/service 层测试为零，见 PROJECT_SPEC §5——这是规格债而非接口债）；❌/🟠 项显式记录于 §5.4。
- 前端 `pnpm build && tsc --noEmit` 通过且运行时无契约不匹配请求。

---

## 6. R4 前端界面需求（原风格 · Unix 工具向 · Apple System）

### 6.1 口述原文

> 前端界面基于原风格，要求 Unix 工具向和 Apple-System。

### 6.2 基线与原则

**原风格** = `DESIGN.md` 的 Crate 设计系统 + OpenSpec `design-mock` 规范固化的交互行为。R4 是在其上强化两种气质，不是重做。

**R4-1 Apple System（继承 + 补课）**

- 视觉语言 MUST 沿用 `DESIGN.md` token，新增颜色 MUST 先入 token 表再入 CSS。
- 明暗双主题（顶栏切换、`dockge.theme` 持久化、首访跟随系统、无 FOUC、控制台岛恒暗）——✅ 已实施。
- zh-CN/en-US 双语（`dockge.lang` 持久化，zh-CN 基语言）——✅ 已实施（315 键，类型强制对齐；2026-09-12 走查清零：视图标题/资源计数/stateLabel 全部 i18n 化）。
- iOS 风格确认对话框 / 开关 / 按压反馈 / 视图淡入 / 轮询不闪动——✅ 已实施（design-mock 系列变更归档）。
- 无障碍底线：WCAG 2.2 AA、键盘可达、`prefers-reduced-motion`、375/768/1280 无横向溢出。

**R4-2 Unix 工具向（强化）**

- **U-1 等宽即数据面**：机器数据 MUST 用 mono + tabular figures；长 ID 可一键复制。
- **U-2 控制台岛为一等公民**：日志/终端/compose 输出深色岛；容器行仅 [日志、终端、启动/停止] 三键；终端类型收敛为容器 exec 与栈 compose-logs（决策 Q5，✅ 已实施）。
- **U-3 命令可预期**：栈操作回显等价命令；输出面板纯文本、可选择、可复制。
- **U-4 键盘优先**：`⌘/Ctrl+K` 全局搜索 MUST 保持（✅ 已有）；列表 `j/k` 导航 SHOULD 逐步补齐（未实施）。
- **U-5 安静**：toast 只反馈主动操作；SSE 状态变化只更新数据（✅ 已实施）。
- **U-6 密度**：紧凑行高、uppercase micro-label、tonal + 1px 边界。

**R4-3 布局模式（沿用）**：主从工作区（列表 320-380px + 详情）、栈三栏工作台、表格页 15/页客户端分页、Sheet 右抽屉、移动端全屏任务页——✅ 已实施（`redesign-resource-workspaces` 变更，任务 13/13 完成）。

### 6.3 落地状态与剩余项

| 项 | 状态 |
| --- | --- |
| 主题/双语/交互细节/主从工作区/分页 | ✅ 已实施（design-mock 系列 + redesign-resource-workspaces 变更） |
| YAML 编辑器 | ✅ CodeMirror 6 + 校验闭环（`POST /stacks/validate` 语法/compose 语义草稿校验，防抖自动触发，错误行号定位；2026-09-19 `add-compose-validation-loop`） |
| `j/k` 列表键盘导航 | ❌ 未实施（SHOULD 级） |
| 零散硬编码文案（如 Settings 标题、stateLabel 英文映射） | 🟡 残留，走查时清理 |

---

## 7. 工程规范：Go Style（强制）

### 7.1 口述原文

> 代码要求符合 go-style。

### 7.2 参照标准（按优先级）

1. Go 官方：Effective Go、Go Code Review Comments、Go Proverbs。
2. Uber Go Style Guide（项目骨架 nunu 的底座约定）。
3. 工作区技能 `golang-microservice-design` 为日常写码/评审的触发规范。

### 7.3 项目级强制约定（与现状代码一致，保持并固化）

- **分层依赖单向**：`handler → service → repository`；接口定义在消费侧（`setupChecker`/`proxyLoginService` 最小接口模式），实现侧 `var _ X = (*x)(nil)` 编译期断言。
- **错误**：哨兵错误集中在 `api/v1` 与 repository 层（`ErrNotFound/ErrConflict`）；底层错误 `%w` 包装向上；handler 统一经 `handleServiceError` 映射。
- **context**：所有 IO 签名 `ctx` 为首参并透传（docker/compose 调用带超时：常规 3min、update 10min）。
- **DI**：samber/do，组合根只出现在 `cmd/`；禁止全局单例与 `init()` 副作用。
- **并发**：goroutine 生命周期随 ctx 取消（`ContainerStatusServer` 模式）；长操作互斥防并发踩踏。
- **注释**：导出符号有中文文档注释，写「为什么」不写「是什么」。
- **测试**：表驱动、就近放置；纯函数（状态机、解析、格式化）必须可测；测试辅助跨包共享走 `internal/testutil`（如 `ViperFromYAML`）。
- **命名/依赖**：包名短小全小写；新增依赖须论证；go.mod direct 当前 15 个。
- **API 契约**：请求/响应结构体集中在 `api/v1`，字段加 binding 标签；handler 内不出现匿名请求结构。

### 7.4 验收

- `gofmt -l` 零输出；`go vet ./...` 通过（Linux 环境）；新增包有包注释。
- 评审 checklist 引用本章；违反分层的 PR 打回。

---

## 8. 需求追溯矩阵与决策记录

### 8.1 追溯矩阵

| 口述条目 | 需求条目 | 关联整改 | 当前状态 |
| --- | --- | --- | --- |
| 1. 注册登录 / 首启强制设密 | R1-1 ~ R1-7 | P2、P7、P9 | 主体 ✅；F11 已落地；F4 已随 Q7 移除 |
| 2. 中间件免登录方案（oidc 等） | R2 模式 A/B/C/D | P2、P3、P4 | A/B/D ✅；C 未实现；P3/P4 未修 |
| 3. 全面整理后端接口，对齐前端 | R3 §5.3 / §5.4 | P1、P5-P12、P15 | F2/F3/F5/F6/F7/F8/F10/F12 ✅；F1/F4/F9/F11 保留 |
| 4. 前端原风格 + Unix 工具向 + Apple System | R4-1 ~ R4-3 | P12、P14 | 主体 ✅；编辑器债务与键盘导航保留 |
| （附加）代码符合 go-style | §7 全章 | — | 持续约束 |

### 8.2 决策记录

| 编号 | 问题 | 决策 | 状态 |
| --- | --- | --- | --- |
| Q1（2026-09-09） | 是否支持多用户？ | 支持：admin/member 两档，设置页管理，不开放自助注册 | 🟡 数据模型就绪；F11 端点与 UI 未实施 |
| Q2（2026-09-09） | OIDC 回调种会话方式 | 一次性 code 换本地 JWT（不引入 Cookie） | **修订（随实施）**：实际落地为 httpOnly `dockge_token` cookie 会话 + 前端 `credentials: 'include'`，未实现 exchange 端点；以本记录为准 |
| Q3（2026-09-09） | 预格式化字符串改结构化字段？ | 改：数值字段 + 前端格式化 | ✅ 已实施（F12） |
| Q4（2026-09-09） | 栈日志通道 | 后端补 SSE 端点 | ❌ 未实施（F1）；当前栈日志由终端 WS 承载，如维持现状需显式关闭该决策 |
| Q5（2026-09-09） | 是否实现宿主 shell？ | 不实现（安全红线） | ✅ 已实施（F10） |
| Q6（2026-09-12 新增） | 平台支持范围 | 代码事实：`pkg/pty` 依赖 Linux 专有 ioctl，服务器构建与 `go test ./...` 仅在 Linux 通过；macOS/Windows 不支持 | 记录在案；如需跨平台须立项移植 PTY 层 |
| Q7（2026-09-12 新增） | 2FA/TOTP 是否保留 | 移除：单机自部署场景下 SSO（OIDC/proxy）+ 密码 + 多用户管理已覆盖认证需求，2FA 属冗余能力；同日曾完整落地（后端 3 端点 + 前端闭环），按用户决策整体撤销 | 前后端全删：`POST /v1/2fa`、`/me/2fa*` 端点、`service` 3 用例、`DockgeUser` 2FA 字段、`pkg/totp`、`pquerna/otp` 与 `qrcode` 依赖、Login 验证码步骤与 SecurityCard 开关；密码强度校验保留（`service/password.go`）；存量 bbolt 中的 2FA JSON 字段反序列化时被安全忽略 |

### 8.3 变更记录

| 版本 | 日期 | 内容 |
| --- | --- | --- |
| v1.0 | 2026-09-09 | 首次整理：口述 4 条 + go-style 全量扩写；沉淀 P1-P15、F1-F10 |
| v1.1 | 2026-09-09 | 决策落定（Q1-Q5）；新增 F11/F12 |
| v1.2 | 2026-09-12 | **对齐代码基线全量修订**：并入 requirements-v2.md（认证落地细节）后删除该文件；P1-P15 与 F1-F12 逐项标注实际状态；接口总表改为 `http.go` 实际注册路由；新增 Q2 修订与 Q6（Linux-only 平台限制）、CheckSession 未接线发现 |
