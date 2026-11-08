export interface LogRecord { text: string; time: string }
// 时间游标取保留窗口最早时间；重连按记录出现次数去重，重复文案不会被误吞。
export class LogReplay {
  private history: LogRecord[] = [];
  private characters = 0;
  private replay = new Map<string,number>();
  private seen = new Map<string,number>();
  begin(): string | undefined {
    this.replay.clear(); this.seen.clear();
    for (const record of this.history) {
      const key = JSON.stringify(record);
      this.replay.set(key,(this.replay.get(key) || 0)+1);
    }
    const times = this.history.map(r => r.time).filter(Boolean).sort();
    return times[0];
  }
  push(record: LogRecord): string {
    const key = JSON.stringify(record), count = (this.seen.get(key) || 0)+1;
    this.seen.set(key,count);
    if (record.time && count <= (this.replay.get(key) || 0)) return '';
    this.history.push(record);
    this.characters += record.text.length + record.time.length;
    while (this.history.length && (this.history.length > 500 || this.characters > 1024 * 1024)) {
      const expired = this.history.shift()!;
      this.characters -= expired.text.length + expired.time.length;
    }
    // seen 仅需保留正在重放的键，实时日志不应无限增长。
    if (!this.replay.has(key)) this.seen.delete(key);
    return record.text;
  }
}
