## 1. 期一：骨架（布局/路由/主题/认证页）

- [x] 1.1 清空原型：删除旧 views/components/widgets/styles、`docs/prototype/`；保留 lib/i18n/Terminal/StackEditor
- [x] 1.2 视觉 token 重写：上游主色/渐变/pill/shadow-box/GitHub 暗色 token 表 + 基础工具类（手写 CSS，无 Bootstrap）
- [x] 1.3 Layout：顶栏（logo + 标题 + Home + 头像菜单[Scan Stacks Folder/Settings/Logout]）+ 移动端适配
- [x] 1.4 路由：`/`、`/compose`、`/compose/:name`、`/terminal/:stack/:service/:type`、`/settings/:tab`、`/setup`；Login 条件渲染
- [x] 1.5 Login/Setup 页复刻（floating labels、Remember me、错误 alert、语言下拉）
- [x] 1.6 构建 + 启动验证

## 2. 期二：栈核心（首页/列表/详情/编辑器/终端）

- [x] 2.1 首页：统计卡片（active/exited/inactive）+ Docker Run 转换器 + 栈列表侧栏（搜索/状态 pill/外部栈半透明）
- [x] 2.2 栈详情骨架：标题行（状态 pill + 操作按钮组双态）+ 双栏布局
- [x] 2.3 右栏编辑器：compose CM6（Dracula 风）+ 编辑态 .env 编辑器 + Networks 卡片；校验闭环迁移（防抖/诊断/四态 pill）
- [ ] 2.4 左栏容器卡片：服务名/镜像/状态/端口徽章/终端入口/单服务启停/CPU 内存统计；编辑态容器增删与配置编辑（写回 YAML 保留注释）
- [ ] 2.5 浏览态合并日志终端 + 操作进度终端（deploy/update/down 输出）
- [x] 2.6 容器终端页 `/terminal/:stack/:service/:type`（复用 xterm WS）
- [ ] 2.7 后端补缺：`POST /stacks/:name/services/:service/:op` + `GET /stacks/:name/stats`（+ 测试）
- [ ] 2.8 离开确认（未保存修改）+ Discard + x-dockge.urls 徽章

## 3. 期三：设置与打磨

- [x] 3.1 设置五子页：General/Appearance（主题三态+语言）/Security（改密+Disable Auth）/Global Env/About
- [x] 3.2 i18n 文案对齐上游 zh-CN/en locale 结构
- [x] 3.3 顶栏 New Update 徽章（version check 已有端点）
- [ ] 3.4 DESIGN.md 重写为上游复刻视觉契约；PROJECT_SPEC §3/§4/§6、REQUIREMENTS §6、UI-ARCHITECTURE 同步

## 4. 期四：验收

- [ ] 4.1 全页浏览器走查（明暗 × zh/en）+ 上游 README 截图人工比对
- [ ] 4.2 `make verify`（build + test 全绿）+ 启动冒烟
- [ ] 4.3 openspec 归档准备（verify-change）
