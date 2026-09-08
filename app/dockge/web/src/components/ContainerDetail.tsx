import { For, Show, createSignal } from "solid-js";
import { ArrowLeft, FileText, Play, RotateCw, Square, Terminal, Trash2 } from "lucide-solid";

import type { ContainerInspectData, ContainerRow } from "../api/api";
import { LogStream } from "./LogStream";
import { TerminalPane } from "./Terminal";
import { SpinnerBlock, StatusBadge } from "./widgets";

type Tool = "logs" | "terminal";

export function ContainerDetail(props: {
  container: ContainerRow;
  inspect: ContainerInspectData | null;
  busy: boolean;
  onBack: () => void;
  onAction: (action: "start" | "stop" | "restart") => void;
  onRemove: () => void;
}) {
  const [tool, setTool] = createSignal<Tool>("logs");

  return (
    <article class="detail-workspace" aria-label={`${props.container.name} 容器详情`}>
      <header class="detail-workspace-header">
        <button class="workspace-back" onClick={props.onBack}>
          <ArrowLeft size={16} /> 容器列表
        </button>
        <div class="detail-heading">
          <div class="detail-title-row">
            <h2>{props.container.name}</h2>
            <StatusBadge state={props.container.state} />
          </div>
          <p class="mono detail-id">{props.container.id}</p>
        </div>
        <div class="detail-actions">
          <Show
            when={props.container.state === "running"}
            fallback={
              <button class="btn btn-primary" disabled={props.busy} onClick={() => props.onAction("start")}>
                <Play size={14} /> 启动
              </button>
            }
          >
            <button class="btn btn-secondary" disabled={props.busy} onClick={() => props.onAction("stop")}>
              <Square size={14} /> 停止
            </button>
          </Show>
          <button class="btn btn-secondary" disabled={props.busy} onClick={() => props.onAction("restart")}>
            <RotateCw size={14} /> 重启
          </button>
          <button class="btn btn-danger" disabled={props.busy} onClick={props.onRemove}>
            <Trash2 size={14} /> 删除
          </button>
        </div>
      </header>

      <div class="detail-workspace-body">
        <section class="detail-summary" aria-label="容器摘要">
          <div class="summary-item"><span>镜像</span><strong>{props.inspect?.Config?.Image ?? props.container.image}</strong></div>
          <div class="summary-item"><span>状态</span><strong>{props.container.status || "—"}</strong></div>
          <div class="summary-item"><span>端口</span><strong>{props.container.ports || "无"}</strong></div>
          <div class="summary-item"><span>Stack</span><strong>{props.container.stack || "—"}</strong></div>
          <div class="summary-item"><span>创建时间</span><strong>{props.inspect?.Created ? new Date(props.inspect.Created).toLocaleString() : "—"}</strong></div>
          <div class="summary-item"><span>IP 地址</span><strong>{props.inspect?.NetworkSettings?.IPAddress || "—"}</strong></div>
        </section>

        <section class="tool-panel">
          <div class="segmented" role="tablist" aria-label="容器工具">
            <button role="tab" aria-selected={tool() === "logs"} classList={{ active: tool() === "logs" }} onClick={() => setTool("logs")}>
              <FileText size={14} /> 日志
            </button>
            <button role="tab" aria-selected={tool() === "terminal"} classList={{ active: tool() === "terminal" }} onClick={() => setTool("terminal")}>
              <Terminal size={14} /> 终端
            </button>
          </div>
          <Show when={tool() === "logs"} fallback={<TerminalPane name={props.container.id} type="exec" />}>
            <LogStream containerId={props.container.id} />
          </Show>
        </section>

        <Show when={props.inspect} fallback={<SpinnerBlock />}>
          <div class="disclosure-list">
            <details>
              <summary>环境变量 <span>{props.inspect?.Config?.Env?.length ?? 0}</span></summary>
              <div class="disclosure-body mono-list">
                <For each={props.inspect?.Config?.Env ?? []} fallback={<p class="text-dim">未设置环境变量</p>}>
                  {(value) => <code>{value}</code>}
                </For>
              </div>
            </details>
            <details>
              <summary>挂载 <span>{props.inspect?.Mounts?.length ?? 0}</span></summary>
              <div class="disclosure-body">
                <For each={props.inspect?.Mounts ?? []} fallback={<p class="text-dim">无挂载</p>}>
                  {(mount) => (
                    <div class="metadata-row">
                      <strong>{mount.Name ?? mount.Source ?? "—"}</strong>
                      <span>{mount.Destination ?? "—"} · {mount.Type ?? "bind"} · {mount.Mode ?? "rw"}</span>
                    </div>
                  )}
                </For>
              </div>
            </details>
            <details>
              <summary>网络 <span>{Object.keys(props.inspect?.NetworkSettings?.Networks ?? {}).length}</span></summary>
              <div class="disclosure-body">
                <For each={Object.entries(props.inspect?.NetworkSettings?.Networks ?? {})} fallback={<p class="text-dim">无网络</p>}>
                  {([name, network]) => <div class="metadata-row"><strong>{name}</strong><span>{network.IPAddress || "—"}</span></div>}
                </For>
              </div>
            </details>
          </div>
        </Show>
      </div>
    </article>
  );
}
