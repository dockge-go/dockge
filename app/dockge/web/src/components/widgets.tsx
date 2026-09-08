// 小型展示组件：状态徽标、空态、统计卡、标题区。
import { Show, type JSX } from "solid-js";
import { stateLabel, statusClass } from "../api/format";

export function StatusBadge(props: { state: string }) {
  return <span class={`status-badge ${statusClass(props.state)}`}>{stateLabel(props.state)}</span>;
}

/** 栈状态码：0 未知 / 1 未部署 / 2 已创建 / 3 运行中 / 4 已停止 */
export function StackStatusBadge(props: { status: number; label: string }) {
  const cls = () =>
    props.status === 3 ? "status-running" : props.status === 2 ? "status-paused" : "status-exited";
  return <span class={`status-badge ${cls()}`}>{props.label}</span>;
}

export function LiveDot() {
  return <span class="live-dot" title="Live" />;
}

export function EmptyState(props: { title: string; desc?: string; icon?: JSX.Element }) {
  return (
    <div class="empty-state">
      <Show when={props.icon}>
        <div style={{ opacity: 0.4, "margin-bottom": "16px" }}>{props.icon}</div>
      </Show>
      <div class="empty-state-title">{props.title}</div>
      <Show when={props.desc}>
        <div class="empty-state-desc">{props.desc}</div>
      </Show>
    </div>
  );
}

export function StatCard(props: {
  label: string;
  value: JSX.Element;
  tone?: "running" | "stopped" | "warning";
  live?: boolean;
  onClick?: () => void;
}) {
  return (
    <div class="stat-card" onClick={props.onClick} role={props.onClick ? "button" : undefined}>
      <div class="stat-card-label">
        {props.label}
        <Show when={props.live}>
          <LiveDot />
        </Show>
      </div>
      <div class={`stat-card-value ${props.tone ?? ""}`}>{props.value}</div>
    </div>
  );
}

export function SectionHeader(props: { title: JSX.Element; subtitle?: JSX.Element; actions?: JSX.Element }) {
  return (
    <div class="section-header">
      <div>
        <h2 class="section-title">{props.title}</h2>
        <Show when={props.subtitle}>
          <p class="section-subtitle">{props.subtitle}</p>
        </Show>
      </div>
      <Show when={props.actions}>
        <div class="section-actions">{props.actions}</div>
      </Show>
    </div>
  );
}

export function SpinnerBlock(props: { label?: string }) {
  return (
    <div class="loading-block">
      <span class="spinner" />
      {props.label ?? "加载中…"}
    </div>
  );
}
