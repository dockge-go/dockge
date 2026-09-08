# Apple 风格交互打磨方案

## 现状（巡视结论）

`.btn:active { transform: scale(0.98) }` 已存在，但 `.btn-icon`/`.lang-btn`/表格行均无按压反馈；8 处 `confirm()` 原生对话框；Show all 原生 checkbox；`navigateTo`/`renderCurrentView` 均直接替换 innerHTML。

## 方案

### 1. 按压反馈（纯 CSS）

```css
.btn-icon:active, .lang-btn:active, .tab:active { transform: scale(0.96); }
.data-table tbody tr:active { background: color-mix(in oklab, var(--fg), transparent 94%); }
```

行高亮用 fg 的 color-mix 而非固定灰，明暗两主题自适应。`.btn-icon` 补 transition transform。

### 2. iOS 确认对话框

- 结构：`#dialog-overlay`（fixed、rgba 遮罩 + blur、opacity 过渡）内 `#dialog-alert`（居中 300px 卡片、radius-lg 18px、var(--bg)、elev-raised、出现动画 scale 0.96→1 + fade，--ease-standard）。
- 按钮：垂直排布、顶部 1px 分隔线（--border-soft）、按钮间 1px 分隔；取消为普通前景色、破坏性确认为 --danger、均 600 字重、:hover 浅灰、:active 深灰（iOS alert 交互）。
- JS：`confirmDialog(msg, onConfirm, { destructive: true })`；Enter=确认、Escape=取消；打开时焦点落确认按钮。回调式替代同步 confirm。
- 调用点替换（8 处）：doRemove、clearStoppedContainers、removeStack、clearUnusedImages、clearUnusedVolumes、clearUnusedNetworks、sysdf 的 pruneAll 内联 onclick 与 prune 按钮内联 onclick（后两处改为具名函数 sysdfPruneAll/sysdfPrune 便于绑定）。
- 字典新增 `common.ok`：zh '确认' / en 'OK'。

### 3. iOS 开关

```html
<label class="switch-row"><span class="switch"><input type="checkbox" ...><span class="switch-knob"></span></span> 显示全部</label>
```

轨道 40x24 pill：关闭 var(--border)，开启 var(--success)；圆钮 20px 白色，transform translateX 过渡 --motion-fast --ease-standard；input 视觉隐藏仅留键盘可达（focus-visible 时轨道外环）。

### 4. 视图切换过渡

```css
.content-area.nav-anim .view-section { animation: view-in var(--motion-base) var(--ease-standard); }
@keyframes view-in { from { opacity: 0; transform: translateY(4px); } }
```

`navigateTo` 渲染前 `classList.add('nav-anim')`；`renderCurrentView`（轮询/操作刷新）渲染前 `remove('nav-anim')`。

## 取舍

- 不改原生 `prompt()`（重命名/打标签等输入场景）：输入对话框形态与删除确认不同，且不在本次 Containers/Stacks 列表交互主路径上，留待后续统一。
- 表格行不加 user-select:none：容器 ID/镜像名是用户高频复制目标，保留文本选择优先于原生应用观感。
