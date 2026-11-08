/* 日志按连续字节流解析，不按 SSE 帧切行；保留未换行末行，控制码不泄漏。 */
export interface LogLine { id: string; raw: string }
export class LogBuffer {
  private rows: LogLine[] = [];
  private nextId = 1;
  private characters = 0;
  private line = '';
  private cursor = 0;
  private state: 'text' | 'escape' | 'csi' | 'osc' | 'oscEscape' = 'text';
  private csi = '';
  push(text: string): void {
    for (const char of text) {
      if (this.state === 'osc') { if (char === '\x07') this.state = 'text'; else if (char === '\x1b') this.state = 'oscEscape'; continue; }
      if (this.state === 'oscEscape') { this.state = char === '\\' ? 'text' : 'osc'; continue; }
      if (this.state === 'escape') { this.state = char === '[' ? 'csi' : char === ']' ? 'osc' : 'text'; this.csi = ''; continue; }
      if (this.state === 'csi') {
        if (char >= '@' && char <= '~') { if (char === 'K') { if (this.csi === '2') this.line = ''; else this.line = this.line.slice(0, this.cursor); } this.state = 'text'; }
        else if (this.csi.length < 64) this.csi += char;
        continue;
      }
      if (char === '\x1b') { this.state = 'escape'; continue; }
      if (char === '\r') { this.cursor = 0; continue; }
      if (char === '\n') { this.rows.push({id: String(this.nextId++), raw: this.line}); this.characters += this.line.length + 1; this.line = ''; this.cursor = 0; this.trim(); continue; }
      if (char === '\b') { this.cursor = Math.max(0, this.cursor - 1); continue; }
      if (char < ' ' && char !== '\t') continue;
      if (this.line.length < 65536 || this.cursor < this.line.length) {
        this.line = this.cursor === this.line.length ? this.line + char : this.line.slice(0, this.cursor) + char + this.line.slice(this.cursor + char.length); this.cursor += char.length;
      }
    }
    this.trim();
  }
  // 行身份跨输出片段保持稳定；只在绘制时复制有界的显示窗口。
  snapshot(): LogLine[] {
    return [...this.rows, ...(this.line ? [{id: String(this.nextId), raw: this.line}] : [])];
  }
  private trim(): void {
    while (this.rows.length && (this.rows.length > (this.line ? 499 : 500) || this.characters + this.line.length > 1024 * 1024)) {
      this.characters -= this.rows.shift()!.raw.length + 1;
    }
  }
  clear() { this.rows = []; this.characters = 0; this.line = ''; this.cursor = 0; this.state = 'text'; this.csi = ''; }
}
