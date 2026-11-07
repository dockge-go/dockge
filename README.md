# Dockge

**English** | [简体中文](README.zh-CN.md)

A single-host Compose stack manager, rewritten in Go from [louislam/dockge](https://github.com/louislam/dockge). Gin, bbolt and samber/do power the backend; the SolidJS frontend is embedded in one executable.

## Features

- Create, edit, validate, save and deploy `compose.yaml` in the browser. Files remain on disk. Only real configuration directories inside stacks count as stacks; the panel and outside Compose projects are excluded.
- Manage local containers, images, volumes and networks; view logs and open container terminals.
- Preview stack deletion. Exclusive containers, volumes, networks, images and the stack directory are removed; shared/external resources and bind sources outside the stack remain, with reasons listed.
- Apple System light/dark themes, with CPU, memory, disk, load, swap and uptime on the dashboard.
- One management account with optional OIDC. No multi-user administration, remote agents, Kubernetes workload management or Prometheus dependency.

## Deploy the current source

Docker Engine and the Compose plugin are required. Native builds also need Go (see `go.mod`), Node.js 24, pnpm 12 and Python 3. Dockerfile builds do not require Go or Node.js on the host.

```bash
git clone https://github.com/dockge-go/dockge.git
cd dockge
mkdir -p stacks
docker compose -f deploy/compose.yaml up -d --build
```

Open `http://<host>:5001` and create the administrator. **There is no default password.** Use an HTTPS reverse proxy in production and restrict access during initial setup.

The example builds `dockge-go:local` from source. The two-mount layout is supported starting with **`0.2.6`**; old `0.2.5` images lack the required entrypoint. To use a release image, set `image` to `makeshit/dockge:0.2.6` (Docker Hub) or `ghcr.io/dockge-go/dockge:0.2.6` (GHCR), remove the `build` section, then run `docker compose -f deploy/compose.yaml pull` and `docker compose -f deploy/compose.yaml up -d --no-build`. Private GHCR packages require pull credentials. Binaries are available under [Releases](https://github.com/dockge-go/dockge/releases). Pin a version or digest in production.

### Kubernetes deployment

[Single-node Kubernetes and OIDC manifests](deploy/kubernetes/README.md) include a Deployment, Service, ConfigMap, Secret and HTTPS Ingress example. Select exactly one node with a Docker Engine socket; a containerd-only Kubernetes node cannot run this image's Compose operations out of the box. HostPath storage preserves data and the fixed internal paths; one replica with `Recreate` prevents concurrent database writers. Supply real OIDC configuration as documented; do not apply the entire directory containing credential examples.

### Mounts and permissions

`deploy/compose.yaml` needs only two mounts: the runtime socket and stack directory. No additional metric mounts are needed.

- Internal paths and port remain `/opt/stacks`, `/app/data`, `5001`. Change only the host side, such as `/your/stacks:/opt/stacks` or `15001:5001`.
- The Docker socket grants near-root host access. Expose the panel only to trusted administrators. Deployment checks report specific runtime errors.
- The image links `/app/data` to `/opt/stacks/.dockge-data`; account, settings and logs persist with stacks. Include this hidden directory in backups. It is not a stack and cannot be deleted through stack APIs. Existing explicit `/app/data` mounts remain supported without migration or overwrite.
- Compose bind sources are resolved by the host runtime. Check YAML source paths after changing the host stack directory. The container terminal is not a host terminal.
- Runtime selection uses `container.cli` / `container.compose` or PATH detection for docker, podman and nerdctl. Supply the corresponding CLI, Compose implementation and connection for other runtimes. The default image contains Docker CLI and the Compose plugin.

## Server metrics

The dashboard samples every 2 seconds; resource lists refresh every 10 seconds. System Info shows 9 essentials. Standard Linux containers read native procfs kernel-global counters without extra metric mounts, privileged mode or hostPID. Hostname and OS come from local runtime `info`, not the image’s `/etc`. Identity is cached for 30 seconds; utilization counters are sampled live.

CPU uses consecutive counter samples, with an initial sampling state. Used memory is `MemTotal − MemAvailable`. Disk space describes **the filesystem containing `/opt/stacks`**, not all disks or the sum of image sizes. Missing files, denied access, failed requests and stale samples show unknown values with specific errors. Hidden pages pause sampling.

Native Linux reads local procfs and the root filesystem. Stack management works on darwin, but server metric sampling is unsupported. Metrics describe the Linux kernel hosting the panel: on Docker Desktop this is the Linux VM, not the macOS/Windows host. LXCFS overrides, inconsistent runtime metadata and nonzero time-namespace offsets fail explicitly. Remote Docker contexts are not mixed into local identity. Container usage sums never substitute for host usage. Existing `/host/proc` mounts remain compatible.

## Migrating an existing data volume

Do not simply remove an existing `/app/data` mount: the new storage location would not contain the old account. Stop the panel, back up all data and stacks, inspect the actual Compose volume name, and copy the entire former `/app/data` into an **empty** host `stacks/.dockge-data` directory, preserving permissions. Build the current image, remove the data and metric-file mounts, start and verify the original account/settings/stacks before retiring the old volume. Never copy an open database or overwrite a populated target. Alternatively keep the legacy data mount; the new image supports it with no metric mounts. No global prune or automatic old-data deletion is performed.

## Account and data recovery

Change a known password in Settings. To recover a forgotten local password:

1. Restrict network access and stop the panel. Ensure no other instance holds the database open.
2. Back up stacks including `.dockge-data`, then move `stacks/.dockge-data/dockge.db` to a secure backup location. With a legacy data mount, operate on that volume instead.
3. Restart and create the new sole administrator through the restricted setup endpoint before restoring access.

This resets the account, settings and persisted JWT key, invalidating old sessions. Independent stack files and runtime containers remain intact. Never modify or delete a running bbolt database. OIDC passwords are managed by the identity provider; see [Kubernetes/OIDC deployment](deploy/kubernetes/README.md).

Removing the panel container does not remove managed stacks or host data. Delete intended stacks and review retained resources first, then stop the panel and handle `.dockge-data` (or the legacy data volume). Built-in networks, the panel itself and resources still referenced by other objects cannot be forcibly removed.

## Development and verification

```bash
make verify          # Build and Go/frontend/maintenance-script tests
make run             # Default port 5001 and conventional paths
make image           # Build dockge-go:local
sh scripts/e2e-auth.sh
```

Native development uses the conventional paths through symlinks or mounts. `DOCKGE_*` variables are advanced native overrides, not a way to change container-internal paths.

Smoke testing a running instance requires an explicit session token, or both username and password:

```bash
# Use a token already in your shell; do not store credentials in scripts or Git.
DOCKGE_TOKEN="$TOKEN" make smoke
```

Smoke testing creates, starts/stops and deletes one randomly named stack. It defaults to `nginx:alpine` (override with `SMOKE_IMAGE`), binds no host port, changes no global settings and cleans no unrelated workloads. On interruption it attempts cleanup; failed cleanup reports the stack name for manual inspection.

Main pushes and pull requests run verification. Release tags `v*` must pass the same build, tests and vulnerability checks before binaries and images are published. Manual branch release runs publish a `dev` image. Pushing main alone does not publish an image or deploy a server.

GHCR uses the workflow `GITHUB_TOKEN` and job-scoped `packages: write`, with no separate publishing token. Docker Hub requires Actions secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` with repository write access; repository variable `DOCKERHUB_IMAGE` overrides the default `makeshit/dockge`. The registry jobs run independently and report failures without blocking one another. Release tag `v0.2.6` publishes image tags `0.2.6`, `0.2` and `latest`; manually rerunning that tag does not also publish `dev`.

MIT License.
