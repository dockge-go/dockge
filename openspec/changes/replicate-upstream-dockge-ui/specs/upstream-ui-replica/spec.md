## Purpose

定义对上游 louislam/dockge（master/1.6-dev 基线）页面布局、导航结构与视觉语言的复刻契约；技术栈保持本项目的 Go + SolidJS 栈，复刻的是用户可见的行为与外观，不是上游实现。

## ADDED Requirements

### Requirement: 全局布局与导航

应用 SHALL 采用上游布局：桌面顶栏（左：logo + 标题；右：Home 入口 + 圆形首字母头像下拉菜单——含用户名、Scan Stacks Folder、Settings、Logout）；无左侧全局导航栏。移动端顶栏隐藏，内容纵向排列。

#### Scenario: 顶栏导航

- **WHEN** 已登录用户处于任意页面
- **THEN** 顶栏可见 logo 与 Dockge 标题，头像菜单提供 Settings 与 Logout；页面间经由首页栈列表与头像菜单导航

### Requirement: 首页仪表盘

首页 SHALL 复刻上游双栏：左栏为「+ Compose」主按钮与栈列表侧栏（搜索框 + 状态 pill + 栈名；外部栈半透明）；右栏为统计卡片（active/exited/inactive 计数）与 Docker Run 转换器（textarea + Convert to Compose，转换结果落入新建栈编辑器）。

#### Scenario: 栈列表搜索

- **WHEN** 用户在侧栏搜索框输入关键字
- **THEN** 栈列表即时按名称过滤

### Requirement: 栈详情双栏工作区

栈详情 SHALL 复刻上游结构（无标签页）：标题行 = 状态 pill + 栈名 + 操作按钮组；左栏 = 容器卡片列表（服务名、镜像、状态、端口徽章、终端入口、单服务启停、CPU/内存统计；编辑态可增删容器与编辑配置）；右栏 = compose 文件名 + CodeMirror 编辑器，编辑态追加 .env 编辑器与 Networks 卡片。浏览态左栏显示合并日志终端。

#### Scenario: 浏览态操作组

- **WHEN** 栈处于浏览态
- **THEN** 提供 Edit / Start(或 Restart) / Update / Stop / Down / Delete；Delete 需确认且成功后返回首页

#### Scenario: 编辑态操作组

- **WHEN** 栈处于编辑态

- **THEN** 提供 Deploy（保存并 up -d）/ Save Draft（仅保存）/ Discard（放弃修改重新加载）

#### Scenario: 操作输出

- **WHEN** 执行 deploy/update/down 等操作
- **THEN** 进度输出显示于栈详情内的终端区域，失败不离开页面

### Requirement: 校验闭环迁移

上游栈编辑器 SHALL 保留本项目已实现的校验闭环：停止输入 2 秒后自动草稿校验（语法 + compose 语义），错误带行号定位到编辑器，工具栏四态指示，保存不被语义校验阻塞。

#### Scenario: 编辑期语义错误

- **WHEN** 用户输入结构非法的 compose YAML
- **THEN** 防抖后错误呈现于编辑器（有行号则行内标记），无需保存或部署

### Requirement: 设置与认证流

设置 SHALL 复刻上游五子页：General（栈目录等只读信息）、Appearance（主题 light/dark/auto + 语言）、Security（改密 + Disable/Enable Auth）、Global Env（环境变量编辑器）、About（版本信息）。认证流复刻：Setup（首次建 admin）、Login（用户名/密码 + Remember me）；无用户管理页。

#### Scenario: Disable Auth

- **WHEN** admin 在 Security 页 Disable Auth 并确认密码
- **THEN** 后续访问免登录；Enable Auth 可恢复

### Requirement: 视觉语言

 SHALL 复刻上游视觉：主色 #74c2ff 与蓝→绿渐变（主按钮/头像）、pill 大圆角（50rem）、卡片圆角 + 大投影 shadow-box、GitHub 风暗色（#0d1117/#161b22/#1d2634/#b1b8c0）、编辑器与终端 JetBrains Mono、状态色 running=蓝/exited=红/inactive=灰。实现 SHALL NOT 引入 Bootstrap 或任何 UI 框架依赖（手写 CSS 复刻）。

#### Scenario: 暗色主题为默认基线

- **WHEN** 用户未选择主题
- **THEN** 跟随系统 prefers-color-scheme；用户可显式选择 light/dark/auto 并持久化
