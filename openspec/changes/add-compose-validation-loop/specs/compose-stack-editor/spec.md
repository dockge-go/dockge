## ADDED Requirements

### Requirement: 编辑期校验闭环

Stack 工作区 SHALL 在编辑过程中被动提供校验反馈，无需用户手动触发：compose 内容停止变更约 2 秒后自动校验（`.env` 变更同样触发，因变量插值依赖两者）；校验 SHALL 同时覆盖 YAML 语法错误与 compose 语义（`docker compose config`）。校验在草稿上进行，SHALL NOT 写入栈目录或创建任何 docker 资源。

#### Scenario: 语法错误定位到行

- **WHEN** 用户输入了 YAML 语法错误（如缺少冒号的映射行）
- **THEN** 编辑器在对应行显示内联诊断标记，工具栏显示错误状态，悬停可见错误信息

#### Scenario: 语义错误暴露于编辑期

- **WHEN** YAML 语法合法但 compose 结构非法（如 `ports` 不是列表、引用了未定义的密钥文件）
- **THEN** 防抖校验返回语义错误并呈现于编辑器，无需等到保存或部署

#### Scenario: 校验通过

- **WHEN** 草稿通过语法与语义校验
- **THEN** 工具栏显示通过状态，编辑器无诊断标记

#### Scenario: 校验失败不阻断保存

- **WHEN** 草稿存在校验错误且用户选择保存
- **THEN** 保存 SHALL 按原有语法规则正常执行，不被语义校验结果阻塞；工作区保持打开

#### Scenario: 无法定位行号的错误

- **WHEN** compose 语义错误不携带行号（如 `image must be a string` 类纯结构错误）
- **THEN** 错误 SHALL 在校验面板/状态中以列表呈现，SHALL NOT 强造行内位置
