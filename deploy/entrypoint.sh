#!/bin/sh
set -eu

# 旧部署显式挂载 /app/data 时继续使用原数据，不迁移或覆盖已有账号。
if ! awk '$5 == "/app/data" { found=1 } END { exit !found }' /proc/self/mountinfo; then
    if ! awk '$5 == "/opt/stacks" { found=1 } END { exit !found }' /proc/self/mountinfo; then
        echo 'Dockge: /opt/stacks 必须持久挂载；不能把账号和栈写进容器可写层。' >&2
        exit 1
    fi
    if [ -L /opt/stacks/.dockge-data ]; then
        echo 'Dockge: /opt/stacks/.dockge-data 必须是真实目录，不能是符号链接。' >&2
        exit 1
    fi
    mkdir -p /opt/stacks/.dockge-data
    chmod 700 /opt/stacks/.dockge-data
    if [ -L /app/data ]; then
        if [ "$(readlink /app/data)" != /opt/stacks/.dockge-data ]; then
            echo 'Dockge: /app/data 指向非预期路径；请停机核对并迁移数据。' >&2
            exit 1
        fi
    else
        # 只移除空目录；若有旧数据则明确阻止启动，不能静默换库。
        if [ -e /app/data ]; then
            if ! rmdir /app/data; then
                echo 'Dockge: /app/data 包含旧数据或不是空目录；请继续挂载原数据，或停机备份后迁移到 stacks/.dockge-data。' >&2
                exit 1
            fi
        fi
        ln -s /opt/stacks/.dockge-data /app/data
    fi
fi

exec dockge-server "$@"
