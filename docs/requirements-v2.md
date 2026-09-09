# Dockge 需求规格文档

> 生成时间：2026-09-09  
> 状态：草稿（待用户确认后进入实施计划）

---

## 一、项目现状速览

| 维度 | 现状 |
|------|------|
| 后端语言 | Go 1.26，Gin 框架 |
| 存储 | bbolt（用户/设置）、文件系统（栈 compose.yaml） |
| 认证方式 | JWT（HS256）+ bcrypt + shake256 密码绑定 |
| 前端框架 | SolidJS（非 VanJS，README 已过期） |
| UI 风格基线 | Apple System（DESIGN.md 已有色板/字体/间距规范） |
| 容器引擎 | Podman v6（优先）+ docker CLI（降级） |
| 终端 | WebSocket + PTY（xterm.js） |
| 包管理 | samber/do/v2 DI、spf13/viper 配置 |

---

## 二、需求清单

### 需求 R-01：强制首次设置密码（首次启动锁定）

**目标**：未初始化时，除 `/v1/setup` 与 `/v1/setup/need` 以外的所有接口均应拒绝访问，直到完成管理员账号创建。

**当前行为**：`CheckNeedSetup` → `CountUsers == 0` 时返回 `needSetup=true`；前端 `/login` 页 `onMount` 重定向到 `/setup`。现有中间件 `StrictAuth` 对 `/login`、`/setup` 等公开端点放行，其他端点强制 JWT。

**要求**：

1. 后端新增中间件 `SetupRequired`（或在现有路由分组上加守卫），当 `needSetup=true` 时，除以下路径外全部返回 403：
   - `GET /v1/setup/need`
   - `POST /v1/setup`
   - `GET /v1/health`
   - `GET /robots.txt`
2. 前端 `/login` 页收到 `needSetup=true` 时无条件跳转 `/setup`，不渲染登录表单。
3. `/setup` 完成并写入首个用户后，后续请求正常经过 JWT 鉴权。
4. `reset-password` 命令（`cmd/reset-password`）在 setup 完成前依然可用（CLI 工具不走 HTTP）。

**验收条件**：
- 全新安装（无用户）访问 `/` → 重定向到 `/setup`
- 访问 `/v1/stacks`（无 token）→ 返回 403 而非 401
- 完成 setup 后正常登录流程不变

---

### 需求 R-02：中间件拦截鉴权方案（Traefik / Authelia / OIDC）

**目标**：支持通过反向代理传递已认证用户身份，避免重复登录；同时兼容纯内部 JWT 模式。

#### 2.1 中间件层设计

新增 `middleware/proxy_auth.go`，支持三种模式（由配置 `security.auth.mode` 控制）：

```go
// auth_mode 枚举
const (
    AuthModeJWT      = "jwt"       // 默认：JWT Bearer 头
    AuthModeProxy    = "proxy"     // 反向代理传递 X-Auth-* 头
    AuthModeOIDC     = "oidc"      // OIDC userinfo 端点验证 ID Token
    AuthModeDisabled = "disable"   // 免登录（已有 disableAuth 设置项）
)
```

**Proxy 模式**：读取请求头（可配置）：
- `X-Forwarded-User` / `Remote-User` → 用户名
- `X-Forwarded-Email` → 邮箱（可选）
- `X-Forwarded-Name` → 显示名（可选）

中间件将识别的用户名在用户表中查找或自动创建（`AutoProvision`），签发短期 JWT 后写入 `Set-Cookie`（httpOnly + SameSite=Lax），前端 JS 不可读（防 XSS 窃取）。

**OIDC 模式**：
- 支持标准 OIDC Discovery（`.well-known/openid-configuration`）
- 配置字段：`issuer`、`client_id`、`client_secret`、`scopes`
- 流程：前端重定向 → OIDC Provider → 回调端点 `/v1/oidc/callback` → 换取 ID Token → 验签 → 签发内部 JWT

```yaml
# config/dockge/local.yml 扩展
security:
  auth:
    mode: jwt           # jwt | proxy | oidc | disable
    proxy:
      username_header: X-Forwarded-User
      email_header: X-Forwarded-Email     # 仅读取，不持久化
      name_header: X-Forwarded-Name       # 仅读取，不持久化
      auto_provision: true                # 自动创建不存在用户
      default_role: viewer                # viewer | admin
    oidc:
      providers:                          # 多 Provider，key 为内部标识
        github:
          label: "GitHub"
          issuer: "https://github.com"
          client_id: ""
          client_secret: ""
          scopes: ["openid", "profile", "email"]
          username_claim: preferred_username
          role_claim: ""
        custom:
          label: "企业 SSO"
          issuer: ""
          client_id: ""
          client_secret: ""
          scopes: ["openid", "profile"]
          username_claim: email
```

#### 2.2 前端适配

- `/login` 页在 `security.auth.mode == proxy` 时提供"使用单点登录"按钮（隐藏密码字段）
- `OIDC` 模式下登录按钮触发 `window.location = /v1/oidc/auth`
- JWT cookie 模式下，前端 `api.ts` 的 `request()` 自动携带 `credentials: 'include'`（CORS 开启时）
- SSE/WebSocket 连接改用 cookie（无需 `?token=` 查询参数，但仍保留作为兼容）

**验收条件**：
- `mode=proxy` 时，Traefik/Authelia 设置 `X-Forwarded-User` 头，用户无需密码直接登录
- `mode=oidc` 时，跳转 OIDC Provider 登录，回调后自动建立会话
- `mode=jwt` 时（默认），原有行为完全不变

---

### 需求 R-03：后端 API 完整对齐前端原型

**目标**：逐接口核对前端 `api.ts` 调用与后端路由/handler/DTO 的一致性，消除缺口与歧义。

#### 3.1 现有 API 对照表

| 前端调用 | 后端路由 | 状态 | 备注 |
|----------|---------|------|------|
| `POST /v1/login` | `authHandler.Login` | ✅ | 含 2FA 中间态 |
| `POST /v1/setup` | `authHandler.Setup` | ✅ | |
| `GET /v1/setup/need` | `authHandler.NeedSetup` | ✅ | |
| `POST /v1/auto-login` | `authHandler.AutoLogin` | ⚠️ | 位于 `strictAuthRouter` 但无需认证，存在逻辑矛盾 |
| `GET /v1/me` | `authHandler.Me` | ✅ | |
| `PUT /v1/me/password` | `authHandler.ChangePassword` | ✅ | |
| `POST /v1/me/2fa/enable` | `authHandler.Enable2FA` | ✅ | 前端入口暂缓 |
| `DELETE /v1/me/2fa` | `authHandler.Disable2FA` | ✅ | 前端入口暂缓 |
| `GET /v1/me/disableauth` | `authHandler.GetDisableAuth` | ✅ | 前端入口暂缓 |
| `POST /v1/me/disableauth` | `authHandler.ToggleDisableAuth` | ✅ | 前端入口暂缓 |
| `GET /v1/stacks` | `stackHandler.List` | ✅ | |
| `POST /v1/stacks` | `stackHandler.Create` | ✅ | |
| `GET /v1/stacks/:name` | `stackHandler.Get` | ✅ | |
| `PUT /v1/stacks/:name` | `stackHandler.Update` | ✅ | |
| `DELETE /v1/stacks/:name` | `stackHandler.Delete` | ✅ | |
| `POST /v1/stacks/:name/:op` | `stackHandler.Op` | ✅ | op=start|stop|restart|down|update |
| `GET /v1/stacks/:name/logs` | 缺失 | ❌ | 前端未直接调用，日志在 WebSocket |
| `GET /v1/stacks/:name/logs/stream` | 缺失 | ❌ | 前端未直接调用，SSE 由 terminal 承载 |
| `GET /v1/docker/version` | `dockerHandler.Version` | ✅ | |
| `GET /v1/docker/info` | `dockerHandler.Info` | ✅ | |
| `GET /v1/docker/containers` | `dockerHandler.Containers` | ✅ | 含分页 |
| `GET /v1/docker/containers/stream` | `dockerHandler.ContainerStatusStream` | ✅ | SSE |
| `GET /v1/docker/containers/:id/inspect` | `dockerHandler.ContainerInspect` | ✅ | |
| `POST /v1/docker/containers/:id/start` | `dockerHandler.StartContainer` | ✅ | |
| `POST /v1/docker/containers/:id/stop` | `dockerHandler.StopContainer` | ✅ | |
| `POST /v1/docker/containers/:id/restart` | `dockerHandler.RestartContainer` | ✅ | |
| `DELETE /v1/docker/containers/:id` | `dockerHandler.RemoveContainer` | ✅ | |
| `POST /v1/docker/containers/prune` | `dockerHandler.PruneContainers` | ✅ | |
| `GET /v1/docker/images` | `dockerHandler.DockerImages` | ✅ | |
| `DELETE /v1/docker/images/:id` | `dockerHandler.RemoveImage` | ✅ | |
| `POST /v1/docker/images/pull` | `dockerHandler.PullImage` | ✅ | |
| `POST /v1/docker/images/prune` | `dockerHandler.PruneImages` | ✅ | |
| `GET /v1/docker/networks` | `dockerHandler.Networks` | ✅ | |
| `GET /v1/docker/networks/:name` | `dockerHandler.NetworkInspect` | ✅ | |
| `DELETE /v1/docker/networks/:name` | `dockerHandler.RemoveNetwork` | ✅ | |
| `POST /v1/docker/networks/create` | `dockerHandler.NetworkCreate` | ✅ | |
| `POST /v1/docker/networks/prune` | `dockerHandler.PruneNetworks` | ✅ | |
| `GET /v1/docker/volumes` | `dockerHandler.DockerVolumes` | ✅ | |
| `DELETE /v1/docker/volumes/:name` | `dockerHandler.RemoveVolume` | ✅ | |
| `POST /v1/docker/volumes/prune` | `dockerHandler.PruneVolumes` | ✅ | |
| `GET /v1/docker/stats/stream` | `dockerHandler.StatsStream` | ✅ | SSE |
| `GET /v1/docker/df` | `dockerHandler.DockerDf` | ✅ | |
| `GET /v1/settings/globalenv` | `settingsHandler.GetGlobalEnv` | ✅ | |
| `PUT /v1/settings/globalenv` | `settingsHandler.SetGlobalEnv` | ✅ | |
| `POST /v1/composerize` | `composerizeHandler.Convert` | ✅ | |
| `GET /v1/version/check` | `dockerHandler.VersionCheck` | ✅ | |
| `GET /v1/terminal/:name/:type` | `terminalHandler.WebSocket` | ✅ | name=stackName; type=host\|compose-logs\|exec |

#### 3.2 缺口与修复项

1. **`/v1/auto-login` 注册位置错误**：位于 `strictAuthRouter`（需 JWT）下，但语义上无需认证。**修复**：移至 `noAuthRouter`。
2. **缺少栈日志 REST 端点**：前端使用 WebSocket 终端承载，但若需要独立 REST 日志端点（支持 tail 参数），需补充。当前状态：**前端未调用，暂不实现**。
3. **`/v1/docker/containers` 缺少分页参数**：前端已实现客户端分页，后端返回全量即可，但大数据集时（>1000容器）应支持 `?limit=&offset=`。当前状态：**暂不实现，记录为增强项**。
4. **OIDC 路由缺失**：`/v1/oidc/auth`、`/v1/oidc/callback` 尚未定义。**见需求 R-02**。

---

### 需求 R-04：UI 风格统一为 Unix Tools × Apple System

**目标**：在前端现有 Apple 设计语言基础上，融入 Unix 工具美学（终端感、信息密度、monospace 优先）。

#### 4.1 字体规范（已在 `app.css` 中定义，需强化）

```css
/* 现有 */
--font-display: "SF Pro Display", "SF Pro Icons", "Helvetica Neue", Helvetica, Arial, sans-serif;
--font-body: "SF Pro Text", "SF Pro Icons", "Helvetica Neue", Helvetica, Arial, sans-serif;
--font-mono: "SF Mono", ui-monospace, "JetBrains Mono", Menlo, Monaco, Consolas, monospace;
```

**增强规则**：
- 所有数据展示（ID、镜像名、路径、命令）强制使用 `--font-mono`
- 状态值（running/exited）使用系统字体 + 状态色点，不使用 mono
- 堆栈名称（stack name）使用 `--font-display`，保持可读性
- 终端面板保持纯 mono，不受主题影响

#### 4.2 交互细节

| 元素 | Unix 风格 | Apple 风格 | 取舍 |
|------|----------|-----------|------|
| 边框 | 1px solid，无明显圆角 | 圆角 8-12px | 取 `--radius-sm`（8px），边框保持 1px |
| 阴影 | 无或极弱 | `--elev-raised` | 卡片仅 hover 时加 shadow，默认 flat |
| 按钮 | 方角为主 | 圆角 | 主操作保留 pill，次操作用 sm-radius |
| 表格 | 行分隔明显，斑马纹可选 | 简洁分隔线 | 保留当前简洁分隔线，不加斑马纹 |
| 终端区 | 纯黑底，明亮前景色 | — | 保持 `#0d0d0d` 深底 |
| 状态指示 | 文字前加 Unicode 符号 | 彩色点 | 保留彩色圆点（更清晰） |

#### 4.3 具体改造项

1. **顶栏**：在右上角加入"系统信息"快捷入口（当前仅在 `/sysinfo` 页面），显示 Docker/Podman 版本、引擎连接状态
2. **侧栏**：增加键盘快捷键提示（如 `1` ~ `9` 快速跳转），tooltip 形式展示
3. **表格**：关键操作列（删除）使用更醒目的红色，hover 时显示整行高亮而非仅行背景
4. **Toast**：保持现有深色风格，增加"撤销"能力（仅删除类操作）
5. **认证页**：在密码登录下方增加"SSO 登录"按钮（R-02 完成后启用）
6. **响应式**：768px 以下侧栏全屏覆盖，详情 Sheet 占满宽度

**验收条件**：
- 所有 mono 数据字段使用 SF Mono 渲染
- 暗黑模式下终端区保持 `#0d0d0d` 底色
- 移动端下侧栏可收起，详情占据全屏

---

## 三、配置扩展（`config/dockge/*.yml`）

```yaml
security:
  jwt:
    key: "change-me-for-production"
    token_ttl_hours: 168          # 默认 7 天
  auth:
    mode: jwt                     # jwt | proxy | oidc | disable
    proxy:
      username_header: X-Forwarded-User
      email_header: X-Forwarded-Email
      name_header: X-Forwarded-Name
      auto_provision: true
      default_role: admin
    oidc:
      issuer: ""
      client_id: ""
      client_secret: ""
      scopes: ["openid", "profile", "email"]
      username_claim: email
      redirect_url: ""            # 自动推导：https://<host>/v1/oidc/callback
```

---

## 四、数据模型变更

### 4.1 `DockgeUser` 新增字段

```go
// app/dockge/internal/model/stack.go
type DockgeUser struct {
    // ... 现有字段 ...
    Role       string `json:"role"`       // "admin" | "viewer"（新增）
    ExternalID string `json:"external_id,omitempty"` // OIDC provider 返回的 sub，用于防重复 provision
    Source     string `json:"source"`     // "local" | "oidc" | "proxy"（新增）
}
```

> Proxy 模式仅使用用户名，不存储邮箱/显示名（已确认）。

### 4.2 `settings` bucket 新增键

| Key | Type | 说明 |
|-----|------|------|
| `auth.mode` | string | `jwt` / `proxy` / `oidc` / `disable` |
| `oidc.issuer` | string | OIDC Issuer URL |
| `oidc.client_id` | string | |
| `oidc.scopes` | string | JSON 数组字符串 |

---

## 五、实施顺序建议

| 阶段 | 内容 | 依赖 |
|------|------|------|
| P0 | R-01 强制首次设置密码（路由修正 + 403 守卫） | 无 |
| P0 | R-03 API 对照修复（`/auto-login` 路由位置修正） | 无 |
| P1 | R-02 Proxy 模式中间件（`X-Forwarded-User`，仅用户名） | P0 |
| P1 | R-04 UI 细节加固（mono 字体、终端深底不变） | P0 |
| P2 | R-02 OIDC 多 Provider（providers 配置、回调流程、前端选择器） | P1 |
| P2 | R-03 栈日志 REST 端点（可选增强） | P1 |

---

## 六、已确认事项

| # | 问题 | 决策 |
|---|------|------|
| 1 | Proxy 自动 provision 用户是否存邮箱/名字 | **否**，仅用用户名映射，不持久化额外字段 |
| 2 | OIDC 是否支持多 Provider | **是**，配置可定义多个 named issuer，前端提供选择器 |
| 3 | 侧栏键盘快捷键 | **不做**，连 tooltip 也不加 |
| 4 | R-01 优先独立实施 | **不做**，与 R-02 一并处理 |

---

## 七、OIDC 多 Provider 设计（更新）

### 7.1 配置结构

```yaml
security:
  auth:
    mode: jwt              # jwt | proxy | oidc | disable
    proxy:
      username_header: X-Forwarded-User
      auto_provision: true
    oidc:
      providers:           # 可定义多个命名 provider
        github:
          label: "GitHub"
          issuer: "https://github.com"
          client_id: ""
          client_secret: ""
          scopes: ["openid", "profile", "email"]
          username_claim: preferred_username
          role_claim: ""
        custom:
          label: "企业 SSO"
          issuer: "https://sso.example.com"
          client_id: ""
          client_secret: ""
          scopes: ["openid", "profile"]
          username_claim: email
```

### 7.2 路由

| 路径 | 方法 | 说明 |
|------|------|------|
| `GET /v1/oidc/providers` | 公开 | 返回可用 provider 列表（含 label） |
| `GET /v1/oidc/:provider/auth` | 公开 | 重定向到 OIDC Provider 授权页 |
| `GET /v1/oidc/:provider/callback` | 公开 | 接收 code，换取 ID Token，验签，签发内部 JWT cookie |
| `POST /v1/oidc/:provider/logout` | 需认证 | 清除内部会话，可选重定向到 provider logout URL |

### 7.3 前端登录页变化

- 当 `mode=oidc` 且有 providers 时，登录页显示 provider 选择按钮列表
- 点击后跳转 `/v1/oidc/:provider/auth`
- 回调回来后存入 httpOnly cookie，前端通过 `/v1/me` 验证会话
- 同时保留密码登录入口（mode 可混合 `jwt+oidc`）

### 7.4 `DockgeUser.Source` 字段

新增字段区分来源，便于审计：
```go
type DockgeUser struct {
    // ... 现有字段 ...
    Source     string `json:"source"`     // "local" | "oidc" | "proxy"
    ExternalID string `json:"external_id,omitempty"` // OIDC 时记录 provider+sub
}
```
