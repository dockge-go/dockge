# Dockge

Manage single-host Docker Compose stacks from your browser: edit YAML, deploy services, follow logs and open container terminals. One administrator, with password or OIDC sign-in.

[Getting started](https://github.com/dockge-go/dockge#readme) · [Deployment / OIDC](https://github.com/dockge-go/dockge/blob/main/deploy/README.md)

Use the [Compose manifest](https://github.com/dockge-go/dockge/blob/main/deploy/compose.yaml), replace its local image with a published `makeshit/dockge` version, and remove `build`. Pin a version or digest. The GHCR mirror is `ghcr.io/dockge-go/dockge`.

Mount the runtime socket and stack directory. Internal paths are `/opt/stacks`, `/app/data`, port `5001`; change only external mount paths and ports. The two-mount layout requires version `0.2.6` or newer. Account data persists in `stacks/.dockge-data`; include hidden files in backups. Existing explicit data mounts remain supported.

There is no default password. Create the administrator on first visit, restrict access and use HTTPS. Runtime socket access grants extensive host privileges. Removing the panel does not remove managed workloads or data.

Stack deletion lists exclusive and retained shared/external resources before removal. Dashboard disk usage describes the stack directory's filesystem, not every host disk.

## 简体中文

在浏览器中管理单机 Docker Compose 栈：编辑 YAML、部署服务、查看日志、进入容器终端。一个管理员，可使用密码或 OIDC 登录。

[快速开始](https://github.com/dockge-go/dockge/blob/main/README.zh-CN.md) · [部署与 OIDC](https://github.com/dockge-go/dockge/blob/main/deploy/README.md)

仅挂载运行时 socket 和栈目录，内部固定为 `/opt/stacks`、`/app/data`、端口 `5001`。两挂载布局需 `0.2.6` 或更新版本。备份栈时包含 `.dockge-data`；旧的独立数据挂载仍支持。

没有默认密码，首次访问创建管理员。请限制入口并使用 HTTPS；运行时 socket 提供较高的宿主权限。卸载面板不自动删除栈或业务数据。删除栈前核对清理范围；共享和外部资源保留。磁盘用量表示栈目录所在文件系统。
