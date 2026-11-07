#!/bin/sh
# 在临时数据库与测试 HTTP 服务中验证认证，不启动或结束现有实例。
set -eu
REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$REPO_ROOT"
go test -count=1 ./internal/service ./internal/server -run 'Test(LoginNoEnumeration|SingleUserSessionAndPassword|OIDC.*|API)$'
