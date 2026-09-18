# UI 与实时数据流纲要

> 回答三个问题：**页面上有什么**（§4 页面地图）、**数据怎么来**（§2 长连接清单 + §3 更新保证矩阵）、**为什么这样设计**（§5 决策记录）。视觉与组件契约见 [DESIGN.md](../DESIGN.md)，本文是它的上层架构纲要。

## 1. 一图看懂数据流

```
                      ┌───────────── REST 快照（登录/操作后拉全量）────────────┐
                      │  info · stacks · containers · images · networks · volumes │
                      └───────────────────────┬──────────────────────────────────┘
                                              ▼
后端数据源 ──► store snapshot（Solid 信号，全应用唯一数据快照）
  │                    ▲                ▲
  │  ┌──长连接 4 条─────┼────────────────┤
  │  │                 │                │
   │  │  ①容器状态流(SSE,全局)  字段级 merge 进 snapshot.containers + 全量镜像列表进 snapshot.images
  │  │  ②主机资源(SSE,仪表盘页) ──► Dashboard 组件局部信号
  │  │  ③容器日志(SSE,详情tab) ────► LogStream 组件局部
  │  └─ ④终端(WS,终端面板)   ──────► TerminalPane 组件局部
  │
  └─ docker events / procfs / docker logs -f / PTY
```

**核心原则：一份快照，两种供给。** 列表/计数类数据活在 `store.snapshot`（唯一），由 REST 建底、由长连接保鲜；页面局部数据（日志/终端/资源曲线）随组件生命周期建立与销毁，不进全局快照。

## 2. 长连接清单（全部就这 4 条）

| # | 连接 | 端点 / 协议 | 生命周期（谁建谁断） | 推送内容与频率 | 降级行为 |
|---|---|---|---|---|---|
| ① | **容器状态流** | `GET /v1/docker/containers/stream`（SSE） | **全局**：登录/开机自检成功即建（`store.startContainerStatusStream`），登出即断 | 全量容器的 `id/state/status` 帧 + 全量镜像列表（`sizeBytes`/`createdAt`）：`docker events` 事件驱动（500ms 防抖）+ 30s 兜底采样 + 15s 心跳；新连接立即回放末帧；后端任一资源（容器/栈/镜像）采集失败则整帧不推 | 断线由 EventSource 自动重连；token 失效 → 401 → 跳登录 |
| ② | **主机资源流** | `GET /v1/docker/stats/stream`（SSE） | **仪表盘页内**：`Dashboard` 挂载建、卸载断 | 系统 CPU/内存帧，固定 2s | 数据源不可用（非 Linux 无 `/proc`）→ 每帧推 `stats_unavailable` 错误码（兼作心跳），UI 显示「实时统计不可用」 |
| ③ | **容器日志流** | `GET /v1/docker/containers/:id/logs`（SSE） | **容器详情·日志 tab 内**：`LogStream` 挂载建、卸载断 | `docker logs -f --tail=N` 的日志行 | 连接断即流停，重开 tab 重建 |
| ④ | **终端** | `GET /v1/terminal/:name/:type`（**WebSocket**，非 SSE） | **终端面板内**：`TerminalPane` 挂载建、卸载断；type = 容器 exec / 栈 compose-logs | PTY 字节流双向 | 会话结束写黄色提示并保持断开态 |

> SSE/WS 无法自定义请求头 → 认证走 `?token=`（本地密码模式）或同源 cookie 自动附带（OIDC/proxy 模式），与 StrictAuth 的三来源优先级一致。

## 3. 数据更新保证矩阵（哪类数据、靠什么更新、多快）

| 你看到的数据 | 更新方式 | 预期时延 | 链路 |
|---|---|---|---|
| 容器**状态徽标**（运行中/已停止）、状态变化 toast | SSE 帧直推，字段级更新 | **< 1s** | ①帧 → merge 只改 state/status，不闪整行 |
| **运行容器计数**（侧栏 `7/9` 徽标、仪表盘统计卡/占比条） | SSE 帧到达即重算 running/total | **< 1s** | ①帧 → store 重算 `docker.containersRunning/Total` |
| **新容器出现在列表/计数** | 帧发现未知 id → 精准 REST（仅 containers+info 两请求，5s 节流） | **≤ 5s** | ①帧触发 → `refreshContainers()` 补全 name/image 等字段 |
| 主机 CPU/内存曲线 | ②SSE 固定帧 | 2s | 仅 Linux 有数据源 |
| 容器日志 / 终端输出 | ③④流式 | 实时 | 组件局部，不进快照 |
| **镜像列表与计数**（侧栏镜像徽标、镜像页表格） | SSE 帧直推全量镜像列表（`sizeBytes`/`createdAt` 结构化字段），`imagesTotal` 由帧内列表实时推导 | **< 1s**（事件驱动） | ①帧 → `applyContainerStatus` 原子写入 `snapshot.images` 与 `docker.imagesTotal`，徽标与列表同源——要删一起删、要留一起留；后端任一资源采集失败则整帧不推，前端保持旧值 |
| 栈/网络/卷的**列表** | REST 快照：进入页面读 snapshot，**增删操作成功后自动重拉** | 操作后即时 | 无对应事件流 |
| 顶栏/仪表盘「刷新」按钮 | 全量 REST 对账 | 手动 | **兜底手段**：怀疑 SSE 漏帧或数据不一致时对账用——不承担日常更新职责 |

**结论：容器状态与计数是长连接驱动的，不需要手动刷新。** 手动刷新只对账；镜像/卷/网络这类无事件流的资源，更新点在「你刚做完操作」的时刻（操作成功即自动重拉）。

## 4. 页面地图（11 屏三类形态）

| 形态 | 页面 | 布局 | 实时性 |
|---|---|---|---|
| 总览 | 仪表盘 | 统计卡×5 → 状态占比+资源曲线 → 最近容器 5 条 | ①② |
| 主从工作区 | 容器、栈 | 左列表 320-380px（行内启停/删除）+ 右详情/工作台 | ①（列表状态即时变） |
| 纯表格 | 镜像、卷、网络 | 标题+操作 → 表格 → 15/页客户端分页；轻操作走 Sheet 抽屉 | 镜像列表：①SSE（镜像事件驱动）；卷/网络：REST（操作后刷新） |
| 系统 | 系统信息、磁盘占用、设置 | 卡片式 | REST |

栈工作台（编辑发生地）：文件栏 / CodeMirror 编辑器（深色岛）/ 部署摘要三栏 + 固定操作栏；失败不关工作区、输出贴编辑器。

## 5. 设计决策记录（为什么不是「全都推」）

1. **容器状态帧只带 `id/state/status` 三个字段**：字段级更新不闪行（DESIGN.md §6 硬规矩）、帧体最小；代价是新容器缺完整字段 → 用「帧发现未知 id → 节流精准 REST」闭环，兼得两者。镜像列表不同——它直接随帧全量推送（`sizeBytes`/`createdAt` 结构化），因为镜像的 size/created 是展示必需字段且单机列表量级可控（几十条以内），事件驱动 + 30s 兜底频率下帧体可接受。
2. **容器状态流全局唯一**：多页面共享同一份 snapshot（Solid 信号天然细粒度更新），避免每页各开一条连接重复推送。
3. **资源/日志/终端随组件生命周期**：离开页面即断流，连接数与页面强相关，不浪费后端采样。
4. **计数可由帧推导，不单独推送**：total/running 是 snapshot.containers 的派生值，帧到达即重算，避免第二套计数真源。镜像计数同理——`imagesTotal` 由帧内镜像列表实时推导，与列表同源（要删一起删、要留一起留）。
5. **手动刷新保留为对账手段**：SSE 的最终一致（防抖/兜底帧间隔）意味着秒级窗口内可能有极小偏差，刷新按钮是「强制对账」逃生门，日常用不到。
6. **认证三来源**（Bearer → `?token=` → cookie）：让同一条长连接在 jwt/oidc/proxy 三种登录模式下无差别工作。

## 6. 相关文档

- 视觉 token / 组件契约：[DESIGN.md](../DESIGN.md)
- 认证闭环与部署：[AUTH-DEPLOY.md](AUTH-DEPLOY.md)、[prototype/auth-flow.html](prototype/auth-flow.html)
- 愿景与差距：[VISION.md](VISION.md)
