## 1. 组件与样式

- [x] 1.1 按压反馈：`.btn-icon/.lang-btn :active` 缩放 + 表格行 `:active` 高亮（color-mix 自适应明暗）（验证：样式表断言 .btn-icon:active 与 tr:active 规则存在）
- [x] 1.2 iOS 确认对话框组件：dialog-overlay/dialog-alert CSS + `confirmDialog(msg, onConfirm)`，Escape/遮罩关闭，字典加 `common.ok`（验证：实测 18px 圆角、blur(10px) 毛玻璃、确认 iOS 红 rgb(255,69,58)、取消/确认分支行为正确）
- [x] 1.3 iOS 开关组件替换 Show all checkbox（验证：实测关闭灰轨 rgb(210,210,215)、开启绿轨 rgb(22,163,74)+圆钮 translateX(16px)、切换后 4 行↔7 行；实测中发现并修复两处缺陷：all 分支 label 未替换、伪元素选择器误写 input:checked::before 改为 .switch:has(input:checked)）

## 2. 调用点替换与过渡

- [x] 2.1 8 处 confirm() 替换：doRemove/clearStoppedContainers/removeStack/clearUnusedImages/clearUnusedVolumes/clearUnusedNetworks/sysdfPruneAll/sysdfPrune（验证：grep 原生 confirm 残留 0 处）
- [x] 2.2 视图切换淡入：navigateTo 加 nav-anim、renderCurrentView 移除（验证：导航 animationName=view-in，轮询/操作刷新 animationName=none）

## 3. 验证与归档

- [x] 3.1 语法（3 块通过）/字典奇偶（282=282）/死引用扫描（switch-row 全分支、无原生 confirm）
- [x] 3.2 浏览器实测：按压样式就位、删除容器/栈走新对话框（确认执行+取消不动两分支）、开关切换、视图过渡、轮询无闪烁、英文态（Logs/Exec/Stop、Cancel/OK）与中文态（取消/确认）双语验证；暗色对话框截图经视觉模型检查通过
- [x] 3.3 openspec sync-specs → archive
