## ADDED Requirements

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
