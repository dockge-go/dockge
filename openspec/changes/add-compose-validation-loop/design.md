## Context

CodeMirror 6 已落地（语法高亮/行号/undo），但校验仅有「`services:` 行存在性」检查（`StackEditor.tsx`）；后端 `Save` 只做 `yaml.Unmarshal` 语法解析，错误原文返回，不带结构化行号。全仓无 `docker compose config` 调用。分层单向：handler → service → repository，compose CLI 全部经 `internal/repository/cli.go` 的 `runDocker*` 执行器。

本变更新增两条硬约束：**Go 代码精简**（复用既有模式，不新增抽象层）、**交互最低心智开销**（无新增手动按钮，被动式反馈）。

## Goals / Non-Goals

**Goals:**

- 语法 + 语义错误在编辑期（防抖 2s）暴露，带行号定位（尽力而为）。
- 校验零副作用：临时目录执行，不落盘、不创建 docker 资源。
- 交互零新增操作：复用 CodeMirror lint 自带交互（gutter/悬停/面板/点击跳转）。

**Non-Goals:**

- 前端浏览器内 YAML 解析（不为击键级语法 lint 引入第二个解析路径）。
- 自动补全、查找替换、`.env` 高亮（「输入效率」维度另期）。
- 保存强阻塞、`docker compose config` 之外的深度检查。

## Decisions

**D1 错误统一走后端，前端不引入 YAML 解析器。**
`yaml.v3` 的 `SyntaxError/TypeError` 自带行列信息；compose 语义错误由 `docker compose config` stderr 提取。前端一条数据路径（防抖 → `POST /stacks/validate` → `setDiagnostics`），替代方案「前端即时语法 lint」需要引入 `yaml` npm 依赖与第二套诊断合并逻辑，违背精简约束，且 localhost 往返约 2.1s 的反馈延迟可接受。

**D2 校验端点返回数据而非错误。**
`POST /v1/stacks/validate` 恒返回 200 + `{valid, errors:[{line, message}]}`（`line=0` 表示无行号）。校验失败是正常数据，不是请求失败；仅请求本身 malformed 才 4xx。避免前端对「业务失败」做双路径解析。

**D3 repository 只做「执行」，service 做「解析」。**
`ValidateCompose(ctx, yaml, env)` 写 `os.MkdirTemp` 临时目录（compose.yaml + 非空 .env，defer RemoveAll），`runDockerIn` 以临时目录为 workdir、随机 `-p` 项目名执行 `docker compose config`，超时 15s，返回输出 + err——与 `StackOp` 同构。行号提取（`yaml.v3` 类型断言 + stderr 正则 `line (\d+)`）放 service 层纯函数，可表驱动测试。

**D4 诊断注入用 `setDiagnostics`，不自研 linter source。**
扩展链挂 `linter(() => [])`（启用 lint state）+ `lintGutter()`；远端结果经 `lib/validate.ts` 纯函数（错误 → `{from,to,message,severity}`，行号超界/为 0 时跳过行内标记仅保留面板项）转换后 dispatch。切换到 `.env` 文件时清空诊断（同一编辑器实例换 doc，compose 的偏移量对 env doc 无意义）。

**D5 触发时机由信号驱动，保存不额外触发。**
`createEffect(on([yaml, env]))` 防抖 2s + AbortController 取消在途。打开栈/composerize 落入/yaml 信号任何变化都会走同一 effect，无需在 save/open 代码里插桩。保存成功后内容未变，最近一次校验结果即为保存内容的结果。

**D6 保存语义不变。**
`Save` 保持现有校验（名称 + 非空 + 语法），不加 `config` 阻塞——草稿允许保存 WIP，错误由编辑器常驻呈现；部署失败本就有输出回显闭环。

## Risks / Trade-offs

- [compose 语义错误多数不带行号] → 降级为面板列表项呈现，不虚标位置；YAML 语法错误（用户最常犯）始终有行号。
- [每次防抖都起 docker 子进程] → 15s 超时 + AbortController 单飞；单机场景 `config` 纯本地计算（毫秒级），可接受。
- [gin 静态段 `validate` 与 `:name` 参数段并存] → gin radix tree 支持静态优先；启动即验证，若冲突改走 `/stacks-validate`。
- [`config` 会做变量插值，`${VAR}` 缺失时报错] → 语义上正确（部署时同样会缺）；env 草稿随请求同送，`.env` 编辑即可消错。
- [外部栈（只读）也触发校验] → 有信息价值（解释栈为何损坏）且零副作用，保持统一，不为只读态开分支。

## Migration Plan

纯增量：新端点 + 前端被动校验，无数据迁移。回滚 = 移除前端调用（后端端点无害留存）。

## Open Questions

（无）
