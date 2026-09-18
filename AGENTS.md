# AGENTS.md — AI 代理工作指引

Dockge：[louislam/dockge](https://github.com/louislam/dockge) 的**一比一复刻**（Go 重写）。单机 Docker Compose 栈管理面板。

## 硬约束

- **技术栈**：Gin + bbolt + samber/do 后端；SolidJS + Vite 前端；go:embed 单二进制。不引入其他框架。
- **单机**：不做多主机/Agent；全部栈操作经 docker compose CLI 子进程完成。
- **编辑体验重中之重**：web 编辑 compose.yaml 的体验（编辑器、校验闭环、保存/部署反馈）是最高优先级，任何改动不得使其退化。
- **一比一基准 = 上游 master**：任何 UI/功能改动前 MUST 先查看上游对应实现（`frontend/src` 源码）再动手；完成后 MUST 与上游逐项复核（布局/交互/文案/视觉）。时时对比、持续对齐；发现偏差先修齐再继续。有意偏差仅限：无多 Agent、无 /console 宿主 shell、双语（zh/en）、不引 Bootstrap。
- **代码精简**：无冗余代码、无冗余功能。上游没有的功能不实现；复刻不再使用的代码与端点删除，不保留「以防万一」。

## 工程约定（最小集）

- 分层单向：handler → service → repository；哨兵错误集中在 `api/v1` 与 repository 层。
- 导出符号写中文注释（说「为什么」）；前端禁 `any`/`@ts-ignore`；i18n 文案进 `web/src/i18n/{zh-CN,en-US}.ts`（措辞对齐上游 locale）。
- 测试就近、表驱动；完成改动后跑 `make verify`。

## 常用命令

```bash
make web-build    # 前端 tsc + vite 构建
make run          # 启动（127.0.0.1:5001）
make build        # 单二进制 bin/dockge-server
make test         # go test + 前端测试
make verify       # build + test
```
