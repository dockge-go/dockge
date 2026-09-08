## 1. 基础设施

- [x] 1.1 分页组件：PAGE_SIZE=15、state.pagination、paginate()/paginationHtml()、筛选变化重置页码（验证：注入 21 条容器实测 第1页15/第2页6、"第 2/2 页 · 共 21 条"、越界收敛、单页隐藏控件）
- [x] 1.2 dialogInput 输入对话框（iOS 风格，消息+输入框+取消/创建，Enter 提交，创建态确认按钮蓝色）（验证：新建栈输入名称生效）
- [x] 1.3 LiveFeed 模拟长连接：删除 4s 随机轮询与自发 toast（含 nowRunning 等 3 键），6s 低频确定性事件（health/restart/die）（验证：20 秒实测事件 8→10、toast 容器全程 0 条、事件类型确定）

## 2. 列表分页接入

- [x] 2.1 containers/images/volumes/networks/events 五视图接入分页（验证：镜像 8 条无控件、容器 21 条两页）
- [x] 2.2 toggleAll 筛选切换重置页码（验证：实现含 pagination.containers=1）

## 3. 容器详情推入页

- [x] 3.1 renderContainerDetail：面包屑+操作区+概要四格+Tabs（统计/环境变量/网络/挂载只读）；showDetail 退役、搜索/列表引用迁移 openContainerDetail（验证：点行进详情、topbar 显容器名、面包屑返回、侧栏高亮容器项；实测中发现并修复 stackServices 把 volumes 段 pg_data 误解析为服务）
- [x] 3.2 详情页随 LiveFeed 事件更新状态（验证：LiveFeed tick 调 renderCurrentView 覆盖详情路由）

## 4. 栈工作台

- [x] 4.1 双栏布局 + 左栏栈列表（选中高亮、New Stack 入口）（验证：active 高亮 monitoring、空态提示）
- [x] 4.2 右栏工作台：顶栏（名称/状态/日志/启停/删除/部署）+ 服务 pills（从 compose 解析、弹日志含终端入口）+ 编辑器 + 部署终端（验证：web-app pills web/api/db、终端提示、操作按钮四枚）
- [x] 4.3 New Stack：dialogInput 创建模板栈并直接进入工作台；Deploy 逐行输出部署日志（存回 _deployLog 重渲保留）并更新状态（验证：my-new-app 创建→选中→编辑器模板→deploy→down→up、徽标运行中、终端 3 行输出）
- [x] 4.4 showStackDetail/showNewStackSheet Sheet 退役、栈视图移动端折叠（验证：grep 无死引用；CSS 900px 断点）

## 5. 验证与归档

- [x] 5.1 字典增删维护（265=265 无重复无死键）+ 语法 + 死引用扫描（showDetail/showStackDetail/EVENT_POOLS/nowRunning 均无残留）
- [x] 5.2 浏览器实测全链路 + 中英双语（英文 New Stack/pills/View Service Logs/Deploy/Crumb Containers 均正确）+ 暗色截图经视觉模型检查（工作台双栏比例合理、详情页布局清晰，均符合 Apple 简洁风格）
- [x] 5.3 openspec sync-specs → archive
