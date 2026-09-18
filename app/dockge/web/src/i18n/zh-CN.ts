// zh-CN 词典（基准语言）：上游 dockge 复刻版的扁平 key。
// 措辞对齐 louislam/dockge 的 zh-CN locale 风格（部署/保存草稿/放弃/合并日志等）。
const dict = {
  "app.title": "Dockge",

  // ---- 通用 ----
  "common.ok": "确定",
  "common.cancel": "取消",
  "common.save": "保存",
  "common.close": "关闭",
  "common.loading": "加载中…",
  "common.error": "出错了",
  "common.language": "语言",
  "common.theme": "主题",
  "theme.light": "明亮",
  "theme.dark": "暗黑",
  "theme.auto": "自动",

  // ---- 顶栏 ----
  "nav.home": "首页",
  "nav.settings": "设置",
  "nav.logout": "退出登录",
  "nav.scanStacks": "扫描 Stacks 目录",

  // ---- 登录 / 初始化 ----
  "login.title": "登录到 Dockge",
  "login.username": "用户名",
  "login.password": "密码",
  "login.remember": "记住我",
  "login.submit": "登录",
  "setup.title": "创建管理员账号",
  "setup.username": "用户名",
  "setup.password": "密码",
  "setup.repeat": "重复密码",
  "setup.submit": "创建",
  "setup.welcome": "Dockge",

  // ---- 首页 ----
  "home.active": "运行中",
  "home.exited": "已退出",
  "home.inactive": "未部署",
  "home.dockerRunTitle": "Docker Run",
  "home.dockerRunDesc": "把 docker run 命令转换为 compose.yaml",
  "home.dockerRunPlaceholder": "docker run -d --name nginx -p 8080:80 nginx",
  "home.convert": "转换为 Compose",
  "home.convertSuccess": "已转换，请在编辑器中确认",

  // ---- 栈列表侧栏 ----
  "stacklist.searchPlaceholder": "搜索…",
  "stacklist.empty": "还没有 Stack",
  "stacklist.addFirst": "点击 + Compose 创建第一个 Stack",

  // ---- 栈详情 ----
  "compose.newTitle": "新建 Stack",
  "compose.name": "Stack 名称",
  "compose.namePlaceholder": "my-app",
  "compose.nameHelp": "仅小写字母、数字、下划线与连字符",
  "compose.deploy": "🚀 部署",
  "compose.saveDraft": "💾 保存草稿",
  "compose.discard": "放弃",
  "compose.edit": "编辑",
  "compose.start": "▶ 启动",
  "compose.restart": "↻ 重启",
  "compose.update": "⬇ 更新",
  "compose.stop": "⏹ 停止",
  "compose.down": "Down（停止并移除）",
  "compose.delete": "删除",
  "compose.confirmDelete": "确认删除",
  "compose.confirmDeleteDesc": "将执行 compose down 并删除 {name} 的全部文件，不可恢复。",
  "compose.containers": "容器",
  "compose.addContainer": "添加容器",
  "compose.terminal": "终端（合并日志）",
  "compose.progress": "操作输出",
  "compose.leaveConfirm": "有未保存的修改，确定离开？",
  "compose.discardConfirm": "放弃全部未保存的修改？",
  "compose.networks": "网络",
  "compose.externalStack": "外部 Stack（只读）",
  "compose.noContainers": "尚未部署任何容器",

  // ---- 容器卡片 ----
  "container.terminal": "终端",
  "container.image": "镜像",
  "container.ports": "端口",
  "container.cpu": "CPU",
  "container.mem": "内存",

  // ---- 终端 ----
  "term.pageTitle": "容器终端",
  "term.sessionEnded": "会话已结束",

  // ---- 编辑器（校验闭环沿用） ----
  "editor.checking": "校验中…",
  "editor.validCompose": "compose 有效",
  "editor.invalidCompose": "{n} 个问题",
  "editor.exitFullscreen": "退出全屏",
  "editor.fullscreen": "全屏",
  "editor.lines": "{n} 行",
  "editor.readonly": "只读",

  // ---- 设置 ----
  "settings.general": "通用",
  "settings.appearance": "外观",
  "settings.security": "安全",
  "settings.globalEnv": "环境变量",
  "settings.about": "关于",
  "settings.stackDir": "Stacks 目录",
  "settings.theme": "主题",
  "settings.language": "语言",
  "settings.saved": "已保存",
  "settings.changePassword": "修改密码",
  "settings.currentPassword": "当前密码",
  "settings.newPassword": "新密码",
  "settings.repeatPassword": "重复新密码",
  "settings.disableAuth": "关闭认证",
  "settings.enableAuth": "开启认证",
  "settings.disableAuthDesc": "关闭后任何人均可访问本面板，需输入当前密码确认。",
  "settings.globalEnvDesc": "写入 .env 的全局变量，对所有 Stack 的变量插值生效。",
  "settings.version": "版本",
  "settings.upstream": "上游项目",
  "settings.newUpdate": "有新版本",

  // ---- toast ----
  "toast.refreshed": "已刷新",
  "toast.deploying": "正在部署 {name}…",
  "toast.deployDone": "{name} 部署完成",
  "toast.saved": "已保存",
  "toast.draftSaved": "草稿已保存",
  "toast.deleted": "已删除 {name}",
  "toast.sseRunning": "开始运行",
  "toast.sseExited": "已退出",
  "toast.ssePaused": "已暂停",
  "toast.scanDone": "扫描完成",
};

export default dict;
