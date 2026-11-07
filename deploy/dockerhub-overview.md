# Dockge

Single-host Compose stack management, rewritten in Go from [louislam/dockge](https://github.com/louislam/dockge). One executable with an embedded SolidJS frontend. Create, edit, validate and deploy YAML; view container logs and open container terminals. One administrator, optional OIDC, Apple System light/dark themes. No Prometheus dependency.

## Deployment

Use only the runtime-socket and stacks mounts in the [deployment manifest](https://github.com/dockge-go/dockge/blob/main/deploy/compose.yaml). Replace its local build image with `makeshit/dockge:0.2.6` and omit the `build` section. The GHCR mirror is `ghcr.io/dockge-go/dockge:0.2.6` (private packages require credentials). Pin a version or digest. The two-mount layout is supported starting with 0.2.6; old 0.2.5 images lack the required entrypoint.

Internal paths and port are fixed: `/opt/stacks`, `/app/data`, `5001`. Change only host-side bind paths and ports. The runtime socket grants near-root access to the host; only trusted administrators should access the panel. Protect the endpoint with HTTPS.

On first visit, create the sole administrator. There is no default password. For a forgotten local password, restrict access, stop the panel, back up the database and stacks, move `/app/data/dockge.db` to a secure backup location, then restart and create a new administrator. This resets account/settings and old sessions, not stack files or runtime containers. Never modify an open database. OIDC account recovery belongs to the identity provider.

The image persists `/app/data` through `stacks/.dockge-data`; include hidden files in backups. Existing explicit data mounts remain supported. Native Linux procfs supplies kernel-global metrics without extra mounts; virtualized or unavailable sources are reported, not replaced with container values. Only stacks-directory configuration folders count as stacks.

Stack deletion previews exclusive resources and retained shared/external resources. Uninstalling the panel does not automatically delete managed workloads or host data. Server disk metrics describe the filesystem containing the stack directory, not all host disks.

[Source and documentation](https://github.com/dockge-go/dockge) · [Kubernetes/OIDC deployment](https://github.com/dockge-go/dockge/blob/main/deploy/kubernetes/README.md)

## 简体中文

单机 Compose 栈管理面板，Go 后端与内嵌 SolidJS 前端。支持 YAML 新增、编辑、校验、部署、删除，容器日志与容器终端。单管理员，可配置 OIDC，使用 Apple System 明暗主题，不依赖 Prometheus。

请参考[部署清单](https://github.com/dockge-go/dockge/blob/main/deploy/compose.yaml)配置运行时 socket 和 stacks 两个挂载，将本地构建镜像替换为 `makeshit/dockge:0.2.6` 并移除 `build` 部分。GHCR 同版本为 `ghcr.io/dockge-go/dockge:0.2.6`（私有包需凭据）。生产固定版本或摘要。两挂载布局从 0.2.6 起支持，旧 0.2.5 缺少新入口，不能直接套用。

内部路径和端口保持 `/opt/stacks`、`/app/data`、`5001`；只修改宿主侧映射。运行时 socket 提供接近宿主 root 的权限，须限制访问并配置 HTTPS。

首次访问创建唯一管理员，无默认密码。忘记本地密码时，先限制入口并停机，备份栈与数据库，再将 `/app/data/dockge.db` 移到安全备份位置，重启后重新初始化。这会重置账号、设置并使旧会话失效，不删除栈文件或运行时容器。不要修改正在使用的数据库；OIDC 密码在身份提供方重置。

新镜像将 `/app/data` 连接到 stacks 内的 `.dockge-data`，备份须包含隐藏目录；旧的独立数据挂载继续兼容。标准 Linux procfs 提供真实内核计数，无需额外挂载；虚拟化或不可用来源明确报错，不以容器数据代替。编排栈仅统计 stacks 内配置目录，面板自身和目录外项目不纳入。

删除栈前核对专属资源和共享保留清单。卸载面板不会自动删除栈或宿主数据。服务器磁盘指标表示栈目录所在文件系统，而非所有磁盘总量。
