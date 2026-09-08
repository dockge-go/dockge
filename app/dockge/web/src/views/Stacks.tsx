import { For, Show, createEffect, createMemo, createSignal, onMount } from "solid-js";
import { Layers, Play, Plus, RotateCw, Square, Trash2 } from "lucide-solid";

import { api, type StackDetail, type StackOp } from "../api/api";
import { confirmDialog } from "../components/Confirm";
import { Pagination } from "../components/Pagination";
import { StackWorkspace } from "../components/StackWorkspace";
import { EmptyState, SectionHeader, StackStatusBadge } from "../components/widgets";
import { errText } from "../api/format";
import { clampPage, paginate } from "../lib/pagination";
import { pendingNewStack, pendingStack, refresh, setPendingNewStack, setPendingStack, snapshot, toast } from "../store/index";

const STARTER_YAML = `services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
`;

type WorkspaceMode = "list" | "detail" | "create";

export function Stacks() {
  const [page, setPage] = createSignal(1);
  const [mode, setMode] = createSignal<WorkspaceMode>("list");
  const [detail, setDetail] = createSignal<StackDetail | null>(null);
  const [name, setName] = createSignal("");
  const [yaml, setYaml] = createSignal(STARTER_YAML);
  const [env, setEnv] = createSignal("");
  const [runCommand, setRunCommand] = createSignal("");
  const [output, setOutput] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  const rows = createMemo(() => snapshot()?.stacks ?? []);
  const visibleRows = createMemo(() => paginate(rows(), page()));
  const containerCount = (stack: string) => (snapshot()?.containers ?? []).filter((container) => container.stack === stack).length;

  createEffect(() => setPage((value) => clampPage(value, rows().length)));

  const openDetail = async (stackName: string, keepOutput = false) => {
    setMode("detail");
    setName(stackName);
    setDetail(null);
    if (!keepOutput) setOutput("");
    try {
      const data = await api.stack(stackName);
      setDetail(data);
      setYaml(data.yaml);
      setEnv(data.env);
    } catch (error) {
      toast(errText(error), "error");
      setMode("list");
    }
  };

  const openCreate = () => {
    setMode("create");
    setDetail(null);
    setName("");
    setYaml(STARTER_YAML);
    setEnv("");
    setRunCommand("");
    setOutput("");
  };

  onMount(() => {
    const stack = pendingStack();
    if (stack) {
      setPendingStack(null);
      void openDetail(stack);
    }
    if (pendingNewStack()) {
      setPendingNewStack(false);
      openCreate();
    }
  });

  const executeOperation = async (operation: StackOp) => {
    if (!name()) return false;
    setOutput(`$ docker compose ${operation}\n`);
    try {
      const result = await api.stackOp(name(), operation);
      setOutput(result.output || "操作完成，无额外输出");
      toast(`Stack ${name()} ${operation} 完成`, "success");
      await refresh(false);
      await openDetail(name(), true);
      return true;
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
      return false;
    }
  };

  const runOperation = async (operation: StackOp) => {
    if (busy()) return;
    setBusy(true);
    try {
      await executeOperation(operation);
    } finally {
      setBusy(false);
    }
  };

  const saveExisting = async (): Promise<boolean> => {
    const data = detail();
    if (!data?.managed) return false;
    try {
      await api.saveStack(data.name, yaml(), env());
      toast("Stack 文件已保存", "success");
      return true;
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
      return false;
    }
  };

  const createStack = async (deploy: boolean) => {
    const stackName = name().trim();
    if (!/^[a-z0-9_-]+$/.test(stackName)) return toast("Stack 名称格式不正确", "error");
    setBusy(true);
    try {
      await api.createStack(stackName, yaml(), env());
      await refresh(false);
      await openDetail(stackName);
      if (deploy) await executeOperation("start");
      else toast("Stack 已保存，尚未部署", "success");
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
    } finally {
      setBusy(false);
    }
  };

  const convert = async () => {
    if (!runCommand().trim()) return;
    setBusy(true);
    try {
      const result = await api.composerize(runCommand().trim());
      setYaml(result.composeTemplate);
      toast("已转换到 Compose 编辑器", "success");
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!name() || !(await confirmDialog("删除 Stack", `将执行 down 并删除 ${name()} 的配置目录，此操作不可恢复。`))) return;
    try {
      await api.deleteStack(name());
      setMode("list");
      setDetail(null);
      await refresh(false);
      toast("Stack 已删除", "success");
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
    }
  };

  const primary = async () => {
    if (mode() === "create") {
      await createStack(true);
      return;
    }
    if (await saveExisting()) {
      await runOperation(detail()?.status === 3 ? "update" : "start");
    }
  };

  return (
    <div class="view-section workspace-view">
      <SectionHeader title="Stacks" subtitle="本机 Docker Compose 工作区" actions={<button class="btn btn-primary" onClick={openCreate}><Plus size={14} /> New Stack</button>} />
      <div class="master-detail stack-master-detail" classList={{ "has-detail": mode() !== "list" }}>
        <section class="master-pane" aria-label="Stack 列表">
          <Show when={rows().length > 0} fallback={<EmptyState title="还没有 Stack" desc="创建 Compose Stack，或把已有目录放入 stacks 目录。" icon={<Layers size={44} />} />}>
            <div class="resource-list">
              <For each={visibleRows()}>
                {(stack) => (
                  <article class="resource-row" classList={{ selected: mode() === "detail" && name() === stack.name }} tabindex="0" onClick={() => void openDetail(stack.name)} onKeyDown={(event) => event.key === "Enter" && void openDetail(stack.name)}>
                    <div class="resource-row-main"><strong>{stack.name}</strong><span>{stack.composeFileName || "外部 Stack"}</span></div>
                    <StackStatusBadge status={stack.status} label={stack.statusLabel} />
                    <p>{containerCount(stack.name)} 个容器 · {stack.managed ? "托管" : "只读外部"}</p>
                    <div class="resource-row-actions" onClick={(event) => event.stopPropagation()}>
                      <Show when={stack.status === 3} fallback={<button class="btn-icon" aria-label="启动" onClick={() => { setName(stack.name); void runOperation("start"); }}><Play size={14} /></button>}>
                        <button class="btn-icon" aria-label="停止" onClick={() => { setName(stack.name); void runOperation("stop"); }}><Square size={14} /></button>
                      </Show>
                      <button class="btn-icon" aria-label="重启" onClick={() => { setName(stack.name); void runOperation("restart"); }}><RotateCw size={14} /></button>
                      <button class="btn-icon danger" aria-label="删除" onClick={() => { setName(stack.name); void remove(); }}><Trash2 size={14} /></button>
                    </div>
                  </article>
                )}
              </For>
            </div>
            <Pagination page={page()} total={rows().length} onPageChange={setPage} />
          </Show>
        </section>

        <Show when={mode() !== "list"} fallback={<div class="detail-placeholder"><p>选择一个 Stack，或创建新的 Compose Stack</p></div>}>
          <StackWorkspace mode={mode() === "create" ? "create" : "detail"} detail={detail()} name={name()} yaml={yaml()} env={env()} output={output()} busy={busy()} runCommand={runCommand()} onBack={() => setMode("list")} onNameChange={setName} onYamlChange={setYaml} onEnvChange={setEnv} onRunCommandChange={setRunCommand} onConvert={() => void convert()} onSave={() => mode() === "create" ? void createStack(false) : void saveExisting()} onPrimary={() => void primary()} onOperation={(operation) => void runOperation(operation)} onRemove={() => void remove()} />
        </Show>
      </div>
    </div>
  );
}
