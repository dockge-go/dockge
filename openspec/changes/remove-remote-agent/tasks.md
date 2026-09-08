## 1. 规格与安全边界

- [x] 1.1 新增 `single-host-boundary` 规格，定义单机管理范围、远程浏览器访问边界和禁止的跨主机代理行为。

## 2. 后端删除

- [x] 2.1 新增迁移测试，证明新数据库初始化后只包含 `users` 与 `settings` bucket，不创建 `agents` bucket。
- [x] 2.2 删除 Agent HTTP 路由、Handler 与 DI provider。
- [x] 2.3 删除 Agent 服务、远程 HTTP 客户端、API DTO、领域模型和 bbolt Agent CRUD。
- [x] 2.4 从数据库初始化与迁移初始化中移除 `agents` bucket；保持旧数据库可打开且不访问遗留 bucket。

## 3. 文档与验证

- [x] 3.1 更新 README 的产品定位、功能对照、API 概览和安全提示，删除远程 Agent 声明。
- [x] 3.2 运行 gofmt、go test ./...、go vet ./... 与 openspec validate，确认无 Agent 后端符号或路由残留；Go 全量命令因当前 Windows 环境无法编译既有 Linux 专用 PTY/Podman 依赖而受阻。
