// 仪表盘：统计卡、容器状态占比、实时 CPU/内存（/docker/stats/stream SSE）、最近容器。
import { For, Show, createSignal, onCleanup, onMount } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { Plus, RefreshCw } from "lucide-solid";
import { getToken, type DockerStats } from "../api/api";
import { refresh, setPendingNewStack, snapshot } from "../store/index";
import { LiveDot, SectionHeader, SpinnerBlock, StatCard, StatusBadge } from "../components/widgets";
import { shortId } from "../api/format";

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
          <StatCard label="Running" live tone="running" value={counts().running} onClick={() => navigate("/containers")} />
          <StatCard label="Stopped" tone="stopped" value={counts().stopped} onClick={() => navigate("/containers")} />
          <StatCard label="Paused" tone="warning" value={counts().paused} onClick={() => navigate("/containers")} />
          <StatCard label="Stacks" value={snapshot()!.docker.stacksTotal} onClick={() => navigate("/stacks")} />
          <StatCard label="Images" value={snapshot()!.images.length} onClick={() => navigate("/images")} />
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
            New Stack
          </button>
          <button class="btn btn-secondary" onClick={() => void refresh()}>
            <RefreshCw size={14} />
            Refresh
          </button>
        </div>

        <div style={{ display: "grid", "grid-template-columns": "1fr 1fr", gap: "16px", "margin-bottom": "24px" }}>
          <div class="chart-card">
            <div class="chart-card-title">Container Status</div>
            <Show
              when={counts().total > 0}
              fallback={<p class="text-dim" style={{ margin: "12px 0 0" }}>暂无容器</p>}
            >
              <div class="bar-stacked" style={{ "margin-top": "12px" }}>
                <div
                  class="bar-seg running"
                  title={`${counts().running} running`}
                  style={{ width: `${(counts().running / counts().total) * 100}%` }}
                />
                <div
                  class="bar-seg paused"
                  title={`${counts().paused} paused`}
                  style={{ width: `${(counts().paused / counts().total) * 100}%` }}
                />
                <div
                  class="bar-seg exited"
                  title={`${counts().stopped} stopped`}
                  style={{ width: `${(counts().stopped / counts().total) * 100}%` }}
                />
              </div>
              <div class="chart-legend">
                <span class="chart-legend-item">
                  <span class="chart-legend-dot" style={{ background: "var(--success)" }} />
                  {counts().running} Running
                </span>
                <span class="chart-legend-item">
                  <span class="chart-legend-dot" style={{ background: "var(--warn)" }} />
                  {counts().paused} Paused
                </span>
                <span class="chart-legend-item">
                  <span class="chart-legend-dot" style={{ background: "var(--meta)" }} />
                  {counts().stopped} Stopped
                </span>
              </div>
            </Show>
          </div>
          <div class="chart-card">
            <div class="chart-card-title">Resource Usage</div>
            <div style={{ "margin-top": "12px" }}>
              <div class="resource-row">
                <span class="resource-label">CPU</span>
                <div class="resource-track">
                  <div class="resource-fill cpu" style={{ width: `${stats()?.cpuUsage ?? 0}%` }} />
                </div>
                <span class="resource-val">{(stats()?.cpuUsage ?? 0).toFixed(0)}%</span>
              </div>
              <div class="resource-row">
                <span class="resource-label">Memory</span>
                <div class="resource-track">
                  <div class="resource-fill mem" style={{ width: `${stats()?.memPercent ?? 0}%` }} />
                </div>
                <span class="resource-val">{(stats()?.memPercent ?? 0).toFixed(0)}%</span>
              </div>
              <p class="stat-detail">
                内存 {(stats()?.memUsage ?? 0).toFixed(0)} MB / {((stats()?.memTotalMB ?? 0) / 1024).toFixed(1)} GB
              </p>
            </div>
          </div>
        </div>

        <SectionHeader
          title={
            <>
              Recent Containers <LiveDot />
            </>
          }
          subtitle={`${counts().total} total · ${counts().running} running`}
          actions={
            <button class="btn btn-secondary" onClick={() => navigate("/containers")}>
              View All
            </button>
          }
        />
        <table class="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Status</th>
              <th>Image</th>
              <th>Ports</th>
              <th>Uptime</th>
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
