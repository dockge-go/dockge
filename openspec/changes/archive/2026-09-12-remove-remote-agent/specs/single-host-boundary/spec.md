## Purpose

定义 Dockge 的单机 Docker/Podman 管理边界，并排除跨主机凭据代理与控制能力。

## ADDED Requirements

### Requirement: 单机运行时管理边界

Dockge SHALL 只管理与该实例部署在同一主机上的 Docker 或 Podman 运行时及其 compose 栈、容器、镜像、网络和卷。

#### Scenario: 管理部署主机的 compose 栈
- **WHEN** 已认证用户请求本地栈或 Docker 资源操作
- **THEN** Dockge 仅调用部署主机上的运行时或 compose CLI，不连接其他主机

### Requirement: 跨主机访问由外部网络层提供

Dockge MAY 被远程浏览器访问，但跨主机访问 SHALL 由每台目标主机独立部署的 Dockge 实例与外部网络访问层提供，不得由 Dockge 保存或代理其他主机的控制凭据。

#### Scenario: 管理另一台主机
- **WHEN** 管理员需要管理另一台 Docker 主机
- **THEN** 管理员访问部署在该目标主机上的独立 Dockge 实例；该实例不经由其他 Dockge 转发请求

### Requirement: 不保存远程主机凭据

Dockge SHALL NOT 接收、持久化、缓存或使用其他主机的 Dockge 登录凭据、访问令牌或连接 URL。

#### Scenario: 请求已删除的 Agent API
- **WHEN** 客户端请求任何 `/v1/agents` 路径
- **THEN** 服务返回 HTTP 404，且不会建立到其他主机的网络连接或读写 Agent 凭据
