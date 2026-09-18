# Dockge 项目规格书（SDD / TDD 视角）

> 版本：v1.1（2026-09-12）
> 性质：现状基线（As-Is）+ 目标状态（To-Be）+ 测试与工作流规格。
> 本文档是项目的**规格总入口**；逐条需求详见 `docs/REQUIREMENTS.md`，增量变更走 `openspec/changes/`。
> 按项目规范：正文 zh-CN，规格关键字（SHALL / MUST / SHOULD）保留英文。

---

## 目录

1. [产品定位与边界](#1-产品定位与边界)
2. [系统架构现状（As-Is）](#2-系统架构现状as-is)
3. [能力规格清单（含测试映射）](#3-能力规格清单含测试映射)
4. [刻意不做的事（Non-Goals）](#4-刻意不做的事non-goals)
5. [测试现状与 TDD 策略](#5-测试现状与-tdd-策略)
6. [SDD 工作流规格](#6-sdd-工作流规格)
7. [目标状态（To-Be：项目应该是什么样子）](#7-目标状态to-be项目应该是什么样子)
8. [已知债务与风险清单](#8-已知债务与风险清单)
9. [文档维护规则](#9-文档维护规则)

---

## 1. 产品定位与边界

**Dockge（dockge-go/dockge）** 是 [louislam/dockge](https://github.com/louislam/dockge) 的**从零 Go 语言复刻**（非 fork，无共享 git 历史），基于 nunu-monorepo 骨架，面向单机 Docker/Podman 环境的 Compose 栈管理面板。

### 1.1 核心主张

- **单机边界**：SHALL 只管理本机容器引擎。多主机/Agent 能力已通过 `openspec/changes/remove-remote-agent/` 移除（后端 + 前端清理均完成）。
- **Podman 优先**：容器操作 SHALL 优先使用 Podman v6 Go bindings（rootful/rootless socket 自动发现），失败时回退 `docker` CLI；Compose 编排统一走 `docker compose` CLI。
- **单二进制交付**：前端经 `go:embed` 打入二进制，`make build` 产出 `bin/dockge-server`（版本由 `git describe` 注入）。
- **状态解析对齐上游**：栈状态机（0–4）与上游 `StackStatusFromString` 语义一致，便于用户迁移。

### 1.2 规模基线（2026-09-12 工作树实测）

| 维度 | 数值 |
|---|---|
| Go 源文件 / LOC（非测试） | 59 文件 / 7,072 行 |
| Go 测试 | 10 文件 / 26 个测试函数（stdlib testing，无 testify/mock） |
| 前端 TS+TSX LOC | ~4,337 行（`app/dockge/web/src/`，11 个视图） |
| 前端测试 | 3 文件 / 7 个测试（node:test，`web/tests/`） |
| i18n | zh-CN / en-US 各 299 键（类型强制对齐） |
| go.mod 直接依赖 | 15 |
| CI | **无** |
| 提交历史 | 29 commits（2026-09-08 起）+ 未提交清理改动（冗余清理：死防御分支、冗余透传、游离注释、过度导出，见 2026-09-12 巡逻） |
| 构建平台 | **仅 Linux**（`pkg/pty` 依赖 `TIOCGPTN`/`TIOCSPTLCK`/`/dev/pts`；darwin 构建 `go build ./app/dockge/cmd/server` 直接失败） |

---

## 2. 系统架构现状（As-Is）

### 2.1 技术栈

| 层 | 选型 | 证据 |
|---|---|---|
| 后端 | Go 1.26 + Gin v1.12 + samber/do v2（DI）+ Viper + Zerolog | `go.mod`, `app/dockge/cmd/server/main.go` |
| 存储 | bbolt（用户/设置）+ 文件系统（栈目录） | `internal/repository/repository.go` |
| 容器引擎 | Podman v6 bindings 优先，docker CLI 回退 | `internal/repository/{podman,container,cli}.go` |
| 实时通信 | SSE ×3（容器状态/主机指标/容器日志）+ gorilla WebSocket ×1（终端） | `api/v1/container_status.go`, `internal/handler/terminal.go` |
| 认证 | 四模式 `security.auth.mode`：jwt / proxy / oidc / disable | `internal/security/`, `internal/{authproxy,authoidc}/` |
| 前端 | SolidJS 1.9 + @solidjs/router + Kobalte + xterm.js 6 + lucide-solid + Vite 8 + pnpm | `app/dockge/web/package.json` |
| 样式 | 手写 CSS 设计系统（"Crate"，Apple 风格），无 UI 框架 | `web/src/styles/app.css`, `DESIGN.md` |

### 2.2 后端分层

```
app/dockge/
  cmd/{server,migration,reset-password}/   # 3 个入口
  internal/
    handler/      # HTTP 层（auth/oidc/stack/docker/terminal/settings/composerize）
    service/      # 用例层（auth/docker/stack/settings）
    repository/   # 基础设施（bbolt/文件系统/podman 绑定/docker CLI/procfs）
    middleware/   # CORS / RequestLog / StrictAuth / ProxyAuth / SetupRequired
    server/       # HTTP 装配（http.go）、SSE 状态服务、一次性迁移服务
    security/     # 认证模式枚举与解析
    authproxy/    # 受信 CIDR + 身份头配置
    authoidc/     # OIDC Manager/Provider/State（Discovery、PKCE、验签）
    composerize/  # 原生 docker run → compose 解析器
    version/      # 版本常量（ldflags 注入）
    testutil/     # 跨包测试辅助（ViperFromYAML）
  api/v1/         # DTO、{code,message,data} 响应封套、哨兵业务错误
pkg/              # jwt / hash / pty / rate / log / config / app / server
```

依赖方向严格单向：handler → service → repository；DI 通过 `samber/do` 在 `cmd/server/main.go` 组装。

### 2.3 关键机制

- **认证**：JWT HS256，claims 绑定 `shake256(passwordHash)`（改密即失效）；bcrypt 存储；登录限速 10 次/分钟（IP+账号键滑动窗口，容量 4096，`pkg/rate`）。token 读取顺序 Bearer → `?token=` → `dockge_token` httpOnly cookie（proxy/oidc 模式种 cookie）。`AuthService.CheckSession`（逐请求校验用户存在/启用/哈希未变）已实现但**未接线** StrictAuth（见 D12）。
- **认证模式四选一**：`jwt`（默认）/ `proxy`（ProxyAuth 中间件，受信 CIDR fail-closed，自动开户）/ `oidc`（多 Provider，Auth Code + PKCE S256，回调种 cookie）/ `disable`（auto-login）。
- **实时容器状态**：`docker events` 监听 → 500ms 防抖 + 30s 心跳 ticker → StatusHub 扇出 → SSE 15s 心跳帧，新连接回放最后一帧；帧仅含 id/state/status（有测试锁定）。
- **终端**：WebSocket 升级后分两类：`compose-logs`（只读 `docker compose logs -f --tail`）与 `exec`（`docker exec -it`，bash→sh 探测回退）；PTY 为手写 `/dev/ptmx` + ioctl（`pkg/pty/pty.go`，**仅 Linux 构建**）；出于安全不提供宿主 shell。
- **Composerize**：原生 Go 的 `docker run` → compose 转换器（shell 风格分词 + 完整 flag 表），`internal/composerize/`。
- **i18n**：zh-CN 为基语言（260 个扁平 key），en-US 通过 `Record<MsgKey,string>` 类型强制对齐；响应式 `t()`，`localStorage` 持久化。
- **主题**：首屏前内联脚本设置 `data-theme`（无 FOUC），跟随系统直到手动选择；终端/编辑器/日志区恒暗色。

### 2.4 文档 vs 代码偏差

2026-09-12 文档对齐后，已知偏差**清零**。历史偏差（均已修复，留档备查）：

| 历史偏差 | 处置 |
|---|---|
| README 声称 CodeMirror 6 编辑器 | 实为 textarea；README 已改为如实描述（CodeMirror 为登记债务） |
| README 引用 `TODO.md` / `ARCHITECTURE.md` / `DOCKGE_COMPARISON.md` / `openspec/project.md` / `openspec/README.md` | 均不存在；死链已移除，改为指向 `docs/` |
| README 声称提供宿主 shell 终端 | 与 Q5 决策相反；已纠正 |
| README API 表含不存在的 `GET /v1/docker/stats`，缺 `/v1/auth/config`、`/v1/oidc/*`、`/v1/2fa`、`/v1/health` 等 | 已按 `http.go` 实际路由重写 |
| REQUIREMENTS 称登录限速 20/min 全局 | 实为 10/min IP+账号；v1.2 已修正 |
| `docs/requirements-v2.md` 与主需求文档编号体系冲突 | 已并入 REQUIREMENTS v1.2 后删除 |

---

## 3. 能力规格清单（含测试映射）

状态图例：✅ 已实现并测试 · 🟡 已实现未测试 · 🟠 后端完成前端缺 UI · ❌ 未实现 · ⛔ 刻意移除

### 3.1 认证与安全

| 能力 | 规格（SHALL） | 状态 | 测试证据 | 实现证据 |
|---|---|---|---|---|
| 首启向导 | 首次启动 SHALL 锁定全部端点（白名单外 403），直至设置管理员 | ✅ | `web_test.go`（SetupRequired ×3 + ProxyAuth 组合 ×1） | `middleware/` SetupRequired, `views/Setup.tsx` |
| 密码登录 + JWT | SHALL 签发绑定密码哈希的 HS256 token；IP+账号限流 10/min | 🟡 | 限流与登录服务无测试 | `service/auth.go`, `pkg/rate` |
| ~~TOTP 2FA~~ | ⛔ 已移除（2026-09-12 决策 Q7：功能冗余） | — | — | 端点/用例/模型字段/pkg-totp 全删；密码强度校验保留（`service/password.go`） |
| 免登/自动登录 | disable 模式 + auto-login（需当前密码开启） | 🟠 | 无 | `service/auth.go AutoLogin/ToggleDisableAuth` |
| 反代头认证 | SHALL 按 CIDR 受信（空列表 fail-closed），自动开户，种 httpOnly cookie | ✅ | `authproxy_test.go`（4）+ `web_test.go`（ProxyAuth ×1） | `internal/authproxy/`, `middleware/web.go` |
| OIDC SSO | SHALL 多 Provider、Auth Code + PKCE、登录页 SSO 按钮、admin_groups 提权 | 🟡 | `manager_test.go`（4，不含回调流） | `internal/authoidc/`, `handler/oidc.go`, `Login.tsx` |
| 改密失效令牌 | token SHALL 绑定 shake256(密码哈希) | 🟡 | 无 | `pkg/jwt/jwt.go` |
| 多用户管理 | SHALL 提供 admin/member 角色、用户 CRUD 端点与设置页管理 | ✅ | `user_test.go`（repo 层）+ `web_test.go`（CheckSession 接线） | 2026-09-12 F11 落地：`/v1/users*` 5 端点 + `UsersCard` UI + 自保/末位 admin 守卫 |
| 停用即时失效 | 被停用用户 token SHALL 立即失效 | ✅ | `web_test.go` TestStrictAuthChecksSession | 2026-09-12 D12 接线：StrictAuth 逐请求 CheckSession |

### 3.2 栈（Stack）管理

| 能力 | 规格（SHALL） | 状态 | 测试证据 | 实现证据 |
|--- |--- |--- |--- |--- |
| 栈列表/状态 | SHALL 区分托管栈与外部栈（只读展示），状态 0–4 对齐上游 | ✅ | `stack_test.go`（3）, `compose_test.go`（3） | `repository/{stack,compose}.go` |
| CRUD + 生命周期 | start/stop/restart/down/update；操作超时 3min、更新 10min | 🟡 | handler/service 层无测试 | `handler/stack.go`, `service/stack.go` |
| compose + .env 编辑 | SHALL 接受 4 种 compose 文件名；YAML 校验；名称规则；草稿校验闭环（语法 + compose 语义，防抖自动，行号定位） | 🟡 | 名称/渲染/校验解析有测试（`stack_validate_test.go`） | `repository/stack.go`, `service/stack.go` |
| 合并日志 | compose-logs WebSocket 流 | 🟡 | 无 | `handler/terminal.go` |
| 栈日志 SSE | `GET /v1/stacks/:name/logs/stream`（决策 Q4） | ⛔ | — | **决策关闭（2026-09-12）**：栈日志统一由 WS 终端（compose-logs）承载，孤儿前端分支与后端死链已删（F1 关闭） |
| 容器内 exec 终端 | PTY + xterm + resize 控制消息 | 🟡 | 无 | `handler/terminal.go`, `pkg/pty` |
| 主机 shell | ⛔ 刻意不提供（安全边界，决策 Q5） | — | — | `handler/terminal.go`（未知类型 400） |

### 3.3 容器 / 镜像 / 网络 / 卷 / 系统

| 能力 | 规格（SHALL） | 状态 | 测试证据 | 实现证据 |
|---|---|---|---|---|
| 容器 CRUD + 批量 + 清理 + 客户端分页（15/页） | 超出上游 | 🟡 | 前端 `pagination` 纯函数有测试；后端无 | `handler/docker.go`, `views/Containers.tsx` |
| SSE 容器状态流 | events 防抖扇出，心跳 + 末帧回放，帧仅 id/state/status | 🟡 | 后端帧字段有测试 + 前端 `container-status` 纯函数有测试 | `api/v1/container_status.go`, `push/hub.go` |
| 镜像拉取/删除/清理（sizeBytes/createdAt 结构化） | 超出上游 | 🟡 | `image_test.go`（2，解析与格式化） | `repository/image.go` |
| 网络列表（含 driver）/创建/删除/清理 | 超出上游 | 🟡 | 无 | `repository/network.go` |
| 卷列表/删除/清理 | 超出上游 | 🟡 | 无 | `repository/volume.go` |
| 主机 CPU/内存 SSE | procfs 采样，2s 帧；帧只含系统指标（2026-09-12 起移除无人消费的 per-container 采样，省去每 2s 一次的 `docker stats` 执行） | 🟡 | 无 | `repository/stats.go`, `Dashboard.tsx` |
| 磁盘占用（sizeBytes/reclaimableBytes）/ 系统信息页 | `docker df` 等价 + info | 🟡 | 无 | `repository/system.go`, `views/Sys{Df,Info}.tsx` |
| Composerize | docker run → compose（原生解析器） | ✅ | `composerize_test.go` + helpers | `internal/composerize/` |

### 3.4 平台

| 能力 | 规格（SHALL） | 状态 | 测试证据 |
|---|---|---|---|
| i18n（zh-CN 基准 / en-US，315 键） | 字典 SHALL 类型强制 key 对齐，杜绝缺译 | ✅ | 类型系统保证（无运行时测试） |
| 主题（亮/暗/跟随系统，无 FOUC） | ✅ | 无 |
| 全局搜索（Cmd/Ctrl+K）、全局环境变量、版本检查（对比上游 releases，5min 缓存，ldflags 版本） | ✅ | 无 |
| Dockerfile / 发布流程 / git tag | ❌ **均无**（`.dockerignore` 为孤儿文件） | — |

---

## 4. 刻意不做的事（Non-Goals）

以下为**设计决策**而非欠账，新需求 SHALL NOT 无视此边界：

1. **多主机/Agent 架构** —— `openspec/changes/remove-remote-agent/` 已移除 `/v1/agents*` 与 legacy `agents` bucket（有迁移测试锁定）。单机是安全边界的一部分。
2. **主机 shell 终端** —— 只提供容器内 exec 与 compose 日志（决策 Q5）。
3. **UI 框架引入** —— 手写 CSS 设计系统（`DESIGN.md`），不接受 Bootstrap/MUI 类依赖。
4. **Windows / macOS 宿主终端** —— `pkg/pty` 非 Linux 平台为 stub（构建通过，exec 终端返回不支持）；终端能力仅 Linux（决策 Q6）。

---

## 4a. 单机安全提示

- 栈管理接口等同于授予宿主机 docker 权限，请勿将服务暴露到不可信网络。
- CORS 当前回显任意 Origin 且允许凭证（D2）、WebSocket CheckOrigin 全放行（P4）——生产部署 MUST 置于反代之后或绑定 127.0.0.1。
- 迁移种子账号为弱口令 `admin/123456`（D3），首启后 MUST 立即改密。

---

## 5. 测试现状与 TDD 策略

### 5.1 现状盘点（2026-09-12 实测）

**Go（23 个测试 / 9 文件，stdlib testing，无 testify/mock）：**

| 层 | 覆盖 | 缺口 |
|---|---|---|
| middleware（httptest） | SetupRequired ×3、ProxyAuth×StrictAuth 组合 ×1 | StrictAuth 三种 token 来源、CORS 无测试 |
| authproxy | TrustedIP 匹配/fail-closed/FromViper ×4 | — |
| authoidc | Manager 条件初始化/Provider 解析/admin 映射 ×4 | OIDC 回调流、PKCE 端到端无测试 |
| repository | stack ×4（含外部栈 Get 的 compose ls 单次调用回归）、compose ×3、image ×2 | container/podman/cli（CLI 注入防护、流扫描竞态）无测试 |
| composerize | Convert ×1（表驱动） | 边界 flag 覆盖薄 |
| api/v1 / server | 状态帧字段 ×1、迁移 bucket ×1 | — |
| **handler / service / DI 装配** | **0** | **最大空白：路由契约、SSE 流、终端 WS 全裸奔** |

**前端（7 个测试 / 3 文件，node:test，仅纯函数）：** `pagination`（3）/ `container-status`（2）/ `compose-summary`（2）。无组件测试、无渲染测试、无 E2E。

**工程：** 无 CI、无覆盖率工具、无 golangci-lint、无 eslint/prettier。`make verify` = `build + test`。`make test` 与服务器构建**须在 Linux 执行**（D10）；前端 `pnpm test` 脚本已修复为 `node --test tests/*.test.ts`（D11 已关闭）。

### 5.2 TDD 纪律（SHALL）

1. **Red-Green-Refactor**：任何新能力/缺陷修复 SHALL 先写失败测试再写实现。缺陷修复的 PR SHALL 包含一个能复现该缺陷的测试。
2. **测试金字塔目标配比**：
   - **单元**（composerize、stack 解析、auth 逻辑、i18n key 对齐）—— 快，无 IO；
   - **集成**（httptest 起 Gin 路由 + 假 repository 接口，覆盖 handler→service 契约、SSE 帧、WS 协议）—— 当前最大空白，优先补齐；
   - **冒烟**（`make verify` 全绿 + 起服务后 `/health`、`/setup/need`、登录-建栈-启停一条链路）。
3. **契约即规格**：`api/v1/` 的 DTO 与响应封套是前后端契约，SHALL 有往返（round-trip）测试。
4. **每个 bug 一个回归测试**：修 bug 前先写 `TestBug_<issue>_<现象>` 让它红。
5. **不可测的必须可注入**：时间、随机、容器引擎——通过接口注入（现有 `setupChecker`/`proxyLoginService` 最小接口模式即为此），禁止在 service 层直接 `exec`。

### 5.3 覆盖率验收线（SHOULD，达成即转入强制）

| 阶段 | 门槛 |
|---|---|
| 立即 | `make test` 全绿进入 CI；新增代码不得降低现有通过数 |
| 短期 | handler/service 集成测试覆盖 §3 全部 🟡 项的失败路径（401/403/404/超时） |
| 中期 | Go 行覆盖率 ≥ 60%（`go test -cover`），composerize/stack/auth 核心包 ≥ 80% |

---

## 6. SDD 工作流规格

项目已启用 [OpenSpec](../openspec/config.yaml)（schema: spec-driven，产物 zh-CN）。变更生命周期 SHALL 为：

```
openspec-new-change（写 spec delta + tasks）
  → 人工批准
  → openspec-apply-change（实现，测试先行）
  → openspec-verify-change（对照 spec 逐条核验）
  → openspec-archive-change（合入 openspec/specs/ 主规格）
```

**文档分工（单一事实来源原则）：**

| 文档 | 职责 |
|---|---|
| `docs/PROJECT_SPEC.md`（本文） | 项目全景：As-Is 基线 + To-Be 规格 + 测试/工作流纪律 |
| `docs/REQUIREMENTS.md` | 逐条需求（R1–R4）的口述、条目、落地状态、P/F 清单与决策记录 |
| `openspec/specs/` | 经归档的**当前有效**能力规格（权威） |
| `openspec/changes/` | 进行中/已归档的增量变更 |
| `DESIGN.md` | 视觉与交互契约（Crate 设计系统） |
| `README.md` | 使用与快速上手（须与本文对齐） |
| `AGENTS.md` | AI 代理工作指引（文档地图 + 构建/测试命令 + 规范索引） |

**规则：** 规格与代码冲突时，SHALL 先改其一并同步另一，禁止长期分叉（历史偏差见 §2.4，已清零）。已归档变更 6 个：design-mock 系列 4 个（2026-09-08）、`redesign-resource-workspaces`、`remove-remote-agent`（2026-09-12 归档）。

---

## 7. 目标状态（To-Be：项目应该是什么样子）

> 本章是权威事实源；人读的图景化导航与 UI 原型见 [VISION.md](VISION.md)（其内容引用本章，不另行产生数字）。

### 7.1 工程质量门（Quality Gates）

项目 SHALL 具备一条**机器强制的验收管线**，任何变更合并前 MUST 全绿：

```
CI（GitHub Actions，运行于 Linux）:
  1. golangci-lint run            # 当前未配置 → 目标：零告警准入
  2. pnpm --dir app/dockge/web test   # 须先修复 D11 脚本
  3. web: tsc --noEmit            # 已有，须进 CI
  4. go test ./... -race -cover   # 目标 ≥60%
  5. make build                   # 单二进制可产出
```

### 7.2 交付形态

- SHALL 有 `Dockerfile`（多阶段：pnpm web-build → go build → distroless/alpine 运行），删除孤儿 `.dockerignore` 或使其生效。
- SHALL 建立发布流程：git tag（语义化版本）+ `-ldflags` 注入版本（注入机制已就绪）+ Release 附产物；版本检查端点（现在对比上游 louislam/dockge releases）SHALL 切换为对比本项目 releases。
- SHALL 修复嵌入时序：当前 `web/dist/index.html` 已提交但其引用的 hash 资产被 gitignore，直接 `go build` 会嵌入坏壳。`make build` 流程正确；SHALL 要么提交全量 dist，要么在 `go:embed` 前置校验。

### 7.3 能力补全（按优先级，对应 REQUIREMENTS 剩余 F 项）

| 优先级 | 项 | 验收场景（示例） |
|---|---|---|
| P0 | handler/service 集成测试骨架（httptest + 假 repo） | 全路由 401/403/404 契约测试绿 |
| P0 | ~~2FA 前端入口（F4）~~ | ⛔ **已移除**（2026-09-12 决策 Q7：功能冗余，整体撤销） |
| P0 | ~~多用户管理（F11：`/v1/users*` 5 端点 + Settings UI + CheckSession 接线关闭 D12）~~ | ✅ **已完成**（2026-09-12：5 端点 + UsersCard + CheckSession 接线；停用即时 401 有测试锁定） |
| P1 | ~~栈日志 SSE（F1）或显式关闭决策 Q4~~ | ✅ **已关闭**（2026-09-12 决策维持 WS 终端，删除前后端死链） |
| P1 | CORS 收敛白名单 + WS CheckOrigin 校验（D2/P4） | 非白名单 Origin 收不到 `Access-Control-Allow-Credentials` |
| P1 | auto-login 公开端点下线（P2/R1-7） | 公开组不再注册 `/v1/auto-login` |
| P1 | ~~编辑器升级 CodeMirror 6（关闭 `DESIGN.md` 债务）~~ | ✅ **已完成**（2026-09-12：YAML 高亮 + 行号 + 当前行 + undo + Tab 缩进） |
| P2 | 迁移种子密码强制首启修改（D3） | `123456` 仅存在于首启向导强制流程内 |
| P2 | 拉取/prune 流式进度与回收统计（F9） | prune 返回数量/空间入 toast |

### 7.4 应有的测试形态（金字塔示意）

```
        /  E2E 冒烟（1 条：登录→建栈→起→停→删）  \
      /   组件测试（SolidJS testing-library：表单、   \
    /      分页、终端挂载）                            \
  /        集成（httptest×Gin×假repo：路由契约、SSE/WS 协议）\
/          单元（composerize/stack/jwt/rate/i18n——已有基础最厚）\
```

当前金字塔**只有底座**，中部（handler/service 集成）是系统性缺口，应最先补齐——这也正是 TDD 里"测行为不测实现"性价比最高的层。

---

## 8. 已知债务与风险清单

| 编号 | 类型 | 描述 | 证据 | 处置建议 |
|---|---|---|---|---|
| D1 | 构建风险 | `web/dist/index.html` 引用的 hash 资产未提交，裸 `go build` 嵌入坏壳 | `.gitignore`, `web/dist/` | §7.2 |
| D2 | 安全 | CORS 中间件反射任意 Origin 且允许凭证 | `middleware/web.go:37` | §7.3 P1 |
| D3 | 安全 | 迁移种子 `admin/123456` 弱口令 | `cmd/migration` | §7.3 P2 |
| D4 | 文档 | ~~README 死链 ×4、CodeMirror 失实、限速数值不一致~~ | — | ✅ **已关闭**（2026-09-12 README 全量重写 + REQUIREMENTS v1.2） |
| D5 | 测试 | handler/service/装配层 0 测试 | §5.1 | §7.3 P0 |
| D6 | 工程 | 无 CI / lint / 覆盖率 | — | §7.1 |
| D7 | 工程 | 无 Dockerfile / 发布流程 / git tag | — | §7.2 |
| D8 | 整洁 | ~~`AGENTS.md` 为空文件；仓库根残留 166KB 原型 HTML~~ | — | ✅ **已关闭**（2026-09-12 填充 AGENTS.md、删除原型 HTML） |
| D9 | 测试 | ~~`web_test.go` 存在重复用例~~ | — | ✅ **已关闭**（现为 SetupRequired ×3 各自独立） |
| D10 | 平台 | ~~`pkg/pty` 依赖 Linux 专有 ioctl，darwin/windows 构建直接失败~~ | — | ✅ **已关闭**（按平台分文件 `pty_linux.go` + `pty_other.go` stub：非 Linux 构建通过、exec 终端返回 ErrUnsupported；2026-09-19 darwin 上验证 `go build` + `go test ./...` 全绿） |
| D11 | 工程 | ~~前端 `pnpm test` 脚本 `node --test tests/` 在 Node 24 下目录参数解析失败~~ | — | ✅ **已关闭**（2026-09-12 改为 `node --test tests/*.test.ts`） |
| D12 | 安全 | ~~`AuthService.CheckSession` 已实现但未接线 StrictAuth——被停用用户的 token 存活至 7 天 TTL~~ | — | ✅ **已关闭**（2026-09-12 StrictAuth 接线 CheckSession，随 F11 落地；TestStrictAuthChecksSession 锁定） |
| D13 | 缺陷 | 前后端 DTO 字段失配：`DfCategory`（前端 `size/reclaimable: string` vs 后端 `sizeBytes/reclaimableBytes: int`）、`ImageRow`（`size/created: string` vs `sizeBytes/createdAt: unix`）、`Networks.list`（前端 `string[]` vs 后端 `{name,driver}[]`）——SysDf/Images 对应列渲染空白、Networks 把对象当字符串编码 | `web/src/api/api.ts` ↔ `api/v1/dockge.go` | 对齐 api.ts 类型到后端结构化字段（另需前端字节格式化） |
| D14 | 整洁 | 4 个源文件纯 LOC 超 250 上限：`handler/docker.go`（340）、`service/auth.go`（330）、`repository/container.go`（313）、`service/docker.go`（257）——按职责拆分（如 docker.go 的 SSE 流处理独立成文件）需在行为锁定（D5 handler/service 测试）后进行 | 2026-09-12 冗余巡逻实测 | 先补 D5 测试再拆分；本次巡逻仅记录不重构 |
| D15 | 分层 | `AuthService.GetLatestVersion`（GitHub release 查询）归置在认证服务上，且 error 返回值恒为 nil（调用方 `VersionCheck` 的 err 分支为死路径） | `service/auth.go` ↔ `handler/docker.go` VersionCheck | 迁到 version 域并去掉恒 nil 的 error 返回；涉及接口签名变更，随 F11 一并处理 |

---

## 9. 文档维护规则

1. **变更即文档**：任何 openspec 变更归档时，SHALL 同步更新本文的 §3 状态列与 §8 债务表。
2. **偏差即债务**：发现代码与规格不符，先记入 §2.4 或 §8，再决定改哪边。
3. **本文档不是需求收集处**：新需求走 `REQUIREMENTS.md` 增章或 openspec 变更，本文只维护全景与纪律。
4. 版本号随重大结构变更递增；每次更新在文首标注日期与版本。
