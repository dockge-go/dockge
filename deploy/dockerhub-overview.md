# Dockge

**English** | [简体中文](#dockge-简体中文)

Self-hosted Docker Compose stack manager for a single machine — a Go rewrite of [louislam/dockge](https://github.com/louislam/dockge). One binary, no Node runtime: create, edit, start/stop and view logs of compose stacks in the browser, while compose.yaml stays a plain file in your own directory.

## Quick start

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

Or without a compose file:

```bash
docker run -d --name dockge -p 5001:5001 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /opt/stacks:/opt/stacks \
  -v dockge-data:/app/data \
  makeshit/dockge
```

Open `http://<host>:5001` and create the admin account on first visit (no default credentials).

## Three things you must know

1. **Stacks live in `/opt/stacks`** — same path inside and outside the container. Compose files stay on your host, editable by hand, and survive the panel. To move it, change the bind mount and `DOCKGE_STACKS_DIR` together.
2. **Mounting `/var/run/docker.sock` equals handing host root to this container** — run it on trusted machines only. If you forget the mount, the panel still loads: startup logs and `/v1/health` will tell you `Cannot connect to the Docker daemon`.
3. **Ports and reverse proxies** — the in-container port is fixed at 5001; to change the host port edit only the left side (`"15001:5001"`). Behind a reverse proxy, set the external hostname in panel settings.

Timezone, stacks directory and container runtime (docker / podman / nerdctl) are all configurable in the panel — the image needs no environment variables.

## Forgot the admin password

```bash
docker compose stop dockge          # the embedded DB allows a single process; stop first
docker compose run --rm dockge reset-password
docker compose start dockge
```

`reset-password` asks for the username and a new password (≥6 chars with letters and digits, same rule as the web UI).

# Dockge 简体中文

[English](#dockge) | **简体中文**

单机版 Docker Compose 栈管理器：[louislam/dockge](https://github.com/louislam/dockge) 的 Go 重写版。单个二进制、无 Node 运行时：在网页里创建、编辑、启停、查看日志，compose.yaml 始终留在你自己的目录里。

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
