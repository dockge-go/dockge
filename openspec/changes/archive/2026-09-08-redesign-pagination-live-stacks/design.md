# 分页 / 长连接实时 / 详情页与栈工作台重设计

## 现状与问题

- 各列表无分页；mock 数据少看不出来，移植后不可用。
- 4 秒 setInterval：每个非系统容器 15% 概率随机翻转状态并弹 🟢/🔴/🟡 toast，同时每 8 秒随机事件——噪声源。
- 容器/栈详情都是 560px Sheet：容器 5 Tab + 顶部 4 操作按钮 + footer 3 按钮挤在一栏；栈编辑器在 Sheet 里高度受限。
- New Stack 是表单 Sheet（名称 + 空白 YAML textarea），创建后与编辑/部署脱节。

## 方案

### 1. 分页（全列表统一）

- `state.pagination = { containers:1, images:1, volumes:1, networks:1, stacks:1, events:1 }`，`PAGE_SIZE = 15`。
- `paginate(arr, key)` 返回 `{ slice, page, pages, total }`；`paginationHtml(key, total)` 生成控件（‹ 按钮 / 第 p/n 页 · 共 t 条 / › 按钮），越界自动收敛；toggleAll 等筛选变化重置对应页码为 1。
- 渲染接入：renderContainers/renderImages/renderVolumes/renderNetworks/renderStacks(左栏栈较多时自身滚动不分页，工作台模式下左栏为滚动列表——**决定：栈左栏为滚动列表不分页，主列表数据量通常个位数；其余五个视图分页**。重新审视 spec：spec 写了编排栈分页……更正：栈工作台左栏滚动列表即可满足简洁；为守 spec，左栏超过 15 条也分页？不——修改 spec 措辞不必，实现"分页或等价的滚动列表"？不行，spec 已写。简单起见：左栏列表分页同样支持（>15 时底部分页），代码同组件复用，成本低。
- 事件视图：分页作用于已捕获事件（新事件到来若在第 1 页且当前在第 1 页则增量可见——直接重渲染当前页即可）。

### 2. LiveFeed（模拟 WebSocket）

- 删除 4 秒随机翻转轮询与 8 秒随机事件池；删除 nowRunning/nowStopped/nowPaused 自发 toast 及字典键。
- 新 `LiveFeed`：每 6 秒最多产生 1 个事件（30% 概率静默跳过），事件类型固定化：`health`（不变状态）、`restart`（running→短暂 updating→running）、`oom-stop`（低频，running→exited）。事件更新 state.containers + addEvent() + 若当前视图受影响则 renderCurrentView()（无 nav-anim 不闪烁）。
- 仪表盘统计/容器列表由同一 state 派生，自动一致。
- conn-bar（已有）作为连接指示，事件视图计数实时。

### 3. 容器详情推入页

- 路由扩展：`state.route` 支持 `'container-detail'`，`state.detailContainerId`；viewRenderers 注册 renderContainerDetail；侧边栏无对应项（高亮 containers）。
- renderContainerDetail：面包屑（‹ 容器列表 / nginx-web）+ 状态徽标 + 右侧操作（日志/终端弹框、启停/重启/删除=confirmDialog）；概要卡行（镜像/端口/运行时长/创建时间，四格 grid）；Tabs（统计/环境/网络/挂载——复用现有 tab 内容，只读）。
- showDetail 退役；搜索 action、openStackToService 残留引用改 openContainerDetail。
- Sheet 仍用于：表单（新建容器/镜像/卷/网络、复制、打标签）、日志/终端弹框。

### 4. 栈工作台（参照原版 dockge）

- renderStacks 重写为双栏：
  - 左栏（240px）：New Stack 按钮 + 栈列表（状态点 + 名称；active 高亮 accent；点击 selectStack）；分页复用。
  - 右栏工作台（选中栈）：
    - 顶栏：栈名（display 字体）+ 状态徽标 + 右侧 [启动/停止] [删除] [部署 primary]
    - 服务 pills 行：`web ●` `api ●` `db ○`，点击 → showServiceLogs（日志弹框含终端按钮入口：日志弹框 footer 加"终端"按钮 → showServiceExec）
    - 编辑器：现有 editor-wrap 组件（行号 + textarea + Validate/Format/Fullscreen 工具栏）
    - 底部终端面板（240px 高）：`$ docker compose up -d` 部署输出流；平时显示提示行；Deploy 点击后逐行输出模拟日志（定时 append），完成 toast + 状态更新 up。
  - 未选中态：空状态提示"选择或创建一个栈"。
- New Stack：dialogInput（扩展 iOS 对话框：消息 + 输入框 + 取消/创建）输入名称 → state.stacks.unshift({模板 compose}) → selectStack → 工作台即编辑器。
- 移动端 (<900px)：左栏折叠为顶部横向 chips 滚动行。
- showStackDetail Sheet 退役；栈相关 Sheet 只留单服务日志/终端。

### 5. 字典

新增：pg.info/pg.prev/pg.next（title 用 aria-label）；crumb.containers（返回容器列表）；ctn.detail（详情页面包屑用容器名即可不需键？面包屑第一段用 nav.containers ✓）；stack.workbench 相关：stack.noSelection、stack.nameLabel、stack.create、stack.namePh、stack.deployTo（部署到终端提示）、toast.deployStarted、toast.deployDone、stack.termHint。
删除：toast.nowRunning/nowStopped/nowPaused；sheet.newStack（改对话框）、stack.editor 保留（工具栏 tab 名不再用？工作台无 tab——stack.editor/stack.services 键复用在别处？服务 pills title 用 stack.viewServiceLogs ✓。stack.editor/stack.services/editHelp/save/redeploy 若工作台取代则删——保留 stack.deploy ✓）。

## 取舍

- 容器详情页不改 URL hash（原型无路由同步需求）。
- Events 分页从最新往前排（保持现有倒序展示习惯：最新在上）。
- 栈左栏分页虽实际很少触发，为契约一致性实现（>15 才显示控件）。
