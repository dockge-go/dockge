#!/bin/sh
# 需要显式会话令牌或账号密码；只修改本次生成的栈，不改全局设置。
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec python3 "$SCRIPT_DIR/smoke.py"
