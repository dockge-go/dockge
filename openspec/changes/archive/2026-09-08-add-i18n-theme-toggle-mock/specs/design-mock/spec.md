## Purpose

定义 Dockge Web UI 设计稿（单文件 HTML 原型）的明暗主题切换与多语言切换行为，作为后续真实前端移植的交互与视觉契约。

## ADDED Requirements

### Requirement: 明暗主题切换

设计稿 SHALL 提供顶栏主题切换控件，可在 light 与 dark 两套配色间切换；两套配色 SHALL 覆盖页面全部常规区域（背景、侧边栏、卡片、表格、表单、按钮、徽标、下拉、Sheet、Toast）。

#### Scenario: 点击切换主题
- **WHEN** 用户点击顶栏主题切换按钮
- **THEN** 页面立即在明暗两套配色间切换，切换按钮图标随之变化（太阳/月亮），当前主题可见且可区分

#### Scenario: 主题持久化
- **WHEN** 用户手动选择主题后刷新页面
- **THEN** 页面以用户上次选择的主题渲染（localStorage 键 `dockge.theme`）

#### Scenario: 首次访问跟随系统
- **WHEN** 用户首次访问（无存储选择）
- **THEN** 页面按系统 `prefers-color-scheme` 渲染；在未手动选择前，系统主题变化时页面实时跟随

### Requirement: 无闪烁主题应用

主题 SHALL 在首帧绘制前生效，不得出现先浅色后暗色的闪烁（FOUC）。

#### Scenario: 暗色用户刷新页面
- **WHEN** 已选择 dark 的用户刷新页面
- **THEN** 首帧即为暗色，无浅色闪现

### Requirement: 多语言切换

设计稿 SHALL 提供 zh-CN 与 en-US 双语支持及顶栏语言切换控件；切换 SHALL 覆盖全部可见文案：侧边栏导航与分组、顶栏（标题、搜索、按钮的可见文字与 aria/placeholder 属性）、各视图标题与副标题、表头、按钮、空状态、表单标签与帮助文字、Sheet 标题与内容、Toast、confirm/prompt 对话框、页面 title 与 html lang 属性。

#### Scenario: 切换语言后全部可见文案更新
- **WHEN** 用户将语言从中文切换到英文（或反向）
- **THEN** 当前视图与静态骨架的全部可见文案立即更新为目标语言，无需刷新页面

#### Scenario: 重渲染使用当前语言
- **WHEN** 应用在任一语言下发生数据刷新（如 4 秒轮询重渲染）或打开任一 Sheet/Toast
- **THEN** 新渲染的内容使用当前所选语言

#### Scenario: 语言持久化
- **WHEN** 用户选择语言后刷新页面
- **THEN** 页面以用户上次选择的语言渲染（localStorage 键 `dockge.lang`），默认语言为 zh-CN

### Requirement: 控制台区域主题不变性

终端、YAML 编辑器、事件流、日志查看器等控制台类区域 SHALL 在明暗两种主题下保持深色控制台外观，不随主题切换。

#### Scenario: 明色主题下打开容器终端
- **WHEN** 主题为 light 且用户打开容器 Exec 终端
- **THEN** 终端区域仍为深色控制台外观，文字清晰可读
