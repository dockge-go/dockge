# realtime-resource-views Specification

## Purpose

定义 REST 资源列表与专用状态、日志长连接的边界：资源数据经 REST 加载，实时状态、主机指标与日志各走专用长连接，互不混用。

## Requirements

### Requirement: 资源列表通过 REST 加载

Containers 与 Stacks 列表 SHALL 通过 REST 接口加载；镜像、端口、Stack 归属、配置文件等列表字段 SHALL NOT 由长连接整表推送。

#### Scenario: 进入 Containers 页面
- **WHEN** 用户进入已认证应用或执行手动刷新
- **THEN** 前端通过 REST 获取完整容器列表，并按每页 15 条展示

#### Scenario: 进入 Stacks 页面
- **WHEN** 用户进入已认证应用或执行手动刷新
- **THEN** 前端通过 REST 获取完整 Stack 列表，不为 Stack 列表建立长连接

### Requirement: 容器状态使用专用长连接

Containers 与 Dashboard SHALL 共享 `/v1/docker/containers/stream` 长连接；状态帧 SHALL 只包含容器 ID、state 与 status，并按 ID 合并到 REST 列表。

#### Scenario: 容器状态变化
- **WHEN** 容器状态长连接收到新状态帧
- **THEN** Containers 列表、已打开详情和 Dashboard 容器统计同步更新，其他 REST 字段保持不变

### Requirement: 主机状态使用专用长连接

Dashboard 的 CPU 与内存状态 SHALL 使用 `/v1/docker/stats/stream` 长连接。

#### Scenario: 主机状态帧到达
- **WHEN** stats 长连接收到 CPU 与内存采样
- **THEN** Dashboard 更新主机资源状态，不重新加载资源列表

### Requirement: 日志使用专用长连接

Stack 日志与 Container 日志 SHALL 分别使用其现有日志长连接持续输出。

#### Scenario: 查看 Stack 日志
- **WHEN** 用户打开 Stack 日志
- **THEN** 前端订阅该 Stack 的日志长连接，并在关闭视图时断开

#### Scenario: 查看 Container 日志
- **WHEN** 用户打开 Container 日志
- **THEN** 前端订阅该 Container 的日志长连接，并在关闭视图时断开

### Requirement: 不提供通用 Events 功能

系统 SHALL NOT 提供 Events 页面、Events 导航或 `/v1/events` 通用事件端点。

#### Scenario: 访问旧 Events 路径
- **WHEN** 客户端访问 `/events` 或 `/v1/events`
- **THEN** 系统不提供 Events 功能或兼容响应
