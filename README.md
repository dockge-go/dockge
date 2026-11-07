# Dockge

**English** | [简体中文](README.zh-CN.md)

A single-host Compose stack manager, rewritten in Go from [louislam/dockge](https://github.com/louislam/dockge). Gin, bbolt and samber/do power the backend; the SolidJS frontend is embedded in one executable.

## Features

- Create, edit, validate, save and deploy `compose.yaml` in the browser. Files remain on disk.
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

The example builds the local image `dockge-go:local`. Prebuilt binaries are available under [Releases](https://github.com/dockge-go/dockge/releases); the default image publishing target is `makeshit/dockge`. Existing releases do not automatically include current source changes. Pin a verified version or image digest in production.

### Mounts and permissions

`deploy/compose.yaml` includes the runtime socket, stack directory, data volume and read-only host information files.

- Internal paths and port remain `/opt/stacks`, `/app/data`, `5001`. Change only the host side, such as `/your/stacks:/opt/stacks` or `15001:5001`.
- The Docker socket grants near-root host access. Expose the panel only to trusted administrators. Deployment checks report specific runtime errors.
- Persist stacks and `/app/data` (database and logs). Compose volume names may include a project prefix; inspect the actual name before backing up.
- Compose bind sources are resolved by the host runtime. Check YAML source paths after changing the host stack directory. The container terminal is not a host terminal.
- Runtime selection uses `container.cli` / `container.compose` or PATH detection for docker, podman and nerdctl. Supply the corresponding CLI, Compose implementation and connection for other runtimes. The default image contains Docker CLI and the Compose plugin.

## Server metrics

The dashboard samples every 2 seconds; resource lists refresh every 10 seconds. System Info shows 9 essentials. Linux containers read 8 fixed read-only host files included in the manifests, not the entire host `/proc` or root directory. No privileged mode or hostPID is needed.

CPU uses consecutive counter samples, with an initial sampling state. Used memory is `MemTotal − MemAvailable`. Disk space describes **the filesystem containing `/opt/stacks`**, not all disks or the sum of image sizes. Missing files, denied access, failed requests and stale samples show unknown values with specific errors. Hidden pages pause sampling.

Native Linux reads local procfs and the root filesystem. Stack management works on darwin, but server metric sampling is unsupported. Metrics describe the panel host, not a remote runtime.

## Account and data recovery

Change a known password in Settings. To recover a forgotten local password:

1. Restrict network access and stop the panel. Ensure no other instance holds the database open.
2. Back up the data volume and stacks, then move `dockge.db` to a secure backup location.
3. Restart and create the new sole administrator through the restricted setup endpoint before restoring access.

This resets the account, settings and persisted JWT key, invalidating old sessions. Independent stack files and runtime containers remain intact. Never modify or delete a running bbolt database. OIDC passwords are managed by the identity provider; see [Kubernetes/OIDC deployment](deploy/kubernetes/README.md).

Removing the panel container does not remove managed stacks or host data. Delete intended stacks and review retained resources first, then stop the panel and handle its data volume. Built-in networks, the panel itself and resources still referenced by other objects cannot be forcibly removed.

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

Main pushes and pull requests run verification. Release tags `v*` must pass the same build, tests and vulnerability checks before binaries and images are published. Manual release runs publish a `dev` image. Pushing main alone does not publish an image or deploy a server.

MIT License.
