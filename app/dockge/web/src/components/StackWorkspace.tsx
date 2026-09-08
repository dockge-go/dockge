import { For, Show, createMemo, createSignal } from "solid-js";
import { ArrowLeft, FileCode2, FileKey2, Play, RotateCw, Square, Terminal, Trash2, Wand2 } from "lucide-solid";

import type { StackDetail, StackOp } from "../api/api";
import { summarizeCompose } from "../lib/compose-summary";
import { StackEditor } from "./StackEditor";
import { TerminalPane } from "./Terminal";
import { StackStatusBadge, StatusBadge } from "./widgets";

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
    <article class="stack-workspace" aria-label={props.mode === "create" ? "新建 Stack" : `${props.name} Stack 详情`}>
      <header class="stack-workspace-header">
        <button class="workspace-back" onClick={props.onBack}><ArrowLeft size={16} /> Stack 列表</button>
        <div class="stack-workspace-title">
          <Show
            when={props.mode === "create"}
            fallback={<><h2>{props.name}</h2><Show when={props.detail}>{(detail) => <StackStatusBadge status={detail().status} label={detail().statusLabel} />}</Show></>}
          >
            <h2>New Stack</h2>
          </Show>
        </div>
        <Show when={props.detail}>
          {(detail) => (
            <div class="detail-actions">
              <Show when={detail().status === 3} fallback={<button class="btn btn-secondary" disabled={props.busy} onClick={() => props.onOperation("start")}><Play size={14} /> 启动</button>}>
                <button class="btn btn-secondary" disabled={props.busy} onClick={() => props.onOperation("stop")}><Square size={14} /> 停止</button>
              </Show>
              <button class="btn btn-secondary" disabled={props.busy} onClick={() => props.onOperation("restart")}><RotateCw size={14} /> 重启</button>
              <button class="btn btn-danger" disabled={props.busy} onClick={props.onRemove}><Trash2 size={14} /> 删除</button>
            </div>
          )}
        </Show>
      </header>

      <div class="stack-workspace-body">
        <aside class="stack-file-rail">
          <Show when={props.mode === "create"}>
            <label class="form-label" for="stack-name">Stack 名称</label>
            <input id="stack-name" class="form-input" value={props.name} placeholder="my-app" onInput={(event) => props.onNameChange(event.currentTarget.value.toLowerCase())} />
            <p class="form-help">小写字母、数字、连字符或下划线</p>
          </Show>
          <p class="rail-label">文件</p>
          <button class="file-button" classList={{ active: file() === "compose" }} onClick={() => { setFile("compose"); setCenter("editor"); }}><FileCode2 size={15} /> compose.yaml</button>
          <button class="file-button" classList={{ active: file() === "env" }} onClick={() => { setFile("env"); setCenter("editor"); }}><FileKey2 size={15} /> .env</button>
          <Show when={props.mode === "create"}>
            <div class="composerize-box">
              <label class="rail-label" for="docker-run">docker run 转换</label>
              <textarea id="docker-run" value={props.runCommand} placeholder="docker run -d -p 8080:80 nginx" onInput={(event) => props.onRunCommandChange(event.currentTarget.value)} />
              <button class="btn btn-secondary" disabled={!props.runCommand.trim() || props.busy} onClick={props.onConvert}><Wand2 size={13} /> 转换到 Compose</button>
            </div>
          </Show>
          <Show when={props.mode === "detail"}>
            <p class="rail-label">工具</p>
            <button class="file-button" classList={{ active: center() === "logs" }} onClick={() => setCenter("logs")}><Terminal size={15} /> Stack 日志</button>
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
          <h3>部署摘要</h3>
          <p>从当前 Compose 文本即时推导，保存时以后端校验为准。</p>
          <div class="summary-counts">
            <div><span>Services</span><strong>{summary().services.length}</strong></div>
            <div><span>Ports</span><strong>{summary().ports}</strong></div>
            <div><span>Volumes</span><strong>{summary().volumes}</strong></div>
            <div><span>Networks</span><strong>{summary().networks || "default"}</strong></div>
          </div>
          <div class="service-summary">
            <For each={summary().services} fallback={<p class="text-dim">等待 services 配置</p>}>
              {(service) => {
                const container = () => props.detail?.containers.find((item) => item.service === service || item.name === service);
                return (
                  <div class="service-summary-row">
                    <div><strong>{service}</strong><span>{container()?.name ?? "未部署"}</span></div>
                    <Show when={container()}>
                      {(item) => <><StatusBadge state={item().state} /><button class="btn-icon" aria-label={`打开 ${service} 终端`} onClick={() => openExec(item().id)}><Terminal size={14} /></button></>}
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
          <span>{props.mode === "create" ? "创建失败时保留当前草稿与输出" : "保存或部署失败时保留未提交内容"}</span>
          <button class="btn btn-secondary" disabled={props.busy} onClick={props.onSave}>{props.mode === "create" ? "仅保存" : "保存"}</button>
          <button class="btn btn-primary" disabled={props.busy || !props.name || !props.yaml.trim()} onClick={props.onPrimary}>
            {props.mode === "create" ? "创建并部署" : props.detail?.status === 3 ? "保存并重新部署" : "保存并部署"}
          </button>
        </footer>
      </Show>
    </article>
  );
}
