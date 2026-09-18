# AGENTS.md — AI 代理工作指引

Dockge：louislam/dockge 的 Go 复刻（单机 Compose 栈管理面板）。Gin + bbolt + samber/do 后端，SolidJS + Vite 前端，go:embed 单二进制。

## 文档地图（动手前先读）

| 文档 | 用途 |
| --- | --- |
| `docs/PROJECT_SPEC.md` | **规格总入口**：架构基线、能力清单与测试映射、Non-Goals、债务清单（D1–D12） |
| `docs/REQUIREMENTS.md` | 需求事实来源：R1–R4、问题清单 P1–P15、整改清单 F1–F12、决策记录 Q1–Q6 |
| `docs/VISION.md` | 愿景导航：目标图景与差距地图（引用 PROJECT_SPEC/REQUIREMENTS，不复制数字）；UI 原型见 `docs/prototype/vision.html` |
| `docs/AUTH-DEPLOY.md` | 认证部署操作指南：OIDC 直连（Authentik/Keycloak）与 Traefik forwardAuth（Authelia）两套落地步骤 |
| `DESIGN.md` | 视觉与交互契约（Crate 设计系统）；新增颜色 MUST 先入 token 表 |
| `docs/UI-ARCHITECTURE.md` | UI 与实时数据流纲要：4 条长连接清单、数据更新保证矩阵、页面地图、设计决策 |
| `openspec/` | 规格流水线：`specs/` 为当前有效能力规格，`changes/` 为增量变更（新功能走 openspec 变更，勿直改 `specs/`） |

## 硬约束

- **平台**：构建与 `go test ./...` 仅在 **Linux** 通过（`pkg/pty` 使用 Linux 专有 ioctl）；macOS 上 `go build ./app/dockge/cmd/server` 会失败。
- **分层单向**：`handler → service → repository`；接口定义在消费侧（最小接口模式）；DI 组合根只在 `cmd/`。
- **Non-Goals**（`PROJECT_SPEC` §4）：不做多主机/Agent、不做宿主 shell 终端、不引 UI 框架（CodeMirror 6 已于 2026-09-12 解禁引入，仅限栈编辑器）。
- **错误**：哨兵错误集中在 `api/v1` 与 repository 层，`%w` 包装；handler 统一 `handleServiceError`。
- **注释/语言**：导出符号写中文文档注释（「为什么」而非「是什么」）；文档与 openspec 产物一律 zh-CN，SHALL/MUST 保留英文。
- **测试**：表驱动、就近放置；新能力先写失败测试（Red-Green-Refactor）；测试辅助跨包共享放 `internal/testutil`。
- **前端**：不允许 `any` / `@ts-ignore` / `@ts-expect-error` / `@ts-nocheck`；i18n 文案必须进 `web/src/i18n/{zh-CN,en-US}.ts`（类型强制对齐）。

## 常用命令

```bash
make migrate      # 初始化 bbolt（破坏性重建，种子 admin/123456）+ stacks 目录
make web-build    # 前端 tsc + vite 构建
make run          # 启动（127.0.0.1:5001）
make build        # 单二进制 bin/dockge-server（git describe 注入版本）
make test         # go test ./... + 前端测试（须在 Linux 跑，pkg/pty 平台限制见 PROJECT_SPEC D10）
make verify       # build + test
```

## 文档同步义务

代码与文档不允许长期分叉：改代码时同步更新 `PROJECT_SPEC.md` §3 状态列与 §8 债务表；openspec 变更归档时同步 §6 变更史。发现偏差先记入 `PROJECT_SPEC` §2.4 或债务表，再决定改哪边。
