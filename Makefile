.PHONY: bootstrap run web-build build image test verify smoke

# 一键初始化：构建前端（数据库 bucket 由服务启动时幂等创建）
bootstrap: web-build

# 启动 dockge 服务（端口 5001）；与 build 同样注入版本号，
# 否则本地 run 后端恒为 dev、与前端注入的 git 版本不一致，About 页误报
run:
	go run -ldflags="-X dockge/internal/version.Version=$(VERSION)" ./cmd

# 构建前端（tsc 类型检查 + vite，产物内嵌进 Go 二进制）
web-build:
	HUSKY=0 pnpm --dir ./web install --frozen-lockfile
	VERSION="$(VERSION)" pnpm --dir ./web build

# 构建静态单二进制（内嵌前端），避免携带开发机动态链接器路径，保证容器可运行。
# 版本号由 git describe 注入；race 测试仍使用测试命令自己的 CGO 配置。
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
build: web-build
	mkdir -p ./bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X dockge/internal/version.Version=$(VERSION)" -o ./bin/dockge-server ./cmd

# 构建容器镜像；版本号同样由 git describe 注入，免去手记 --build-arg VERSION
image:
	docker build -f deploy/Dockerfile --build-arg VERSION=$(VERSION) -t dockge-go:local .

# 发布前冒烟：对运行中的实例 + 真实容器运行时跑全链路接口检查
smoke:
	sh scripts/smoke.sh

test:
	go test ./...
	pnpm --dir ./web test
	python3 -m unittest discover -s scripts -p '*_test.py'

verify: build test
