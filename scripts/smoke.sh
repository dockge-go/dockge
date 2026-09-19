#!/bin/sh
# 端到端冒烟：对运行中的实例 + 真实容器运行时执行全链路检查（发布前门禁）。
# 用法：BASE=http://127.0.0.1:5001 DOCKGE_USER=admin DOCKGE_PASS=123456 sh scripts/smoke.sh
# 退出码非 0 表示存在失败项。
set -u

BASE="${BASE:-http://127.0.0.1:5001}"
# 注意不要用 USER/PASS 作为变量名：USER 是系统环境变量（会取到登录名）
DOCKGE_USER="${DOCKGE_USER:-admin}"
DOCKGE_PASS="${DOCKGE_PASS:-123456}"
STACK="smoke-$$"

pass=0
fail=0
chk() { # chk 名称 期望 实际
	if [ "$2" = "$3" ]; then
		printf "  ✓ %-34s %s\n" "$1" "$3"
		pass=$((pass + 1))
	else
		printf "  ✗ %-34s 期望=%s 实际=%s\n" "$1" "$2" "$3"
		fail=$((fail + 1))
	fi
}
code() { curl -s -o /dev/null -w "%{http_code}" "$@"; }
body() { curl -s "$@"; }

# JSON 由单一 printf 构造：避免 shell 引号嵌套把参数拆成多个 -d（曾因此误报 400）
json_login() { printf '{"username":"%s","password":"%s"}' "$1" "$2"; }
json_stack() { printf '{"name":"%s","yaml":"services:\\n  web:\\n    image: nginx:latest\\n    ports:\\n      - \\"28095:80\\"\\n","env":""}' "$1"; }
json_bad_yaml() { printf '{"name":"%s","yaml":"services: ["}' "$1"; }
json_global_env() { printf '{"content":"TZ=Asia/Shanghai\\n"}'; }
json_hostname() { printf '{"hostname":"dockge.example.com"}'; }
json_docker_run() { printf '{"dockerRunCommand":"docker run -d --name nginx -p 8080:80 nginx"}'; }

echo "== 基础 =="
chk "健康检查" 200 "$(code "$BASE/v1/health")"
chk "首页（内嵌前端）" 200 "$(code "$BASE/")"
chk "robots.txt" 200 "$(code "$BASE/v1/robots.txt")"
chk "未鉴权访问受保护端点" 401 "$(code "$BASE/v1/stacks")"

echo "== 认证 =="
bad=$(json_login "$DOCKGE_USER" "definitely-wrong")
chk "错误密码" 401 "$(code -X POST "$BASE/v1/login" -H 'Content-Type: application/json' -d "$bad")"
chk "错误密码提示可辨识" 1 "$(body -X POST "$BASE/v1/login" -H 'Content-Type: application/json' -d "$bad" | grep -c '用户名或密码错误')"
ok=$(json_login "$DOCKGE_USER" "$DOCKGE_PASS")
TOKEN=$(body -X POST "$BASE/v1/login" -H 'Content-Type: application/json' -d "$ok" |
	python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["accessToken"])' 2>/dev/null || true)
if [ -z "$TOKEN" ]; then
	echo "  ✗ 无法登录（检查 BASE/DOCKGE_USER/DOCKGE_PASS）"
	exit 1
fi
echo "  ✓ 登录获取令牌"
AUTH="Authorization: Bearer $TOKEN"
chk "当前用户" 200 "$(code "$BASE/v1/me" -H "$AUTH")"

echo "== 栈全链路（真实运行时） =="
chk "创建栈" 200 "$(code -X POST "$BASE/v1/stacks" -H "$AUTH" -H 'Content-Type: application/json' -d "$(json_stack "$STACK")")"
chk "栈列表含新栈" 1 "$(body "$BASE/v1/stacks" -H "$AUTH" | grep -c "$STACK")"
chk "栈详情" 200 "$(code "$BASE/v1/stacks/$STACK" -H "$AUTH")"
chk "非法栈名被拒" 400 "$(code -X POST "$BASE/v1/stacks" -H "$AUTH" -H 'Content-Type: application/json' -d '{"name":"Bad Name","yaml":"services: {}"}')"
chk "非法 YAML 被拒" 400 "$(code -X PUT "$BASE/v1/stacks/$STACK" -H "$AUTH" -H 'Content-Type: application/json' -d "$(json_bad_yaml "$STACK")")"

start_out=$(body -X POST "$BASE/v1/stacks/$STACK/start" -H "$AUTH")
case "$start_out" in
*Started* | *Running*) echo "  ✓ 启动（流式输出）"; pass=$((pass + 1)) ;;
*) echo "  ✗ 启动：$start_out"; fail=$((fail + 1)) ;;
esac
chk "资源统计" 200 "$(code "$BASE/v1/stacks/$STACK/stats" -H "$AUTH")"
chk "网络列表" 200 "$(code "$BASE/v1/stacks/networks" -H "$AUTH")"
chk "未知操作被拒" 400 "$(code -X POST "$BASE/v1/stacks/$STACK/bogus" -H "$AUTH")"
chk "不存在的栈" 404 "$(code "$BASE/v1/stacks/no-such-stack-$$" -H "$AUTH")"
chk "停止并移除" 200 "$(code -X POST "$BASE/v1/stacks/$STACK/down" -H "$AUTH")"
chk "删除栈" 200 "$(code -X DELETE "$BASE/v1/stacks/$STACK" -H "$AUTH")"
chk "删除后不可见" 404 "$(code "$BASE/v1/stacks/$STACK" -H "$AUTH")"

echo "== 设置与工具 =="
chk "全局 env 读取" 200 "$(code "$BASE/v1/settings/globalenv" -H "$AUTH")"
chk "全局 env 写入" 200 "$(code -X PUT "$BASE/v1/settings/globalenv" -H "$AUTH" -H 'Content-Type: application/json' -d "$(json_global_env)")"
chk "全局 env 读回一致" 1 "$(body "$BASE/v1/settings/globalenv" -H "$AUTH" | grep -c Asia/Shanghai)"
chk "主机名读取" 200 "$(code "$BASE/v1/settings/primaryhostname" -H "$AUTH")"
chk "主机名写入" 200 "$(code -X PUT "$BASE/v1/settings/primaryhostname" -H "$AUTH" -H 'Content-Type: application/json' -d "$(json_hostname)")"
chk "主机名读回一致" 1 "$(body "$BASE/v1/settings/primaryhostname" -H "$AUTH" | grep -c dockge.example.com)"
chk "composerize" 200 "$(code -X POST "$BASE/v1/composerize" -H "$AUTH" -H 'Content-Type: application/json' -d "$(json_docker_run)")"
chk "终端端点需鉴权" 401 "$(code "$BASE/v1/terminal/x/compose-logs")"

echo "== 已移除端点（不得复活） =="
for p in /v1/agents /v1/console/enabled /v1/stacks/validate /v1/oidc/providers /v1/auth/config /v1/version/check; do
	chk "404 $p" 404 "$(code "$BASE$p" -H "$AUTH")"
done

echo
echo "结果：通过 $pass / 失败 $fail"
[ "$fail" = 0 ]
