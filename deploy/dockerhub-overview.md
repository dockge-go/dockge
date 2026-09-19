# Dockge

单机版 Docker Compose 栈管理器：在网页里创建、编辑、启停、查看日志，compose.yaml 始终留在你自己的目录里。

## 快速开始

```yaml
services:
  dockge:
    image: makeshit/dockge:latest
    container_name: dockge
    restart: unless-stopped
    ports:
      - "5001:5001"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /opt/stacks:/opt/stacks
      - dockge-data:/app/data
volumes:
  dockge-data:
```

不想写文件的话：

```bash
docker run -d --name dockge -p 5001:5001 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /opt/stacks:/opt/stacks \
  -v dockge-data:/app/data \
  makeshit/dockge
```

访问 `http://<主机>:5001`，首次进入引导创建管理员账号（无默认口令）。

## 三件必须知道的事

1. **栈在 `/opt/stacks`**：容器内外同一路径，compose 文件在宿主上直接可见、可直接编辑，不用本面板了也还在。要换目录就同时改挂载与 `DOCKGE_STACKS_DIR`。
2. **必须挂 `/var/run/docker.sock`**：这等于把宿主 root 权限交给该容器，请只在可信主机运行。忘了挂也不会白屏——启动日志与 `/v1/health` 会直接告诉你是 `Cannot connect to the Docker daemon`。
3. **端口与反代**：容器内固定 5001；映射到宿主其他端口只改冒号左侧（如 `"15001:5001"`），反代场景在面板设置里填外部主机名。

时区、栈目录、容器运行时（docker / podman / nerdctl）等都已在面板内可设，镜像默认无需任何环境变量。

## 忘记管理员密码

```bash
docker compose stop dockge          # 数据库一次只允许一个进程访问，须先停服务
docker compose run --rm dockge reset-password
docker compose start dockge
```

`reset-password` 会依次询问用户名、新密码（≥6 位且含字母和数字，与网页端同一规则）。
