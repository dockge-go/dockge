/* 会话结束时回收在途请求；旧响应即便晚到，也不能继续后续操作。 */
export class RequestScope {
  private controllers = new Set<AbortController>();

  async run<T>(work: (signal: AbortSignal) => Promise<T>, external?: AbortSignal | null): Promise<T> {
    external?.throwIfAborted();
    const controller = new AbortController();
    const abort = () => controller.abort(external?.reason);
    external?.addEventListener('abort', abort, { once: true });
    this.controllers.add(controller);
    try {
      const result = await work(controller.signal);
      controller.signal.throwIfAborted();
      return result;
    } finally {
      external?.removeEventListener('abort', abort);
      this.controllers.delete(controller);
    }
  }

  cancel() {
    const active = [...this.controllers];
    this.controllers.clear();
    active.forEach(controller => controller.abort());
  }
}

export const requests = new RequestScope();
