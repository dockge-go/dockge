// fetch SSE 允许 Authorization 头；增量拆帧保留跨块 UTF-8 与多行 data。
export interface SSEEvent { event: string; data: unknown }
export async function readSSE(response: Response, onEvent: (event: SSEEvent) => void | Promise<void>, signal?: AbortSignal): Promise<void> {
  if (!response.ok) {
    const text = await response.text(); let message = text;
    try { message = JSON.parse(text).message || text; } catch { /* 保留非 JSON 错误原文 */ }
    throw new Error(message || `HTTP ${response.status}`);
  }
  if (!response.headers.get('Content-Type')?.includes('text/event-stream')) throw new Error('响应不是 SSE / Expected text/event-stream');
  if (!response.body) throw new Error('响应缺少 SSE 输出流');
  const reader = response.body.getReader(), decoder = new TextDecoder();
  let pending = '', event = 'message', data: string[] = [], last = Date.now(), frameSize = 0;
  const idle = setInterval(() => { if (Date.now() - last > 30000) void reader.cancel('SSE 超过 30 秒未收到数据'); }, 5000);
  const abort = () => { void reader.cancel(); };
  signal?.addEventListener('abort', abort, { once: true });
  try {
    if (signal?.aborted) throw signal.reason || new Error('SSE 已取消');
    for (;;) {
      const { value, done } = await reader.read();
      if (signal?.aborted) throw signal.reason || new Error('SSE 已取消');
      if (Date.now() - last > 30000) throw new Error('SSE 超过 30 秒未收到数据');
      if (value) last = Date.now();
      pending += decoder.decode(value, { stream: !done });
      let end: number;
      while ((end = pending.indexOf('\n')) >= 0) {
        const line = pending.slice(0, end).replace(/\r$/, ''); pending = pending.slice(end + 1);
        if (!line) {
          if (data.length) await onEvent({ event, data: JSON.parse(data.join('\n')) });
          if (signal?.aborted) throw signal.reason || new Error('SSE 已取消');
          event = 'message'; data = []; frameSize = 0;
        } else if (line.startsWith('event:')) event = line.slice(6).trim();
        else if (line.startsWith('data:')) { frameSize += line.length; if (frameSize > 16 * 1024 * 1024) throw new Error('SSE 帧超过 16 MiB，连接已关闭'); data.push(line.slice(5).replace(/^ /, '')); }
      }
      if (pending.length > 16 * 1024 * 1024) throw new Error('SSE 帧超过 16 MiB，连接已关闭');
      if (done) { if (pending || data.length) throw new Error('SSE 帧不完整，连接已中断'); return; }
    }
  } finally { clearInterval(idle); signal?.removeEventListener('abort', abort); await reader.cancel().catch(() => {}); reader.releaseLock(); }
}
export function eventBytes(data: unknown): Uint8Array {
  if (typeof data !== 'string') throw new Error('SSE bytes 不是 base64 字符串');
  return Uint8Array.from(atob(data), c => c.charCodeAt(0));
}
