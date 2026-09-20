# Dockge

**English** | [简体中文](README.zh-CN.md)

A web panel for Docker Compose stacks on a single machine — a Go rewrite of [louislam/dockge](https://github.com/louislam/dockge).

One binary, no Node runtime. The SolidJS frontend is embedded in the Go executable — copy it to the server and run.

## Why

- Your compose files stop scattering across the server. All stacks live in one directory (`/opt/stacks` by default).
- Edit compose.yaml in the browser. It gets validated before saving — typos surface immediately, not when the stack half-starts.
- compose.yaml stays a plain file on disk. The single source of truth. Quit Dockge anytime; `docker compose` still works.
- Single-host only, on purpose. Not another Portainer.
- podman and nerdctl work too (auto-detected; compose command configurable).

## Quick start

```bash
mkdir -p /opt/stacks
docker run -d --name dockge --restart unless-stopped \
  -p 5001:5001 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /opt/stacks:/opt/stacks \
  -v dockge-data:/app/data \
  makeshit/dockge:0.1
```

Open `http://<host>:5001`, create the admin account. Done.

Three footguns:

1. No docker socket mounted — the panel loads, every stack action fails.
2. `/opt/stacks` not bind-mounted — stacks die with the container.
3. (Solved) The JWT key is generated randomly on first start and persisted — nothing to configure.

All three trigger startup warnings and show up in the panel's self-check.

## Also

- Binaries for linux/darwin, amd64/arm64: [Releases](https://github.com/dockge-go/dockge/releases) — `./dockge-server` (zero config, sensible defaults)
- From source: `make verify`, `make run`
- Forgot your password: `docker exec -it dockge dockge-server reset-password`
- Zero env vars needed: mount `/your/stacks:/opt/stacks` and you're done. All knobs exist as `DOCKGE_*` env vars for special cases — `DOCKGE_HTTP_PORT`, `DOCKGE_STACKS_DIR`, `DOCKGE_SECURITY_JWT_KEY`, `DOCKGE_CONTAINER_CLI`, `DOCKGE_CONTAINER_COMPOSE`, `DOCKGE_LOG_*`

MIT License.
