## Why

用户决定放弃现有自研原型 UI（Crate 设计系统 + 资源工作区），改为**功能与页面布局一比一复刻上游 louislam/dockge**（基线 master/1.6-dev，2026-04），技术栈不变（Go + Gin + bbolt + samber/do；SolidJS + Vite；go:embed 单二进制）。编辑 compose.yaml 的体验仍是最高优先级（校验闭环整体迁移到新编辑器）。

## What Changes

- **清空现有原型**：删除自研视图（容器/镜像/网络/数据卷/系统信息/磁盘占用/用户管理七页）、Crate 样式体系；`DESIGN.md` 重写为上游视觉契约。
- **复刻上游页面**：首页（统计卡片 + docker-run 转换器）、栈列表侧栏（搜索 + 状态 pill）、栈详情（双栏：容器卡片 + CodeMirror 编辑器）、容器终端、设置五子页（general/appearance/security/globalEnv/about）、Setup/Login。
- **认证 UI 对齐单用户模型**：Security 子页 = 改密 + Disable Auth；删除用户管理页。后端 OIDC/forwardAuth 能力保留（无 UI 入口）。
- **后端补缺**：单服务 start/stop/restart、单容器 CPU/内存统计轮询端点。
- **偏差登记（有意）**：无多 Agent（single-host-boundary 维持）、无 /console 宿主 shell（Q5 维持）、语言保留 zh/en 双语（上游 32 语言不含）、不引 Bootstrap（手写 CSS 复刻视觉，Non-Goal #3 维持）。

## Capabilities

### New Capabilities

- `upstream-ui-replica`: 上游 dockge 的布局/页面/视觉复刻契约（顶栏、首页双栏、栈详情双栏、设置子页、单用户认证流）。

### Modified Capabilities

- `compose-stack-editor`: 工作区从三栏改为上游双栏（容器卡片栏 + compose/.env 上下排列编辑器），校验闭环迁移至新编辑器，操作按钮组对齐上游语义（Deploy/Save Draft/Discard vs Edit/Start/Restart/Update/Stop/Down/Delete）。
- `resource-workspaces`: 该能力随资源页面移除而整体下线（REMOVED）。

## Impact

- 前端：`web/src` 大部分重写（views/components/styles/store/api）；保留 lib 工具、i18n 基建（文案对齐上游 zh-CN/en）、xterm 终端组件、CodeMirror 编辑器（换 Dracula 风主题 + 校验闭环）。
- 后端：新增 2 个小端点（单服务操作、单容器统计）；其余 API 不动。
- 文档：`DESIGN.md` 重写、`PROJECT_SPEC.md` §3/§4/§6、`REQUIREMENTS.md` §6、`docs/UI-ARCHITECTURE.md` 更新；`docs/prototype/vision.html` 删除。
- 已迁移资产：校验闭环（`/v1/stacks/validate` + 防抖 + lint 注入）原样保留。
