## Why

远程 Agent 让单个 Dockge 实例保存其他主机的登录凭据，并代理高权限 Docker 操作。该能力扩大了信任边界，要求额外的主机身份、凭据保护、最小权限、审计与密钥轮换体系；当前产品不提供这些控制，且远程栈聚合未完成。

## What Changes

- 将 Dockge 定义为单机 Docker/Podman Web 管理面板：一个实例只管理其部署主机上的容器运行时与 compose 栈。
- 移除后端的远程 Agent API、依赖注入、服务、远程 HTTP 客户端、凭据持久化模型及 bbolt `agents` bucket 初始化。
- 已有数据库中的遗留 `agents` bucket 不再读取或写入；本变更不重写用户数据或删除现有数据库文件。
- README 更新为单机定位，并说明跨主机运维应在每台目标主机独立部署 Dockge，由 VPN、反向代理或零信任访问层提供远程访问。

## Capabilities

### New Capabilities

- `single-host-boundary`: 单机运行时管理的产品与安全边界。

### Removed Capabilities

- `remote-agent-management`: 保存远程 Dockge 凭据、代理远程栈操作及聚合远程资源。

## Impact

- 删除 `/v1/agents*` HTTP 端点及其后端实现。
- 新建数据库不再创建 `agents` bucket；旧 bucket 被保留但不再访问。
- 本次不修改前端 Agent UI，避免把后端安全收缩与 UI 重设计混为一次变更；前端对已删除 API 的引用将在独立 UI 变更中清理。
