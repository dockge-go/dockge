# design-mock Specification

## Purpose

定义 Dockge Web UI 设计稿（单文件 HTML 原型）的明暗主题切换与多语言切换行为，作为后续真实前端移植的交互与视觉契约。

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

### Requirement: 容器列表操作直达与极简

容器列表每行 SHALL 仅提供 日志、终端、启动/停止（与当前状态相反的动作）三个操作键；重启与删除操作 SHALL 位于容器详情 Sheet 的 footer；行内 SHALL 不再提供详情箭头按钮，整行点击即进入详情。

#### Scenario: 点日志直接弹日志框
- **WHEN** 用户点击容器行的日志按钮
- **THEN** 立即弹出该容器的独立日志 Sheet，不途经容器详情

#### Scenario: 点终端直接进终端
- **WHEN** 用户点击容器行的终端按钮
- **THEN** 立即弹出该容器的独立终端 Sheet 且输入框获得焦点，不途经容器详情

#### Scenario: 行操作键数量
- **WHEN** 渲染任一非系统容器行
- **THEN** 操作区仅含 日志、终端、启动/停止 三个操作键；系统容器行仅显示系统徽标

#### Scenario: 重启与删除入口
- **WHEN** 用户需要重启或删除容器
- **THEN** 通过点击行进入详情 Sheet，在 footer 执行（footer 含 启动/停止、重启、删除）

### Requirement: 容器详情纯查看

容器详情 Sheet SHALL 仅承载查看项：Tab 限定为 Info/Stats/Env/Network/Mounts；Network Tab SHALL 只读展示已连接网络，不提供连接/断开操作；日志与终端 SHALL 不再作为详情 Tab 出现（已由独立弹框承载）。

#### Scenario: 详情 Tab 集合
- **WHEN** 用户打开容器详情
- **THEN** Tab 仅显示 Info/Stats/Env/Network/Mounts，无 Logs/Exec

#### Scenario: 网络 Tab 只读
- **WHEN** 用户查看容器详情 Network Tab
- **THEN** 仅展示已连接网络名称与 IP，无 Connect/Disconnect 交互

### Requirement: 编排栈日志聚合与服务清单首屏

栈列表行的日志按钮 SHALL 直接弹出该栈全部服务日志的聚合框（按服务名分段/前缀区分）；栈详情 Sheet SHALL 默认首屏展示服务清单（Services Tab 在前），YAML Editor 为第二 Tab；栈列表行 SHALL 仅含 日志、启动/停止、删除 三个操作键（移除无功能的 Edit 按钮）。

#### Scenario: 点栈日志弹聚合日志
- **WHEN** 用户点击栈行的日志按钮
- **THEN** 立即弹出包含该栈全部服务日志的聚合 Sheet，日志按服务名区分，不打开栈详情

#### Scenario: 栈详情首屏为服务清单
- **WHEN** 用户点击栈行打开详情
- **THEN** 默认选中 Services Tab，服务清单（名称/镜像/端口/状态/单服务日志与终端钻取）直接可见；Editor 为第二 Tab

#### Scenario: 栈行操作键数量
- **WHEN** 渲染任一栈行
- **THEN** 操作区仅含 日志、启动/停止、删除 三个操作键，无 Edit 死按钮

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
- **WHEN** 用户在容器详情 footer 点击移除
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

导航切换视图时 SHALL 有轻微淡入过渡；周期性数据刷新（轮询重渲染）SHALL NOT 触发过渡，避免周期性闪烁。

#### Scenario: 点击侧边栏导航
- **WHEN** 用户从 Containers 切换到 Stacks
- **THEN** 新视图以淡入方式出现

#### Scenario: 轮询刷新
- **WHEN** 4 秒轮询触发当前视图重渲染
- **THEN** 内容更新但不播放过渡动画

### Requirement: 列表分页

所有资源列表（容器、镜像、数据卷、网络、编排栈、事件）SHALL 分页展示，每页 15 条；底部 SHALL 提供分页控件（上一页/下一页与 页码/总条数 信息）；仅一页时不显示控件；筛选条件变化（如 仅运行中/全部）SHALL 重置回第一页。

#### Scenario: 超过 15 条自动分页
- **WHEN** 某列表数据超过 15 条
- **THEN** 首页仅展示 15 条，分页控件显示当前页/总页数与总条数，可翻页浏览

#### Scenario: 筛选切换重置页码
- **WHEN** 用户在容器列表切换 Show all 开关
- **THEN** 列表回到第一页重新计算分页

### Requirement: 长连接实时数据

容器状态、事件流与仪表盘容器统计 SHALL 由单一模拟 WebSocket 长连接事件源驱动：连接状态可见（已连接指示）；数据变化以低频、确定性的容器事件呈现并记入事件流；SHALL NOT 周期性随机翻转容器状态；系统自发变化 SHALL NOT 弹出 toast 打扰（toast 仅用于用户主动操作的反馈）。

#### Scenario: 实时状态更新
- **WHEN** 长连接推送容器状态变化事件
- **THEN** 容器列表/详情与仪表盘统计随之更新，事件流新增对应记录，无 toast 弹出

#### Scenario: 连接状态可见
- **WHEN** 用户打开事件视图
- **THEN** 可见连接指示与已捕获事件计数

### Requirement: 容器详情推入页

点击容器行 SHALL 导航至全宽容器详情页（非弹层）：顶部为 面包屑（容器列表 / 容器名）+ 状态徽标 + 操作区（启动/停止、重启、删除、日志、终端——日志与终端为独立弹框）；其下为概要行（镜像、端口、运行时长、创建时间）与查看 Tabs（统计/环境/网络/挂载，均只读）。

#### Scenario: 进入与返回
- **WHEN** 用户点击容器行
- **THEN** 内容区切换为该容器的全宽详情页，面包屑可返回容器列表

#### Scenario: 详情页操作直达
- **WHEN** 用户在详情页点击日志或终端
- **THEN** 直接弹出对应弹框；点击启停/重启/删除立即生效并更新页面状态

### Requirement: 编排栈工作台

Stacks 视图 SHALL 为双栏工作台：左栏为紧凑栈列表（状态点 + 名称，点击选中），右栏为选中栈的工作区——顶部（栈名、状态、服务日志、启动/停止、删除、部署）、服务 pill 行（展示栈内服务与状态，点击弹单服务日志，日志弹框含终端入口）、compose.yaml 编辑器（带行号）、底部部署终端面板；新建栈 SHALL 通过输入名称对话框创建模板并直接进入工作台编辑，一键部署在终端面板滚动输出并更新栈状态；编辑与部署在同一工作台完成，SHALL NOT 使用弹层承载栈编辑。

#### Scenario: 选中栈查看工作台
- **WHEN** 用户在左栏点击某栈
- **THEN** 右栏展示该栈的工作区：名称与状态、服务 pills、可编辑的 compose.yaml、部署终端

#### Scenario: 新建并部署
- **WHEN** 用户点击 New Stack 输入名称确认
- **THEN** 栈创建并以模板 compose.yaml 直接在工作台打开；编辑后点击部署，终端面板滚动输出部署日志，完成后栈状态更新

#### Scenario: 服务钻取
- **WHEN** 用户点击服务 pill
- **THEN** 弹出该服务的日志弹框（含终端入口）
