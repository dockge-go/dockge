## Purpose

定义 Compose 代码优先的 Stack 新建和编辑体验。

## ADDED Requirements

### Requirement: Compose 优先工作区

New Stack 与托管 Stack 编辑 SHALL 使用左侧名称/文件导航、中央 Compose 编辑器和右侧部署摘要的统一工作区；移动端 SHALL 按名称与文件、编辑器、摘要的顺序纵向重排。

#### Scenario: 桌面创建新 Stack
- **WHEN** 用户在桌面视口打开 New Stack
- **THEN** 页面同时展示左侧名称/文件导航、中央 Compose 编辑器和右侧部署摘要，编辑器为主要视觉区域

### Requirement: 新建栈保存与部署

New Stack SHALL 提供“仅保存”和“创建并部署”两个动作，其中“创建并部署”为主操作。

#### Scenario: 创建并部署成功
- **WHEN** 用户提交合法名称与 Compose YAML 并选择创建并部署
- **THEN** 系统先创建栈文件，再启动该栈，并打开新栈的工作区

#### Scenario: 创建成功但部署失败
- **WHEN** 栈文件已创建但启动失败
- **THEN** 工作区保持打开并显示 compose 输出，栈显示为已保存未部署，用户可修改或重试

### Requirement: 编辑失败保留上下文

校验、保存或重新部署失败时，Stack 工作区 SHALL 保留未提交文本、当前文件和操作输出，不得自动关闭。

#### Scenario: 保存托管 Stack 失败
- **WHEN** 用户保存修改后的 Compose 或 `.env` 且服务返回错误
- **THEN** 工作区保持打开，未提交文本与当前文件不变，并在编辑器附近显示错误信息
