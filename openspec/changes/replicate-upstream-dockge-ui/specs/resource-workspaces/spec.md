## REMOVED Requirements

### Requirement: 主从资源工作区

**Reason**: 一比一复刻上游 louislam/dockge 页面集，上游无容器/镜像/网络/数据卷/系统信息/磁盘占用/用户管理页面；相关前端视图与导航移除（后端 API 保留，可用于未来恢复）。
**Migration**: 资源管理经由 docker CLI 完成；如需恢复资源页面，另立 openspec 变更。
