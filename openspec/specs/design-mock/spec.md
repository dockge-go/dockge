# design-mock Specification

## Purpose

定义 Dockge Web UI 的主题、双语、控制台外观与 Apple 风格交互细节契约。本规格源自单文件 HTML 设计原型（原型文件已退役删除），现已由 SolidJS 前端（`app/dockge/web/`）实现。资源分页、实时数据通道、主从工作区与 Compose 编辑器等结构性交互契约分别由 `resource-pagination`、`realtime-resource-views`、`resource-workspaces`、`compose-stack-editor` 规格承载，本规格不重复定义。

## Requirements

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
- **WHEN** 应用在任一语言下发生数据刷新（实时状态更新重渲染）或打开任一 Sheet/Toast
- **THEN** 新渲染的内容使用当前所选语言

#### Scenario: 语言持久化
- **WHEN** 用户选择语言后刷新页面
- **THEN** 页面以用户上次选择的语言渲染（localStorage 键 `dockge.lang`），默认语言为 zh-CN

### Requirement: 控制台区域主题不变性

终端、YAML 编辑器、日志查看器等控制台类区域 SHALL 在明暗两种主题下保持深色控制台外观，不随主题切换。

#### Scenario: 明色主题下打开容器终端
- **WHEN** 主题为 light 且用户打开容器 Exec 终端
- **THEN** 终端区域仍为深色控制台外观，文字清晰可读

### Requirement: 容器列表行操作

容器列表行点击 SHALL 进入该容器的主从详情（见 `resource-workspaces`）；行内 SHALL 提供启动/停止（与当前状态相反的动作）、重启、删除三个操作键；删除 SHALL 经二次确认对话框执行。

#### Scenario: 行操作键集合
- **WHEN** 渲染任一非系统容器行
- **THEN** 操作区含 启动/停止、重启、删除 三个操作键；系统容器行仅显示系统徽标

#### Scenario: 行删除确认
- **WHEN** 用户点击容器行的删除按钮
- **THEN** 弹出确认对话框，确认后才执行删除

#### Scenario: 行点击进入详情
- **WHEN** 用户点击容器行主体（非操作键区域）
- **THEN** 该行选中并在相邻详情区打开该容器的工作区，日志与终端经详情工具切换使用

### Requirement: Apple 风格按压反馈

所有可点击元素 SHALL 提供按压反馈：图标按钮与语言切换按钮按下时轻微缩放；表格行按下时整行高亮（iOS 列表 cell 行为），松开恢复。

#### Scenario: 按下图标按钮
- **WHEN** 用户按下容器行的任意图标操作按钮
- **THEN** 按钮出现轻微缩放反馈，与现有 `.btn:active` 行为一致

#### Scenario: 按下表格行
- **WHEN** 用户按住容器/栈列表行
- **THEN** 整行出现按下高亮，松开后恢复 hover/常态

### Requirement: iOS 风格确认对话框

所有删除/清空类确认 SHALL 使用应用内 Apple 风格对话框呈现：居中圆角卡片、毛玻璃遮罩、消息文本、按钮分隔排布（取消为普通色、破坏性确认为红色、垂直分隔线），出现时有缩放淡入动画；SHALL NOT 使用浏览器原生 `confirm()`。

#### Scenario: 删除容器确认
- **WHEN** 用户在容器行或详情中点击删除
- **THEN** 弹出居中卡片式确认对话框（非原生 confirm），确认按钮为红色，点击确认后执行删除并关闭对话框

#### Scenario: 清空类确认
- **WHEN** 用户点击 清空已停止/清空未使用镜像/数据卷/网络/磁盘清理
- **THEN** 同样弹出该对话框；取消则不执行任何操作

### Requirement: iOS 风格开关

Containers 视图的 "Show all" 筛选 SHALL 使用 iOS 风格开关（圆角轨道 + 白色圆钮，开启时轨道为成功色，切换有滑动动画），SHALL NOT 使用原生 checkbox。

#### Scenario: 切换显示全部
- **WHEN** 用户切换 Show all 开关
- **THEN** 开关圆钮滑动、轨道变色，列表立即在 仅运行中/全部 之间切换

### Requirement: 视图切换过渡

导航切换视图时 SHALL 有轻微淡入过渡；数据刷新导致的重渲染 SHALL NOT 触发过渡，避免周期性闪烁。

#### Scenario: 点击侧边栏导航
- **WHEN** 用户从 Containers 切换到 Stacks
- **THEN** 新视图以淡入方式出现

#### Scenario: 实时数据刷新
- **WHEN** 实时状态更新触发当前视图重渲染
- **THEN** 内容更新但不播放过渡动画

### Requirement: 实时更新的安静反馈

系统自发状态变化（长连接推送）SHALL 只更新数据与连接状态指示，SHALL NOT 弹出 toast 打扰；toast 仅用于用户主动操作的反馈；实时连接状态 SHALL 以状态点与「已连接/重连中」文案表达（流契约见 `realtime-resource-views`）。

#### Scenario: 实时状态更新不弹窗
- **WHEN** 容器状态长连接推送新状态帧
- **THEN** 列表/详情与仪表盘统计随之更新，无 toast 弹出

#### Scenario: 断连与恢复
- **WHEN** 长连接断开或恢复
- **THEN** 仅状态点与连接文案变化，恢复后自动续流，不打断用户操作
