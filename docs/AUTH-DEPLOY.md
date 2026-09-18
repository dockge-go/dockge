# 认证部署指南：OIDC 拦截登录 与 Traefik 反向代理

> 本文回答「让 Dockge 被 OIDC 拦截登录 / 放在 Traefik 后面需要做哪些操作」。交互式闭环演练见 [prototype/auth-flow.html](prototype/auth-flow.html)，行为规格与代码事实见 [REQUIREMENTS.md](REQUIREMENTS.md) R2 节。配置项均对应 `config/dockge/*.yml` 实际读取的键。

## 0. 选型：三种接入方式

| 方式 | `security.auth.mode` | 适合 | 谁来认证 |
|---|---|---|---|
| **A. 直连 OIDC** | `oidc` | 面板直接暴露（或仅过一层路由），由面板自己跳 IdP | Dockge ↔ IdP（Auth Code + PKCE） |
| **B. Traefik 反代 + Authelia** | `proxy` | 已有 Authelia/Authentik 统一门户 | Traefik forwardAuth 委托门户，Dockge 收身份头 |
| **C. Traefik 路由 + 直连 OIDC** | `oidc` | TLS 由 Traefik 终止，认证仍由面板完成 | 同 A，仅需配对外部地址 |

任何模式都保留本地密码登录兜底通道（管理员锁死逃生门）。

---

## 场景 A：直连 OIDC（以 Authentik 为例）

### A1. IdP 侧操作（Authentik）

1. 管理界面 → **Applications → Providers → Create** → 类型选 **OAuth2/OpenID Provider**；
2. 关键字段：
   - Authorization flow: `implicit-consent`（或显式授权）；
   - Client type: **Confidential**；
   - Redirect URI: `https://dockge.example.com/v1/oidc/authentik/callback`
     （格式固定为 `{外部地址}/v1/oidc/{provider-id}/callback`，provider-id 即你 yml 里的键名）；
   - Scopes 勾选 `openid`、`profile`、`email`；
3. 记下 **Client ID** 与 **Client Secret**；
4. 组映射：建组 `dockge-admin` 并把管理员加进去（名字任意，与 yml 的 `admin_groups` 对应即可）。

> Keycloak 差异：创建 Client 后在 *Settings* 里把 **Valid redirect URIs** 填同一格式；`issuer` 即 realm 地址 `https://kc.example.com/realms/<name>`；组需通过 mapper 放入 `groups` claim。

### A2. Dockge 侧配置

```yaml
http:
  host: 0.0.0.0        # 监听所有接口（若前面有反代）
  port: 5001
  base_url: https://dockge.example.com   # ✦ 关键：外部可达地址，OIDC redirect 由此生成；
                                         #   不配置则回退 http.host:port（内网地址会导致回调失败）

security:
  auth:
    mode: oidc
    oidc:
      providers:
        authentik:                      # ← 这个键名进入回调 URL，改了须同步 IdP 侧
          label: "Authentik"            # 登录页按钮文案
          issuer: https://idp.example.com/application/o/dockge/
          client_id: <Client ID>
          client_secret: <Client Secret>
          username_claim: preferred_username   # 可选，默认即此
          groups_claim: groups                 # 可选，默认即此
          scopes: [openid, profile, email]     # 可选，默认即此
          admin_groups: [dockge-admin]         # 命中任一组 → admin 角色
```

### A3. 验证

1. `make run`（或部署后）访问 `https://dockge.example.com/login`；
2. 登录页应出现「Authentik」按钮 → 点击 → IdP 授权页 → 授权 → 回到面板仪表盘；
3. 失败时（如 state 校验失败）应回到登录页并显示红色错误条，而非裸 JSON——若仍见 JSON 说明版本早于 2026-09-12 修复；
4. 查看启动日志：`init oidc provider "authentik"` 无报错即发现成功（构造时即连 IdP 做 discovery，配置错会启动失败，这是有意为之）。

---

## 场景 B：Traefik 反向代理 + Authelia（proxy 模式）

链路：`浏览器 → Traefik(:443) → forwardAuth(Authelia /api/verify) → Remote-User 头 → Dockge(:5001) → 本地会话 cookie`。

### B1. docker-compose.yml（三件套）

```yaml
networks:
  proxy-net:

services:
  traefik:
    image: traefik:v3.7
    command:
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --entrypoints.web.address=:80
      - --entrypoints.websecure.address=:443
      - --entrypoints.websecure.http.tls=true
      # 官方建议：在 EntryPoint 层声明受信代理，替代已弃用的 trustForwardHeader
      # - --entrypoints.websecure.forwardedHeaders.trustedIPs=172.18.0.0/16
    ports: ["80:80", "443:443"]
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    networks: [proxy-net]

  authelia:
    image: authelia/authelia:latest
    volumes: ["./authelia:/config"]
    environment:
      TZ: Asia/Shanghai
    labels:
      - traefik.enable=true
      - traefik.http.routers.authelia.rule=Host(`auth.example.com`)
      - traefik.http.routers.authelia.entrypoints=websecure
      - traefik.http.routers.authelia.tls=true
      - traefik.http.services.authelia.loadbalancer.server.port=9091
    networks: [proxy-net]

  dockge:
    image: ghcr.io/yourname/dockge:latest     # 或本地构建（见附录最小 Dockerfile）
    volumes:
      - ./dockge-data:/storage                # bbolt + stacks 目录
      - ./dockge.yml:/config/dockge.yml:ro
    labels:
      - traefik.enable=true
      - traefik.http.routers.dockge.rule=Host(`dockge.example.com`)
      - traefik.http.routers.dockge.entrypoints=websecure
      - traefik.http.routers.dockge.tls=true
      - traefik.http.services.dockge.loadbalancer.server.port=5001
      # 核心：forwardAuth 委托 Authelia 校验，未登录 302 门户
      - traefik.http.middlewares.dockge-auth.forwardauth.address=http://authelia:9091/api/verify?rd=https%3A%2F%2Fauth.example.com
      # 通过校验后，把 Authelia 响应头复制进转发给 Dockge 的请求
      - traefik.http.middlewares.dockge-auth.forwardauth.authResponseHeaders=Remote-User,Remote-Groups,Remote-Email
      - traefik.http.routers.dockge.middlewares=dockge-auth@docker
    networks: [proxy-net]
```

> 标签写法与动态配置文件等价；文件方式则在 `http.middlewares.dockge-auth.forwardAuth` 下写同样的 `address` / `authResponseHeaders`（参数语义见 [Traefik 官方 ForwardAuth 文档](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/forwardauth/)）。较新的 Traefik v3 还支持 `authSigninURL` 直接指定 401 跳转地址。

### B2. Dockge 侧配置（dockge.yml）

```yaml
http:
  host: 0.0.0.0
  port: 5001

data:
  db:
    user:
      dsn: /storage/dockge.db
dockge:
  stacks_dir: /storage/stacks

security:
  auth:
    mode: proxy
    proxy:
      # ★ 必填：Traefik 容器所在的 Docker 网段。留空 = fail-closed，所有代理认证被拒绝。
      # 查法：docker network inspect <网络名> | grep -A3 Containers   （或直接用网段）
      trusted_proxies: ["172.18.0.0/16"]
      # ★ 必填（与上面 labels 的 authResponseHeaders 对应）：
      username_header: Remote-User
      # 首次遇到的门户用户自动开户；false 则仅允许已存在账号
      auto_provision: true
```

**两处必须与 Traefik 对齐**：`username_header` ↔ `authResponseHeaders` 里的头名；`trusted_proxies` ↔ Traefik 容器实际网段。任何一处不匹配 → 401。

### B3. Authelia 侧要点（configuration.yml）

```yaml
session:
  domain: example.com        # 顶层域：门户 cookie 需覆盖 auth. 与 dockge. 两个子域
  cookie: { name: authelia_session }
access_control:
  rules:
    - domain: dockge.example.com
      policy: one_factor     # 或 two_factor
authentication_backend: { … }   # 按你的用户目录配置
```

### B4. 验证

1. 未登录访问 `https://dockge.example.com/` → 被 302 到 `auth.example.com` 门户；
2. 门户登录后回跳 → 直接进入 Dockge（无面板登录页）；侧栏底部引擎状态亮绿；
3. `docker exec dockge …`（或 bbolt 查询）确认自动开户的用户存在、来源为 `proxy`；
4. 攻击面自检：`curl -H "Remote-User: admin" http://<dockge主机>:5001/v1/me` → 401（伪造头 + 非受信来源 = fail-closed）。

---

## 场景 C：Traefik 只做路由，认证走 OIDC

组合 A + B 的路由部分：去掉 `dockge-auth` 中间件标签，Dockge 配置按 **A2**（`mode: oidc` + `http.base_url: https://dockge.example.com`）。适用于没有 Authelia、但需要 TLS/域名的部署。

---

## 故障排查

| 症状 | 最可能原因 | 处置 |
|---|---|---|
| **镜像拉取挂起/超时** | 主机访问 Docker Hub 被网络阻断（面板只是转发了 `docker pull` 的结果） | 给 Docker 配置镜像加速后重启引擎：Docker Desktop → Settings → Docker Engine；OrbStack → `~/.orbstack/config/docker.json`；Linux → `/etc/docker/daemon.json`。加入 `"registry-mirrors": ["https://docker.1ms.run", "https://docker.1panel.live", "https://docker.m.daocloud.io"]`（2026-09 实测可用，失效时自行更换）。先用 `docker pull hello-world` 验证主机连通，通了面板即可拉取 |
| 仪表盘「实时统计不可用」 | 数据源 `/proc` 仅 Linux 提供；面板运行在 macOS/Windows 或无 procfs 的环境 | 属预期降级（连接保持重试）；生产部署在 Linux 主机即恢复实时 CPU/内存 |
| 启动即失败 `init oidc provider … discover` | issuer 写错 / IdP 不可达 | 核对 issuer（Authentik 为 `…/application/o/<slug>/`，Keycloak 为 realm 地址） |
| IdP 报 `redirect_uri mismatch` | 回调地址不符 | 核对 IdP 侧 Redirect URI = `base_url + /v1/oidc/<id>/callback`；确认 `http.base_url` 已配置 |
| 回调后停在裸 JSON 错误页 | 版本早于 2026-09-12 | 更新（已修复为 302 回登录页带错误提示） |
| proxy 模式永远 401 | `trusted_proxies` 网段不含 Traefik 容器 IP | `docker network inspect` 查实际网段；或忘了配（空 = fail-closed，属预期） |
| proxy 模式 401 但链路看似正常 | `username_header` 与 `authResponseHeaders` 名字不一致 | 统一为 `Remote-User`（或自定义并两边同步） |
| 门户登录了仍 302 回门户 | Authelia session domain 未覆盖 dockge 子域 | `session.domain` 配顶层域 `example.com` |
| SSE / 终端连不上（oidc/proxy 模式） | 旧版本曾要求 ?token= | 同源请求自动带 cookie，当前版本无需处理；检查反代是否缓冲 SSE（Traefik 默认支持流式） |

## 附录：最小 Dockerfile（正式发布流程未建前的过渡样例）

```dockerfile
FROM node:24-alpine AS web
WORKDIR /src
COPY app/dockge/web ./
RUN corepack enable && pnpm install --frozen-lockfile && pnpm build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
COPY --from=web /src/dist app/dockge/web/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/dockge ./app/dockge/cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/dockge /dockge
VOLUME /storage
ENTRYPOINT ["/dockge", "-conf", "/config/dockge.yml"]
```

> 注：正式的发布流程（版本注入、多架构、Release 产物）是 VISION 路线图 P2 项（债务 D7），此 Dockerfile 仅为立即可用的过渡样例。
