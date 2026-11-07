#!/bin/sh
# 认证全链路 e2e：自管沙箱实例（临时数据目录 + 独立端口复用 5001 前先清残留），
# 覆盖 setup→登录→用户 CRUD→改密/重置失效→防枚举→限流。退出码非 0 表示有失败。
# 断言链顺序固定：限流探测（⑲）必须最后——IP 限流会污染前序请求。
set -u

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
SERVER="$REPO_ROOT/bin/dockge-server"
[ -x "$SERVER" ] || { echo "✗ bin/dockge-server 不存在，先 make build"; exit 1; }

pass=0
fail=0
chk() { # chk 名称 期望 实际
	if [ "$2" = "$3" ]; then
		printf "  ✓ %-40s %s\n" "$1" "$3"
		pass=$((pass + 1))
	else
		printf "  ✗ %-40s 期望=%s 实际=%s\n" "$1" "$2" "$3"
		fail=$((fail + 1))
	fi
}
code() { curl -s -o /dev/null -w "%{http_code}" "$@"; }
body() { curl -s "$@"; }
json() { printf '{"username":"%s","password":"%s"}' "$1" "$2"; }
json_user() { printf '{"username":"%s","password":"%s","role":"%s"}' "$1" "$2" "$3"; }
json_pw() { printf '{"password":"%s"}' "$1"; }
# token <响应体>：提取 accessToken
token() { printf '%s' "$1" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["accessToken"])' 2>/dev/null || true; }

pkill -f 'bin/dockge-server' 2>/dev/null || true
SANDBOX=$(mktemp -d /tmp/opencode/dockge-e2e-XXXXXX)
mkdir -p /tmp/opencode
START_LOG="$SANDBOX/server.log"
cleanup() {
	pkill -f 'bin/dockge-server' 2>/dev/null || true
	rm -rf "$SANDBOX"
}
trap cleanup EXIT INT TERM

BASE=http://127.0.0.1:5001
DOCKGE_DATA_DB_USER_DSN="$SANDBOX/dockge.db" DOCKGE_STACKS_DIR="$SANDBOX/stacks" "$SERVER" >"$START_LOG" 2>&1 &
ready=0
i=0
while [ $i -lt 60 ]; do
	if [ "$(code "$BASE/v1/setup/need")" = "200" ]; then ready=1; break; fi
	sleep 0.5
	i=$((i + 1))
done
[ "$ready" = "1" ] || { echo "✗ 实例 30s 未就绪（日志：$START_LOG）"; tail -5 "$START_LOG"; exit 1; }

AUTHZ() { printf 'Authorization: Bearer %s' "$1"; }

echo "== ⓪ 嵌入前端仍在服务 =="
chk "首页 200" 200 "$(code "$BASE/")"
chk "首页含新前端标志" 1 "$(body "$BASE/" | grep -c 'dockge.theme')"

echo "== ①-④ setup 与会话 =="
chk "needSetup=true" 1 "$(body "$BASE/v1/setup/need" | grep -c '"needSetup":true')"
r=$(body -X POST "$BASE/v1/setup" -H 'Content-Type: application/json' -d "$(json admin Admin123)")
ADMIN_TOKEN=$(token "$r")
[ -n "$ADMIN_TOKEN" ] && chk "setup 得 accessToken" ok ok || chk "setup 得 accessToken" ok "empty"
AUTH="Authorization: Bearer $ADMIN_TOKEN"
chk "GET /v1/me username=admin" 1 "$(body "$BASE/v1/me" -H "$AUTH" | grep -c '"username":"admin"')"
chk "needSetup 变 false" 1 "$(body "$BASE/v1/setup/need" | grep -c '"needSetup":false')"

echo "== ⑤-⑥ 登录与防枚举 =="
chk "错密码 401" 401 "$(code -X POST "$BASE/v1/login" -H 'Content-Type: application/json' -d "$(json admin WrongPass9)")"
m1=$(body -X POST "$BASE/v1/login" -H 'Content-Type: application/json' -d "$(json admin WrongPass9)" | python3 -c 'import sys,json;print(json.load(sys.stdin)["message"])' 2>/dev/null)
m2=$(body -X POST "$BASE/v1/login" -H 'Content-Type: application/json' -d "$(json ghost WrongPass9)" | python3 -c 'import sys,json;print(json.load(sys.stdin)["message"])' 2>/dev/null)
chk "未知用户与错密码 401 文案逐字一致" "$m1" "$m2"
chk "文案不含停用（不泄露存在性）" 0 "$(printf '%s' "$m2" | grep -c '停用')"

echo "== ⑦-⑬ 用户 CRUD =="
chk "无 token GET /v1/users 401" 401 "$(code "$BASE/v1/users")"
chk "创建 member bob" 200 "$(code -X POST "$BASE/v1/users" -H "$AUTH" -H 'Content-Type: application/json' -d "$(json_user bob Bob123 member)")"
list=$(body "$BASE/v1/users" -H "$AUTH")
chk "列表含 bob+member+created 整数" 1 "$(printf '%s' "$list" | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];u=[x for x in d if x["username"]=="bob"][0];print(1 if u["role"]=="member" and isinstance(u["created"],int) else 0)' 2>/dev/null)"
chk "重复创建同名 409" 409 "$(code -X POST "$BASE/v1/users" -H "$AUTH" -H 'Content-Type: application/json' -d "$(json_user bob Bob123 member)")"
chk "弱密码 400（至少6位）" 1 "$(body -X POST "$BASE/v1/users" -H "$AUTH" -H 'Content-Type: application/json' -d "$(json_user carol short member)" | grep -c '至少6位')"
chk "用户名含空格 400（不合法）" 1 "$(body -X POST "$BASE/v1/users" -H "$AUTH" -H 'Content-Type: application/json' -d "$(json_user "a b" Pass123 member)" | grep -c '用户名不合法')"
chk "role=superuser 400（角色不合法）" 1 "$(body -X POST "$BASE/v1/users" -H "$AUTH" -H 'Content-Type: application/json' -d "$(json_user dave Pass123 superuser)" | grep -c '角色不合法')"

echo "== ⑭-⑯ 权限与删除保护 =="
r=$(body -X POST "$BASE/v1/login" -H 'Content-Type: application/json' -d "$(json bob Bob123)")
BOB_TOKEN=$(token "$r")
BOB_AUTH="Authorization: Bearer $BOB_TOKEN"
[ -n "$BOB_TOKEN" ] && chk "bob 登录" ok ok || chk "bob 登录" ok empty
chk "member GET /v1/users 403" 403 "$(code "$BASE/v1/users" -H "$BOB_AUTH")"
chk "member DELETE /v1/users/admin 403" 403 "$(code -X DELETE "$BASE/v1/users/admin" -H "$BOB_AUTH")"
chk "admin 重置 bob 密码 200" 200 "$(code -X PUT "$BASE/v1/users/bob/password" -H "$AUTH" -H 'Content-Type: application/json' -d "$(json_pw Bob456)")"
chk "bob 旧 token 即失效 401" 401 "$(code "$BASE/v1/me" -H "$BOB_AUTH")"
r=$(body -X POST "$BASE/v1/login" -H 'Content-Type: application/json' -d "$(json bob Bob456)")
chk "bob 新密码登录 200" 200 "$(code -X POST "$BASE/v1/login" -H 'Content-Type: application/json' -d "$(json bob Bob456)" -o /dev/null)" 2>/dev/null || true
chk "bob 新密码登录得 token" ok "$(printf '%s' "$(body -X POST "$BASE/v1/login" -H 'Content-Type: application/json' -d "$(json bob Bob456)")" | grep -c accessToken >/dev/null && echo ok || echo empty)"

r=$(body -X POST "$BASE/v1/users" -H "$AUTH" -H 'Content-Type: application/json' -d "$(json_user e2 E2pass123 admin)")
E2_TOKEN=$(token "$(body -X POST "$BASE/v1/login" -H 'Content-Type: application/json' -d "$(json e2 E2pass123)")")
E2_AUTH="Authorization: Bearer $E2_TOKEN"
chk "e2(admin) 登录" ok "$( [ -n "$E2_TOKEN" ] && echo ok || echo empty)"
chk "admin 自删（两名 admin）403" 403 "$(code -X DELETE "$BASE/v1/users/admin" -H "$AUTH")"
chk "e2 删 admin 200" 200 "$(code -X DELETE "$BASE/v1/users/admin" -H "$E2_AUTH")"
chk "e2 自删唯一 admin 409" 409 "$(code -X DELETE "$BASE/v1/users/e2" -H "$E2_AUTH")"

echo "== ⑰-⑱ 自改密与重复 setup =="
NEW_E2_TOKEN=$(token "$(body -X POST "$BASE/v1/login" -H 'Content-Type: application/json' -d "$(json e2 E2pass123)")")
chk "e2 自改密 200" 200 "$(code -X PUT "$BASE/v1/me/password" -H "Authorization: Bearer $NEW_E2_TOKEN" -H 'Content-Type: application/json' -d "$(printf '{"oldPassword":"E2pass123","newPassword":"E2pass456"}')")"
chk "e2 旧 token 失效 401" 401 "$(code "$BASE/v1/me" -H "$E2_AUTH")"
chk "重复 setup 409" 409 "$(code -X POST "$BASE/v1/setup" -H 'Content-Type: application/json' -d "$(json eve Eve12345)")"

echo "== ⑲ 限流（必须最后，IP 级污染） =="
lcode=000
i=0
while [ $i -lt 15 ]; do
	lcode=$(code -X POST "$BASE/v1/login" -H 'Content-Type: application/json' -d "$(json admin WrongPass9)")
	[ "$lcode" = "429" ] && break
	i=$((i + 1))
done
chk "连续失败触发 429" 429 "$lcode"

echo ""
if [ "$fail" -eq 0 ]; then
	printf "E2E_AUTH_OK (%d assertions passed)\n" "$pass"
	exit 0
else
	printf "E2E_AUTH_FAILED: %d passed, %d failed\n" "$pass" "$fail"
	exit 1
fi
