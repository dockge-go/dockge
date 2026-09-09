// 小型展示组件：状态徽标、空态、统计卡、标题区。
import { Show, type JSX } from "solid-js";
import { stateLabel, statusClass } from "../api/format";
import { t } from "../i18n";

export function StatusBadge(props: { state: string }) {
  return <span class={`status-badge ${statusClass(props.state)}`}>{stateLabel(props.state)}</span>;
}

/** 栈状态码：0 未知 / 1 未部署 / 2 已创建 / 3 运行中 / 4 已停止（与原版 Dockge 语义一致）。
 *  徽标文案按状态码本地化；后端 statusLabel 仅在出现未知码时兜底。 */
const STACK_STATUS_KEY = ["stack.statusUnknown", "stack.notDeployed", "stack.statusCreated", "stack.statusRunning", "stack.statusStopped"] as const;

export function StackStatusBadge(props: { status: number; label: string }) {
  const cls = () =>
    props.status === 3 ? "status-running" : props.status === 2 ? "status-paused" : "status-exited";
  const label = () => {
    const key = STACK_STATUS_KEY[props.status];
    return key ? t(key) : props.label;
  };
  return <span class={`status-badge ${cls()}`}>{label()}</span>;
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
      {props.label ?? t("common.loading")}
    </div>
  );
}
