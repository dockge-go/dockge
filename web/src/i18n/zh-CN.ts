// zh-CN 词典（基准语言）：上游 dockge 复刻版的扁平 key。
// 措辞逐条对齐 louislam/dockge 的 zh-CN locale（部署/保存/放弃/停止并置于非活动状态等）。
const dict = {
  "app.title": "Dockge",

  // ---- 通用 ----
  "common.ok": "确定",
  "common.cancel": "取消",
  "common.save": "保存",
  "common.close": "关闭",
  "common.loading": "加载中…",
  "common.language": "语言",
  "theme.light": "明亮",
  "theme.dark": "暗黑",
  "theme.auto": "自动",

  // ---- 顶栏 ----
  "nav.home": "主页",
  "nav.settings": "设置",
  "nav.logout": "登出",
  "nav.scanStacks": "扫描堆栈文件夹",
  "nav.signedInAs": "当前用户： {name}",

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
  "setup.passwordsNoMatch": "两次输入的密码不一致。",
  "setup.submit": "创建",
  "setup.welcome": "Dockge",

  // ---- 首页 ----
  "home.active": "已启动",
  "home.exited": "已退出",
  "home.inactive": "未启动",
  "home.dockerRunTitle": "Docker启动",
  "home.convert": "转换为Compose格式",
  "home.convertSuccess": "已转换，请在编辑器中确认",

  // ---- 栈列表侧栏 ----
  "stacklist.searchPlaceholder": "搜索…",
  "stacklist.empty": "组合你的第一个堆栈！",

  // ---- 栈详情 ----
  "compose.newTitle": "Compose",
  "compose.name": "堆栈名称",
  "compose.namePlaceholder": "my-app",
  "compose.nameHelp": "仅小写字母",
  "compose.noServices": "请先添加至少一个容器",
  "compose.deploy": "部署",
  "compose.saveDraft": "保存",
  "compose.discard": "放弃",
  "compose.edit": "编辑",
  "compose.start": "启动",
  "compose.restart": "重启",
  "compose.update": "更新",
  "compose.stop": "停止",
  "compose.down": "停止并置于非活动状态",
  "compose.delete": "删除",
  "compose.confirmDelete": "删除",
  "compose.confirmDeleteDesc": "你确定要删除这个堆栈吗?",
  "compose.containers": "容器",
    "compose.logs": "日志",
  "compose.live": "实时",
  "compose.ended": "已断开",
  "compose.terminal": "终端",
  "compose.leaveConfirm": "你正在编辑堆栈，确定要离开吗？",
  "compose.unmanaged": "这个堆栈不由Dockge管理。",
  "compose.terminalTitle": "终端 - {service} ({stack})",
  "compose.switchShell": "切换到 {shell}",
  "compose.addContainerName": "新的容器名称...",
  "compose.addContainer": "添加容器",
  "compose.containerExists": "容器名称已存在",
  "compose.containerNameEmpty": "容器名称不能为空",
  "compose.deleteContainer": "删除容器",

  // ---- 容器卡片 ----

  // ---- 终端 ----
  "term.sessionEnded": "会话已结束",

  // ---- 服务配置表单（编辑态，上游 ArrayInput/ArraySelect 等价物） ----
  "form.image": "镜像",
  "form.ports": "端口",
  "form.volumes": "数据卷",
  "form.restartPolicy": "重启策略",
  "form.env": "环境变量",
  "form.dependsOn": "依赖",
  "form.networks": "网络",
  "form.addListItem": "添加 {name}",
  "form.selectNetwork": "选择网络...",
  "form.longSyntax": "此处不支持长语法，请使用 YAML 编辑器。",
  "stats.detail": "资源详情",
  "stats.cpu": "CPU",
  "stats.memory": "内存",
  "stats.networkIO": "网络 I/O",
  "stats.blockIO": "磁盘 I/O",

  "policy.always": "总是",
  "policy.unlessStopped": "除非停止",
  "policy.onFailure": "失败时",
  "policy.no": "从不",

  // ---- 网络卡（上游 NetworkInput 等价物） ----
  "networks.internal": "内部网络",
  "networks.addInternal": "添加内部网络",
  "networks.external": "外部网络",
  "networks.none": "无外部网络",
  "networks.namePlaceholder": "网络名称...",

  // ---- 设置 ----
  "settings.general": "常规",
  "settings.appearance": "外观",
  "settings.security": "安全",
  "settings.globalEnv": "全局 .env",
  "settings.about": "关于",
  "settings.primaryHostname": "主机名",
  "settings.primaryHostnameHelp": "未设置:沿用当前主机名",
  "settings.autoGet": "自动获取",
  "settings.theme": "主题",
  "settings.language": "语言",
  "settings.saved": "已保存",
  "settings.changePassword": "更换密码",
  "settings.currentPassword": "当前密码",
  "settings.newPassword": "新密码",
  "settings.repeatPassword": "重复以确认新密码",
  "settings.updatePassword": "更新密码",
  "settings.passwordNotMatch": "两次输入的密码不一致。",
  "settings.currentUser": "当前用户",
  "settings.advanced": "进阶",
  "settings.disableAuthMsg1": "你确定要{action}吗?",
  "settings.disableAuthWord": "禁用身份验证",
  "settings.disableAuthMsg2": "该选项设计用于某些场景，{scenarios}，如果你不清楚这个选项的作用，不要禁用验证！",
  "settings.scenarios": "例如在Dockge之上接入第三方认证，比如Cloudflare Access、Authelia或其他认证机制",
  "settings.disableAuthCarefully": "请谨慎使用该选项!",
  "settings.confirmDisable": "我已了解风险，确认禁用",
  "settings.leave": "离开",
  "settings.disableAuth": "禁用验证",
  "settings.enableAuth": "启用验证",
  "settings.globalEnvDesc": "写入 .env 的全局变量，对所有 Stack 的变量插值生效。",
  "settings.frontendVersion": "前端版本",
  "settings.versionMismatch": "前端版本与后端版本不一致！",
  "settings.version": "版本",

  // ---- 部署自检 ----
  "deploy.title": "部署自检",
  "deploy.runtimeOk": "容器运行时可用",
  "deploy.runtimeFail": "容器运行时不可用：{error}",
  "deploy.runtimeHint": "请确认已挂载 docker socket（-v /var/run/docker.sock:/var/run/docker.sock）",
  "deploy.stacksOk": "栈目录已挂载：{path}",
  "deploy.stacksFail": "栈目录未挂载宿主目录：{path}，容器重建会丢失栈",
  "deploy.stacksHint": "请挂载宿主目录（-v /opt/stacks:/opt/stacks）",
  "deploy.cli": "容器 CLI",

  // ---- toast ----
  "toast.refreshed": "已刷新",
  "toast.draftSaved": "草稿已保存",
  "toast.deleted": "已删除 {name}",
  "toast.scanDone": "扫描完成",
};

export default dict;
