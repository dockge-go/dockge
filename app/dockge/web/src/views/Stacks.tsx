import { For, Show, createEffect, createMemo, createSignal, on, onCleanup, onMount } from "solid-js";
import { Layers, Play, Plus, RotateCw, Square, Trash2 } from "lucide-solid";

import { api, type StackDetail, type StackOp } from "../api/api";
import { confirmDialog } from "../components/Confirm";
import { Pagination } from "../components/Pagination";
import { StackWorkspace } from "../components/StackWorkspace";
import { EmptyState, SectionHeader, StackStatusBadge } from "../components/widgets";
import { errText } from "../api/format";
import { clampPage, paginate } from "../lib/pagination";
import { listNav } from "../lib/listnav";
import type { ValidationState } from "../lib/validate";
import { pendingNewStack, pendingStack, refresh, setPendingNewStack, setPendingStack, snapshot, toast } from "../store/index";
import { t } from "../i18n";

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
  const [validation, setValidation] = createSignal<ValidationState>({ status: "checking" });
  let listHost: HTMLDivElement | undefined;

  const rows = createMemo(() => snapshot()?.stacks ?? []);
  const visibleRows = createMemo(() => paginate(rows(), page()));
  const containerCount = (stack: string) => (snapshot()?.containers ?? []).filter((container) => container.stack === stack).length;

  createEffect(() => setPage((value) => clampPage(value, rows().length)));

  // 校验闭环（被动）：工作区打开时，compose/env 任一变更防抖 2s 后草稿校验；
  // 序号守卫丢弃在途过期响应。打开栈、composerize 落入、保存均经 yaml/env
  // 信号触发同一循环，无需在各处插桩。
  let validateTimer: ReturnType<typeof setTimeout> | undefined;
  let validateSeq = 0;
  createEffect(
    on(
      () => [mode(), yaml(), env()] as const,
      ([current]) => {
        if (current === "list") return;
        validateSeq++;
        setValidation({ status: "checking" });
        clearTimeout(validateTimer);
        validateTimer = setTimeout(() => {
          const seq = validateSeq;
          api
            .validateStack(yaml(), env())
            .then((result) => {
              if (seq === validateSeq) setValidation({ status: "done", result });
            })
            .catch((error: unknown) => {
              if (seq === validateSeq) {
                setValidation({ status: "done", result: { valid: false, errors: [{ line: 0, message: errText(error) }] } });
              }
            });
        }, 2000);
      },
    ),
  );
  onCleanup(() => clearTimeout(validateTimer));

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
    if (listHost) onCleanup(listNav(listHost));
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
      setOutput(result.output || t("stack.opNoOutput"));
      toast(t("toast.stackOpDone", { name: name(), op: operation }), "success");
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
      toast(t("toast.stackSaved"), "success");
      return true;
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
      return false;
    }
  };

  const createStack = async (deploy: boolean) => {
    const stackName = name().trim();
    if (!/^[a-z0-9_-]+$/.test(stackName)) return toast(t("toast.stackNameInvalid"), "error");
    setBusy(true);
    try {
      await api.createStack(stackName, yaml(), env());
      await refresh(false);
      await openDetail(stackName);
      if (deploy) await executeOperation("start");
      else toast(t("toast.stackCreated"), "success");
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
      toast(t("toast.stackConverted"), "success");
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!name() || !(await confirmDialog(t("stack.confirmRemove"), t("stack.confirmRemoveDesc", { name: name() })))) return;
    try {
      await api.deleteStack(name());
      setMode("list");
      setDetail(null);
      await refresh(false);
      toast(t("toast.stackDeleted"), "success");
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
      <SectionHeader title={t("view.stacks")} subtitle={t("stack.workspaceSubtitle")} actions={<button class="btn btn-primary" onClick={openCreate}><Plus size={14} /> {t("stack.new")}</button>} />
      <div class="master-detail stack-master-detail" classList={{ "has-detail": mode() !== "list" }}>
        <section class="master-pane" aria-label={t("aria.stackList")}>
          <Show when={rows().length > 0} fallback={<EmptyState title={t("stack.noStacks")} desc={t("stack.noStacksDesc")} icon={<Layers size={44} />} />}>
            <div class="resource-list" ref={listHost}>
              <For each={visibleRows()}>
                {(stack) => (
                  <article class="resource-row" classList={{ selected: mode() === "detail" && name() === stack.name }} tabindex="0" onClick={() => void openDetail(stack.name)} onKeyDown={(event) => event.key === "Enter" && void openDetail(stack.name)}>
                    <div class="resource-row-main"><strong>{stack.name}</strong><span>{stack.composeFileName || t("stack.external")}</span></div>
                    <StackStatusBadge status={stack.status} label={stack.statusLabel} />
                    <p>{t("stack.containersCount", { n: containerCount(stack.name) })} · {stack.managed ? t("stack.managed") : t("stack.readonlyExternal")}</p>
                    <div class="resource-row-actions" onClick={(event) => event.stopPropagation()}>
                      <Show when={stack.status === 3} fallback={<button class="btn-icon" aria-label={t("act.start")} onClick={() => { setName(stack.name); void runOperation("start"); }}><Play size={14} /></button>}>
                        <button class="btn-icon" aria-label={t("act.stop")} onClick={() => { setName(stack.name); void runOperation("stop"); }}><Square size={14} /></button>
                      </Show>
                      <button class="btn-icon" aria-label={t("act.restart")} onClick={() => { setName(stack.name); void runOperation("restart"); }}><RotateCw size={14} /></button>
                      <button class="btn-icon danger" aria-label={t("common.delete")} onClick={() => { setName(stack.name); void remove(); }}><Trash2 size={14} /></button>
                    </div>
                  </article>
                )}
              </For>
            </div>
            <Pagination page={page()} total={rows().length} onPageChange={setPage} />
          </Show>
        </section>

        <Show when={mode() !== "list"} fallback={<div class="detail-placeholder"><p>{t("stack.selectHint")}</p></div>}>
          <StackWorkspace mode={mode() === "create" ? "create" : "detail"} detail={detail()} name={name()} yaml={yaml()} env={env()} validation={validation()} output={output()} busy={busy()} runCommand={runCommand()} onBack={() => setMode("list")} onNameChange={setName} onYamlChange={setYaml} onEnvChange={setEnv} onRunCommandChange={setRunCommand} onConvert={() => void convert()} onSave={() => mode() === "create" ? void createStack(false) : void saveExisting()} onPrimary={() => void primary()} onOperation={(operation) => void runOperation(operation)} onRemove={() => void remove()} />
        </Show>
      </div>
    </div>
  );
}
