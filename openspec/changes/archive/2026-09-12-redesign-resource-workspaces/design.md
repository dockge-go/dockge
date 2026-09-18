## Context

前端为 SolidJS + Kobalte + Vite，已有 Apple-inspired CSS token。旧 `/v1/events` 把栈、容器、镜像、网络和卷整表作为通用快照推送，混淆了资源加载与实时状态。用户已批准主从工作区方案 A，并明确资源列表使用 REST，仅容器状态、主机状态和日志使用长连接。

## Goals / Non-Goals

**Goals:**
- 所有资源列表形成一致的 15 条分页心智。
- 明确 REST 清单与专用 SSE 状态流的边界。
- 保持详情上下文，减少 Container 与 Stack 的同权导航项。
- 让 Compose 编辑成为 Stack 新建与编辑的中心任务。
- 删除 `@ts-nocheck` 和已失效的 Agent 前端入口。

**Non-Goals:**
- 不做服务端分页、虚拟列表或无限滚动。
- 不新增可视化 Compose 表单生成器、模板市场或新的状态库。
- 不修改 Docker/compose 后端操作语义。

## Decisions

- **客户端统一分页**：资源列表由 REST 拉取，使用一个纯函数和一个共享 Pagination 组件，每页固定 15 条。
- **删除通用 Events**：删除 Events 页面、导航、`/v1/events` 端点和前端事件派生状态，不保留兼容入口。
- **专用容器状态流**：`/v1/docker/containers/stream` 仅推送容器 `id/state/status`；前端按 ID 合并到 REST 容器列表，不通过 SSE 覆盖镜像、端口、Stack 等列表字段。
- **专用主机与日志流**：Dashboard CPU/内存继续使用 `/v1/docker/stats/stream`；Stack 与 Container 日志继续使用各自日志 SSE。
- **主从工作区**：桌面列表与详情并排且分别拥有明确滚动区域；移动端详情替代列表并提供返回按钮。
- **Container 渐进披露**：状态、镜像、端口、所属 Stack 和主要操作首屏可见；Logs/Terminal 是主工具切换；环境、挂载和网络使用折叠分组。
- **Compose 代码优先**：创建和编辑共用 StackWorkspace。右侧摘要只展示可从文本轻量推导的 service 名称、端口/卷/网络计数；后端仍是最终 YAML 校验边界。
- **创建并部署**：先创建栈文件，再执行 start。创建成功但部署失败时保留工作区、显示输出，并允许重试；“仅保存”只创建文件。

## Risks / Trade-offs

- 客户端分页仍会通过 REST 接收完整列表；只有资源量明显增长并产生测量证据时才改服务端分页。
- textarea 无法提供完整 YAML AST 诊断；本变更不为此新增 CodeMirror 依赖。
- 新建后部署是两次 API 调用，可能出现“已创建但未部署”；UI 必须准确展示该状态而不是回滚已保存文件。
