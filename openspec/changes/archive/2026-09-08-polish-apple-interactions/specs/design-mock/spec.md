## ADDED Requirements

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
