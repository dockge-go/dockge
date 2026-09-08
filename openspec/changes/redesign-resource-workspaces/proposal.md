## Why

当前资源列表缺少统一分页，Container 与 Stack 详情依赖多 Tab Sheet，Stack 新建、编辑、操作输出和服务状态集中在一个超大组件中，造成定位成本与上下文切换。旧 `/v1/events` 还通过 SSE 推送全部资源列表，混淆了 REST 资源读取与实时状态流。

## What Changes

- Containers、Stacks、Images、Volumes、Networks 统一按每页 15 条客户端分页；Dashboard 最近容器保持 5 条摘要。
- 删除 Events 前端页面与后端通用事件接口。
- Containers 与 Stacks 列表通过 REST 加载；Containers 与 Dashboard 仅通过 `/v1/docker/containers/stream` 合并容器实时状态。
- 主机 CPU/内存和 Stack/Container 日志继续使用各自长连接。
- Container 与 Stack 详情改为 Apple System 风格主从工作区：桌面保留列表上下文，移动端详情全屏。
- Container 详情从 6 个同权 Tab 收敛为摘要、Logs/Terminal 主工具和环境/挂载/网络渐进披露。
- New Stack 与 Stack 编辑改为 Compose 代码优先工作区：左文件导航、中编辑器、右部署摘要；新建支持“仅保存”和“创建并部署”。
- 清理后端 Agent 删除后残留的前端 Agent UI、类型和 mock 数据。

## Capabilities

### New Capabilities
- `resource-pagination`: 资源列表统一分页行为。
- `realtime-resource-views`: REST 资源列表与专用状态/日志长连接契约。
- `resource-workspaces`: Container 与 Stack 主从工作区。
- `compose-stack-editor`: Compose 优先的新建与编辑流程。

### Modified Capabilities
- `single-host-boundary`: 前端移除远程 Agent 管理入口和数据结构。

## Impact

- 主要修改 `app/dockge/web/src` 下的资源视图、全局 store、API 类型、mock 数据和样式。
- 新增少量无状态分页与工作区组件，不引入新 UI 框架或状态库。
- 不增加后端分页接口，不修改本地 Docker 操作语义。
