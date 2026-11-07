# Dockge

Single-host Compose stack management, rewritten in Go from [louislam/dockge](https://github.com/louislam/dockge). One executable with an embedded SolidJS frontend. Create, edit, validate and deploy YAML; view container logs and open container terminals. One administrator, optional OIDC, Apple System light/dark themes. No Prometheus dependency.

## Deployment

Use the mounts and server-metric files in the [deployment manifest](https://github.com/dockge-go/dockge/blob/main/deploy/compose.yaml). Replace its local build image with the verified version of this published image and omit the `build` section. Pin a version or digest matching the release you intend to deploy.

Internal paths and port are fixed: `/opt/stacks`, `/app/data`, `5001`. Change only host-side bind paths and ports. The runtime socket grants near-root access to the host; only trusted administrators should access the panel. Protect the endpoint with HTTPS.

On first visit, create the sole administrator. There is no default password. For a forgotten local password, restrict access, stop the panel, back up the database and stacks, move `/app/data/dockge.db` to a secure backup location, then restart and create a new administrator. This resets account/settings and old sessions, not stack files or runtime containers. Never modify an open database. OIDC account recovery belongs to the identity provider.

Stack deletion previews exclusive resources and retained shared/external resources. Uninstalling the panel does not automatically delete managed workloads or host data. Server disk metrics describe the filesystem containing the stack directory, not all host disks.

[Source and documentation](https://github.com/dockge-go/dockge) · [Kubernetes/OIDC deployment](https://github.com/dockge-go/dockge/blob/main/deploy/kubernetes/README.md)

## 简体中文

单机 Compose 栈管理面板，Go 后端与内嵌 SolidJS 前端。支持 YAML 新增、编辑、校验、部署、删除，容器日志与容器终端。单管理员，可配置 OIDC，使用 Apple System 明暗主题，不依赖 Prometheus。

请参考[部署清单](https://github.com/dockge-go/dockge/blob/main/deploy/compose.yaml)配置运行时、数据卷和只读宿主指标文件，将本地构建镜像替换为本镜像已验证的版本并移除 `build` 部分。生产固定版本或摘要。

内部路径和端口保持 `/opt/stacks`、`/app/data`、`5001`；只修改宿主侧映射。运行时 socket 提供接近宿主 root 的权限，须限制访问并配置 HTTPS。

首次访问创建唯一管理员，无默认密码。忘记本地密码时，先限制入口并停机，备份栈与数据库，再将 `/app/data/dockge.db` 移到安全备份位置，重启后重新初始化。这会重置账号、设置并使旧会话失效，不删除栈文件或运行时容器。不要修改正在使用的数据库；OIDC 密码在身份提供方重置。

删除栈前核对专属资源和共享保留清单。卸载面板不会自动删除栈或宿主数据。服务器磁盘指标表示栈目录所在文件系统，而非所有磁盘总量。
