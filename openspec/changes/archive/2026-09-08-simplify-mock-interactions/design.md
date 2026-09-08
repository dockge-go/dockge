# 交互精简技术方案

## 背景

现状问题（均来自对 `crateman_web_index (3).html` 的实测）：

1. 容器行终端按钮 `showDetail + setTimeout(switchTab 'tab-exec')` 绕路且依赖 100ms 竞态延时。
2. 栈行日志按钮调用 `openStackToService`：打开详情后 click 第一个 `.tab`（实际切到 Editor），名字叫"查看服务日志"却看不到日志。
3. 容器详情 7 个 Tab 混排：Logs/Exec（功能弹窗类）与 Info/Stats/Env/Net/Mounts（查看类）并列；Network Tab 内嵌 Connect/Disconnect 修改操作。
4. 容器行 6 键（日志/终端/启停/重启/详情箭头/删除）、栈行 4 键（日志/Edit 死按钮/启停/删除）。

## 方案

### 1. 容器行极简（renderContainers.renderRow）

- 操作键收敛为：日志（`showContainerLogs`）、终端（新 `showContainerTerminal`）、启动/停止（状态取反，保留原 doAction）。
- 删除 `detailBtn`（详情箭头）与行内 重启/删除/`actBtns` 中多余按钮——重启/删除已在详情 footer。
- 系统容器行维持仅系统徽标。

### 2. 终端/日志独立弹框

- 抽出 `terminalHtml(cid)` 生成终端标记（欢迎行、提示行、输入行），`showDetail` 的 Exec Tab 与新 `showContainerTerminal(cid)` 共用；弹框标题 `sheet.terminalTitle`，footer 仅 Close。
- `handleTermInput` 原样复用（按容器 id 定位终端节点）。
- 详情 Tab 集合改为 Info/Stats/Env/Network/Mounts，删除 Logs/Exec Tab。

### 3. Network Tab 只读

- 仅渲染已连接网络列表（名称 + IP）。
- 删除 `connectNetwork`/`disconnectNetwork` 函数、window 暴露、`net-connect-select` 下拉与相关字典键（ctn.connectNet/ctn.connect/ctn.disconnect/toast.selectNet/toast.netConnected/confirm.disconnectNet/toast.netDisconnected，zh+en 各 7 键）。

### 4. 栈日志聚合

- 服务日志数据提升为常量 `SERVICE_LOGS`（web/api/db），`showServiceLogs` 改读它。
- 新 `showStackLogs(id)`：按服务顺序拼接全部日志（每段前缀 `[web]` 等已内含），单一只读 log-viewer 弹框，标题 `sheet.logsTitle {name: 栈名}`。
- 栈行日志按钮改绑 `showStackLogs`；删除 `openStackToService` 及其 window 暴露。

### 5. 栈详情首屏服务清单

- `showStackDetail` 中 Tab 顺序调整为 Services（默认 active）在前、Editor 在后；对应 `stack-tab-services` 默认显示、`stack-tab-editor` 默认 none。
- 服务行保留单服务日志/终端钻取按钮（showServiceLogs/showServiceExec）。
- 栈行删除 Edit 死按钮（无 onclick 的装饰按钮），行点击进详情即达 Editor。

### 6. 字典维护

- 删 7 键（见上）；无新增键需求（终端/日志弹框标题复用 sheet.terminalTitle/sheet.logsTitle）。
- 保持 zh/en 键集合一致（296 → 289）。

## 取舍

- 详情顶部"复制到宿主机/导出/提交为镜像/重命名"四键保留：它们是显式容器操作入口（非查看项混排），且无更合适的落点；不属本次问题域。
- 不做行内"更多 ⋯"下拉菜单：单文件原型引入菜单组件增加复杂度，与最小心智开销目标相悖。
