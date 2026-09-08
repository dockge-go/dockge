## Context

设计稿是单文件原型（约 2000 行，1 个 `<style>` + 1 个 `<script>`）：设计系统由 `:root` CSS 变量驱动（浅色配色）；静态骨架仅侧边栏/顶栏/Sheet 少量文案（中文），绝大多数文案位于 10 个 render 函数的模板字符串中（英文为主，中英混杂）；存在 4 秒轮询定时 `renderCurrentView()` 重建 innerHTML，以及大量 inline onclick。

## Goals / Non-Goals

**Goals:**
- 明暗两套完整配色，切换即时生效、无 FOUC、可持久化、未选择时跟随系统。
- zh-CN / en-US 全量文案覆盖（含 Toast、confirm、prompt、aria/placeholder/title 属性）。
- 切换语言后无需刷新，重渲染内容自动使用当前语言。

**Non-Goals:**
- 不改真实前端（app/dockge/web），不做翻译文件抽离/构建管线。
- 不做 zh/en 之外的语言；不做 RTL。
- 终端/编辑器/事件流/日志查看器保持深色控制台外观，不随主题变化。

## Decisions

- **翻译做在渲染源头而非 DOM 后处理**：4 秒轮询会整体重建 innerHTML，任何基于 DOM 遍历的替换都会被冲掉。JS 模板内文案全部改为 `t(key, vars)`；`t` 支持 `{n}` 等占位符插值。备选方案（MutationObserver 持续翻译）被否决：复杂且脆。
- **静态骨架用 `data-i18n`（文本）/ `data-i18n-attr`（属性）声明 + `applyStaticI18n()` 应用**：语言切换或初始化时一次性更新侧边栏/顶栏/Sheet 等静态节点。
- **暗色 = `:root[data-theme="dark"]` 变量覆盖块**：设计系统纪律良好，绝大多数颜色走变量，暗色板只需覆盖约 20 个变量（Apple 暗色系：`--bg:#000`、`--surface:#1c1c1e`、`--fg:#f5f5f7`、`--accent:#0a84ff` 等）。少量硬编码色就地治理：`.btn-clear:hover` 的 `#fef2f2` 改 `color-mix(var(--danger))`（两主题通用）；`.warning-banner` 增加暗色专用规则。备选（filter/invert 方案）被否决：破坏品牌色。
- **控制台类区域（终端/编辑器/事件流/日志）保持深色**：其底色本就是深色固定值（#0d0d0d 等），在两种主题下不变，浅色主题下作为"控制台窗口"呈现是常见设计。
- **主题初始化放 head 内联脚本**：在 `<style>` 前读取 localStorage 并设置 `document.documentElement.dataset.theme`，防 FOUC；未手动选择时监听 `matchMedia` 变化实时跟随；用户手动切换后即以手动选择为准。
- **新增独立 `<script>` 块（位于主脚本之前）承载 i18n 字典与主题逻辑**：保证主脚本首次 `navigateTo('dashboard')` 时 `t()` 已可用；切换控件的事件绑定用 `DOMContentLoaded`。
- **存储键与默认值**：`dockge.theme`（light/dark）、`dockge.lang`（zh/en），默认 zh-CN；页面 title 随语言切换，`<html lang>` 同步。
- **文案字典规模**：约 170 键 × 2 语言，集中在新增 head 脚本内，key 按视图/控件分组命名（`nav.*`、`dash.*`、`ctn.*`、`stack.*`、`img.*`、`vol.*`、`net.*`、`evt.*`、`sys.*`、`set.*`、`sheet.*`、`toast.*`、`confirm.*`、`common.*`）。

## Risks / Trade-offs

- 文案量大、散布在模板字符串中，逐条替换易漏：以浏览器四象限（明/暗 × 中/英）逐视图截图验收兜底，遗漏文案在验收中暴露。
- `color-mix(in oklab)` 需 Chrome 111+ / Safari 16.2+：原型仅面向现代浏览器验收，可接受。
- inline onclick 字符串中的 toast/confirm 消息与普通模板同等对待，全部改走 `t()`，存在字符串拼接转义风险：替换时保持原有 `esc()` 习惯。
- i18n 字典使单文件体积增加约 8-10KB：原型阶段可接受。
