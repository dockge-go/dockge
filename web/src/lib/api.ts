/* 所有页面只消费服务端数据；错误原文保留，流式接口识别业务失败标记。 */
import { AuthAPI } from './auth';
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
export async function stream(path: string, onOutput: (text: string) => void, body?: unknown, signal?: AbortSignal): Promise<void> {
  const response = await fetch('/v1' + path, { method: 'POST', headers: authHeaders(), body: body === undefined ? undefined : JSON.stringify(body), signal });
  if (!response.ok) { const text = await response.text(); let message = text; try { message = JSON.parse(text).message || text; } catch { /* 保留运行时原文 */ } throw new Error(message || `HTTP ${response.status}: ${path}`); }
  if (!response.body) throw new Error(`响应缺少输出流: ${path}`);
  const reader = response.body.getReader(), decoder = new TextDecoder();
  let output = '';
  try {
    for (;;) {
      const { value, done } = await reader.read();
      const text = decoder.decode(value, { stream: !done });
      output += text; if (text) onOutput(text);
      if (done) break;
    }
  } finally { reader.releaseLock(); }
  const failed = /(?:^|\n)\[error\] ([\s\S]*)/.exec(output);
  if (failed) throw new Error(new LogBuffer().push(output).join('\n').trim() || failed[1].trim());
}
export function socketURL(name: string, type: string, extra: Record<string, string> = {}): string {
  const token = AuthAPI.token();
  if (!token) throw new Error('会话已失效，请重新登录 / Session expired');
  const query = new URLSearchParams({ token, ...extra });
  return `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/v1/terminal/${encodeURIComponent(name)}/${type}?${query}`;
}
