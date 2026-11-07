# Dockge

[English](README.md) | **简体中文**

单机 Compose 栈管理面板，[louislam/dockge](https://github.com/louislam/dockge) 的 Go 重写。后端使用 Gin、bbolt 和 samber/do，SolidJS 前端内嵌在单个可执行文件中。

## 功能

- 在浏览器中新增、编辑、校验、保存和部署 `compose.yaml`；配置保留在文件系统中。
- 管理本机容器、镜像、卷和网络，查看容器日志并进入容器终端。
- 删除栈前列出清理范围；删除专属容器、卷、网络、镜像和栈目录。共享资源、external 资源及栈外绑定目录保留并列出原因。
- Apple System 明暗主题；仪表盘显示 CPU、内存、磁盘、负载、交换空间和运行时长。
- 单管理账号；支持 OIDC。不提供多用户、远程 Agent 或 Kubernetes 工作负载管理，不依赖 Prometheus。

## 从当前源码部署

需要 Docker Engine 与 Compose 插件；源码构建需要 Go（版本见 `go.mod`）、Node.js 24、pnpm 12 和 Python 3。通过 Dockerfile 构建时不需要在宿主安装 Go 或 Node.js。

```bash
git clone https://github.com/dockge-go/dockge.git
cd dockge
mkdir -p stacks
docker compose -f deploy/compose.yaml up -d --build
```

首次访问 `http://<主机>:5001` 创建管理员账号，**没有默认密码**。生产环境请通过 HTTPS 反向代理访问，并限制首次初始化入口。

示例使用本地源码镜像 `dockge-go:local`。预构建二进制见 [Releases](https://github.com/dockge-go/dockge/releases)。使用发布镜像时，将 `deploy/compose.yaml` 中的 `image` 替换为以下任一来源，并移除 `build` 部分后执行 `docker compose -f deploy/compose.yaml up -d`：

- GHCR：`ghcr.io/dockge-go/dockge:0.2.5`
- Docker Hub：`makeshit/dockge:0.2.5`

两者支持 `linux/amd64`、`linux/arm64`。生产固定明确版本或镜像摘要，不使用 `dev`。GHCR 首次发布需确认包可见性；私有包需要拉取凭据。

### Kubernetes 部署

提供[单节点 Kubernetes 与 OIDC 部署清单](deploy/kubernetes/README.md)，包含 Deployment、Service、ConfigMap、Secret 与 HTTPS Ingress 配置示例。必须选定具有 Docker Engine socket 的唯一节点；仅有 containerd 的普通 K8s 节点无法直接运行本镜像的 Compose 功能。数据采用 hostPath 持久化，内部路径不变；单副本与 `Recreate` 防止数据库多写。按部署说明填写真实 OIDC 配置，不能直接应用含示例凭据的整个目录。

### 挂载与权限

`deploy/compose.yaml` 包含运行时 socket、栈目录、数据卷及宿主信息的只读挂载：

- 容器内部固定使用 `/opt/stacks`、`/app/data`、端口 `5001`。换栈目录只修改宿主侧，例如 `/你的栈目录:/opt/stacks`；换宿主端口只修改冒号左侧。
- Docker socket 授予面板接近宿主 root 的权限，只能向可信管理员开放。运行时不可达时，部署自检会显示具体错误。
- 栈目录必须持久化，数据库与日志位于 `/app/data`。Compose 命名卷的实际名称可能包含项目前缀，备份前应通过 `inspect` 确认。
- Compose 的绑定源由宿主运行时解析；修改宿主栈路径时，检查 YAML 中的绑定源路径。容器内终端不是宿主终端。
- 其他运行时可通过 `container.cli` / `container.compose` 配置或 PATH 探测选用 podman、nerdctl；需自行提供相应 CLI、Compose 实现和连接。默认镜像包含 Docker CLI 与 Compose 插件。

## 服务器指标

仪表盘每 2 秒采样，资源列表每 10 秒刷新。系统信息只显示 9 项主要信息。Linux 容器通过清单中的 8 个固定只读文件读取宿主指标，不挂载整个宿主 `/proc` 或根目录，也不需要 privileged、hostPID。

CPU 使用相邻计数器差分，首样本显示采样中；内存已用为 `MemTotal − MemAvailable`。磁盘显示 **`/opt/stacks` 所在文件系统** 的空间，不是所有磁盘总量或镜像大小之和。缺少文件、权限不足、请求失败或数据过期时显示未知和具体原因。隐藏页面暂停采样。

裸机 Linux 读取本机 procfs 和根分区；darwin 支持栈管理，但服务器指标采样不受支持。指标范围是面板所在主机，而非远程运行时。

## 账号与数据恢复

已知密码时，在设置页修改密码。忘记本地密码时：

1. 限制面板网络入口并停止服务，确认没有其他实例打开同一数据库。
2. 备份数据卷和栈目录；在数据卷内将 `dockge.db` 移到安全备份位置。
3. 重新启动，在受限入口创建新的唯一管理员，然后恢复访问。

这会重置账号、设置和持久化 JWT 密钥，旧会话失效；不会删除独立栈文件或运行时容器。不要在运行中修改或删除 bbolt 数据库。OIDC 账号的密码由身份提供方管理，参见 [Kubernetes 与 OIDC 部署说明](deploy/kubernetes/README.md)。

卸载面板容器不会自动删除栈和宿主数据。先逐个清理需要删除的栈并核对保留清单，再停机和处理面板数据卷。内建网络、面板自身和仍被其他对象引用的资源不能强制删除。

## 开发与验证

```bash
make verify          # 前后端构建、Go/前端/运维脚本测试
make run             # 默认端口 5001，使用约定路径
make image           # 构建 dockge-go:local
sh scripts/e2e-auth.sh
```

裸机开发也使用约定位置，通过软链或挂载连接本地数据；`DOCKGE_*` 是裸机高级覆盖，不用于更改容器内部路径。

运行中实例的冒烟测试需要显式提供会话令牌，或同时提供账号和密码：

```bash
# 使用当前 shell 中已有的令牌；不要把凭据写进脚本或提交到仓库。
DOCKGE_TOKEN="$TOKEN" make smoke
```

冒烟会创建、启停和删除一个随机名称的专属栈，默认使用 `nginx:alpine`（可通过 `SMOKE_IMAGE` 指定），不绑定宿主端口、不修改全局设置、不清理其他业务。异常退出会尝试清理专属栈；清理失败会报告栈名，需要人工核对。

推送 main 或提交 PR 会运行验证；发布标签 `v*` 经同样的构建、测试和漏洞检查后才发布二进制及镜像。手动发布流程只发布 `dev` 镜像。推送 main 本身不会发布镜像或部署到服务器。

GHCR 使用工作流的 `GITHUB_TOKEN` 与任务级 `packages: write` 权限，无需另设发布 Token。Docker Hub 使用 Actions Secrets `DOCKERHUB_USERNAME`、`DOCKERHUB_TOKEN`（目标仓库写权限），可通过 Repository Variable `DOCKERHUB_IMAGE` 覆盖默认 `makeshit/dockge`。两个镜像任务独立执行，任一仓库失败会明确报错，不阻断另一仓库。发布 tag `v0.2.5` 对应镜像标签 `0.2.5`、`0.2`、`latest`；重新手动运行该 tag 不会额外发布 `dev`。

MIT License.
