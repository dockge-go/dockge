# Dockge 需求规格文档

> 更新时间：2026-09-09
> 状态：已实施（auth 子系统重构完成）

---

## 一、项目现状速览

| 维度 | 现状 |
|------|------|
| 后端语言 | Go 1.26，Gin 框架 |
| 存储 | bbolt（用户/设置）、文件系统（栈 compose.yaml） |
| 认证方式 | JWT（HS256）/ 反向代理头 / OIDC（Auth Code + PKCE）/ 免登录，由 `security.auth.mode` 统一控制 |
| 前端框架 | SolidJS（非 VanJS，README 已过期） |
| UI 风格基线 | Apple System（DESIGN.md 已有色板/字体/间距规范） |
| 容器引擎 | Podman v6（优先）+ docker CLI（降级） |
| 终端 | WebSocket + PTY（xterm.js） |
| 包管理 | samber/do/v2 DI、spf13/viper 配置 |

---

## 二、需求清单

### 需求 R-01：强制首次设置密码（首次启动锁定）

**目标**：未初始化时，除 setup 引导与健康检查外的所有接口均拒绝访问。

**实现**：`middleware/web.go` → `SetupRequired` 中间件，挂载到 `noAuthRouter`，当 `CountUsers == 0` 时对非白名单路径返回 403。

**白名单路径**（不需要用户存在即可访问）：

| 路径 | 说明 |
|------|------|
| `GET /v1/setup/need` | 查询是否需要初始化 |
| `POST /v1/setup` | 创建首个管理员 |
| `GET /v1/health` | 健康检查 |
| `GET /robots.txt` | 爬虫协议 |
| `POST /v1/auto-login` | 免登录模式下的自动登录 |

**前端行为**：`Login.tsx` 调用 `GET /v1/auth/config` 获取 `disableAuth` 和 `providers`；`GET /v1/setup/need` 返回 `needSetup=true` 时跳转 `/setup`。

**验收条件**：
- 全新安装（无用户）访问 `/v1/stacks` → 403
- 完成 setup 后正常登录流程不变
- `reset-password` 命令（CLI）在 setup 完成前依然可用

---

### 需求 R-02：多模式认证（JWT / Proxy / OIDC / Disable）

**目标**：统一认证模式枚举，支持四种模式由配置 `security.auth.mode` 控制。

#### 2.1 认证模式枚举

```go
// app/dockge/internal/security/security.go
type Mode string
const (
    ModeJWT     Mode = "jwt"     // 默认：本地 JWT 登录
    ModeProxy   Mode = "proxy"   // 受信反向代理头认证
    ModeOIDC    Mode = "oidc"    // OIDC 授权码 + PKCE
    ModeDisable Mode = "disable" // 免登录（配合 disableAuth 设置项）
)
```

`security.AuthMode(conf)` 读取配置，非法值回退到 `jwt`。

#### 2.2 Proxy 模式

**配置命名空间**：`security.auth.proxy.*`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `trusted_proxies` | `[]string`（CIDR） | `[]` | 受信来源网段；为空时拒绝一切代理认证（fail-closed） |
| `username_header` | `string` | `X-Forwarded-User` | 携带用户名的请求头 |
| `auto_provision` | `bool` | `true` | 是否自动创建不存在的本地用户 |

**安全模型**：
- 仅当 `Request.RemoteAddr` 命中 `trusted_proxies` 中的 CIDR 时才读取身份头（`authproxy.TrustedIP`）
- 未配置任何受信网段时一律拒绝，杜绝「无 CIDR 也信任代理头」的开放风险
- 非法 CIDR 静默忽略（fail-closed）

**流程**：`middleware/web.go` → `ProxyAuth` 中间件（仅当 `mode=proxy` 时挂载到 `strictAuthRouter`）读取请求头 → `authService.ProxyLogin` 验证来源 IP + 映射/创建本地用户 → 写入 httpOnly `dockge_token` cookie + `ctxProxyAuthDone` 标记 → `StrictAuth` 据此跳过 JWT 校验。

#### 2.3 OIDC 模式

**配置命名空间**：`security.auth.oidc.providers.<id>.*`

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `label` | `string` | `""` | 登录页展示名（为空时显示 "SSO"） |
| `issuer` | `string` | 必填 | IdP issuer URL |
| `client_id` | `string` | 必填 | OIDC client ID |
| `client_secret` | `string` | 必填 | OIDC client secret |
| `scopes` | `[]string` | `[]` | 授权 scope（代码中无默认值，需显式配置） |
| `username_claim` | `string` | `""` | 优先使用 `preferred_username`，回退 `email` |
| `groups_claim` | `string` | `""` | 优先使用 `groups` |
| `admin_groups` | `[]string` | `[]` | 命中任一组时自动授予 admin 角色 |

**流程**：
1. `GET /v1/oidc/:provider/auth`：生成 state + PKCE verifier（verifier 仅存 Provider 内存），以 `oidc_state` httpOnly cookie 绑定浏览器，302 重定向到 IdP 授权页
2. `GET /v1/oidc/:provider/callback`：校验 code + state（与 cookie 比对），换取并验签 ID Token，提取 username + admin 标志
3. `authService.OIDCLogin`：查找或自动创建本地用户（OIDC 始终自动 provision），签发本地 JWT
4. 清除 `oidc_state` cookie，写入 httpOnly `dockge_token` cookie，302 重定向到 `/`

**PKCE**：`S256` code challenge，verifier 随机 32 字节 base64url。

**启动优化**：`authoidc.NewManager` 仅当 `mode=oidc` 时执行 OIDC Discovery 网络请求；其他模式返回空管理器，避免启动时不必要的网络调用。

#### 2.4 Disable 模式

`GET /v1/me/disableauth` 读取开关；`POST /v1/me/disableauth` 切换（开启免登录需校验当前密码）。

`POST /v1/auto-login`：仅当 `disableAuth=true` 时以首个活跃用户自动登录，否则 401。

#### 2.5 JWT 模式（默认）

`POST /v1/login` 校验用户名密码，签发绑定密码哈希摘要的 JWT。开启 2FA 的账号先返回中间令牌，再通过 `POST /v1/2fa` 校验 TOTP 验证码。

#### 2.6 前端适配

- `GET /v1/auth/config`（公开端点）返回 `{mode, providers: [{id, info:{label}}], disableAuth}`
- `Login.tsx` 根据 `mode` 和 `providers` 渲染 SSO 按钮列表
- OIDC 按钮触发 `window.location = /v1/oidc/:provider/auth`（`type="submit"`）
- `api.ts` 的 `request()` 自动携带 `credentials: 'include'`（CORS 已开启）
- `store/index.ts` 的 `authed` 初始值为 `false`，`boot()` 无条件尝试 `api.me()`
- 所有 API 请求 401 时无条件调用 `unauthorizedHandler()` 清除状态

#### 2.7 会话管理

- Cookie 名：`dockge_token`，httpOnly + SameSite=Lax
- Session TTL：7 天（`service.SessionTTL = time.Hour * 24 * 7`）
- JWT claims 绑定密码哈希摘要（`shake256`），改密后旧 token 自动失效
- 前端 localStorage key 保持 `TOKEN_KEY = "crate_token"` 不变
- 登录限流：每 IP+账号 10 次/分钟，超出返回 401

---

### 需求 R-03：后端 API 完整对齐前端原型

#### 3.1 认证相关端点

| 前端调用 | 后端路由 | 认证 | 说明 |
|----------|---------|------|------|
| `POST /v1/login` | `authHandler.Login` | 公开 | 含 2FA 中间态 |
| `POST /v1/setup` | `authHandler.Setup` | 公开 | 仅当用户数为 0 |
| `GET /v1/setup/need` | `authHandler.NeedSetup` | 公开 | 返回 `needSetup` |
| `POST /v1/2fa` | `authHandler.Check2FA` | 公开 | TOTP 验证码校验 |
| `POST /v1/auto-login` | `authHandler.AutoLogin` | 公开 | 仅 disableAuth=true 时有效 |
| `GET /v1/auth/config` | `settingsHandler.AuthConfig` | 公开 | 返回 mode/providers/disableAuth |
| `GET /v1/health` | `dockerHandler.Health` | 公开 | 健康检查 |
| `GET /v1/robots.txt` | inline handler | 公开 | `User-agent: * Disallow: /` |
| `GET /v1/oidc/:provider/auth` | `oidcHandler.Auth` | 公开 | 重定向到 OIDC Provider |
| `GET /v1/oidc/:provider/callback` | `oidcHandler.Callback` | 公开 | OIDC 回调，签发 cookie |
| `GET /v1/me` | `authHandler.Me` | 需认证 | 当前用户信息 |
| `PUT /v1/me/password` | `authHandler.ChangePassword` | 需认证 | 修改密码 |
| `POST /v1/me/2fa/enable` | `authHandler.Enable2FA` | 需认证 | 启用 2FA |
| `DELETE /v1/me/2fa` | `authHandler.Disable2FA` | 需认证 | 禁用 2FA |
| `GET /v1/me/disableauth` | `authHandler.GetDisableAuth` | 需认证 | 读取免登录开关 |
| `POST /v1/me/disableauth` | `authHandler.ToggleDisableAuth` | 需认证 | 切换免登录模式 |

#### 3.2 栈管理端点

| 前端调用 | 后端路由 | 说明 |
|----------|---------|------|
| `GET /v1/stacks` | `stackHandler.List` | 栈列表（含外部栈与状态） |
| `POST /v1/stacks` | `stackHandler.Create` | 创建栈 |
| `GET /v1/stacks/:name` | `stackHandler.Get` | 栈详情 |
| `PUT /v1/stacks/:name` | `stackHandler.Update` | 保存文件 |
| `DELETE /v1/stacks/:name` | `stackHandler.Delete` | 删除栈 |
| `POST /v1/stacks/:name/:op` | `stackHandler.Op` | op=start|stop|restart|down|update |

#### 3.3 Docker 资源端点

| 前端调用 | 后端路由 | 说明 |
|----------|---------|------|
| `GET /v1/docker/version` | `dockerHandler.Version` | 引擎版本摘要 |
| `GET /v1/docker/info` | `dockerHandler.Info` | 仪表盘汇总 |
| `GET /v1/docker/containers` | `dockerHandler.Containers` | 容器列表（含分页） |
| `GET /v1/docker/containers/stream` | `dockerHandler.ContainerStatusStream` | SSE 状态流 |
| `GET /v1/docker/containers/:id/inspect` | `dockerHandler.ContainerInspect` | 容器详情 |
| `GET /v1/docker/containers/:id/logs` | `dockerHandler.ContainerLogs` | 容器日志 |
| `POST /v1/docker/containers/:id/start` | `dockerHandler.StartContainer` | 启动容器 |
| `POST /v1/docker/containers/:id/stop` | `dockerHandler.StopContainer` | 停止容器 |
| `POST /v1/docker/containers/:id/restart` | `dockerHandler.RestartContainer` | 重启容器 |
| `DELETE /v1/docker/containers/:id` | `dockerHandler.RemoveContainer` | 删除容器 |
| `POST /v1/docker/containers/prune` | `dockerHandler.PruneContainers` | 清理已停止容器 |
| `GET /v1/docker/images` | `dockerHandler.DockerImages` | 镜像列表 |
| `DELETE /v1/docker/images/:id` | `dockerHandler.RemoveImage` | 删除镜像 |
| `POST /v1/docker/images/pull` | `dockerHandler.PullImage` | 拉取镜像 |
| `POST /v1/docker/images/prune` | `dockerHandler.PruneImages` | 清理未使用镜像 |
| `GET /v1/docker/networks` | `dockerHandler.Networks` | 网络列表 |
| `GET /v1/docker/networks/:name` | `dockerHandler.NetworkInspect` | 网络详情 |
| `DELETE /v1/docker/networks/:name` | `dockerHandler.RemoveNetwork` | 删除网络 |
| `POST /v1/docker/networks/create` | `dockerHandler.NetworkCreate` | 创建网络 |
| `POST /v1/docker/networks/prune` | `dockerHandler.PruneNetworks` | 清理未使用网络 |
| `GET /v1/docker/volumes` | `dockerHandler.DockerVolumes` | 数据卷列表 |
| `DELETE /v1/docker/volumes/:name` | `dockerHandler.RemoveVolume` | 删除数据卷 |
| `POST /v1/docker/volumes/prune` | `dockerHandler.PruneVolumes` | 清理未使用数据卷 |
| `GET /v1/docker/stats/stream` | `dockerHandler.StatsStream` | SSE 系统统计 |
| `GET /v1/docker/df` | `dockerHandler.DockerDf` | 磁盘使用 |

#### 3.4 其他端点

| 前端调用 | 后端路由 | 说明 |
|----------|---------|------|
| `GET /v1/settings/globalenv` | `settingsHandler.GetGlobalEnv` | 全局环境变量 |
| `PUT /v1/settings/globalenv` | `settingsHandler.SetGlobalEnv` | 写入全局环境变量 |
| `POST /v1/composerize` | `composerizeHandler.Convert` | docker run → compose |
| `GET /v1/version/check` | `dockerHandler.VersionCheck` | 版本更新检查 |
| `GET /v1/terminal/:name/:type` | `terminalHandler.WebSocket` | 终端 WebSocket（host|compose-logs|exec） |

---

## 三、配置参考（`config/dockge/*.yml`）

```yaml
security:
  jwt:
    key: change-me-for-production   # JWT 签名密钥
  auth:
    mode: jwt                       # jwt | proxy | oidc | disable
    proxy:
      trusted_proxies: []           # 受信 CIDR 列表（为空时拒绝一切代理认证）
      username_header: X-Forwarded-User
      auto_provision: true          # 自动创建不存在用户
    oidc:
      providers:                    # 多 Provider，key 为内部标识
        github:
          label: "GitHub"
          issuer: "https://github.com"
          client_id: ""
          client_secret: ""
          scopes: ["openid", "profile", "email"]
          username_claim: preferred_username
          groups_claim: ""
          admin_groups: []
        enterprise:
          label: "企业 SSO"
          issuer: "https://sso.example.com"
          client_id: ""
          client_secret: ""
          scopes: ["openid", "profile"]
          username_claim: email
          groups_claim: groups
          admin_groups: ["admin", "ops"]
```

---

## 四、会话与安全模型

### 4.1 会话生命周期

| 维度 | 值 |
|------|-----|
| Cookie 名 | `dockge_token` |
| Cookie 属性 | httpOnly, SameSite=Lax, Path=/ |
| TTL | 7 天（`SessionTTL = 24h * 7`） |
| JWT 签名 | HS256 |
| 绑定机制 | claims 中包含 `shake256(password)` 摘要 |

### 4.2 Token 传递链

`StrictAuth` 中间件按优先级读取 token：
1. `Authorization: Bearer <token>` 请求头
2. `?token=<token>` 查询参数（WebSocket/SSE 兼容）
3. `dockge_token` httpOnly cookie（proxy/OIDC 模式）

### 4.3 登录限流

- 滑动窗口：每 `IP|username` 组合 10 次/分钟
- 超限后返回 `401 Unauthorized`
- 键容量上限：4096

### 4.4 外部身份映射

Proxy 和 OIDC 模式通过 `externalLogin` 公共方法映射到本地用户：
- 查找已有本地用户（按 username 匹配）
- 不存在时若 `auto_provision=true`（Proxy）或 OIDC 模式，自动创建
- 自动创建的用户密码为随机 256 位值，不可用于密码登录
- OIDC 创建的用户可由 `admin_groups` 配置授予 admin 角色

### 4.5 路由分组

| 组名 | 中间件 | 用途 |
|------|--------|------|
| `noAuthRouter` | `SetupRequired` | 公开端点（登录/setup/OIDC/health/auth config） |
| `strictAuthRouter` | `StrictAuth` + 可选 `ProxyAuth` | 需认证端点（所有数据操作） |

`ProxyAuth` 仅在 `mode=proxy` 时挂载，完成后设置 `ctxProxyAuthDone=true`，`StrictAuth` 据此跳过 JWT 校验。

---

## 五、数据模型

### 5.1 `DockgeUser` 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | `uint` | 用户 ID |
| `Username` | `string` | 用户名（唯一） |
| `Nickname` | `string` | 显示名 |
| `Password` | `string` | bcrypt 密码哈希（外部用户为随机值） |
| `Role` | `string` | `"admin"` / `"member"`（空值按 admin 展示） |
| `Active` | `bool` | 是否启用 |
| `TwofaStatus` | `bool` | 2FA 是否启用 |
| `TwofaSecret` | `string` | TOTP 密钥 |
| `TwofaLastToken` | `string` | 上次使用的 TOTP（防重放） |
| `Source` | `string` | `"local"` / `"oidc"` / `"proxy"` |

---

## 六、源码映射

| 模块 | 文件 | 职责 |
|------|------|------|
| 认证模式枚举 | `internal/security/security.go` | `Mode` 类型 + `AuthMode(conf)` |
| Proxy 配置 + IP 校验 | `internal/authproxy/authproxy.go` | `Config`, `FromViper`, `TrustedIP` |
| OIDC 配置解析 | `internal/authoidc/config.go` | `ProviderConfig`, `ParseProviders`, `adminFor` |
| OIDC Provider 管理 | `internal/authoidc/manager.go` | `Manager`, `NewManager`（条件网络发现） |
| OIDC Provider 实现 | `internal/authoidc/provider.go` | Discovery, PKCE, token 验签, claims 提取 |
| OIDC State 管理 | `internal/authoidc/state.go` | nonce + PKCE verifier 存储 |
| 认证服务 | `internal/service/auth.go` | Login, ProxyLogin, OIDCLogin, session, 2FA |
| OIDC HTTP 处理 | `internal/handler/oidc.go` | Auth (redirect), Callback (code→cookie) |
| 设置/认证配置 | `internal/handler/settings.go` | `AuthConfig` 端点 |
| 中间件 | `internal/middleware/web.go` | StrictAuth, ProxyAuth, SetupRequired, CORS |
| 路由注册 | `internal/server/http.go` | 路由分组、中间件挂载 |
| 前端 API 层 | `web/src/api/api.ts` | credentials:include, 401 处理 |
| 前端状态管理 | `web/src/store/index.ts` | authed 初始 false, boot() |
| 前端登录页 | `web/src/views/Login.tsx` | mode 感知, SSO 按钮列表 |
| 配置文件 | `config/dockge/{local,prod}.yml` | security.auth.* 完整配置 |
| DTO 清理 | `api/v1/dockge.go` | 已移除 ExternalAuthStatusData, OIDCExchangeRequest |

---

## 七、测试覆盖

| 测试文件 | 覆盖范围 |
|----------|---------|
| `internal/authproxy/authproxy_test.go` | TrustedIP CIDR 匹配、fail-closed、空列表、IPv4-mapped |
| `internal/authoidc/manager_test.go` | NewManager 条件初始化（非 oidc 模式返回空）、ProviderIDs 排序 |
| `internal/middleware/web_test.go` | StrictAuth（header/cookie/query 三种来源）、ProxyAuth（trusted IP/非 trusted/空 header）、SetupRequired（白名单/需 setup/已有用户） |

---

## 八、非目标（不做）

| 项 | 决策 |
|----|------|
| 侧栏键盘快捷键 | 不做 |
| 前端 stub 目录 (`web/stub/`) | 不创建 |
| 纯 API stub 模式 | 不实现 |
| 额外 JWT claims 字段 | 不添加 |
| OIDC 用户资料持久化 | 不做（仅 username 映射） |
| Proxy 模式存储邮箱/显示名 | 不做 |
| `rollup-plugin-visualizer` | 不安装 |
