## 1. 设计系统与基础组件

- [x] 1.1 建立根目录 `DESIGN.md`，固化 Apple System token、主从工作区、分页、实时状态和响应式规则。
- [x] 1.2 新增纯分页工具与共享 Pagination 组件，固定每页 15 条并覆盖越界修正。

## 2. 实时数据与单机边界

- [x] 2.1 清理前端 Agent 类型、API、Settings 与 SysInfo 入口，使前端符合单机边界。
- [x] 2.2 删除前后端 Events 页面、导航、Handler 与 `/v1/events` 路由。
- [x] 2.3 Containers/Stacks 通过 REST 加载；新增仅含 `id/state/status` 的容器状态 SSE，并在前端按 ID 合并。
- [x] 2.4 保留主机 stats、Stack 日志 WebSocket 和 Container 日志 SSE，不新增其他长连接。

## 3. 资源详情工作区

- [x] 3.1 将 Container 页面改为桌面主从、移动端全屏详情，并以摘要 + Logs/Terminal + 折叠元数据取代 6 个 Tab。
- [x] 3.2 将 Stack 列表与详情改为主从工作区，保留生命周期操作、服务状态、日志与只读外部栈。

## 4. Stack 创建与编辑

- [x] 4.1 拆分 Stack 编辑器、摘要和工作区组件，删除 `@ts-nocheck`。
- [x] 4.2 实现 Compose 代码优先的 New Stack，支持 docker run 转换、仅保存、创建并部署及失败后保留上下文。

## 5. 全列表分页与验证

- [x] 5.1 为 Containers、Stacks、Images、Volumes、Networks 接入 15 条分页，Dashboard 保持 5 条摘要。
- [x] 5.2 运行 typecheck、production build 和 OpenSpec 严格校验。
- [x] 5.3 在 375px、768px、1280px 完成生产前端浏览器视觉与交互验收；真实后端运行受当前 Windows 缺少 Linux PTY/Podman 运行环境阻断。
