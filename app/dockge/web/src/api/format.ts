// 通用格式化与展示辅助。

export function shortId(id: string): string {
  return (id || "").replace(/^sha256:/, "").slice(0, 12);
}

export function errText(e: unknown): string {
  if (e instanceof Error) return e.message;
  return String(e);
}

const STATE_LABELS: Record<string, string> = {
  running: "Running",
  exited: "Exited",
  paused: "Paused",
  created: "Created",
  dead: "Dead",
  restarting: "Restarting",
  removing: "Removing",
};

export function stateLabel(state: string): string {
  return STATE_LABELS[state] || state;
}

export function statusClass(state: string): string {
  if (state === "running") return "status-running";
  if (state === "paused") return "status-paused";
  return "status-exited";
}

export function timeAgo(ms: number): string {
  if (!ms) return "—";
  const s = Math.floor((Date.now() - ms) / 1000);
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
}

export function formatBytes(b: number): string {
  if (!b) return "0 B";
  const k = 1024;
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(b) / Math.log(k));
  return `${parseFloat((b / Math.pow(k, i)).toFixed(1))} ${units[i]}`;
}
