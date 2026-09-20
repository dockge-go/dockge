# syntax=docker/dockerfile:1
# 单二进制镜像：前端构建 → Go 构建 → 精简运行镜像（仅 docker CLI + 证书 + 二进制）。
# 构建：docker build -f deploy/Dockerfile -t dockge-go --build-arg VERSION=$(git describe --tags --always) .

# ---- 前端构建 ----
# --platform=$BUILDPLATFORM：始终在构建机原生架构上跑。前端产物是静态文件，与架构无关；
# 若在 QEMU 模拟的 arm64 下跑，Node/V8（vite build）会 SIGILL 崩溃（exit 132）。
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
ARG VERSION=dev
ENV VERSION=${VERSION}
RUN pnpm build

# ---- 后端构建 ----
# 同上跑原生架构，由 GOARCH 交叉编译到目标架构（CGO_ENABLED=0），避免 QEMU 模拟。
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS backend
WORKDIR /src
# GOPROXY 可覆盖（受限网络下用镜像源，例如 --build-arg GOPROXY=https://goproxy.cn,direct）
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
ARG VERSION=dev
# TARGETARCH 是 buildkit 内置平台参数，交叉编译的目标架构由此取
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w -X dockge/internal/version.Version=${VERSION}" \
    -o /out/dockge-server ./cmd

# ---- 运行镜像 ----
FROM alpine:3.22
# docker CLI 与 compose 插件：栈操作经子进程调用；ca-certificates 供版本检查等出网请求；
# tini 作 PID 1：栈操作会启动子进程组，孤儿进程需由 init 回收（否则容器内累积僵尸）。
RUN apk add --no-cache docker-cli docker-cli-compose ca-certificates tzdata tini
COPY --from=backend /out/dockge-server /usr/local/bin/dockge-server
WORKDIR /app
# 零配置文件：容器内路径由 ENV 提供，用户编排经 environment 段覆盖（如 DOCKGE_STACKS_DIR）
ENV DOCKGE_STACKS_DIR=/opt/stacks \
    DOCKGE_DATA_DB_USER_DSN=/app/data/dockge.db \
    DOCKGE_LOG_MODE=both \
    DOCKGE_LOG_ENCODING=json \
    DOCKGE_LOG_FILE_NAME=/app/data/logs/dockge.log
VOLUME /app/data
EXPOSE 5001

# OCI 元数据：Docker Hub 页面据此关联源码仓库与版本。
ARG VERSION=dev
LABEL org.opencontainers.image.title="Dockge" \
      org.opencontainers.image.description="Self-hosted Docker Compose stack manager (single-host)" \
      org.opencontainers.image.source="https://github.com/dockge-go/dockge" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}"

# 健康检查：容器内无 curl，用 busybox wget；/v1/health 免鉴权，响应体含运行时自检结果。
# 端口跟随 DOCKGE_HTTP_PORT（shell 形式 CMD 才能展开变量）。
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null "http://127.0.0.1:${DOCKGE_HTTP_PORT:-5001}/v1/health" || exit 1

# 挂载宿主 docker socket 与数据卷：
#   -v /var/run/docker.sock:/var/run/docker.sock -v dockge-data:/app/data
ENTRYPOINT ["/sbin/tini", "--", "dockge-server"]
