// 仪表盘：统计卡、容器状态占比、实时 CPU/内存（/docker/stats/stream SSE）、最近容器。
import { For, Show, createSignal, onCleanup, onMount } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { Plus, RefreshCw } from "lucide-solid";
import { getToken, type DockerStats } from "../api/api";
import { refresh, setPendingNewStack, snapshot } from "../store/index";
import { LiveDot, SectionHeader, SpinnerBlock, StatCard, StatusBadge } from "../components/widgets";
import { shortId } from "../api/format";
import { t } from "../i18n";

export function Dashboard() {
  const navigate = useNavigate();
  const [stats, setStats] = createSignal<DockerStats | null>(null);

  onMount(() => {
    const es = new EventSource(`/v1/docker/stats/stream?token=${encodeURIComponent(getToken())}`);
    es.onmessage = (ev) => {
      if (!ev.data) return;
      try {
        setStats(JSON.parse(ev.data) as DockerStats);
      } catch {
        // 忽略坏帧
      }
    };
    onCleanup(() => es.close());
  });

  const counts = () => {
    const cs = snapshot()?.containers ?? [];
    return {
      running: cs.filter((c) => c.state === "running").length,
      stopped: cs.filter((c) => c.state === "exited").length,
      paused: cs.filter((c) => c.state === "paused").length,
      total: cs.length,
    };
  };

  return (
    <div class="view-section">
      <Show when={snapshot()} fallback={<SpinnerBlock />}>
        <div class="stats-grid">
          <StatCard label={t("dash.labelRunning")} live tone="running" value={counts().running} onClick={() => navigate("/containers")} />
          <StatCard label={t("dash.labelStopped")} tone="stopped" value={counts().stopped} onClick={() => navigate("/containers")} />
          <StatCard label={t("dash.labelPaused")} tone="warning" value={counts().paused} onClick={() => navigate("/containers")} />
          <StatCard label={t("view.stacks")} value={snapshot()!.docker.stacksTotal} onClick={() => navigate("/stacks")} />
          <StatCard label={t("dash.labelImages")} value={snapshot()!.images.length} onClick={() => navigate("/images")} />
        </div>

        <div style={{ display: "flex", gap: "12px", "flex-wrap": "wrap", "margin-bottom": "24px" }}>
          <button
            class="btn-pill btn-pill-primary"
            onClick={() => {
              setPendingNewStack(true);
              navigate("/stacks");
            }}
          >
            <Plus size={14} />
            {t("stack.new")}
          </button>
          <button class="btn btn-secondary" onClick={() => void refresh()}>
            <RefreshCw size={14} />
            {t("common.refresh")}
          </button>
        </div>

        <div style={{ display: "grid", "grid-template-columns": "1fr 1fr", gap: "16px", "margin-bottom": "24px" }}>
          <div class="chart-card">
            <div class="chart-card-title">{t("dash.containerStatus")}</div>
            <Show
              when={counts().total > 0}
              fallback={<p class="text-dim" style={{ margin: "12px 0 0" }}>{t("dash.noContainers")}</p>}
            >
              <div class="bar-stacked" style={{ "margin-top": "12px" }}>
                <div
                  class="bar-seg running"
                  title={t("legend.running", { n: counts().running })}
                  style={{ width: `${(counts().running / counts().total) * 100}%` }}
                />
                <div
                  class="bar-seg paused"
                  title={t("legend.paused", { n: counts().paused })}
                  style={{ width: `${(counts().paused / counts().total) * 100}%` }}
                />
                <div
                  class="bar-seg exited"
                  title={t("legend.stopped", { n: counts().stopped })}
                  style={{ width: `${(counts().stopped / counts().total) * 100}%` }}
                />
              </div>
              <div class="chart-legend">
                <span class="chart-legend-item">
                  <span class="chart-legend-dot" style={{ background: "var(--success)" }} />
                  {t("legend.running", { n: counts().running })}
                </span>
                <span class="chart-legend-item">
                  <span class="chart-legend-dot" style={{ background: "var(--warn)" }} />
                  {t("legend.paused", { n: counts().paused })}
                </span>
                <span class="chart-legend-item">
                  <span class="chart-legend-dot" style={{ background: "var(--meta)" }} />
                  {t("legend.stopped", { n: counts().stopped })}
                </span>
              </div>
            </Show>
          </div>
          <div class="chart-card">
            <div class="chart-card-title">{t("dash.resourceUsage")}</div>
            <div style={{ "margin-top": "12px" }}>
              <Show
                when={!stats()?.error}
                fallback={
                  <p class="text-dim" style={{ "font-size": "var(--text-xs)", margin: "14px 0 0" }}>
                    {t("dash.statsUnavailable")}
                  </p>
                }
              >
                <div class="resource-row">
                  <span class="resource-label">{t("res.cpu")}</span>
                  <div class="resource-track">
                    <div class="resource-fill cpu" style={{ width: `${stats()?.cpuUsage ?? 0}%` }} />
                  </div>
                  <span class="resource-val">{(stats()?.cpuUsage ?? 0).toFixed(0)}%</span>
                </div>
                <div class="resource-row">
                  <span class="resource-label">{t("res.memory")}</span>
                  <div class="resource-track">
                    <div class="resource-fill mem" style={{ width: `${stats()?.memPercent ?? 0}%` }} />
                  </div>
                  <span class="resource-val">{(stats()?.memPercent ?? 0).toFixed(0)}%</span>
                </div>
                <p class="stat-detail">
                  {t("res.memDetail", { used: (stats()?.memUsage ?? 0).toFixed(0), total: ((stats()?.memTotalMB ?? 0) / 1024).toFixed(1) })}
                </p>
              </Show>
            </div>
          </div>
        </div>

        <SectionHeader
          title={
            <>
              {t("dash.recentContainers")} <LiveDot />
            </>
          }
          subtitle={t("res.subtitle", { total: counts().total, running: counts().running })}
          actions={
            <button class="btn btn-secondary" onClick={() => navigate("/containers")}>
              {t("common.viewAll")}
            </button>
          }
        />
        <table class="data-table">
          <thead>
            <tr>
              <th>{t("common.name")}</th>
              <th>{t("common.status")}</th>
              <th>{t("common.image")}</th>
              <th>{t("common.ports")}</th>
              <th>{t("common.uptime")}</th>
            </tr>
          </thead>
          <tbody>
            <For each={(snapshot()?.containers ?? []).slice(0, 5)}>
              {(c) => (
                <tr onClick={() => navigate("/containers")}>
                  <td>
                    <div class="cell-main">{c.name}</div>
                    <div class="cell-sub">{shortId(c.id)}</div>
                  </td>
                  <td>
                    <StatusBadge state={c.state} />
                    {c.state === "running" && <LiveDot />}
                  </td>
                  <td class="cell-dim">{c.image}</td>
                  <td class="cell-dim">{c.ports || "—"}</td>
                  <td class="cell-dim">{c.status || "—"}</td>
                </tr>
              )}
            </For>
          </tbody>
        </table>
      </Show>
    </div>
  );
}
