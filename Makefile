.PHONY: init bootstrap run migrate web-build build test verify

# 安装 Go 辅助工具（当前无；预留）
init:

# 一键初始化：迁移数据库 + 构建前端
bootstrap: migrate web-build

# 启动 dockge 服务（端口 5001）
run:
	go run ./app/dockge/cmd/server -conf config/dockge/local.yml

# 数据库迁移（破坏性重建所有表：dockge_users/setting/agent，种子 admin/123456）+ 创建 stacks 目录
migrate:
	go run ./app/dockge/cmd/migration

# 构建前端（tsc 类型检查 + vite，产物内嵌进 Go 二进制）
web-build:
	HUSKY=0 pnpm --dir ./app/dockge/web install --frozen-lockfile
	pnpm --dir ./app/dockge/web build

# 构建单二进制（内嵌前端）到 bin/dockge-server
build: web-build
	mkdir -p ./bin
	go build -ldflags="-s -w" -o ./bin/dockge-server ./app/dockge/cmd/server

# 运维脚本：交互式重置指定用户的密码（破坏性，需输入两次确认）
reset-password:
	go run ./app/dockge/cmd/reset-password -conf config/dockge/local.yml

test:
	go test ./...

verify: build test
