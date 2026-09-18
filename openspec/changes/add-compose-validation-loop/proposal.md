## Why

栈编辑器的校验目前形同虚设：前端只检查是否存在一行 `services:`，后端保存仅做 `yaml.Unmarshal`，全仓没有 `docker compose config` 语义校验——非法 service 结构要等到部署时才以原始 CLI 输出爆出。web 编辑 compose.yaml 是本项目的最高优先级体验，需要一个「编辑过程中即暴露错误、并定位到行」的校验闭环。

## What Changes

- 新增 `POST /v1/stacks/validate` 草稿校验端点：接收 yaml + env 草稿，在临时目录内存中执行 `docker compose config`（不落盘、不创建资源），返回结构化错误（带行号，尽力而为）。
- 编辑器接入 `@codemirror/lint`：防抖自动校验（停止输入 2 秒后触发，compose/env 任一变更都重新校验）+ 诊断内联标记（gutter/悬停/面板复用 CodeMirror 自带交互）。
- 编辑器工具栏校验态从「services: 存在性」升级为四态：检查中 / 通过 / 语法错误 / 语义错误（带计数）。
- YAML 语法错误（`yaml.v3` 自带行列号）与 compose 语义错误统一从后端返回，前端不引入 YAML 解析器。

硬约束：Go 代码精简（复用现有 compose CLI 执行器，不新增抽象层）；交互最低心智开销（**无新增手动按钮**，纯被动自动校验 + 单一状态指示；保存不阻塞）。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `compose-stack-editor`: 新增「编辑期校验闭环」需求——被动自动校验、错误定位到行、失败不阻断保存与工作区。

## Impact

- 后端：`api/v1`（DTO + 错误）、`internal/repository/compose.go`（ValidateCompose）、`internal/service/stack.go`（Validate + stderr 解析）、`internal/handler/stack.go`、`internal/server/http.go`（路由）。
- 前端：`web/package.json`（+`@codemirror/lint`）、`web/src/api/api.ts`、`web/src/lib/validate.ts`（新，纯函数）、`web/src/components/StackEditor.tsx`、`web/src/views/Stacks.tsx`、`web/src/i18n/{zh-CN,en-US}.ts`。
- 文档：`PROJECT_SPEC.md` §3.2/§4（顺带校正 textarea 陈旧描述）、`REQUIREMENTS.md` §6.3。
- 非目标：自动补全、查找替换、`.env` 高亮（「输入效率」维度另期）；保存语义校验强阻塞。
