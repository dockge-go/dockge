import { For, Show, createMemo, createSignal } from "solid-js";
import { ArrowLeft, FileCode2, FileKey2, Play, RotateCw, Square, Terminal, Trash2, Wand2 } from "lucide-solid";

import type { StackDetail, StackOp } from "../api/api";
import { summarizeCompose } from "../lib/compose-summary";
import { StackEditor } from "./StackEditor";
import { TerminalPane } from "./Terminal";
import { StackStatusBadge, StatusBadge } from "./widgets";
import { t } from "../i18n";

type FileName = "compose" | "env";
type CenterView = "editor" | "logs" | "exec";

export function StackWorkspace(props: {
  mode: "create" | "detail";
  detail: StackDetail | null;
  name: string;
  yaml: string;
  env: string;
  output: string;
  busy: boolean;
  runCommand: string;
  onBack: () => void;
  onNameChange: (value: string) => void;
  onYamlChange: (value: string) => void;
  onEnvChange: (value: string) => void;
  onRunCommandChange: (value: string) => void;
  onConvert: () => void;
  onSave: () => void;
  onPrimary: () => void;
  onOperation: (operation: StackOp) => void;
  onRemove: () => void;
}) {
  const [file, setFile] = createSignal<FileName>("compose");
  const [center, setCenter] = createSignal<CenterView>("editor");
  const [fullscreen, setFullscreen] = createSignal(false);
  const [execId, setExecId] = createSignal("");
  const summary = createMemo(() => summarizeCompose(props.yaml));
  const managed = () => props.mode === "create" || props.detail?.managed === true;

  const openExec = (id: string) => {
    setExecId(id);
    setCenter("exec");
  };

  return (
    <article class="stack-workspace" aria-label={props.mode === "create" ? t("aria.newStack") : t("aria.stackDetail", { name: props.name })}>
      <header class="stack-workspace-header">
        <button class="workspace-back" onClick={props.onBack}><ArrowLeft size={16} /> {t("stack.list")}</button>
        <div class="stack-workspace-title">
          <Show
            when={props.mode === "create"}
            fallback={<><h2>{props.name}</h2><Show when={props.detail}>{(detail) => <StackStatusBadge status={detail().status} label={detail().statusLabel} />}</Show></>}
          >
            <h2>{t("stack.new")}</h2>
          </Show>
        </div>
        <Show when={props.detail}>
          {(detail) => (
            <div class="detail-actions">
              <Show when={detail().status === 3} fallback={<button class="btn btn-secondary" disabled={props.busy} onClick={() => props.onOperation("start")}><Play size={14} /> {t("act.start")}</button>}>
                <button class="btn btn-secondary" disabled={props.busy} onClick={() => props.onOperation("stop")}><Square size={14} /> {t("act.stop")}</button>
              </Show>
              <button class="btn btn-secondary" disabled={props.busy} onClick={() => props.onOperation("restart")}><RotateCw size={14} /> {t("act.restart")}</button>
              <button class="btn btn-danger" disabled={props.busy} onClick={props.onRemove}><Trash2 size={14} /> {t("common.delete")}</button>
            </div>
          )}
        </Show>
      </header>

      <div class="stack-workspace-body">
        <aside class="stack-file-rail">
          <Show when={props.mode === "create"}>
            <label class="form-label" for="stack-name">{t("stack.name")}</label>
            <input id="stack-name" class="form-input" value={props.name} placeholder="my-app" onInput={(event) => props.onNameChange(event.currentTarget.value.toLowerCase())} />
            <p class="form-help">{t("stack.nameHelp")}</p>
          </Show>
          <p class="rail-label">{t("stack.files")}</p>
          <button class="file-button" classList={{ active: file() === "compose" }} onClick={() => { setFile("compose"); setCenter("editor"); }}><FileCode2 size={15} /> compose.yaml</button>
          <button class="file-button" classList={{ active: file() === "env" }} onClick={() => { setFile("env"); setCenter("editor"); }}><FileKey2 size={15} /> .env</button>
          <Show when={props.mode === "create"}>
            <div class="composerize-box">
              <label class="rail-label" for="docker-run">{t("stack.dockerRunConvert")}</label>
              <textarea id="docker-run" value={props.runCommand} placeholder="docker run -d -p 8080:80 nginx" onInput={(event) => props.onRunCommandChange(event.currentTarget.value)} />
              <button class="btn btn-secondary" disabled={!props.runCommand.trim() || props.busy} onClick={props.onConvert}><Wand2 size={13} /> {t("stack.convertToCompose")}</button>
            </div>
          </Show>
          <Show when={props.mode === "detail"}>
            <p class="rail-label">{t("stack.tools")}</p>
            <button class="file-button" classList={{ active: center() === "logs" }} onClick={() => setCenter("logs")}><Terminal size={15} /> {t("stack.stackLogs")}</button>
          </Show>
        </aside>

        <main class="stack-center">
          <Show when={center() === "editor"}>
            <StackEditor file={file()} yaml={props.yaml} env={props.env} readonly={!managed()} fullscreen={fullscreen()} output={props.output} onYamlChange={props.onYamlChange} onEnvChange={props.onEnvChange} onFullscreenChange={setFullscreen} />
          </Show>
          <Show when={center() === "logs"}><TerminalPane name={props.name} type="compose-logs" /></Show>
          <Show when={center() === "exec" && execId()}><TerminalPane name={execId()} type="exec" /></Show>
        </main>

        <aside class="stack-summary-rail">
          <h3>{t("stack.deploySummary")}</h3>
          <p>{t("stack.deploySummaryDesc")}</p>
          <div class="summary-counts">
            <div><span>{t("stack.services")}</span><strong>{summary().services.length}</strong></div>
            <div><span>{t("common.ports")}</span><strong>{summary().ports}</strong></div>
            <div><span>Volumes</span><strong>{summary().volumes}</strong></div>
            <div><span>Networks</span><strong>{summary().networks || "default"}</strong></div>
          </div>
          <div class="service-summary">
            <For each={summary().services} fallback={<p class="text-dim">{t("stack.waitingServices")}</p>}>
              {(service) => {
                const container = () => props.detail?.containers.find((item) => item.service === service || item.name === service);
                return (
                  <div class="service-summary-row">
                    <div><strong>{service}</strong><span>{container()?.name ?? t("stack.notDeployed")}</span></div>
                    <Show when={container()}>
                      {(item) => <><StatusBadge state={item().state} /><button class="btn-icon" aria-label={t("aria.openTerminal", { service })} onClick={() => openExec(item().id)}><Terminal size={14} /></button></>}
                    </Show>
                  </div>
                );
              }}
            </For>
          </div>
        </aside>
      </div>

      <Show when={managed()}>
        <footer class="stack-workspace-footer">
          <span>{props.mode === "create" ? t("stack.createFailDraft") : t("stack.saveFailDraft")}</span>
          <button class="btn btn-secondary" disabled={props.busy} onClick={props.onSave}>{props.mode === "create" ? t("stack.saveOnly") : t("stack.save")}</button>
          <button class="btn btn-primary" disabled={props.busy || !props.name || !props.yaml.trim()} onClick={props.onPrimary}>
            {props.mode === "create" ? t("stack.createAndDeploy") : props.detail?.status === 3 ? t("stack.saveAndRedeploy") : t("stack.saveAndDeploy")}
          </button>
        </footer>
      </Show>
    </article>
  );
}
