## 1. 明暗主题

- [x] 1.1 head 内联脚本：读取 `dockge.theme`（缺省跟随 `prefers-color-scheme`）并在首帧前设置 `documentElement.dataset.theme`；未手动选择时监听系统主题变化实时跟随（验证：dark 用户刷新首帧即暗色，无浅色闪现）
- [x] 1.2 CSS 暗色块：新增 `:root[data-theme="dark"]` 覆盖约 20 个设计变量（bg/surface/fg/border/accent/状态色/阴影）（验证：切 dark 后侧边栏、卡片、表格、表单、下拉、Sheet、Toast 均为暗色）
- [x] 1.3 硬编码色治理：`.btn-clear:hover` 改 color-mix；`.warning-banner` 增加暗色规则；核对终端/编辑器/事件流/日志保持深色不变（验证：grep 无残留仅浅色可用的硬编码背景）
- [x] 1.4 顶栏主题切换按钮（太阳/月亮 SVG，aria-label 随语言），点击切换并持久化（验证：点击即时变色，刷新后保持）

## 2. 多语言

- [x] 2.1 head 内新增 i18n 核心：zh/en 全量字典（296 键，按 nav/dash/ctn/stack/img/vol/net/evt/sys/set/sheet/toast/confirm/common 分组）、`t(key, vars)` 占位插值、`applyStaticI18n()`（验证：Node 语法检查通过，zh/en 字典键集合完全一致）
- [x] 2.2 静态骨架标注：侧边栏分组/导航项、顶栏搜索与刷新、Sheet 标题等加 data-i18n / 属性标注，含 placeholder 与 aria-label（验证：切换语言后静态部分全部更新）
- [x] 2.3 render 函数文案替换：dashboard/containers/stacks/images/volumes/networks/events/sysinfo/sysdf/settings 及全部 Sheet、Toast、confirm/prompt、topbarTitle、ctrStatusLabel/ctrStatusText、timeAgo、状态徽标改走 `t()`（验证：任一视图下切换语言，表格、按钮、空状态、Toast 全部跟随；4 秒轮询重渲染后语言不回退）
- [x] 2.4 顶栏语言切换控件（中/EN 分段控件），切换时更新 html lang、document.title、持久化并重渲染（验证：切换后 `<html lang>` 与标题变化，刷新后保持；默认 zh-CN）

## 3. 验证与归档

- [x] 3.1 浏览器四象限实测：明/暗 × 中/英，逐视图（Dashboard/Containers/Stacks/Images/Volumes/Networks/Events/System Info/Disk Usage/Settings + 容器详情 Sheet 各 Tab + 新建 Sheet）核对，记录遗漏文案并修复（验证：暗×中与亮×英截图经视觉模型检查无遗漏与对比度问题；英文态 9 视图扫描零中文残留；实测中发现并修复 window.renderCurrentView 未暴露导致切换语言后内容区不刷新的缺陷）
- [x] 3.2 持久化与系统跟随实测：选 dark+en 刷新验证保持；清除 localStorage 后跟随系统；4 秒轮询下语言稳定（验证：浏览器实测通过）
- [x] 3.3 openspec sync-specs → archive-change 归档（验证：openspec list 无活跃变更，归档目录存在）
