## Purpose

定义 Container 与 Stack 详情的低心智开销主从工作区。

## ADDED Requirements

### Requirement: 桌面主从详情

Containers 与 Stacks SHALL 在桌面宽度保留可分页列表，并在同一页面的详情区域展示当前选择。

#### Scenario: 选择资源
- **WHEN** 用户点击 Container 或 Stack 列表行
- **THEN** 该行保持选中标记，详情在相邻区域打开，列表上下文和页码不丢失

### Requirement: 移动端全屏详情

Container 与 Stack 详情 SHALL 在移动端占满内容区域，并提供返回列表的明确操作。

#### Scenario: 移动端打开详情
- **WHEN** 用户在 768px 或更窄的视口选择 Container 或 Stack
- **THEN** 详情替代列表占满内容区域，并显示返回列表的操作

### Requirement: Container 渐进披露

Container 详情 SHALL 首先展示身份、状态、镜像、端口、所属 Stack 和生命周期操作；Logs 与 Terminal SHALL 作为主要工具切换；环境变量、挂载和网络 SHALL 放入可展开分组。

#### Scenario: 查看 Container 基本信息
- **WHEN** 用户打开 Container 详情
- **THEN** 首屏直接显示身份、状态、镜像、端口、所属 Stack 和生命周期操作，环境变量、挂载和网络默认不占据首屏
