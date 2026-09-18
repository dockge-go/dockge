## Context

Checkpoint `59197d5` 固定了重建前状态（清理巡逻 + 校验闭环）。上游规格调研结论：页面/布局/视觉事实全部来自 master 源码模板与样式变量（README 截图不可机读）。本项目硬边界不变：技术栈（Go/Gin/bbolt/do + SolidJS/Vite + go:embed）、单机（single-host-boundary）、无宿主 shell（Q5）、不引 UI 框架（Non-Goal #3）。

## Goals / Non-Goals

**Goals:** 上游页面集与布局一比一；视觉语言一比一（token 级）；校验闭环迁移并保持「编辑体验重中之重」；后端仅补两个小端点。

**Non-Goals:** Bootstrap/vue/socket.io 等上游实现技术；多 Agent；/console；上游 32 语言（保留 zh/en）；资源管理页面恢复。

## Decisions

**D1 复刻保真层级 = 用户可见层。** 布局结构、交互语义、视觉 token 一比一；实现技术全部本项目栈。不引 Bootstrap——用自写 CSS 复刻其观感（grid/pill/shadow-box 工具类），Non-Goal #3 维持。

**D2 视觉 token 重写。** `#74c2ff` 主色 + `linear-gradient(135deg,#74c2ff 0%,#74c2ff 75%,#86e6a9)` 渐变、danger `#dc3545`、warning `#f8a306`、pill `50rem`、卡片 `0.75~1rem` 圆角 + 大投影；暗色 GitHub 风（底 `#0d1117`/header `#161b22`/边框 `#1d2634`/文字 `#b1b8c0`）。`DESIGN.md` 整体重写为「上游复刻视觉契约」，旧 Crate token 表废弃。

**D3 路由对齐上游。** `/`（Dashboard 双栏）、`/compose`（新建）、`/compose/:name`（详情）、`/terminal/:stack/:service/:type`（容器终端页）、`/settings/:tab`、`/setup`；Login 为条件渲染非路由。SolidJS Router 原生支持。

**D4 状态与数据层沿用 REST/WS。** 上游 socket.io 事件映射到既有 API：stackStatusList→容器状态 SSE、terminal→WS 终端、settings→REST。新增端点：`POST /v1/stacks/:name/services/:service/:op`（单服务 start/stop/restart）、`GET /v1/stacks/:name/stats`（单容器 CPU/内存轮询，5s）。栈列表/详情在首页加载 + 定时 refresh（沿用 snapshot 机制，页面级 5s 轮询对齐上游）。

**D5 编辑器迁移而非重写。** `StackEditor`（CM6）保留扩展链，主题换 Dracula 风（thememirror 配色手写为 token）；校验闭环（`/stacks/validate` + 防抖 + `setDiagnostics` + 四态 pill）原样迁移；.env 编辑器上游用 lang-python 高亮，我们沿用无高亮（偏差登记，输入效率维度另期）。compose 与 .env 上下两个编辑器实例，无文件切换器。

**D6 认证 UI 收敛。** Login 复刻（floating labels + Remember me + 错误 alert）；Setup 复刻；Security = 改密 + Disable Auth（后端已有）。用户管理页删除；后端 users API 与 OIDC/proxy 保留（AUTH-DEPLOY.md 工作流不受影响，注明无 UI 入口）。

**D7 i18n 文案对齐上游。** 保留 typed zh/en 基建，键值重组为上游 en/zh-CN locale 的结构（拷贝上游文案），避免自造措辞造成「不像」。

**D8 清空清单。** 删：`web/src/views/*`、Crate 组件与样式、`docs/prototype/`、DESIGN.md 旧内容。留：`lib/`（compose-summary、pagination 等）、i18n 基建、`Terminal.tsx`（xterm，重样式）、`StackEditor` 逻辑、api/store（改造）。后端全部保留。

## Risks / Trade-offs

- [手写 CSS 复刻 Bootstrap 观感有偏差] → token 级对齐 + 逐页浏览器走查；上游截图人工比对（README 7 张图）。
- [删除资源页是功能回退] → 用户明确一比一；后端 API 保留，恢复成本低。
- [YAML↔容器卡片双向联动（上游核心复杂度）] → 首期只做单向（YAML→卡片渲染 + 卡片编辑写回 YAML 保留注释）；双向实时联动二期评估。风险登记不隐式降级：一期卡片编辑写回必须保留注释（copyYAMLComments 思路）。
- [范围大] → 四期交付（骨架/栈核心/设置认证/打磨），每期独立可构建可启动。

## Migration Plan

Checkpoint 已固化；每期结束提交一个 commit + 重建嵌入资产。回滚 = revert 对应期 commit。

## Open Questions

（无——四项决策已经用户默认确认路径处理：严格一比一/UI 单用户/跳过 Console/先提交。）
