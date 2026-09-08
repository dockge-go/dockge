## Why

巡视 Containers 与 Stacks 交互后，发现四处与 Apple 设计系统不符的细节：① 图标按钮与表格行没有按压反馈（Apple HIG 要求所有可点元素有按压响应，`btn` 已有 `:active` 缩放而 `btn-icon`/表格行缺失）；② 删除/清空确认使用原生 `confirm()`，系统级灰框与整体风格冲突，iOS 的确认应是居中圆角卡片 + 按钮分隔排布；③ Containers 的 "Show all" 使用原生 checkbox，而 iOS 的标志性组件是开关（switch）；④ 视图切换直接替换 innerHTML，无过渡，显得生硬。

## What Changes

- 按压反馈补齐：`.btn-icon`/`.lang-btn` 增加 `:active` 缩放，表格行增加 `:active` 按下高亮（iOS 列表 cell 行为）。
- 用 Apple 风格确认对话框（居中卡片、毛玻璃遮罩、圆角 18px、按钮分隔排布、破坏性操作红色、出现缩放动画）替换全部原生 `confirm()` 调用（容器删除、清空停止容器、栈删除、镜像/数据卷/网络清理、磁盘清理）。
- "Show all" 改为 iOS 风格开关（绿色轨道、白色圆钮、滑动动画）。
- 视图切换增加淡入过渡（仅导航时触发，4 秒轮询重渲染不触发，避免周期性闪烁）。

## Capabilities

### New Capabilities

### Modified Capabilities
- `design-mock`: 追加 Apple 风格交互打磨契约——按压反馈、iOS 确认对话框、iOS 开关、视图切换过渡。

## Impact

- 仅改动 `crateman_web_index (3).html`：新增 dialog/switch CSS 与 `confirmDialog()` 组件、替换 8 处原生 confirm 调用点（改为回调式）、字典新增 `common.ok` 一键。
- 确认对话框由同步 `confirm()` 改为异步回调，仅影响原型内部调用点，无外部契约。
