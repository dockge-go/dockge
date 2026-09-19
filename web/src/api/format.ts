// 通用格式化与展示辅助。
export function errText(e: unknown): string {
  if (e instanceof Error) return e.message;
  return String(e);
}

export function statusClass(state: string): string {
  if (state === "running") return "status-running";
  if (state === "paused") return "status-paused";
  return "status-exited";
}
