export default {
  app: { title: 'Dockge — 容器管理', lang: 'zh-CN' },
  nav: { overview: '概览', resources: '资源', system: '系统' },
  pages: {
    dashboard: '仪表盘', containers: '容器', stacks: '栈', images: '镜像',
    volumes: '数据卷', networks: '网络',
    sysinfo: '系统信息', sysdf: '磁盘用量', settings: '设置',
  },
  actions: {
    login: '登录', logout: '退出登录', loading: '加载中…',
    newContainer: '新建容器', newStack: '新建栈', refresh: '刷新',
    start: '启动', stop: '停止', restart: '重启', remove: '删除',
    clearStopped: '清空已停止', clearUnusedImages: '清空未使用镜像',
    clearUnusedVolumes: '清空未使用卷', clearUnusedNetworks: '清空未使用网络',
    pruneAll: '全部清理', scan: '扫描',
  },
  status: {
    running: '运行中', exited: '已停止', paused: '已暂停', dead: '已死亡',
    unknown: '未知', deployed: '已部署', created: '已创建',
  },
  forms: {
    username: '用户名', password: '密码', confirmPassword: '确认密码',
    label: '标签', placeholder: { search: '搜索…' },
  },
  feedback: {
    success: '成功', error: '错误', info: '提示',
    dataRefreshed: '数据已刷新', containerRemoved: '容器已删除',
    stackRemoved: '栈已删除', imageRemoved: '镜像已删除',
    volumeRemoved: '数据卷已删除', networkRemoved: '网络已删除',
    loginFailed: '登录失败，请检查用户名和密码',
    setupSuccess: '管理员账号已创建',
  },
};
