import { For, Show, createSignal } from "solid-js";
import { ArrowLeft, FileText, Play, RotateCw, Square, Terminal, Trash2 } from "lucide-solid";

import type { ContainerInspectData, ContainerRow } from "../api/api";
import { LogStream } from "./LogStream";
import { TerminalPane } from "./Terminal";
import { SpinnerBlock, StatusBadge } from "./widgets";
import { t } from "../i18n";

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
    <article class="detail-workspace" aria-label={t("aria.containerDetail", { name: props.container.name })}>
      <header class="detail-workspace-header">
        <button class="workspace-back" onClick={props.onBack}>
          <ArrowLeft size={16} /> {t("container.backToList")}
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
                <Play size={14} /> {t("act.start")}
              </button>
            }
          >
            <button class="btn btn-secondary" disabled={props.busy} onClick={() => props.onAction("stop")}>
              <Square size={14} /> {t("act.stop")}
            </button>
          </Show>
          <button class="btn btn-secondary" disabled={props.busy} onClick={() => props.onAction("restart")}>
            <RotateCw size={14} /> {t("act.restart")}
          </button>
          <button class="btn btn-danger" disabled={props.busy} onClick={props.onRemove}>
            <Trash2 size={14} /> {t("common.delete")}
          </button>
        </div>
      </header>

      <div class="detail-workspace-body">
        <section class="detail-summary" aria-label={t("aria.containerSummary")}>
          <div class="summary-item"><span>{t("container.image")}</span><strong>{props.inspect?.Config?.Image ?? props.container.image}</strong></div>
          <div class="summary-item"><span>{t("container.status")}</span><strong>{props.container.status || "—"}</strong></div>
          <div class="summary-item"><span>{t("container.ports")}</span><strong>{props.container.ports || t("common.none")}</strong></div>
          <div class="summary-item"><span>{t("container.stack")}</span><strong>{props.container.stack || "—"}</strong></div>
          <div class="summary-item"><span>{t("container.created")}</span><strong>{props.inspect?.Created ? new Date(props.inspect.Created).toLocaleString() : "—"}</strong></div>
          <div class="summary-item"><span>{t("container.ipAddress")}</span><strong>{props.inspect?.NetworkSettings?.IPAddress || "—"}</strong></div>
        </section>

        <section class="tool-panel">
          <div class="segmented" role="tablist" aria-label={t("aria.containerTools")}>
            <button role="tab" aria-selected={tool() === "logs"} classList={{ active: tool() === "logs" }} onClick={() => setTool("logs")}>
              <FileText size={14} /> {t("tab.logs")}
            </button>
            <button role="tab" aria-selected={tool() === "terminal"} classList={{ active: tool() === "terminal" }} onClick={() => setTool("terminal")}>
              <Terminal size={14} /> {t("tab.exec")}
            </button>
          </div>
          <Show when={tool() === "logs"} fallback={<TerminalPane name={props.container.id} type="exec" />}>
            <LogStream containerId={props.container.id} />
          </Show>
        </section>

        <Show when={props.inspect} fallback={<SpinnerBlock />}>
          <div class="disclosure-list">
            <details>
              <summary>{t("container.envVars")} <span>{props.inspect?.Config?.Env?.length ?? 0}</span></summary>
              <div class="disclosure-body mono-list">
                <For each={props.inspect?.Config?.Env ?? []} fallback={<p class="text-dim">{t("container.noEnv")}</p>}>
                  {(value) => <code>{value}</code>}
                </For>
              </div>
            </details>
            <details>
              <summary>{t("container.mounts")} <span>{props.inspect?.Mounts?.length ?? 0}</span></summary>
              <div class="disclosure-body">
                <For each={props.inspect?.Mounts ?? []} fallback={<p class="text-dim">{t("container.noMounts")}</p>}>
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
              <summary>{t("container.networks")} <span>{Object.keys(props.inspect?.NetworkSettings?.Networks ?? {}).length}</span></summary>
              <div class="disclosure-body">
                <For each={Object.entries(props.inspect?.NetworkSettings?.Networks ?? {})} fallback={<p class="text-dim">{t("container.noNetworks")}</p>}>
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
