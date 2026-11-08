const MAX_CHARACTERS = 1024 * 1024;

// 只保留终端显示尾部；ANSI/回车原样交给 xterm，不当作普通日志清洗。
export class TerminalBuffer {
  private output = '';

  push(text: string): string[] {
    this.output += text;
    let minimum = Math.max(0, this.output.length - MAX_CHARACTERS);
    let newline = this.output.length;
    for (let row = 0; row < 500; row++) {
      newline = this.output.lastIndexOf('\n', newline - 1);
      if (newline < 0) break;
      if (row === 499) minimum = Math.max(minimum, newline + 1);
      if (newline === 0) break;
    }
    if (minimum > 0) this.output = terminalTail(this.output, minimum);
    return this.output.split('\n');
  }
}

// 截断不得从半个 Unicode 字符或 ANSI 控制序列中间开始。
function terminalTail(text: string, minimum: number): string {
  let offset = 0;
  while (offset < minimum) {
    if (text[offset] !== '\x1b') { offset += (text.codePointAt(offset) ?? 0) > 0xffff ? 2 : 1; continue; }
    offset++;
    if (text[offset] === '[') {
      offset++;
      while (offset < text.length && !(text[offset] >= '@' && text[offset] <= '~')) offset++;
      if (offset < text.length) offset++;
    } else if (text[offset] === ']') {
      offset++;
      while (offset < text.length) {
        if (text[offset++] === '\x07') break;
        if (text[offset - 1] === '\x1b' && text[offset] === '\\') { offset++; break; }
      }
    } else {
      while (offset < text.length && text[offset] >= ' ' && text[offset] <= '/') offset++;
      if (offset < text.length) offset++;
    }
  }
  return text.slice(offset);
}

type Write = (data: string | Uint8Array, done: () => void) => void;

// 同一时刻只写一次，其间到达的显示状态覆盖待绘制值，不排队保存全部版本。
export class TerminalRenderer {
  private written = '';
  private latest: string | null = null;
  private writing = false;
  private stopped = false;

  constructor(private write: Write, private reset: () => void) {}

  push(output: string): void {
    if (this.stopped) return;
    this.latest = output;
    this.flush();
  }

  stop(): void { this.stopped = true; this.latest = null; this.written = ''; }

  private flush(): void {
    if (this.stopped || this.writing || this.latest === null) return;
    const output = this.latest;
    this.latest = null;
    if (!output.startsWith(this.written)) { this.reset(); this.written = ''; }
    const next = output.slice(this.written.length);
    this.written = output;
    if (!next) return;
    this.writing = true;
    this.write(next, () => { this.writing = false; this.flush(); });
  }
}

// SSE 等待 xterm 消费当前帧，避免高速容器输出形成无界渲染队列。
export function writeTerminal(write: Write, data: Uint8Array, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const cancel = () => { signal.removeEventListener('abort', cancel); reject(signal.reason || new Error('终端输出已取消')); };
    if (signal.aborted) { cancel(); return; }
    signal.addEventListener('abort', cancel, {once:true});
    try {
      write(data, () => { signal.removeEventListener('abort', cancel); resolve(); });
    } catch (error) {
      signal.removeEventListener('abort', cancel);
      reject(error);
    }
  });
}
