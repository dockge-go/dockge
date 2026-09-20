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
3.（已解决）JWT 密钥首次启动随机生成并持久化，无需配置。

三条出问题时启动日志都会告警，面板自检也会标出来。

## 其他

- linux / darwin（amd64 / arm64）二进制：[Releases](https://github.com/dockge-go/dockge/releases)，`./dockge-server`（零配置即跑）
- 源码构建：`make verify`、`make run`
- 忘记密码：`docker exec -it dockge dockge-server reset-password`
- 零环境变量：挂载 `/你的栈目录:/opt/stacks` 即用——容器内路径是固定约定，永不改变。裸机二进制（无挂载概念）才用 `DOCKGE_*` 环境变量：`DOCKGE_STACKS_DIR`、`DOCKGE_HTTP_PORT`、`DOCKGE_SECURITY_JWT_KEY`、`DOCKGE_CONTAINER_CLI`、`DOCKGE_CONTAINER_COMPOSE`、`DOCKGE_LOG_*`

MIT License.
