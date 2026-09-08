## 1. 容器列表与详情

- [x] 1.1 行操作极简：renderRow 操作区仅留 日志/终端/启动-停止 三键，删除详情箭头与行内重启/删除（验证：浏览器实测非系统行操作键恰 3 个：日志/终端/停止；系统行 0 按钮仅徽标；重启/删除仍在详情 footer）
- [x] 1.2 新增 `showContainerTerminal(id)` 独立终端弹框，抽出 `terminalHtml(cid)` 供详情与弹框共用，行终端按钮直绑（验证：实测点终端直接出弹框"nginx-web — 终端"，输入框聚焦，footer 仅关闭）
- [x] 1.3 详情 Tab 精简为 Info/Stats/Env/Network/Mounts，删除 Logs/Exec Tab（验证：实测 Tab 恰 5 个）
- [x] 1.4 Network Tab 只读化：仅展示名称+IP，删除 Connect/Disconnect、connectNetwork/disconnectNetwork 及暴露与字典键（验证：实测 #tab-net 0 个交互控件；连带清理日志工具栏死代码 toggleLogLock/ctn.stream 等 5 键与 common.details/common.edit/img.tagTitle 3 死键）

## 2. 编排栈

- [x] 2.1 服务日志数据提升为 SERVICE_LOGS，新增 `showStackLogs(id)` 聚合弹框，栈行日志按钮改绑并删除 openStackToService（验证：实测"web-app — 日志"弹框含 [web]/[api]/[db] 三段聚合）
- [x] 2.2 栈详情默认 Tab 改为 Services（服务清单首屏可见），Editor 退居第二（验证：实测打开详情默认激活"服务 (3)"，Editor 隐藏）
- [x] 2.3 栈行删除 Edit 死按钮，操作区仅 日志/启停/删除（验证：实测恰 3 键：查看服务日志/停止/删除；down 状态栈也有日志按钮）

## 3. 验证与归档

- [x] 3.1 Node 语法检查（3 块通过）+ 字典键奇偶校验（281=281）+ grep 无死引用（openStackToService/connectNetwork/disconnectNetwork/toggleLogLock/tab-exec/net-connect-select 均无残留）
- [x] 3.2 浏览器实测：容器行三键直达（日志框/终端框/启停）、详情 5 Tab 纯查看、栈日志聚合弹框、栈详情首屏服务清单，中英双语各过一遍（英文态行按钮 Logs/Exec/Stop 与 View Service Logs/Stop/Delete，弹框标题英文，零中文残留；暗色中文终端弹框截图经视觉模型检查通过）
- [x] 3.3 openspec sync-specs → archive
