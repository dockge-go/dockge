// 通用格式化与展示辅助。
export function shortId(id: string): string {
  return (id || "").replace(/^sha256:/, "").slice(0, 12);
}

export function errText(e: unknown): string {
  if (e instanceof Error) return e.message;
  return String(e);
}

export function statusClass(state: string): string {
  if (state === "running") return "status-running";
  if (state === "paused") return "status-paused";
  return "status-exited";
}

const SIZE_UNITS = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];

export function fmtBytes(bytes: number): string {
  if (!bytes || bytes < 0) return "0B";
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), SIZE_UNITS.length - 1);
  const value = bytes / 1024 ** i;
  return `${value >= 100 ? value.toFixed(0) : value >= 10 ? value.toFixed(1) : value.toFixed(2)} ${SIZE_UNITS[i]}`;
}

export function fmtDate(unix: number): string {
  if (!unix) return "";
  return new Date(unix * 1000).toISOString().slice(0, 10);
}
