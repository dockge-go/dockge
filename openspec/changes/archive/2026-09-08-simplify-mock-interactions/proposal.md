## Why

设计稿当前的交互存在绕路与混杂：容器列表点终端要先打开详情 Sheet 再延时切 Tab；栈列表点"日志"实际打开的是详情编辑器（`openStackToService` 行为与名字不符）；容器详情把日志/终端与查看项（Stats/Env/Network/Mounts）混在同一组 Tab，且 Network Tab 内嵌"连接/断开"修改操作，与"查看"心智冲突；列表行操作键过多（容器 6 键、栈 4 键其中 Edit 是无功能的死按钮）。用户要求：操作直达、查看归查看、列表操作键最小心智开销。

## What Changes

- 容器列表行操作键极简为 3 键：日志、终端、启动/停止（状态取反）；重启/删除保留在详情 Sheet footer；移除详情箭头按钮（整行点击即详情）。
- 日志/终端直达：容器与栈的日志按钮直接弹独立日志框；容器终端按钮直接弹独立终端框，不再途经详情 Sheet。
- 容器详情 Sheet 纯查看化：Tab 精简为 Info/Stats/Env/Network/Mounts（移除 Logs/Exec Tab）；Network Tab 只读展示已连接网络，移除"连接到网络/断开连接"操作。
- 栈日志聚合：栈列表行日志按钮弹出该栈全部服务的日志聚合框（带服务名前缀）；单服务日志/终端按钮保留在服务行上作钻取。
- 栈服务清单位置：栈详情 Sheet 默认首屏展示 Services（服务列表），YAML Editor 退为第二 Tab；栈列表行移除无功能的 Edit 死按钮，行为 日志/启停/删除 3 键。

## Capabilities

### New Capabilities

### Modified Capabilities
- `design-mock`: 追加交互契约——容器/栈列表操作直达与极简、容器详情纯查看、栈日志聚合与服务清单首屏。

## Impact

- 仅改动根目录单文件 `crateman_web_index (3).html`：renderContainers/renderStacks 行按钮、showDetail 的 Tab 集、新增 showContainerTerminal/showStackLogs、删除 openStackToService/connectNetwork/disconnectNetwork、栈详情默认 Tab 调整、字典键增删（网络连接类 7 键删除）。
- 不触碰真实前端与后端；无破坏性外部契约（独立原型）。
