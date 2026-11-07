/* 所有页面只消费服务端数据；错误原文保留，流式接口识别业务失败标记。 */
import { AuthAPI } from './auth';
import { readSSE, type SSEEvent } from './sse';
import { LogBuffer } from './log-buffer';
export function authHeaders(): HeadersInit {
  const token = AuthAPI.token();
  if (!token) throw new Error('会话已失效，请重新登录 / Session expired');
  return { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json' };
}
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch('/v1' + path, { ...init, headers: authHeaders() });
  let body: { code: number; message?: string; data: T };
  try { body = await response.json(); } catch { throw new Error(`HTTP ${response.status}: ${path} 响应不是有效 JSON`); }
  if (!response.ok || body.code !== 0) throw new Error(body.message || `HTTP ${response.status}: ${path}`);
  return body.data;
}
export async function events(path: string, onEvent: (event: SSEEvent) => void, signal: AbortSignal, init: RequestInit = {}): Promise<void> {
  const response = await fetch('/v1' + path, { ...init, headers: authHeaders(), signal });
  await readSSE(response, onEvent, signal);
}
export async function stream(path: string, onOutput: (text: string) => void, body?: unknown, signal?: AbortSignal): Promise<void> {
  let output = '', finished = false;
  const controller = new AbortController();
  await events(path, ({event, data}) => {
    if (event === 'output') { const text = String(data); output += text; onOutput(text); }
    if (event === 'error') throw new Error(new LogBuffer().push(output + '\n' + String(data)).join('\n').trim());
    if (event === 'done' && data === true) finished = true;
  }, signal || controller.signal, { method: 'POST', body: body === undefined ? undefined : JSON.stringify(body) });
  if (!finished) throw new Error('操作输出连接已中断，未收到完成确认 / Stream ended without completion');
}
export function terminalPath(name: string, type: string, extra: Record<string, string> = {}): string {
  return `/terminal/${encodeURIComponent(name)}/${type}?${new URLSearchParams(extra)}`;
}
