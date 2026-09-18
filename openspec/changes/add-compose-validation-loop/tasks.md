## 1. 后端

- [ ] 1.1 `api/v1/dockge.go`：新增 `StackValidateRequest{Yaml, Env}` 与 `StackValidateResponse{Valid, Errors[]{Line, Message}}` DTO
- [ ] 1.2 `internal/repository/compose.go`：新增 `ValidateCompose`（MkdirTemp + 写 compose.yaml/.env + `runDockerIn` 执行 `compose config`，15s 超时，defer RemoveAll）
- [ ] 1.3 `internal/service/stack.go`：新增 `Validate`（语法先行：`yaml.v3` 行列号；语义：stderr 正则提行号）+ 纯解析函数；`StackService` 接口加方法
- [ ] 1.4 `internal/handler/stack.go`：新增 `Validate` handler（恒 200，校验结果是数据）
- [ ] 1.5 `internal/server/http.go`：注册 `POST /stacks/validate`（strictAuth）
- [ ] 1.6 表驱动测试：解析纯函数（yaml 行列号、stderr `line N` 提取、无行号降级）

## 2. 前端

- [ ] 2.1 `web/package.json`：新增 `@codemirror/lint` 依赖（pnpm install）
- [ ] 2.2 `web/src/lib/validate.ts`：纯函数——后端错误 → CodeMirror Diagnostic（行号→偏移，越界/0 跳过行内）+ `errorLine` 辅助；`web/tests/validate.test.ts` 表驱动测试
- [ ] 2.3 `web/src/api/api.ts`：`validateStack(yaml, env)` + DTO 类型
- [ ] 2.4 `StackEditor.tsx`：挂 `linter(()=>[])` + `lintGutter()`；`diagnostics` prop → `setDiagnostics`；文件切 env 清空；工具栏四态（检查中/通过/语法错/语义错+计数），移除 `services:` 存在性检查
- [ ] 2.5 `Stacks.tsx`：`validation` 状态信号（idle/checking/result）+ 防抖 2s effect + AbortController；传 `diagnostics`/`validating` 给 `StackWorkspace`→`StackEditor`
- [ ] 2.6 i18n：`zh-CN.ts` + `en-US.ts` 新键（editor.checking/validCompose/syntaxError/semanticError 等）
- [ ] 2.7 样式：校验态 pill 复用 `.editor-validation` 既有类，仅按需补 checking 态

## 3. 构建验证与文档

- [ ] 3.1 `pnpm build`（tsc + vite）通过；`node --test` 前端测试通过
- [ ] 3.2 Linux 上 `go test ./...`（macOS 受 pkg/pty 限制无法跑，见 PROJECT_SPEC D10）
- [ ] 3.3 启动服务，curl 验证 `/v1/stacks/validate`（语法错带行号、语义错、通过三例）
- [ ] 3.4 文档同步：`PROJECT_SPEC.md` §3.2 状态列 + §4 Non-Goal #5 陈旧 textarea 描述校正；`REQUIREMENTS.md` §6.3 编辑器条目更新
