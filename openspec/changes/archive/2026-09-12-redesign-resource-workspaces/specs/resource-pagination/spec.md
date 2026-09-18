## Purpose

定义 Dockge 资源列表统一、可预测的客户端分页行为。

## ADDED Requirements

### Requirement: 资源列表每页 15 条

Containers、Stacks、Images、Volumes 与 Networks 列表 SHALL 每页最多展示 15 条记录，并提供当前范围、总数、上一页和下一页。

#### Scenario: 资源超过一页
- **WHEN** 任一资源列表包含 16 条或更多记录
- **THEN** 首屏只展示前 15 条，用户可进入后续页面查看剩余记录

#### Scenario: 数据减少导致页码越界
- **WHEN** 当前页在删除或实时更新后超过新的总页数
- **THEN** 页面自动落到最后一个有效页，不显示空白越界页

### Requirement: Dashboard 保持摘要

Dashboard SHALL 最多展示 5 个最近容器，且 SHALL NOT 在仪表盘内增加分页控件。

#### Scenario: Dashboard 容器超过 5 个
- **WHEN** 实时快照包含 6 个或更多容器
- **THEN** Dashboard 只展示最近 5 个容器，并通过“查看全部”进入 Containers 分页列表
