# Resource Workspaces Implementation Plan

> **状态：已完成（2026-09-08）**。本计划对应的 OpenSpec 变更 `redesign-resource-workspaces` 已全部实现并归档（tasks 13/13）；本文仅作历史过程记录保留，当前有效规格见 `openspec/specs/`，勿据此执行。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Dockge 资源页重构为统一分页、共享实时数据和 Apple System 主从工作区，并重点完成 Compose 优先的 Stack 新建/编辑流程。

**Architecture:** 完整资源列表通过 REST 加载；全局 store 只维护 `/v1/docker/containers/stream` 容器状态流并按 ID 合并。主机 stats 与 Stack/Container 日志保留专用长连接。Container 与 Stack 页面采用同一 list-detail 空间模型。

**Tech Stack:** SolidJS 1.9、TypeScript 7、@solidjs/router、Kobalte、lucide-solid、Vite、CSS tokens。

**Spec:** `openspec/changes/redesign-resource-workspaces/design.md`

## Global Constraints

- 所有资源列表每页固定 15 条；Dashboard 最近容器固定 5 条。
- 不新增容器轮询或第二条容器 EventSource。
- 不新增 UI 框架、状态库或可视化 Compose 生成器。
- 不允许 `any`、`@ts-ignore`、`@ts-expect-error` 或 `@ts-nocheck`。
- 每次失败操作保留用户编辑上下文。
- 所有 UI 遵循根目录 `DESIGN.md`。

---

### Task 1: Pagination foundation

**Files:**
- Create: `app/dockge/web/src/lib/pagination.ts`
- Create: `app/dockge/web/src/components/Pagination.tsx`
- Test: `app/dockge/web/src/lib/pagination.test.ts`

**Interfaces:**
- Produces: `PAGE_SIZE = 15`, `paginate<T>(items, page)`, `pageCount(total)`, `clampPage(page, total)` and `<Pagination page total onPageChange />`.

- [ ] Write tests for first/last page slices and page clamping, run with the available TypeScript test runner and confirm failure before implementation.
- [ ] Implement pure helpers and the accessible shared component.
- [ ] Run the focused test and `pnpm typecheck`.

### Task 2: Single-host realtime store

**Files:**
- Modify: `app/dockge/web/src/api/api.ts`
- Modify: `app/dockge/web/src/store/index.ts`
- Modify: `app/dockge/web/src/views/Settings.tsx`
- Modify: `app/dockge/web/src/views/SysInfo.tsx`

**Interfaces:**
- Keeps: one container-status `EventSource` owned by `startContainerStatusStream()`.
- Removes: Agent request methods, types, mock routes and view sections.

- [ ] Add a type-level/build check that fails while stale Agent fields remain in `Snapshot` construction.
- [ ] Remove stale Agent contracts and retain the existing snapshot/event derivation.
- [ ] Run `pnpm typecheck`.

### Task 3: Remove Events and specialize realtime streams

**Files:**
- Delete: `app/dockge/web/src/views/Events.tsx`
- Modify: `app/dockge/web/src/store/index.ts`
- Modify: `app/dockge/internal/server/container_status.go`

**Interfaces:**
- Produces: REST resource loading plus status-only container SSE merging.

- [ ] Delete Events routes, navigation, page and generic event state.
- [ ] Add tests proving status frames and merges only carry ID/state/status.
- [ ] Load Containers/Stacks via REST and merge container status by ID.
- [ ] Run tests and `pnpm typecheck`.

### Task 4: Container master-detail

**Files:**
- Modify: `app/dockge/web/src/views/Containers.tsx`
- Create: `app/dockge/web/src/components/ContainerDetail.tsx`
- Modify: `app/dockge/web/src/styles/app.css`

**Interfaces:**
- Consumes: shared `snapshot`, `api.containerInspect`, `LogStream`, `TerminalPane`.
- Produces: selected-row master-detail layout with progressive disclosure.

- [ ] Add component behavior checks for selected detail, mobile return and action propagation.
- [ ] Paginate filtered rows, preserving valid selection and page.
- [ ] Replace the Sheet and six equal tabs with summary, Logs/Terminal segmented control and native disclosure sections.
- [ ] Run focused checks, typecheck and inspect at 375/768/1280.

### Task 5: Stack workspace

**Files:**
- Modify: `app/dockge/web/src/views/Stacks.tsx`
- Create: `app/dockge/web/src/components/StackWorkspace.tsx`
- Create: `app/dockge/web/src/components/StackEditor.tsx`
- Create: `app/dockge/web/src/lib/compose-summary.ts`
- Test: `app/dockge/web/src/lib/compose-summary.test.ts`
- Modify: `app/dockge/web/src/styles/app.css`

**Interfaces:**
- Produces: typed workspace modes `detail | create | edit`, lightweight compose summary, save/create/deploy callbacks.

- [ ] Write failing tests for service-name and ports/volumes/networks summary extraction.
- [ ] Remove `@ts-nocheck` and split editor/workspace concerns.
- [ ] Paginate Stack list and implement selected-row master-detail behavior.
- [ ] Implement create mode with stack name, compose/.env files, docker-run conversion, “仅保存” and “创建并部署”.
- [ ] Keep workspace open with output when create succeeds but deploy fails.
- [ ] Run tests and `pnpm typecheck`.

### Task 6: Remaining resource pagination

**Files:**
- Modify: `app/dockge/web/src/views/Images.tsx`
- Modify: `app/dockge/web/src/views/Volumes.tsx`
- Modify: `app/dockge/web/src/views/Networks.tsx`

**Interfaces:**
- Consumes: Pagination helpers and component.

- [ ] Add page signals and memoized 15-row slices to all three views.
- [ ] Clamp pages after delete/prune and reset to page 1 when filters change.
- [ ] Run `pnpm typecheck`.

### Task 7: Verification

**Files:**
- Verify all files above plus `DESIGN.md` and OpenSpec artifacts.

- [ ] Run focused tests, `pnpm typecheck`, `pnpm build`, and `openspec validate redesign-resource-workspaces --strict`.
- [ ] Run the TypeScript no-excuse audit and confirm no suppression directives remain in changed files.
- [ ] Launch the production preview and use Playwright at 375px, 768px and 1280px across Dashboard, Containers, Stacks, Images, Volumes and Networks.
- [ ] Exercise pagination, live event buffering, Container detail, New Stack save/deploy and Stack edit failure states; fix all visual or interaction defects found.
