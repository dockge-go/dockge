// 原始输出逐段消费，界面每帧只绘制最新状态；关闭或清空时撤销待绘制工作。
export class FrameUpdate {
  private frame: number | null = null;
  constructor(
    private render: () => void,
    private request: (callback: FrameRequestCallback) => number = callback => requestAnimationFrame(callback),
    private abort: (id: number) => void = id => cancelAnimationFrame(id),
  ) {}
  schedule(): void {
    if (this.frame !== null) return;
    this.frame = this.request(() => { this.frame = null; this.render(); });
  }
  flush(): void {
    if (this.frame === null) return;
    this.cancel(); this.render();
  }
  cancel(): void {
    if (this.frame === null) return;
    this.abort(this.frame); this.frame = null;
  }
}
