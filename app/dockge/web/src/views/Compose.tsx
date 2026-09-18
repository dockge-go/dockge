// 栈详情/新建页（上游复刻）：标题行（状态 pill + 操作按钮组双态）+ 双栏
// （左：容器卡片/合并日志；右：CodeMirror 编辑器 compose + .env 上下排列）。
// 校验闭环迁移自原 Stacks 视图：防抖 2s 自动草稿校验 + 诊断注入 + 四态 pill。
import { For, Show, createEffect, createSignal, on, onCleanup, onMount } from "solid-js";
import { A, useNavigate, useParams } from "@solidjs/router";
import { ExternalLink, Play, RotateCw, Square, Terminal } from "lucide-solid";

import { api, type StackDetail, type StackOp } from "../api/api";
import { errText } from "../api/format";
import { confirmDialog } from "../components/Confirm";
import { StackEditor } from "../components/StackEditor";
import { TerminalPane } from "../components/Terminal";
import { t } from "../i18n";
import type { ValidationState } from "../lib/validate";
import { draftYaml, refresh, setDraftYaml, toast } from "../store/index";

const STARTER_YAML = `services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
`;

function statusClass(status: number): string {
  if (status === 3) return "active";
  if (status === 4) return "exited";
  return "";
}

function statusLabel(status: number): string {
  if (status === 3) return t("home.active");
  if (status === 4 || status === 2) return t("home.exited");
  return t("home.inactive");
}

export function Compose() {
  const params = useParams();
  const navigate = useNavigate();
  const isNew = () => params.name === undefined;

  const [detail, setDetail] = createSignal<StackDetail | null>(null);
  const [name, setName] = createSignal("");
  const [yaml, setYaml] = createSignal(STARTER_YAML);
  const [env, setEnv] = createSignal("");
  const [editMode, setEditMode] = createSignal(true);
  const [busy, setBusy] = createSignal(false);
  const [output, setOutput] = createSignal("");
  const [validation, setValidation] = createSignal<ValidationState>({ status: "checking" });
  const [loadedYaml, setLoadedYaml] = createSignal(STARTER_YAML);
  const [loadedEnv, setLoadedEnv] = createSignal("");
  const dirty = () => yaml() !== loadedYaml() || env() !== loadedEnv();

  onMount(() => {
    if (isNew()) {
      const draft = draftYaml();
      if (draft) {
        setYaml(draft);
        setLoadedYaml(draft);
        setDraftYaml(null);
      }
    }
  });

  // 路由参数变化（切换栈 / 新建保存后跳转）→ 加载对应栈
  createEffect(() => {
    const routeName = params.name;
    if (routeName) void load(decodeURIComponent(routeName));
  });

  const load = async (stackName: string) => {
    try {
      const data = await api.stack(stackName);
      setDetail(data);
      setName(data.name);
      setYaml(data.yaml);
      setEnv(data.env);
      setLoadedYaml(data.yaml);
      setLoadedEnv(data.env);
      setEditMode(false);
    } catch (error) {
      toast(errText(error), "error");
      navigate("/", { replace: true });
    }
  };

  // 校验闭环（被动）：compose/env 任一变更防抖 2s 后草稿校验；序号守卫丢弃过期响应
  let validateTimer: ReturnType<typeof setTimeout> | undefined;
  let validateSeq = 0;
  createEffect(
    on(
      () => [editMode(), yaml(), env()] as const,
      () => {
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

  const save = async (): Promise<boolean> => {
    if (isNew()) {
      const stackName = name().trim().toLowerCase();
      if (!/^[a-z0-9_-]+$/.test(stackName)) {
        toast(t("compose.nameHelp"), "error");
        return false;
      }
      try {
        await api.createStack(stackName, yaml(), env());
        await refresh(false);
        navigate(`/compose/${encodeURIComponent(stackName)}`, { replace: true });
        return true;
      } catch (error) {
        setOutput(errText(error));
        toast(errText(error), "error");
        return false;
      }
    }
    const data = detail();
    if (!data?.managed) return false;
    try {
      await api.saveStack(data.name, yaml(), env());
      setLoadedYaml(yaml());
      setLoadedEnv(env());
      return true;
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
      return false;
    }
  };

  const deploy = async () => {
    if (busy()) return;
    setBusy(true);
    try {
      const saved = await save();
      if (saved) await runOp(detail() ? (detail()?.status === 3 ? "update" : "start") : "start");
    } finally {
      setBusy(false);
    }
  };

  const runOp = async (op: StackOp) => {
    const stackName = detail()?.name ?? name().trim().toLowerCase();
    if (!stackName) return;
    setOutput(`$ docker compose ${op}\n`);
    try {
      const result = await api.stackOp(stackName, op);
      setOutput(result.output || "");
      toast(t("toast.saved"), "success");
      await refresh(false);
      await load(stackName);
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
    }
  };

  const remove = async () => {
    const stackName = detail()?.name;
    if (!stackName) return;
    if (!(await confirmDialog(t("compose.confirmDelete"), t("compose.confirmDeleteDesc", { name: stackName })))) return;
    setOutput("$ docker compose down\n");
    try {
      await api.deleteStack(stackName);
      await refresh(false);
      toast(t("toast.deleted", { name: stackName }), "success");
      navigate("/", { replace: true });
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
    }
  };

  const discard = async () => {
    if (!(await confirmDialog(t("compose.discard"), t("compose.discardConfirm"), false))) return;
    if (isNew()) {
      setYaml(STARTER_YAML);
      setEnv("");
    } else if (detail()) {
      setYaml(detail()!.yaml);
      setEnv(detail()!.env);
    }
  };

  const managed = () => isNew() || detail()?.managed === true;
  const editing = () => editMode() && managed();

  return (
    <div>
      <div class="compose-header">
        <h1>
          <Show when={!isNew()} fallback={<span>{t("compose.newTitle")}</span>}>
            <span class={`status-pill ${statusClass(detail()?.status ?? 0)}`}>{statusLabel(detail()?.status ?? 0)}</span>
            <span>{params.name ? decodeURIComponent(params.name) : ""}</span>
          </Show>
        </h1>
        <div class="compose-actions">
          <Show
            when={editing()}
            fallback={
              <Show when={managed()}>
                <button class="btn btn-secondary" onClick={() => setEditMode(true)}>{t("compose.edit")}</button>
                <Show when={detail()?.status === 3} fallback={<button class="btn btn-secondary" disabled={busy()} onClick={() => void runOp("start")}><Play size={14} /> {t("compose.start")}</button>}>
                  <button class="btn btn-secondary" disabled={busy()} onClick={() => void runOp("restart")}><RotateCw size={14} /> {t("compose.restart")}</button>
                </Show>
                <button class="btn btn-secondary" disabled={busy()} onClick={() => void runOp("update")}>{t("compose.update")}</button>
                <button class="btn btn-secondary" disabled={busy()} onClick={() => void runOp("stop")}><Square size={14} /> {t("compose.stop")}</button>
                <button class="btn btn-secondary" disabled={busy()} onClick={() => void runOp("down")}>{t("compose.down")}</button>
                <button class="btn btn-danger" disabled={busy()} onClick={() => void remove()}>{t("compose.delete")}</button>
              </Show>
            }
          >
            <button class="btn btn-primary" disabled={busy()} onClick={() => void deploy()}>{t("compose.deploy")}</button>
            <button class="btn btn-secondary" disabled={busy()} onClick={() => void save().then((ok) => ok && toast(t("toast.draftSaved"), "success"))}>{t("compose.saveDraft")}</button>
            <button class="btn btn-secondary" disabled={busy() || (!dirty() && !isNew())} onClick={() => void discard()}>{t("compose.discard")}</button>
          </Show>
        </div>
      </div>

      <Show when={detail()?.urls?.length}>
        <div class="url-badges">
          <For each={detail()?.urls ?? []}>
            {(url) => (
              <a class="url-badge" href={url} target="_blank" rel="noreferrer">
                <ExternalLink size={11} style={{ "vertical-align": "-1px" }} /> {url}
              </a>
            )}
          </For>
        </div>
      </Show>

      <div class="compose-grid">
        <div>
          <Show when={isNew()}>
            <div class="card">
              <label class="form-label" for="compose-name">{t("compose.name")}</label>
              <input id="compose-name" class="form-input" value={name()} placeholder={t("compose.namePlaceholder")} onInput={(e) => setName(e.currentTarget.value.toLowerCase())} />
              <p class="form-help">{t("compose.nameHelp")}</p>
            </div>
          </Show>

          <div class="card" style={{ "margin-top": isNew() ? "20px" : "0" }}>
            <h4 class="card-title">{t("compose.containers")}</h4>
            <Show
              when={(detail()?.containers ?? []).length > 0}
              fallback={<p class="settings-desc">{managed() ? t("compose.noContainers") : ""}</p>}
            >
              <For each={detail()?.containers ?? []}>
                {(container) => (
                  <div class="container-card">
                    <div class="container-card-row">
                      <div class="container-card-main">
                        <h4>{container.service || container.name}</h4>
                        <div class="container-meta">{container.name}</div>
                        <div class="container-meta mono">{container.status}</div>
                      </div>
                      <div class="container-card-actions">
                        <Show when={container.state === "running"}>
                          <A class="btn-icon" aria-label={t("container.terminal")} href={`/terminal/${encodeURIComponent(params.name ?? "")}/${encodeURIComponent(container.id)}/bash`}>
                            <Terminal size={15} />
                          </A>
                        </Show>
                      </div>
                    </div>
                  </div>
                )}
              </For>
            </Show>
          </div>

          <Show when={!editing() && !isNew() && detail()}>
            <div class="card">
              <h4 class="card-title">{t("compose.terminal")}</h4>
              <TerminalPane name={detail()!.name} type="compose-logs" />
            </div>
          </Show>
        </div>

        <div class="compose-editors">
          <div>
            <div class="editor-file-title">{detail()?.composeFileName || "compose.yaml"}</div>
            <StackEditor
              file="compose"
              yaml={yaml()}
              env={env()}
              readonly={!editing()}
              fullscreen={false}
              validation={validation()}
              output=""
              onYamlChange={setYaml}
              onEnvChange={setEnv}
              onFullscreenChange={() => {}}
            />
          </div>
          <Show when={editing()}>
            <div>
              <div class="editor-file-title">.env</div>
              <StackEditor
                file="env"
                yaml={yaml()}
                env={env()}
                readonly={false}
                fullscreen={false}
                validation={{ status: "done", result: { valid: true, errors: [] } }}
                output=""
                onYamlChange={setYaml}
                onEnvChange={setEnv}
                onFullscreenChange={() => {}}
              />
            </div>
          </Show>
          <Show when={output()}>
            <div class="card">
              <h4 class="card-title">{t("compose.progress")}</h4>
              <pre class="operation-output" style={{ "max-height": "220px" }}>{output()}</pre>
            </div>
          </Show>
        </div>
      </div>
    </div>
  );
}
