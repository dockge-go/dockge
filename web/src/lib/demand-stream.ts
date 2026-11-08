interface DemandStreamOptions {
  active: () => boolean;
  connect: (signal: AbortSignal) => Promise<void>;
  state: (state: 'connecting' | 'ended') => void;
  failed: (error: Error) => void;
  reconnectDelay?: number;
}

// 按需日志的连接与重连归同一面板所有；停止后旧回调不得复活连接。
export class DemandStream {
  private controller: AbortController | null = null;
  private timer: ReturnType<typeof setTimeout> | undefined;

  constructor(private options: DemandStreamOptions) {}

  start(): void {
    if (this.controller || this.timer !== undefined || !this.options.active()) return;
    const current = new AbortController();
    this.controller = current;
    this.options.state('connecting');
    let work: Promise<void>;
    try { work = this.options.connect(current.signal); }
    catch (error) { work = Promise.reject(error); }
    void work.catch(error => {
      if (this.controller === current && !current.signal.aborted && this.options.active()) {
        this.options.failed(error instanceof Error ? error : new Error(String(error)));
      }
    }).finally(() => {
      if (this.controller !== current) return;
      this.controller = null;
      current.abort();
      this.options.state('ended');
      if (this.options.active()) {
        this.timer = setTimeout(() => { this.timer = undefined; this.start(); }, this.options.reconnectDelay ?? 3000);
      }
    });
  }

  stop(): void {
    clearTimeout(this.timer); this.timer = undefined;
    const current = this.controller; this.controller = null;
    current?.abort();
  }
}
