import type { SSEEvent } from './sse';

interface LiveStateOptions {
  active: () => boolean;
  connect: (receive: (event: SSEEvent) => void, signal: AbortSignal) => Promise<void>;
  requestRefresh: (signal: AbortSignal) => Promise<{ revision: number }>;
  receive: (event: SSEEvent) => void;
  disconnected: (error?: Error) => void;
  reconnectDelay?: number;
}

// 一个页面只拥有一条状态连接；刷新只请求新采样，不保存快照或事件历史。
export class LiveState {
  private controller: AbortController | null = null;
  private timer: ReturnType<typeof setTimeout> | undefined;
  private pending: Promise<void> | null = null;
  private resolve: (() => void) | null = null;
  private revision: number | null = 0;
  private lastRevision = 0;
  private refreshRequest = 0;

  constructor(private options: LiveStateOptions) {}

  refresh(force = false): Promise<void> {
    if (!this.options.active()) return Promise.resolve();
    if (this.pending && !force) return this.pending;
    if (!this.pending) this.pending = new Promise<void>(resolve => { this.resolve = resolve; });
    const pending = this.pending;
    clearTimeout(this.timer);
    if (!this.controller) {
      const controller = new AbortController();
      this.controller = controller;
      this.revision = this.lastRevision = 0;
      try {
        void this.options.connect(event => this.receive(controller, event), controller.signal)
          .then(() => this.disconnect(controller), error => this.disconnect(controller, asError(error)));
      } catch (error) {
        this.disconnect(controller, asError(error));
      }
    } else {
      const controller = this.controller;
      // 刷新确认前不接受快照，避免把操作前的在途结果当作刷新成功。
      this.revision = null;
      const request = ++this.refreshRequest;
      let response: Promise<{ revision: number }>;
      try { response = this.options.requestRefresh(controller.signal); }
      catch (error) { response = Promise.reject(error); }
      void response.then(result => {
        if (this.controller !== controller || request !== this.refreshRequest) return;
        if (!result || !Number.isSafeInteger(result.revision) || result.revision < 0) {
          this.disconnect(controller, new Error('SSE 刷新确认缺少有效 revision，无法确认数据新鲜度'));
          return;
        }
        this.revision = Math.max(this.lastRevision, result.revision);
      }).catch(error => {
        if (this.controller !== controller || request !== this.refreshRequest) return;
        this.revision = this.lastRevision;
        this.options.disconnected(asError(error));
        this.settle();
      });
    }
    return pending;
  }

  stop(): void {
    clearTimeout(this.timer);
    const controller = this.controller;
    this.controller = null;
    controller?.abort();
    this.revision = this.lastRevision = 0;
    this.settle();
  }

  private receive(controller: AbortController, event: SSEEvent): void {
    if (this.controller !== controller) return;
    if (event.event === 'snapshot') {
      const revision = (event.data as { revision?: number } | null)?.revision;
      if (typeof revision !== 'number' || !Number.isSafeInteger(revision) || revision < 0) {
        throw new Error('SSE snapshot 缺少有效 revision，无法确认数据新鲜度');
      }
      if (this.revision === null || revision < Math.max(this.revision, this.lastRevision)) return;
      this.options.receive(event);
      this.lastRevision = revision;
      this.settle();
    } else {
      this.options.receive(event);
    }
  }

  private disconnect(controller: AbortController, error?: Error): void {
    if (this.controller !== controller) return;
    this.controller = null;
    controller.abort();
    this.settle();
    this.options.disconnected(error);
    if (this.options.active()) {
      this.timer = setTimeout(() => { void this.refresh(); }, this.options.reconnectDelay ?? 3000);
    }
  }

  private settle(): void {
    this.resolve?.();
    this.resolve = null;
    this.pending = null;
  }
}

function asError(error: unknown): Error {
  return error instanceof Error ? error : new Error(String(error));
}
