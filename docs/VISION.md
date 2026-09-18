# Dockge 愿景：项目想要的样子

> 本文是「目标状态」的人读导航：把散落在 PROJECT_SPEC §7、REQUIREMENTS F/P 项、DESIGN.md 债务表中的目标收敛成一幅完整图景。事实数字与验收场景一律以 [PROJECT_SPEC.md](PROJECT_SPEC.md) / [REQUIREMENTS.md](REQUIREMENTS.md) 为权威来源，本文不复制、只引用。配套 UI 原型见 [prototype/vision.html](prototype/vision.html)。

## 一句话愿景

一间**安静、精确、可信赖的单机容器控制室**：一台主机、一个二进制、一块面板——栈的编辑、部署、观测与清理全部在浏览器里完成，Unix 工具向的数据密度配 Apple 式的克制交互；安全默认收敛，测试管线强制，任何人 fork 后 `make verify` 全绿即可放心改。

## 主题图景：现在 → 想要的样子

### 1. 产品能力：从「单人工具」到「小型团队面板」

- **现在**：✅ 已达成（2026-09-12 F11 落地）——设置页用户管理卡（创建/停用/角色切换，来源可见）；停用账号 token 立即 401（CheckSession 接线，测试锁定）。（2FA/TOTP 已按决策 Q7 整体移除：功能冗余。）

### 2. 编辑体验：从「带行号的 textarea」到「真正的 YAML 工作台」

- **现在**：✅ 已达成（2026-09-12）——CodeMirror 6：YAML 语法高亮、行号、当前行、undo、Tab 缩进；docker run 转换结果直接落入编辑器；失败不关工作区、草稿保留（既有）。

### 3. 安全：从「默认安全但留有收敛空间」到「无可挑剔」

- **现在**：CORS 反射任意 Origin（P3/D2）、WebSocket CheckOrigin 全放行（P4）、`/v1/auto-login` 挂在公开组（P2）、迁移种子 `admin/123456`（D3）。
- **想要的样子**：CORS 白名单 + WS Origin 校验；auto-login 端点下线；首启向导强制改密，弱口令只存在于向导流程内。
- **差距**：见 PROJECT_SPEC §7.3 P1/P2 与 REQUIREMENTS §2.3 P2-P4。

### 4. 工程质量：从「只有底座的测试金字塔」到「机器强制的验收管线」

- **现在**：单元层最厚（composerize/stack/auth 逻辑），handler/service 集成层 0 测试（D5），无 CI、无 lint、无覆盖率（D6）。
- **想要的样子**：GitHub Actions（Linux）跑 golangci-lint + tsc + 前端测试 + `go test ./... -race -cover` + `make build`，任何一项红即拒绝合并；Go 覆盖率 ≥60%，核心包 ≥80%；一条 E2E 冒烟（登录→建栈→起→停→删）。
- **差距**：见 PROJECT_SPEC §7.1 质量门与 §7.4 测试形态。

### 5. 交付：从「能跑」到「能发布」

- **现在**：无 Dockerfile、无发布流程、无 git tag；`web/dist` 嵌入时序有坑（D1：裸 `go build` 嵌坏壳）。
- **想要的样子**：多阶段 Dockerfile（pnpm web-build → go build → distroless）；语义化版本 tag + Release 附产物；版本检查端点对比**本项目** releases 而非上游。
- **差距**：见 PROJECT_SPEC §7.2。

### 6. 体验与卫生：最后一公里

- 列表 `j/k` 键盘导航（U-4 SHOULD 级）；零散硬编码文案清零（Settings 标题等，REQUIREMENTS §6.3）；4 个超 250 纯 LOC 的文件按职责拆分（D14）；`GetLatestVersion` 迁到 version 域并去掉恒 nil 的 error（D15）；prune/拉取的流式进度与回收统计（F9，体验级）。

## 优先级路线图

| 波次 | 内容 | 关闭的项 |
|---|---|---|
| **P0（先做）** | ~~handler/service 集成测试骨架~~ → 起步：repo 层用户操作 + middleware CheckSession 接线已有测试 | D5 的第一块（部分） |
| **P0** | ~~多用户管理 + CheckSession 接线~~ | ✅ F11、D12（2026-09-12 完成）；F4 已随 Q7 移除 |
| **P1** | CORS 白名单 + WS Origin 校验 + auto-login 下线 | P2、P3、P4、D2 |
| **P1** | ~~CodeMirror 6 编辑器~~ | ✅ DESIGN.md 债务（2026-09-12 完成） |
| **P1** | CI 管线上线（lint + test + build） | D6 |
| **P2** | Dockerfile + 发布流程 + 嵌入时序修复 | D1、D7 |
| **P2** | 种子密码强制首启修改；prune 流式进度 | D3、F9 |
| **持续** | j/k 键盘导航已落地（五视图）；剩余：文件拆分等卫生项 | D14、D15（U-4 ✅） |

## 「是我想要的样子」的完成判据

1. `make verify` 在 Linux CI 全绿且覆盖率 ≥60%；
2. ✅ 一个非 admin 的 member 账号：能登录、只能看不能管理、被停用后 token 立即 401（2026-09-12 达成）；
4. ✅ 栈编辑器有语法高亮，保存失败草稿不丢（2026-09-12 达成）；
5. 非白名单 Origin 收不到凭证型 CORS 头；
6. `docker run` 一条命令跑起发布产物，版本号来自 git tag；
7. 全库无超 250 纯 LOC 的源文件（D14 清零）。

## 原型索引

- [docs/prototype/auth-flow.html](prototype/auth-flow.html) —— **认证闭环交互原型**（可逐步演练）：OIDC 直连、Traefik 反代（forwardAuth + Authelia）、本地密码三模式的完整状态机，每步含真实 HTTP 报文与前端行为；配套部署操作指南见 [AUTH-DEPLOY.md](AUTH-DEPLOY.md)。
- [docs/prototype/vision.html](prototype/vision.html) —— 目标形态 UI 原型（单文件、按 DESIGN.md token、明暗双主题）：含「设置页·用户管理」「栈工作台·CodeMirror 编辑器」目标屏与已实现的仪表盘对照。（原型中 2FA 相关节点已随决策 Q7 废弃。）
