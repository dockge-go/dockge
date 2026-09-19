# syntax=docker/dockerfile:1
# 单二进制镜像：前端构建 → Go 构建 → 精简运行镜像（仅 docker CLI + 证书 + 二进制）。
# 构建：docker build -f deploy/Dockerfile -t dockge-go --build-arg VERSION=$(git describe --tags --always) .

# ---- 前端构建 ----
FROM node:24-alpine AS web
WORKDIR /web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
ARG VERSION=dev
ENV VERSION=${VERSION}
RUN pnpm build

# ---- 后端构建 ----
FROM golang:1.26-alpine AS backend
WORKDIR /src
# GOPROXY 可覆盖（受限网络下用镜像源，例如 --build-arg GOPROXY=https://goproxy.cn,direct）
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X dockge/internal/version.Version=${VERSION} -X main.defaultConf=/app/config/dockge.yml" \
    -o /out/dockge-server ./cmd

# ---- 运行镜像 ----
FROM alpine:3.22
# docker CLI 与 compose 插件：栈操作经子进程调用；ca-certificates 供版本检查等出网请求；
# tini 作 PID 1：栈操作会启动子进程组，孤儿进程需由 init 回收（否则容器内累积僵尸）。
RUN apk add --no-cache docker-cli docker-cli-compose ca-certificates tzdata tini
COPY --from=backend /out/dockge-server /usr/local/bin/dockge-server
COPY config/docker.yml /app/config/dockge.yml
WORKDIR /app
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
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:5001/v1/health || exit 1

# 挂载宿主 docker socket 与数据卷：
#   -v /var/run/docker.sock:/var/run/docker.sock -v dockge-data:/app/data
ENTRYPOINT ["/sbin/tini", "--", "dockge-server"]
CMD ["-conf", "/app/config/dockge.yml"]
