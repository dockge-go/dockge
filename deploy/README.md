# 部署与维护

[快速开始](../README.zh-CN.md) · [OIDC](#oidc-登录)

## 路径与数据

容器内部固定为 `/opt/stacks`、`/app/data`、端口 `5001`。更换目录或端口时，只修改挂载、端口映射的外部一侧。`DOCKGE_*` 路径覆盖仅用于高级裸机部署，不用于容器部署。

默认入口将 `/app/data` 连接到 `/opt/stacks/.dockge-data`，账号、设置和日志随栈目录持久化。该隐藏目录不计为栈，也不能通过栈 API 删除。旧的显式 `/app/data` 数据挂载继续使用，不自动迁移。

Compose 的 bind 源由宿主 Docker Engine 解析，不是浏览器或面板解析。绝对路径属于宿主；相对栈文件绑定要求宿主和面板能以相同路径访问文件：推荐宿主栈目录也使用 `/opt/stacks`。其他宿主目录部署包含 `./文件` 绑定的栈时，请改用宿主绝对路径。部署自检显示连接和写权限错误。

## OIDC 登录

身份提供方须支持 Authorization Code 与 PKCE S256；仅允许配置的 `subject` 登录，不按邮箱新增用户。首次 OIDC 登录在空数据库中初始化唯一管理员；已有账号则映射到该账号。启用后禁用本地密码登录、初始化、改密和免登录模式；不能通过本地接口绕过 OIDC。

在自己的 Compose 服务中添加环境配置，密钥由部署环境提供，不提交到仓库：

```yaml
environment:
  DOCKGE_OIDC_ENABLED: "true"
  DOCKGE_OIDC_ISSUER: "${OIDC_ISSUER:?设置提供方 HTTPS issuer}"
  DOCKGE_OIDC_CLIENT_ID: "${OIDC_CLIENT_ID:?设置客户端 ID}"
  DOCKGE_OIDC_CLIENT_SECRET: "${OIDC_CLIENT_SECRET:?设置客户端密钥}"
  DOCKGE_OIDC_REDIRECT_URL: "${OIDC_REDIRECT_URL:?设置 HTTPS 回调地址}"
  DOCKGE_OIDC_SUBJECT: "${OIDC_SUBJECT:?设置唯一管理员 subject}"
```

回调地址须为 `https://你的域名/v1/auth/oidc/callback`，与提供方登记值一致。使用 `openid` scope；校验 state、nonce、PKCE 和 ID Token 签名、issuer、audience、有效期。外部令牌不持久化，登录票据单次使用。提供方不可用时显示具体错误，不自动注册账号。重置外部密码应在身份提供方操作。需要退出 OIDC 时，先限制访问并停机修改配置；OIDC 初始化的账号没有可用的本地密码，应按忘记密码流程备份并重置账号库。

## 升级

先备份栈目录（含隐藏账号目录）与业务卷，然后将自己部署配置的镜像改为 `makeshit/dockge:0.2.12`，或使用已认证的 `ghcr.io/dockge-go/dockge:0.2.12`：

```sh
docker compose -f compose.yaml pull
docker compose -f compose.yaml up -d
```

发布镜像部署不要保留源码 `build` 配置。升级面板不停止业务栈；升级后核对版本、栈列表、实时指标和日志。只支持单机 Docker Compose，旧的集群部署清单不再提供。

## 备份与恢复

1. 停止面板，确认没有其他进程打开数据库。
2. 备份 stacks，**包含 `.dockge-data`**；使用旧数据卷时另行备份实际 `/app/data` 卷。
3. 另行备份业务命名卷、外部卷和栈外绑定源。只复制 YAML 不包含这些业务数据。
4. 恢复至空目标，保留权限，再启动并核对账号、配置和运行时资源。

不要复制正在写入的 bbolt 数据库，也不要覆盖未经核对的目录。

### 旧数据卷迁移

两挂载入口从 `0.2.6` 开始提供。停机备份后，将旧 `/app/data` 的全部内容复制到**空的** `stacks/.dockge-data`，保留权限，再移除旧挂载并使用支持该入口的镜像。确认原账号、设置和栈正常后再处理旧卷。Compose 卷名可能带项目前缀，操作前通过 inspect 确认实际名称。

不需要迁移时，保留旧 `/app/data` 挂载即可；不能直接移除它后期待账号自动转移。

### 忘记密码

没有默认管理员密码。已知密码可在设置页修改；忘记本地密码时：

1. 限制网络入口并停止面板。
2. 备份后，将实际数据目录中的 `dockge.db` 移到安全位置。
3. 重启，在受限入口创建唯一管理员，再恢复访问。

这会重置账号、设置和持久化 JWT 密钥，旧会话失效；栈文件和运行时容器保留。OIDC 密码由身份提供方管理，不能用本地重置绕过唯一身份限制。

## 指标与实时连接

每个页面仅建立一条 `/v1/events` SSE 概览连接，状态到达后覆盖旧值，不保存或回放事件历史。系统查询完成后间隔 2 秒再次真实采样；资源查询完成后间隔 5 秒发起下一轮。刷新和操作完成通过 `POST /v1/events/refresh` 触发新采样，数据仍走原连接；`revision` 只标记刷新边界，不是逐帧增长的事件 ID。


| 数据 | 加载时机 | 更新与释放 |
| --- | --- | --- |
| 容器运行数/总数、栈、镜像、卷、网络及资源列表 | 登录后页面可见，所有路由共享一条概览 SSE | 新采样覆盖旧状态；加载、断线或查询失败时数量显示 `—`，不冒充 `0` |
| CPU、内存、磁盘、负载 | 同一概览 SSE | 真实采样，无结果历史；CPU 首次需要两次计数 |
| 容器日志、栈合并日志、服务日志 | 点击对应入口 | 面板内实时接收，断线 3 秒重连；关闭、切页或注销即释放 |
| 容器 `sh` / `bash` | 点击终端入口 | 一次点击创建一个会话；概览更新不重建。关闭、切页、隐藏或注销结束会话；重连需要手动点击 |
| 部署与镜像下载输出 | 执行相应操作 | 跟随该次操作，不在页面初始化时启动 |

页面隐藏时关闭概览和日志连接；回到页面重新采样，仅仍打开的日志面板重新连接。容器 shell 不自动重建，避免重复执行命令。最后一个概览订阅退出后停止采样。浏览器 Network 中长连接的累计接收字节自然增长，不代表应用在缓存历史。

部署、拉镜像、日志与容器终端分别使用按需 SSE；终端输入和尺寸调整通过 POST。日志及部署输出最多保留 500 行，并有 1,048,576 字符总上限；日志重连去重也使用同样的有界窗口，关闭日志窗口即释放显示内容；终端回滚区最多 5,000 行。容器终端逐帧等待渲染完成，部署显示合并为最新待绘制值，避免高速输出积压。日志逐段解析、每帧合并绘制，已有行保持身份，不随新输出重建整段列表。

日志与终端最多同时打开 16 条连接；超过上限返回明确错误，关闭其他窗口后可重试。Linux 上 CLI 正常退出、取消和读取失败都先清理同组后代，再回收组长；重复取消不会触碰已回收的进程 ID。其他平台只保证安全终止直接进程，不保证后台后代整组清理。

- CPU 由两次内核计数差分计算，首次显示采样中；只保留上一次计数，不缓存 CPU、内存或磁盘占用的采样结果。
- 内存已用为 `MemTotal − MemAvailable`，不将容器用量加总冒充服务器用量。
- 磁盘表示 **`/opt/stacks` 所在文件系统**，不是所有宿主磁盘或镜像大小之和。
- 标准 Linux procfs 提供内核全局计数，不需要额外挂载指标文件或 Prometheus。虚拟覆盖、权限不足及时间命名空间偏移会显示具体原因。
- Docker Desktop 的内核指标属于 Linux VM，不是 macOS/Windows 宿主。darwin 的服务器采样不受支持。

从本机 Docker Engine 读取主机身份，缓存 30 秒；不将远程 Docker context 混入本机指标。旧 `/host/proc` 只读指标挂载仍兼容。

反向代理必须禁用 SSE 缓冲并允许长连接。不要缓存 `/v1/`；不要把认证令牌放入 URL。例如 Nginx 的 `/v1/` 代理：

```nginx
location /v1/ {
    proxy_pass http://127.0.0.1:5001;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}
```

其余页面与静态资源同样转发至 5001；外层配置 HTTPS。

## 删除与卸载

只在 stacks 中发现真实配置目录，不将面板自身或目录外项目作为编排栈。删除栈先预览专属资源和保留原因；共享、external、栈外绑定源不自动删除。独立资源删除仍检查引用关系，内建网络和面板自身受保护。

卸载前，逐个删除需要清理的栈、核对保留资源，再停止面板并处理数据。卸载容器不会删除宿主业务。不要用全机 prune 代替范围清晰的资源清理。

## 构建与发布

原生构建需要 Go（版本见 `go.mod`）、Node.js 24、pnpm 12 和 Python 3；Dockerfile 构建不需要在宿主安装这些工具。

```sh
make verify
make image
# 对运行中实例验证：令牌由当前 shell 提供，不写进脚本或仓库。
DOCKGE_TOKEN="$TOKEN" make smoke
```

冒烟测试创建、启停和删除一个随机名称的栈，默认镜像为 `nginx:alpine`，可通过 `SMOKE_IMAGE` 替换。不绑定宿主端口，不修改全局设置，也不清理其他业务；失败的清理会报告对象名称。

main 推送与 PR 运行 Verify；`v*` tag 通过构建、测试、漏洞检查后发布二进制和镜像。手动分支发布生成 `dev` 镜像；仅推送 main 不发布镜像或更新服务器。

- GHCR：使用任务级 `GITHUB_TOKEN` 和 `packages: write`，无需单独发布令牌。私有包拉取需 `read:packages` 凭据。
- Docker Hub：需要 Actions Secrets `DOCKERHUB_USERNAME`、`DOCKERHUB_TOKEN`；Repository Variable `DOCKERHUB_IMAGE` 可覆盖默认 `makeshit/dockge`。
- 两个仓库的发布任务独立执行。版本 tag `v0.2.12` 对应镜像 `0.2.12`、`0.2`、`latest`，重新运行该 tag 不生成 `dev`。

使用未发布的源码改动时，先构建并推送自己的固定镜像标签，再更新部署。生产不要依赖浮动 `latest`。
