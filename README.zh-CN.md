# Dockge

[English](README.md) | **简体中文**

单机 Docker Compose 栈管理面板 —— [louislam/dockge](https://github.com/louislam/dockge) 的 Go 重写。

一个二进制，没有 Node 运行时。SolidJS 前端直接嵌进 Go 可执行文件，拷到服务器就能跑。

## 为什么

- compose 文件不再散落服务器各处：所有栈集中在一个目录（默认 `/opt/stacks`）。
- 浏览器里改 compose.yaml，保存前先校验——写错当场标出来，不等部署时才炸。
- compose.yaml 始终是磁盘上的普通文件，唯一事实来源。哪天不用 Dockge 了，`docker compose` 照样能跑。
- 只做单机，不做又一个 Portainer。
- podman / nerdctl 也能跑（自动探测；compose 命令可单独配）。

## 快速开始

```bash
mkdir -p /opt/stacks
docker run -d --name dockge --restart unless-stopped \
  -p 5001:5001 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /opt/stacks:/opt/stacks \
  -v dockge-data:/app/data \
  makeshit/dockge:0.1
```

打开 `http://<主机IP>:5001`，建管理员账号，完事。

三个坑：

1. 不挂 docker socket——面板能开，栈操作全废。
2. `/opt/stacks` 不挂宿主机——删容器，栈文件跟着没。
3. 默认 JWT 密钥就是字面上的 `change-me-in-production`，对外部署前用 `APP_SECURITY_JWT_KEY` 换掉。

三条出问题时启动日志都会告警，面板自检也会标出来。

## 其他

- linux / darwin（amd64 / arm64）二进制：[Releases](https://github.com/dockge-go/dockge/releases)，`./dockge-server`（零配置即跑，可选 `-conf`）
- 源码构建：`make verify`、`make run`
- 忘记密码：`docker exec -it dockge dockge-server reset-password`
- 常改的配置：端口（`http.host` / `http.port`）、栈目录（`dockge.stacks_dir`）、运行时（`container.cli`、`container.compose`）

MIT License.
