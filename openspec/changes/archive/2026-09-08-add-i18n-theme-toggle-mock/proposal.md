## Why

crateman_web_index (3).html 是 Dockge Web UI 的设计定稿基础（后续真实前端将按它移植），但目前它是纯静态原型：配色固定浅色、文案中英混杂（静态骨架中文、JS 渲染视图英文），既没有明暗主题切换，也没有可用的多语言机制。在定稿阶段补齐这两个能力，能为后续真实前端的移植确立交互与视觉契约。

## What Changes

- 为设计稿增加明暗主题切换：顶栏新增切换按钮，暗色通过覆盖 CSS 变量实现，选择持久化到 localStorage，首次访问跟随系统 `prefers-color-scheme`。
- 为设计稿增加多语言（zh-CN / en-US）切换：顶栏新增语言切换控件，所有可见文案（侧边栏、视图、表格、表单、Sheet、Toast、确认框、页面标题）随语言切换，选择持久化到 localStorage。
- 统一现有中英混杂文案的归属：以字典为准，zh-CN 为默认语言。
- 明确设计边界：终端、编辑器、事件流、日志查看器等"控制台"类区域在两种主题下保持深色外观。

## Capabilities

### New Capabilities
- `design-mock`: crateman 设计稿原型（单文件 HTML）的 UI 行为契约：主题切换、多语言切换及其持久化。

### Modified Capabilities

## Impact

- 仅改动根目录单文件 `crateman_web_index (3).html`（新增 head 内联脚本、暗色 CSS 变量覆盖块、顶栏两个控件、约 170 条文案字典、render 函数内文案替换为 t() 调用）。
- 不触碰真实前端（app/dockge/web）与后端；真实前端的移植由后续变更提案承担。
- 无破坏性变更：文件为独立原型，无外部消费方。
