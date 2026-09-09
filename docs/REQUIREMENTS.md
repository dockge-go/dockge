# Dockge 需求文档

| 项 | 值 |
| --- | --- |
| 版本 | v1.1（Q1-Q5 决策落定） |
| 日期 | 2026-09-09 |
| 来源 | 需求人口述（4 条），整理人结合全量代码巡视扩写 |
| 代码基线 | `main @ 19ab192` + 未提交改动（`pkg/server/` 新增；`terminal.go`/`compose.go` 仅清理未用 import） |
| 语言约定 | 正文 zh-CN；需求条目保留英文 SHALL / MUST / SHOULD 关键字（与 OpenSpec 约定一致） |

> 本文档是需求的事实来源（Source of Truth）。口述原文逐条收录于各需求章首；其后为整理人依据现状代码（backend Go + SolidJS 前端原型）细化的可实施、可验收条目。凡与现状冲突处，均以「现状差距」小节显式列出，不默认已实现。

---

## 目录

1. [产品定位与总体目标](#1-产品定位与总体目标)
2. [现状巡视结论（代码基线）](#2-现状巡视结论代码基线)
3. [R1 注册登录：首次启动强制设置密码](#3-r1-注册登录首次启动强制设置密码)
4. [R2 反向代理中间件免登录方案（Traefik / Authelia / OIDC）](#4-r2-反向代理中间件免登录方案traefik--authelia--oidc)
5. [R3 后端接口全面整理（对齐前端原型）](#5-r3-后端接口全面整理对齐前端原型)
6. [R4 前端界面需求（原风格 · Unix 工具向 · Apple System）](#6-r4-前端界面需求原风格--unix-工具向--apple-system)
7. [工程规范：Go Style（强制）](#7-工程规范go-style强制)
8. [需求追溯矩阵与待决策项](#8-需求追溯矩阵与待决策项)

---

## 1. 产品定位与总体目标

Dockge（Go 复刻版）是**单机容器编排控制台**：以 docker compose 栈为中心，覆盖栈的创建/编辑/部署/日志/终端，以及容器、镜像、网络、卷、主机资源的日常运维。部署形态为**单二进制**（go:embed 内嵌前端），Podman bindings 优先、docker CLI 降级的双引擎策略。

总体目标（继承自口述，全局约束所有需求）：

- **G-1 单机边界**：不做多 Agent / 远程主机管理（见 OpenSpec `remove-remote-agent`）。
- **G-2 前后端契约唯一**：前端原型（`app/dockge/web/src`）即接口契约基准；后端 MUST 与之完全对齐（见 R3）。
- **G-3 视觉基调不变**：界面基于现有原风格（`DESIGN.md` 的 Apple 式 token 体系）演进，强化「Unix 工具向 + Apple System」气质（见 R4）。
- **G-4 代码风格**：所有 Go 代码 MUST 符合 Go 官方与 Uber Go Style Guide（见 R7 章）。

---

## 2. 现状巡视结论（代码基线）

### 2.1 架构与技术栈

| 层 | 现状 |
| --- | --- |
| 后端骨架 | Go 1.26 + Gin + samber/do（DI）+ bbolt（`users`/`settings` bucket）+ Viper + zerolog + lumberjack |
| 分层 | `cmd/{server,reset-password,migration}` → `internal/handler`（13 个）→ `internal/service`（4 个）→ `internal/repository`（栈文件系统 + compose CLI + podman bindings + docker CLI + bbolt） |
| 编排引擎 | `go.podman.io/podman/v6` bindings 优先，docker CLI 降级；栈编排统一走 `docker compose` CLI（3 分钟常规超时 / 10 分钟 update） |
| 认证 | JWT HS256（7 天 TTL）+ bcrypt + shake256 密码绑定 claim（改密即失效全部旧 token）；TOTP 2FA 后端就绪（防重放）；登录限流 20 次/分钟（全局）；`disableAuth` 免登录模式（凭当前密码开启） |
| 实时通道 | SSE ×3：容器状态流（docker events 驱动，500ms 防抖 + 30s 兜底）、主机 stats 流（2s 帧）、容器日志流；WebSocket ×1：终端（容器 exec / 栈 compose-logs，PTY + xterm） |
| 前端 | **SolidJS 1.9**（非 VanJS）+ @solidjs/router + Kobalte + xterm.js 6 + lucide-solid；单一手写 CSS 设计系统 `app.css`（"Crate · Apple-inspired"，737 行）；Vite 8，dev 代理 `/v1` → `127.0.0.1:5001`（含 ws）；构建产物 go:embed |
| 新增（未提交） | `pkg/server`：把 `Server{Start/Stop(ctx)}` 生命周期接口与通用 Gin HTTP 实现从 app 内提升为公共件，配合 `pkg/app` 统一 server/migration 两个二进制的启停 |

### 2.2 已实现能力地图

登录认证（JWT/2FA 后端/限流）、首启 Setup、栈全生命周期（扫描 + 外部栈合并、CRUD、start/stop/restart/down/update、状态码 0-4 与原版对齐）、容器总览（REST + SSE 增量状态 + 客户端分页 15/页 + 批量清理）、镜像/网络/卷管理、容器 exec 终端、栈聚合日志（WebSocket 版）、主机 CPU/内存实时流、磁盘用量与分类 prune、composerize（stub 版）、全局环境变量（bbolt）、版本检查（硬编码 v1.0.0 对比 louislam/dockge releases）。

### 2.3 已知问题清单（编号引用于后文）

| 编号 | 问题 | 位置 | 严重度 |
| --- | --- | --- | --- |
| P1 | 前端 `LogStream` 栈模式调用 `GET /v1/stacks/:name/logs/stream`，后端**无此路由**（仅 WS 终端承载栈日志） | `web/src/components/LogStream.tsx:23` vs `internal/server/http.go` | 高（契约缺口） |
| P2 | `POST /v1/auto-login` 注册在**公开路由组**；`disableAuth` 一旦开启，任何能触达端口者即获得首个活跃用户会话 | `internal/server/http.go:98`、`service/auth.go:257` | 高（安全） |
| P3 | CORS 中间件回显任意 `Origin` 且 `Allow-Credentials: true` | `internal/middleware/web.go:17` | 高（安全） |
| P4 | WebSocket `CheckOrigin` 全放行 | `internal/handler/terminal.go` upgrader | 中（安全） |
| P5 | composerize 为正则 stub：多端口/卷/环境变量时重复输出 YAML 键，产出**非法 YAML** | `internal/handler/composerize.go` | 中 |
| P6 | `GET /v1/docker/networks` 仅返回名字数组，前端 Driver 列退化为写死 "bridge" | `repository/network.go`、`web/src/views/Networks.tsx:110` | 中 |
| P7 | 2FA 后端就绪但前端无入口：登录遇 `tokenRequired` 直接拒绝 | `web/src/store/index.ts:52`、`views/Login.tsx` | 中（契约缺口） |
| P8 | 版本号硬编码 `"v1.0.0"`（Health 与 VersionCheck） | `internal/handler/docker.go:372,377` | 低 |
| P9 | 登录限流为**全局**令牌桶，非按 IP / 按账号 | `service/auth.go:53` | 中 |
| P10 | 死代码/未接线：`ContainerExecStart`（container.go:208）、文件版 `SaveGlobalEnv/GetGlobalEnv`（stack.go:221/231）、`StartSettingsCleaner` 无调用方、migration `seedJWTSecret` 写入后无人读取（JWT key 只来自 viper）、`httpx.Package` 未用、`DockerService.Stats` 一次性接口无路由 | 多处 | 低（整洁度） |
| P11 | 配置键不一致：local 用 `data.db.dsn` 而代码读 `data.db.user.dsn`（靠默认值兜底）；prod 的 stacks_dir/logs 路径风格与 local 不统一 | `config/dockge/*.yml`、`repository/repository.go:71` | 低 |
| P12 | 前端 i18n 模块（zh-CN/en-US）与若干 CSS（`.url-chips`、`.console-switch`、旧 `.editor-*`）已建未用；`formatBytes/timeAgo`、`StackDetail.urls`、`DockerStats.containers` 声明未消费 | `web/src` 多处 | 低 |
| P13 | 镜像拉取、栈部署、prune 均为同步阻塞请求，无进度流（仅栈操作有 `output` 文本回显） | `handler/docker.go`、`handler/stack.go` | 低（体验，P2 级需求） |
| P14 | 终端 `host` 类型：注释提及宿主 shell，代码实际拒绝（`未知终端类型`） | `handler/terminal.go:50,58` | 低 |
| P15 | `PUT /v1/stacks/:name` 请求体冗余携带 `name`（路径已含），前端照发 | `api/v1/dockge.go:103`、`web/src/api/api.ts:214` | 低（容忍） |

---

## 3. R1 注册登录：首次启动强制设置密码

### 3.1 口述原文

> 要求注册登录。第一次启动时可以强制设置密码，如果设置密码后才能继续访问。

### 3.2 需求条目

**R1-1 首启引导（强制）**
系统首次启动（用户存储为空）时 SHALL 进入初始化状态：任何未认证的业务访问 MUST 被重定向/拦截到初始化引导页；初始化引导页 SHALL 强制设置管理员用户名与密码；密码强度 MUST 满足现有规则（≥6 位且含字母与数字，前后端双重校验）。

**R1-2 引导完成即门禁生效**
初始化完成后（用户数 ≥ 1），系统 SHALL 立即进入受保护状态：除白名单端点（`/v1/login`、`/v1/setup/need`、`/v1/2fa`、`/v1/health`、`/v1/robots.txt`、静态资源、以及 R2 启用的外部身份端点）外，所有 `/v1/*` 请求 MUST 携带有效凭证，否则 401。`POST /v1/setup` 在已有用户时 MUST 返回 409 且不再可用。

**R1-3 会话与凭证**
登录成功 SHALL 签发 JWT（HS256，TTL 7 天，绑定密码哈希 shake256 摘要——改密后旧 token 全部失效）。REST 用 `Authorization: Bearer`，SSE/WebSocket 因浏览器限制 MUST 同时接受 `?token=` 查询参数（现状已实现，保持为硬契约）。

**R1-4 账号模型（决策 Q1：支持多用户）**
系统 SHALL 支持多用户账号：

- `Setup` 创建的首个用户角色为 **admin**；后续账号由 admin 在设置页「用户管理」中创建，**不开放自助注册**。
- 角色两档：**admin**（全部权限，含用户管理、全局环境变量）与 **member**（业务操作：栈/容器/镜像/网络/卷/终端/日志/清理）。管理端点见 §5.3 认证表新增行。
- 管理规则：admin 不可删除自己，不可删除或降级**最后一名 admin**；管理员重置密码走与自改密相同的 bcrypt 同源哈希；被停用（`active=false`）的用户的既有 token MUST 立即失效——现 `StrictAuth` 只验 JWT 摘要，SHALL 追加每请求的用户 `active` 校验（bbolt 单次读，开销可接受）。
- 每用户独立 JWT 与独立 2FA（后端已具备）；昵称与密码自服务沿用 `/v1/me*`。

**R1-5 2FA（可选增强，对齐 R3）**
TOTP 两步验证后端已就绪（启用/停用/校验/防重放）。前端 MUST 补齐入口：登录流程遇 `tokenRequired: true` 时进入验证码步骤（`POST /v1/2fa`），设置页提供启用（展示二维码 URL）与停用。（修 P7）

**R1-6 防爆破**
登录限流 SHOULD 从全局桶改为**按来源 IP + 账号**双维度限流（沿用 `pkg/rate`），失败响应保持统一 401 文案，不泄露用户是否存在。（修 P9）

**R1-7 免登录开关的处置**
现有 `disableAuth` + `POST /v1/auto-login`（P2）在 R2 方案落地后 SHALL 废弃该公开端点；「免登录」诉求统一由 R2 的受信反代 / OIDC 模式承接，且任一外部认证模式开启时本地密码登录 MUST 保留为兜底通道。

### 3.3 验收场景

- **首次启动**：清空 bbolt → 打开任意路由 → 强制跳转 `/setup`；直接调 `GET /v1/stacks` → 401。
- **设置完成**：`POST /v1/setup` 成功返回 `accessToken`，再次 `POST /v1/setup` → 409；此后无 token 访问业务端点 → 401。
- **密码门禁**：带错误/过期 token → 401 且前端统一跳登录页；改密后旧 token → 401。
- **2FA**：启用 2FA 的账号登录 → 返回 `tokenRequired`；提交验证码 → 拿到正式 token；同一验证码重放 → 401。

### 3.4 现状差距

R1-1/R1-2/R1-3 主体已实现（`GET /v1/setup/need` → `/setup` 引导 → JWT 门禁）；差距集中在：R1-4 用户管理端点与角色模型未实现（F11）、R1-5 前端 2FA UI 缺失（P7）、R1-6 限流粒度（P9）、R1-7 auto-login 公开端点风险（P2，并入 R2 一并整改）。

---

## 4. R2 反向代理中间件免登录方案（Traefik / Authelia / OIDC）

### 4.1 口述原文

> 如果使用中间件拦截实现免登录，比如 traefik、authelia 等等，你出个方案。要求支持 oidc 等鉴权。

### 4.2 目标与原则

用户已完成反向代理层的统一身份认证（Authelia / Authentik / oauth2-proxy / nginx auth_request 等），希望 Dockge 信任该层结果，跳过本地登录页。同时要求 Dockge 自身能对接 OIDC 协议的 IdP（Authentik、Keycloak、Casdoor、Google 等）。

设计原则：

- **P-1 本地会话不变**：无论身份来自何处，Dockge 始终签发**本地 JWT**（复用现有 HS256 + StrictAuth + `?token=`）。外部身份只在「换取本地会话」的瞬间参与，SSE/WS/前端其余部分零改动。
- **P-2 本地密码永远是兜底**：任何外部认证模式下 `/v1/login` 仍可用；外部身份源故障不导致锁死。
- **P-3 显式开启、默认关闭**：所有信任行为由配置开关 + 受信网段共同控制，未配置时后端 MUST 忽略一切身份头 / OIDC 参数。
- **P-4 身份映射到本地用户**：外部身份按用户名映射到 bbolt 中的本地用户；不存在时按策略自动开通（auto-provision）或拒绝，映射用户无独立密码（不可用密码登录），删除外部源后可由管理员重置。

### 4.3 方案总览：三种认证模式，可组合

认证模式由配置 `auth.mode` 决定，取值 `local`（默认）| `proxy` | `oidc`，并允许 `local` 与后两者叠加为兜底：

#### 模式 A：`proxy` —— 受信反向代理头（Authelia / Authentik forwardAuth 注入）

请求链路：`浏览器 → Traefik(forwardAuth→Authelia) → 注入 Remote-* 头 → Dockge`。

Dockge 侧行为：

1. 新增公开端点 `GET /v1/auth/proxy`：校验请求来源 IP ∈ `auth.proxy.trusted_proxies`（CIDR 列表），读取 `Remote-User`（必填）、`Remote-Email`、`Remote-Name`、`Remote-Groups`（头名可配置）。
2. 命中本地用户（或按 `auto_provision` 策略开户）→ 签发本地 JWT，返回与 `/v1/login` 相同的 `LoginResponseData`。
3. 前端 boot 时：`GET /v1/me` 401 后尝试一次 `GET /v1/auth/proxy`，成功即无感建立会话；失败再落登录页。登录页在 proxy 模式下显示「通过反向代理认证」入口。
4. 同时给 `StrictAuth` 增加旁路：受信来源 + 合法身份头存在时，直接以映射用户身份放行（这样即使前端没走 `/v1/auth/proxy`，被反代转发的 API 请求也能通过）。

安全边界（MUST）：

- 仅当 `RemoteAddr` 命中 `trusted_proxies` 时才读取身份头；Dockge 自身 MUST NOT 暴露在受信网段之外（文档明示）。
- 启用 proxy 模式时，Dockge 入口 MUST 主动剥除「非受信来源」携带的一切 `Remote-*` 身份头（防头伪造直连）；gin 侧同步收敛 `SetTrustedProxies`。
- 顺手修 P3/P4：CORS 收敛为配置白名单（`http.cors_origins`，默认同源）；WebSocket `CheckOrigin` 校验 Host/Origin 一致性。

#### 模式 B：`oidc` —— Dockge 作为 OIDC Relying Party（Authorization Code + PKCE）

适用于让 Dockge 直连 IdP（不依赖反代转发）：

1. 配置 `auth.oidc.{issuer, client_id, client_secret, redirect_url, scopes, username_claim, groups_claim}`，启动时发现 `.well-known/openid-configuration`（缓存 endpoint 集合与 JWKS）。
2. 新增端点：
   - `GET /v1/auth/oidc/login` → 302 到 IdP 授权端点（state + PKCE challenge 写 HttpOnly Cookie）；
   - `GET /v1/auth/oidc/callback` → 校验 code/state，换 token，验签 ID Token，按 `username_claim`（默认 `preferred_username`）映射/开户，生成**一次性票据**（60 秒有效、单次消费）→ 302 回前端 `/auth/oidc?code=<票据>`；
   - `POST /v1/auth/oidc/exchange` → 前端以 `{code}` 换取本地 JWT（响应与 `/v1/login` 相同），会话落 localStorage——决策 Q2 已拍板：一次性 code 换 JWT，不引入 Cookie；
   - `GET /v1/auth/oidc/status` → 前端探测 OIDC 是否启用（决定登录页是否显示「使用 SSO 登录」）。
3. 可选 `groups_claim` 角色映射（与决策 Q1 对齐）：命中 `admin_groups` → admin，否则 member；未配置 `admin_groups` 时自动开户一律 member，由管理员在「用户管理」中提升。

依赖：引入一个轻量 OIDC 库（推荐 `github.com/coreos/go-oidc/v3` + `golang.org/x/oauth2`），符合 G-4 最小依赖原则。

#### 模式 C：ForwardAuth 端点（供 Traefik `forwardAuth` 直接探测 Dockge 自身会话）

Dockge 额外暴露 `GET /v1/auth/forwardauth`：携带有效本地 JWT → 200；否则 401。使「Dockge 自身登录页」也能充当 Traefik 生态里的认证器（与 Authelia 同位），覆盖「只想用一个 Dockge 账号保护一堆内网服务」的场景。该端点开销极小（纯 JWT 解析），实现成本低，作为方案完整性收录。

### 4.4 配置 schema（新增，并入 `config/dockge/*.yml`）

```yaml
auth:
  mode: local            # local | proxy | oidc（local 可与 proxy/oidc 并存为兜底）
  proxy:
    enabled: false
    trusted_proxies:     # CIDR 列表，命中才信任身份头
      - 127.0.0.1/32
      - 172.16.0.0/12
    headers:
      user: Remote-User
      email: Remote-Email
      name: Remote-Name
      groups: Remote-Groups
    auto_provision: true # 身份不存在时自动开户（无密码，角色默认 member）
  oidc:
    enabled: false
    issuer: https://auth.example.com/application/o/dockge/
    client_id: dockge
    client_secret: "***"
    redirect_url: https://dockge.example.com/v1/auth/oidc/callback
    scopes: [openid, profile, email]
    username_claim: preferred_username
    groups_claim: groups
    admin_groups: [dockge-admins]
```

### 4.5 部署示例（Traefik + Authelia，标签法）

```yaml
# docker-compose.yaml（节选）
services:
  dockge:
    image: dockge
    labels:
      - traefik.http.routers.dockge.middlewares=authelia@docker
  authelia:
    image: authelia/authelia
    # Authelia 配置（identity_providers: oidc）将 dockge 注册为 client，
    # 或走 forwardAuth + Remote-* 头模式（对应方案 A）。

# Traefik forwardAuth 中间件（方案 A）：
#   - traefik.http.middlewares.authelia.forwardauth.address=http://authelia:9091/api/verify
#   - traefik.http.middlewares.authelia.forwardauth.authResponseHeaders=Remote-User,Remote-Email,Remote-Name,Remote-Groups
```

### 4.6 与 R1 的关系

- 首启强制设密（R1-1/R1-2）在 proxy/oidc 模式下**依然生效**：本地管理员密码是 R2 全部模式的兜底与逃生通道，MUST NOT 因外部认证启用而跳过。
- R2 落地时同步执行 R1-7：下线公开组 `POST /v1/auto-login`，`disableAuth` 设置项只读保留一个版本用于迁移提示，随后删除。

### 4.7 分期建议

| 期 | 内容 | 理由 |
| --- | --- | --- |
| P0 | 模式 A（proxy 受信头）+ P3/P4 安全收敛 + auto-login 下线 | 改动最小即可覆盖 Traefik/Authelia 主流用法，且消除现存高危项 |
| P1 | 模式 B（OIDC + PKCE）+ 登录页 SSO 入口 | 满足「支持 oidc 等鉴权」，覆盖无反代场景 |
| P2 | 模式 C（forwardauth 端点）+ groups 映射细化 | 锦上添花 |

### 4.8 验收场景

- **A-无感登录**：Authelia 已认证的用户打开 Dockge → 前端自动换取本地 JWT，直接进入仪表盘，全程无登录页。
- **A-防伪造**：不受信 IP 直连 Dockge 并伪造 `Remote-User: admin` → 401。
- **B-SSO**：登录页点「使用 SSO 登录」→ IdP 认证 → 回调后进入仪表盘；IdP 中注销后 Dockge 本地会话到期前仍有效（本地 JWT 语义）。
- **兜底**：反代/IdP 全部宕机 → 用本地管理员密码经 `/v1/login` 正常登录。

---

## 5. R3 后端接口全面整理（对齐前端原型）

### 5.1 口述原文

> 全面的整理后端接口和需求。要求完全对齐前端的原型。

### 5.2 对齐原则

- **A-1** 前端原型 `web/src/api/api.ts` + 视图/组件内构造的 SSE/WS URL 为**契约基准**；后端 MUST 逐端点对齐路径、方法、请求字段、响应字段与流为语义。
- **A-2** 统一响应包络 `{code, message, data}`，`code=0` 成功；错误哨兵沿用 `api/v1`（401/400/404/409/500），`handleServiceError` 保持现状映射。
- **A-3** 后端就绪但前端未接的端点（2FA、disableAuth 等）不算「对齐」；对齐 = 前端有 UI 且行为闭环，或按决策显式下线。
- **A-4** 预格式化字符串契约迁移为**结构化数据**（决策 Q3 已拍板）：镜像 `size/created` 改为字节数/Unix 时间戳，`DockerDf` 的 `size/reclaimable` 改为字节数，容器 `ports` 拼接串改为结构化端口数组；展示格式化统一由前端完成（启用 `formatBytes/timeAgo`，顺带关闭 P12 相关项）。字段类型变更 MUST 双端同版本一次切换，不留兼容层（见 F12）。

### 5.3 接口总表（基线契约 = 前端）

状态：✅ 已对齐 · ⚠️ 部分对齐 · ❌ 缺失

#### 认证与账号

| 方法与路径 | 认证 | 请求 → 响应 | 前端消费方 | 状态 |
| --- | --- | --- | --- | --- |
| `POST /v1/login` | 公开 | `{username,password}` → `{accessToken,user,tokenRequired?}` | Login、store.login | ✅ |
| `POST /v1/setup` | 公开 | `{username,password}` → 同上 | Setup | ✅ |
| `GET /v1/setup/need` | 公开 | → `{needSetup}` | Login 挂载探测 | ✅ |
| `GET /v1/me` | Bearer | → `{id,username,nickname,twoFA?}` | boot 校验/会话恢复 | ✅ |
| `PUT /v1/me/password` | Bearer | `{oldPassword,newPassword}` → `{}` | Settings | ✅ |
| `POST /v1/2fa` | 公开 | `{username,token}` → `LoginResponseData` | **无 UI**（R1-5 要求补） | ❌ 前端 |
| `POST /v1/me/2fa/enable` | Bearer | → `{qrCodeURL}` | **无 UI** | ❌ 前端 |
| `DELETE /v1/me/2fa` | Bearer | → `{}` | **无 UI** | ❌ 前端 |
| `GET/POST /v1/me/disableauth` | Bearer | 读 `{enabled}` / 写 `{enable,currentPassword}` | **无 UI，且按 R1-7/R2 下线** | ⚠️ 处置 |
| `POST /v1/auto-login` | 公开 | → `LoginResponseData` | 无 UI；**P2 风险，随 R2 移除** | ⚠️ 处置 |
| `GET /v1/users` | Bearer（admin） | → `{list:[{id,username,nickname,role,active,twoFA}]}` | **F11 新增**（Settings 用户管理） | ❌ |
| `POST /v1/users` | Bearer（admin） | `{username,password,role?}` → `{}` | **F11 新增** | ❌ |
| `PUT /v1/users/:id` | Bearer（admin） | `{nickname?,role?,active?}` → `{}` | **F11 新增** | ❌ |
| `PUT /v1/users/:id/password` | Bearer（admin） | `{newPassword}` → `{}`（管理员重置） | **F11 新增** | ❌ |
| `DELETE /v1/users/:id` | Bearer（admin） | → `{}`（禁删自己/最后一名 admin） | **F11 新增** | ❌ |

#### 编排栈

| 方法与路径 | 认证 | 请求 → 响应 | 状态 |
| --- | --- | --- | --- |
| `GET /v1/stacks?filter=` | Bearer | → `{list:[{name,status(0-4),statusLabel,managed,composeFileName?,configFiles?}]}` | ✅ |
| `GET /v1/stacks/:name` | Bearer | → `{...summary,yaml,env,containers[],urls?}` | ✅（`urls` 前端未用，P12） |
| `POST /v1/stacks` | Bearer | `{name,yaml,env}` → `{}` | ✅ |
| `PUT /v1/stacks/:name` | Bearer | 同上（body 冗余 `name`，容忍 P15） | ✅ |
| `DELETE /v1/stacks/:name` | Bearer | → `{output}`（down + 删目录） | ✅ |
| `POST /v1/stacks/:name/:op`（start/stop/restart/down/update） | Bearer | → `{output}` | ✅ |
| `GET /v1/stacks/:name/logs/stream?tail=200` | `?token=`（SSE） | 逐行纯文本 | **❌ 后端缺失（P1）；决策 Q4 已拍板：后端补齐（F1 定稿）** |

#### 容器 / 镜像 / 网络 / 卷 / 系统

| 方法与路径 | 认证 | 请求 → 响应 | 状态 |
| --- | --- | --- | --- |
| `GET /v1/docker/version` | Bearer | `{version,apiVersion,os,arch}` | ✅ |
| `GET /v1/docker/info` | Bearer | `{version,os,arch,stacksTotal,stacksRunning,containersTotal,containersRunning}` | ✅ |
| `GET /v1/docker/containers` | Bearer | `{list:[{id,name,image,state,status,ports[],stack?}]}`（ports 改结构化数组，F12） | ⚠️ |
| `GET /v1/docker/containers/:id/inspect` | Bearer | docker inspect 子集（Id/Created/Config/Mounts/NetworkSettings） | ✅ |
| `POST /v1/docker/containers/:id/{start,stop,restart}` | Bearer | → `{}` | ✅ |
| `DELETE /v1/docker/containers/:id` | Bearer | → `{}` | ✅ |
| `POST /v1/docker/containers/prune` | Bearer | → `{}` | ✅ |
| `GET /v1/docker/images` | Bearer | `{list:[{id,repo,tag,sizeBytes,createdAt}]}`（决策 Q3：结构化，F12） | ⚠️ |
| `POST /v1/docker/images/pull` | Bearer | `{reference}` → `{}`（同步阻塞，P13） | ⚠️ |
| `DELETE /v1/docker/images/:id` · `POST /v1/docker/images/prune` | Bearer | → `{}` | ✅ |
| `GET /v1/docker/networks` | Bearer | `{list:[string]}` **仅名字**（前端要 Driver，P6） | ⚠️ |
| `GET /v1/docker/networks/:name` | Bearer | `{Name,Driver,Created,IPAM}` | ✅ |
| `POST /v1/docker/networks/create` | Bearer | `{name,driver,subnet}` → `{}` | ✅ |
| `DELETE /v1/docker/networks/:name` · `POST /v1/docker/networks/prune` | Bearer | → `{}` | ✅ |
| `GET /v1/docker/volumes` | Bearer | `{list:[{name,driver}]}` | ✅ |
| `DELETE /v1/docker/volumes/:name` · `POST /v1/docker/volumes/prune` | Bearer | → `{}` | ✅ |
| `GET /v1/docker/df` | Bearer | `{list:[{type,count,active,sizeBytes,reclaimableBytes}]}`（决策 Q3：结构化，F12） | ⚠️ |
| `GET /v1/version/check` | Bearer | `{latestVersion,currentVersion,hasUpdate}`（currentVersion 硬编码 P8） | ⚠️ |
| `GET /v1/health` | 公开 | `{status,version}` | ✅ |
| `POST /v1/composerize` | Bearer | `{dockerRunCommand}` → `{composeTemplate}`（stub 产非法 YAML，P5） | ⚠️ |

#### 设置

| 方法与路径 | 认证 | 请求 → 响应 | 状态 |
| --- | --- | --- | --- |
| `GET /v1/settings/globalenv` | Bearer | → `{globalENV}` | ✅ |
| `PUT /v1/settings/globalenv` | Bearer | `{content}` → `{}` | ✅ |

#### 流式端点（协议细节）

| 端点 | 传输 | 认证 | 帧协议 | 状态 |
| --- | --- | --- | --- | --- |
| `/v1/docker/containers/stream` | SSE | `?token=` | `data: {"containers":[{id,state,status}]} `；空 data = 心跳（前端已忽略）；15s 心跳 | ✅ |
| `/v1/docker/stats/stream` | SSE | `?token=` | `data: {cpuUsage,memUsage,memTotalMB,memPercent,containers?}` 每 2s | ✅ |
| `/v1/docker/containers/:id/logs?tail=200` | SSE | `?token=` | 逐行纯文本 | ✅ |
| `/v1/terminal/:name/:type`（`exec`=容器 ID / `compose-logs`=栈名） | WebSocket | `?token=` | 下行 PTY 原始字节；上行按键原始字节 + JSON 控制 `{"type":"resize","data":{"rows","cols"}}` | ✅（`host` 类型 P14） |

### 5.4 整改需求（对齐缺口）

按优先级编号，实施时逐项关闭：

| 编号 | 整改项 | 归属 |
| --- | --- | --- |
| F1 | **补齐栈日志 SSE**（决策 Q4 定稿）：新增 `GET /v1/stacks/:name/logs/stream`，复用 `docker compose logs -f --no-color --tail` 管道，语义与容器日志 SSE 一致，关闭 P1 | 后端 |
| F2 | composerize 重写为可靠解析（shlex 风格分词 + 完整 flag 表，多端口/卷/-env 产出合法 YAML；表驱动用例覆盖 `docker run` 常见形态），关闭 P5 | 后端 |
| F3 | `GET /v1/docker/networks` 返回 `[{name,driver}]`，前端 Driver 列真实渲染，关闭 P6 | 双端 |
| F4 | 前端补 2FA 登录步骤与设置页管理（R1-5），关闭 P7 | 前端 |
| F5 | 版本常量统一：ldflags 注入或单一 `internal/version` 包，`/v1/health` 与 `/version/check` 同源，关闭 P8 | 后端 |
| F6 | 死代码清理（P10 全项）：删 `ContainerExecStart`、文件版 global env、未接线 cleaner、无消费 seedJWTSecret、未用 `httpx.Package`；`StartSettingsCleaner` 要么接线要么删除 | 后端 |
| F7 | 配置键统一（P11）：`data.db.user.dsn` 单一键名；local/prod 的 stacks、logs、db 路径风格一致化 | 后端 |
| F8 | 契约清债（P12）：`DockerStats.containers`、`StackDetail.urls`、`formatBytes/timeAgo`、未用 CSS/i18n 模块——要么排期消费（urls 渲染为可点链接；i18n 接入全部文案），要么删除并同步 TS 类型 | 双端 |
| F9 | 体验增强（P2 级，可后置）：镜像拉取/栈部署输出走 SSE 或复用终端面板流式回显；prune 返回回收统计（数量/空间）入 toast | 后端 |
| F10 | **不实现宿主 shell**（决策 Q5：Web 可达的宿主 shell 是安全红线，终端只允许进入容器）：删除 `terminal.go` 中 `host` 类型的注释/分支与 `Repository.ContainerExecStart` 死代码（关闭 P14）；终端类型收敛为 `exec`（容器 shell）与 `compose-logs` | 后端 |
| F11 | **多用户管理**（决策 Q1）：后端实现 §5.3 认证表新增的 5 个用户端点（bbolt `users` 已支持多用户存储），角色 admin/member，执行 R1-4 管理规则（含 `StrictAuth` 追加 active 校验）；前端 Settings 新增「用户管理」卡片（列表/新建/重置密码/停用/删除 + 2FA 状态列） | 双端 |
| F12 | **结构化字段迁移**（决策 Q3）：`DockerImageData.size/created` → `sizeBytes int64`/`createdAt int64`；`DockerDfCategory.size/reclaimable` → `sizeBytes/reclaimableBytes int64`；容器 `ports` 字符串 → `[]PortMapping{hostIP,hostPort,containerPort,proto}`；前端启用 `formatBytes/timeAgo` 渲染并删除 `"0B"` 字符串比较；双端同版本一次切换，不留兼容层 | 双端 |

### 5.5 验收场景

- 逐条核对 §5.3 表：每个 ✅ 项在接口测试（handler 层表驱动测试或 httptest）中有用例；每个 ❌/⚠️ 项要么关闭要么转为显式决策记录。
- 前端 `pnpm build && tsc --noEmit` 通过且运行时 Network 面板无 404/契约不匹配请求。

---

## 6. R4 前端界面需求（原风格 · Unix 工具向 · Apple System）

### 6.1 口述原文

> 前端界面基于原风格，要求 Unix 工具向和 Apple-System。

### 6.2 基线与原则

**原风格** = `DESIGN.md` 的 Crate 设计系统（Apple 式 token：`#0071e3` 稀缺蓝、SF 字体栈、pill 徽章、8/12/18px 圆角、ring-first 层级、150/220ms 动效）+ OpenSpec `design-mock` 规范中已固化的交互行为。R4 是**在其上强化两种气质**，不是重做：

**R4-1 Apple System（继承 + 补课）**

- 视觉语言 MUST 沿用 `DESIGN.md` 色彩/字体/间距/动效 token，新增颜色 MUST 先入 token 表再入 CSS（既有铁律）。
- MUST 补齐 design-mock 已验收的交互（当前 SolidJS 前端尚未完全落地的部分）：
  - 明暗双主题（顶栏切换、`dockge.theme` 持久化、首访跟随系统、无 FOUC）；**控制台岛（终端/编辑器/日志/事件流）在两种主题下保持深色**。
  - zh-CN/en-US 双语切换（`dockge.lang` 持久化），接入现有 i18n 模块替换全部硬编码文案（关闭 P12 一半）。
  - iOS 风格确认对话框（居中卡片 + 毛玻璃 + 红色破坏性按钮，禁用原生 confirm）、iOS 开关（Show all 筛选）、按压反馈（按钮 scale、整行高亮）、视图切换淡入、轮询刷新不闪动。
- 无障碍底线不变：WCAG 2.2 AA、键盘可达、`prefers-reduced-motion`、375/768/1280 无横向溢出。

**R4-2 Unix 工具向（强化）**

Unix 工具的气质 = **文本优先、可组合、可复制、快、安静**：

- **U-1 等宽即数据面**：一切机器数据（容器/镜像 ID、镜像引用、端口映射、卷路径、时间戳、字节数、状态码）MUST 用 mono 字体栈 + tabular figures 渲染；长 ID 可一键复制（含 toast 反馈）。
- **U-2 控制台岛为一等公民**：日志、终端、compose 输出沿用深色 `#0d0d0d–#1d1d1f` 岛；行级直达——容器行仅 [日志、终端、启动/停止] 三键（design-mock 已定），栈行日志一键聚合；终端支持多标签、resize 同步（协议已有）。终端类型收敛为容器 exec 与栈 compose-logs（决策 Q5：宿主 shell 不实现——Web 可达的宿主 shell 视为安全红线）。
- **U-3 命令可预期**：栈操作头部回显等价命令（`$ docker compose up -d`，现状已有，推广到 prune/pull 等所有引擎操作的操作前后日志）；所有输出面板纯文本、可选择、可复制，不做图文混排。
- **U-4 键盘优先**：`⌘/Ctrl+K` 全局搜索已有，MUST 保持并文档化；列表 `j/k` 或方向键导航、`Enter` 进入、`Esc` 逐层退出 SHOULD 逐步补齐。
- **U-5 安静**：toast 只反馈用户主动操作；系统自发状态变化（SSE 推送）只更新数据不弹窗（design-mock 已定）；实时状态以状态点 + 「已连接/重连中」表达，不打断。
- **U-6 密度**：表格行高紧凑、分组用 uppercase micro-label；数据密度优先于留白，但遵守 Apple 的层级克制（不加卡片阴影，tonal + 1px 边界）。

**R4-3 布局模式（沿用）**：主从工作区（列表 320-380px + 详情）、栈三栏工作台（文件栏/编辑器/部署摘要）、表格页 15/页客户端分页、Sheet 右抽屉、移动端全屏任务页。范围参照 `DESIGN.md` §4-§5，不另行发明。

### 6.3 验收场景

- 明色主题 + 中文下逐页走查：无硬编码英文残留；暗色主题下控制台岛仍深色。
- 任一 ID/路径/端口文本可选中复制；`⌘K` 可搜索并跳转容器/栈/镜像/卷。
- 容器行只有三个操作键；点日志直接弹独立日志 Sheet；删除走 iOS 风格红色确认。
- SSE 断连仅侧栏状态点变红 + 「重连中」，无 toast 轰炸；恢复后自动续流。

---

## 7. 工程规范：Go Style（强制）

### 7.1 口述原文

> 代码要求符合 go-style。

### 7.2 参照标准（按优先级）

1. Go 官方：Effective Go、Go Code Review Comments、Go Proverbs（Rob Pike）。
2. Uber Go Style Guide（项目骨架 nunu 的底座约定）。
3. 工作区技能 `golang-microservice-design`（对上述标准的执行化整理）为日常写码/评审的触发规范。

### 7.3 项目级强制约定（从现状代码提炼，保持并固化）

- **分层依赖单向**：`handler → service → repository`；接口定义在**消费侧**（service 层声明它需要的 repository 能力），实现侧用 `var _ X = (*x)(nil)` 编译期断言（现状已有，保持）。
- **错误**：哨兵错误集中在 `api/v1`（`ErrUnauthorized/ErrBadRequest/...`）与 repository 层（`ErrNotFound/ErrConflict`）；底层错误 MUST `%w` 包装向上；handler 统一经 `handleServiceError` 映射 HTTP 状态，不泄漏内部细节。
- **context**：所有 IO 签名 `ctx context.Context` 为首参并透传（docker/compose 调用全部带超时，现状已符合）。
- **DI**：samber/do，`do.Package` + `do.Lazy` 注册于各层 `Package` 变量，组合根只出现在 `cmd/`；禁止全局单例与 `init()` 副作用。
- **并发**：goroutine 生命周期 MUST 随 ctx 取消（`ContainerStatusServer` 模式）；长操作用互斥量/单飞（`opMutex sync.Map` 模式）防并发踩踏。
- **注释**：导出符号有中文文档注释（现状风格，保持）；注释写「为什么」不写「是什么」。
- **测试**：表驱动、就近放置 `_test.go`；纯函数（状态机、解析、格式化）必须可测（现状 compose/image/stack/migration 已有，沿用）。
- **命名**：包名短小全小写；不缩写（`Repository` 不写 Repo）；布尔用 `Is/Has/Can` 前缀；常量用 `time.Duration` 直接量。
- **依赖最小化**：新增依赖须论证（R2 的 OIDC 库是当前唯一预期新增）；`go.mod` direct 数量保持个位数增长。
- **API 契约**：请求/响应结构体集中在 `api/v1`，字段加 binding 标签；handler 内不出现匿名请求结构（现状 `ToggleDisableAuth` 违例，整改时补结构体）。

### 7.4 验收

- `gofmt -l` 零输出；`go vet ./...` 通过；新增包有包注释。
- 评审 checklist 引用本章；违反分层的 PR 打回。

---

## 8. 需求追溯矩阵与待决策项

### 8.1 追溯矩阵

| 口述条目 | 需求条目 | 关联整改 |
| --- | --- | --- |
| 1. 注册登录 / 首启强制设密 / 设密后才能访问 | R1-1 ~ R1-7 | P2、P7、P9 |
| 2. 中间件免登录方案（traefik/authelia/oidc） | R2 全章（模式 A/B/C、配置 schema、分期） | P2、P3、P4 |
| 3. 全面整理后端接口，完全对齐前端原型 | R3 全章（§5.3 契约总表、§5.4 F1-F10） | P1、P5、P6、P8、P10、P11、P12、P13、P15 |
| 4. 前端原风格 + Unix 工具向 + Apple System | R4-1 ~ R4-3 | P12、P14 |
| （附加）代码符合 go-style | §7 全章 | — |

### 8.2 决策记录（2026-09-09 需求人拍板）

| 编号 | 问题 | 决策 | 落点 |
| --- | --- | --- | --- |
| Q1 | 是否支持多用户？ | **支持多用户**：管理员在设置页创建/管理账号（重置密码/停用/删除），角色分 admin 与 member；不开放自助注册 | R1-4 重写；新增 F11；R2 auto-provision 默认 member、`admin_groups` 提升 |
| Q2 | OIDC 回调种会话方式 | **一次性 code 换本地 JWT**（沿用 localStorage token 模型，不引入 Cookie） | R2 模式 B 定稿；新增 `POST /v1/auth/oidc/exchange` |
| Q3 | 预格式化字符串是否改结构化字段？ | **改**：数值字段 + 前端格式化 | A-4 重写；新增 F12 |
| Q4 | 栈日志通道 | **后端补 SSE 端点**（与容器日志对称） | F1 定稿；P1 关闭路径确定 |
| Q5 | 是否实现宿主 shell？ | **不实现**（安全：终端只能进入容器 shell） | F10 改为删除 host 类型与死代码；R4-2/U-2 同步 |

当前无未决事项；后续决策以同表追加记录。

### 8.3 变更记录

| 版本 | 日期 | 内容 |
| --- | --- | --- |
| v1.0 | 2026-09-09 | 首次整理：口述 4 条 + go-style 全量扩写；完成全量代码巡视并沉淀问题清单 P1-P15、整改清单 F1-F10 |
| v1.1 | 2026-09-09 | 决策落定（Q1-Q5）：多用户（admin/member）、OIDC 一次性 code 换 JWT、结构化字段迁移、后端补栈日志 SSE、不实现宿主 shell；新增 F11/F12，修订 R1-4 / R2 模式 B / A-4 / §5.3 契约表 / U-2 |
