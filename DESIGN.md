# Dockge Design System

## 1. Atmosphere & Identity

Dockge 是一间安静、精确的单机容器控制室。界面采用 Apple transactional UI 的克制层级：中性背景、薄边界、稀缺蓝色操作色和高密度但不拥挤的数据排布。标志性体验是桌面主从工作区：列表始终保留上下文，详情与编辑在同一页面展开；移动端则切换为可返回的全屏任务页。

## 2. Color

| Role | Token | Value | Usage |
| --- | --- | --- | --- |
| Canvas | `--bg` | `#ffffff` | 页面与工作区主背景 |
| Secondary surface | `--surface` | `#f5f5f7` | 侧栏、分组、次级区域 |
| Warm surface | `--surface-warm` | `#fbfbfd` | 表单与空状态背景 |
| Console | `--surface-dark` | `#1d1d1f` | 编辑器、日志、终端 |
| Primary text | `--fg` | `#1d1d1f` | 标题与正文 |
| Secondary text | `--fg-2` | `#424245` | 次级正文 |
| Muted text | `--muted` | `#6e6e73` | 描述与辅助信息 |
| Border | `--border` | `#d2d2d7` | 输入框与主要分隔 |
| Soft border | `--border-soft` | `#e8e8ed` | 列表行与弱分隔 |
| Primary action | `--accent` | `#0071e3` | 主按钮、链接、焦点 |
| Success | `--success` | `#16a34a` | 在线与运行中 |
| Warning | `--warn` | `#eab308` | 暂停与风险提示 |
| Destructive | `--danger` | `#dc2626` | 删除与失败 |

蓝色仅用于主操作、链接、选中和焦点，不作为装饰。控制台区域始终保持深色。新增颜色必须先进入本表，再进入 CSS。

## 3. Typography

| Level | Token | Size | Weight | Usage |
| --- | --- | --- | --- | --- |
| Page title | `--text-xl` | 28px | 600 | 页面与工作区标题 |
| Section title | `--text-lg` | 21px | 600 | 分组与详情标题 |
| Body | `--text-base` | 17px | 400 | 表单正文 |
| Control | `--text-sm` | 14px | 400-600 | 表格、按钮、标签 |
| Metadata | `--text-xs` | 12px | 400-600 | ID、时间、辅助状态 |

- Display: `SF Pro Display`, `SF Pro Icons`, `Helvetica Neue`, Helvetica, Arial, sans-serif。
- Body: `SF Pro Text`, `SF Pro Icons`, `Helvetica Neue`, Helvetica, Arial, sans-serif。
- Mono: `SF Mono`, ui-monospace, `JetBrains Mono`, Menlo, Monaco, Consolas, monospace。
- 数据使用 tabular figures；长 ID 与路径使用 mono 并允许截断或任意断行。

## 4. Spacing & Layout

基础单位为 4px，沿用 `--space-1` 至 `--space-16`。页面最大宽度默认 1200px；主从工作区可扩展至 1440px。

- 桌面（>=1024px）：主列表与详情组成 `list-detail`，列表宽 320-380px，详情为剩余空间。
- 平板（769-1023px）：列表宽 280px；隐藏非关键详情侧栏，编辑器占主区域。
- 移动（<=768px）：列表与详情不并排；详情、创建和编辑占满内容区并提供返回操作。
- 工作区采用固定 header/footer 与唯一可滚动 body；滚动子项必须设置 `min-height: 0`。
- 表格在窄屏转换为重点字段列表，不让主要内容产生双向滚动。

## 5. Components

### Pagination
- **Structure**: 结果范围、页码、上一页、下一页。
- **Behavior**: 固定每页 15 条；数据或筛选使当前页越界时回到最后有效页；切换筛选回第 1 页。
- **States**: 首末页禁用对应按钮；0-15 条仍展示简洁结果范围但不制造无效操作。
- **Accessibility**: `nav` + `aria-label`，当前页使用 `aria-current`，按钮保持可见焦点。

### Live status
- **Structure**: 状态点、已连接/重连中、最近更新时间。
- **Behavior**: Containers 与 Stacks 清单由 REST 加载；Containers 与 Dashboard 共享 `/v1/docker/containers/stream` 容器状态流。主机 CPU/内存与两类日志使用各自专用 SSE。
- **Motion**: 仅连接正常时脉冲；`prefers-reduced-motion` 下静止。

### Master-detail workspace
- **Structure**: 可分页列表、固定详情 header、唯一滚动详情 body、固定操作 footer。
- **Behavior**: 桌面原位选中；移动端详情全屏。删除当前资源后关闭详情并保留合理列表页。
- **States**: 无选择、加载、错误、已删除、超长名称和空字段均有明确表现。

### Container detail
- **Structure**: 首屏摘要、主要操作、Logs/Terminal 分段控件、环境变量/挂载/网络折叠区。
- **Behavior**: 不再用 6 个同权 Tab；状态与摘要持续消费共享快照，inspect 只加载低频元数据。
- **Safety**: 删除保持二次确认；运行控制不与详情导航混排。

### Stack workspace
- **Structure**: 左侧 Stack/文件导航，中间 Compose 编辑器，右侧服务与部署摘要；底部为持久操作栏。
- **Modes**: `detail`、`create`、`edit` 共用空间模型；外部栈为只读。
- **Actions**: 新建主操作“创建并部署”，次操作“仅保存”；编辑主操作“保存并重新部署”，次操作“保存”。
- **Failure**: 校验、保存或部署失败不得关闭工作区；操作输出在编辑器附近展示。

### Resource table
- **Structure**: 页面标题/操作、筛选、数据区域、Pagination。
- **Variants**: Containers、Stacks、Images、Volumes、Networks。
- **Behavior**: 所有列表客户端分页 15 条；Dashboard 最近容器固定 5 条，不使用分页。

## 6. Motion & Interaction

| Type | Duration | Easing | Usage |
| --- | --- | --- | --- |
| Micro | `--motion-fast` 150ms | `--ease-standard` | hover、press、focus |
| Standard | `--motion-base` 220ms | `--ease-standard` | 详情切换、移动端进入 |

只动画 `transform` 与 `opacity`。实时状态更新不闪烁整行；状态颜色和文本直接更新。资源列表、配置与非状态字段不得从长连接整表推送。

## 7. Depth & Surface

采用 mixed 但克制的层级：常规信息依靠 tonal shift 与 1px 边界，只有浮层、菜单和移动端全屏过渡使用 `--elev-raised`。禁止为普通表格行和每个小分组增加卡片阴影。

## 8. Accessibility Constraints & Accepted Debt

### Constraints
- 目标 WCAG 2.2 AA，正文对比度至少 4.5:1，大字至少 3:1。
- 所有行选择、分页、折叠区和主操作可键盘完成，并有清晰焦点。
- 图标按钮必须有可读名称；颜色不是状态的唯一表达。
- 375px、768px、1280px 均不得出现主内容水平溢出。
- 尊重 `prefers-reduced-motion`。

### Accepted Debt

| Item | Location | Why accepted | Owner / Exit |
| --- | --- | --- | --- |
| ~~原型使用 textarea 而非完整 CodeMirror~~ | Stack 编辑器 | — | ✅ **已解决**（2026-09-12 引入 CodeMirror 6：YAML 高亮、行号、当前行、undo、Tab 缩进；深色控制台岛主题） |
