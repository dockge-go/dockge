/* 返回最小替换范围，让键盘编辑进入浏览器原生撤销历史。 */
export interface EditorEdit { from: number; to: number; text: string; start: number; end: number }
export function indentEdit(value: string, start: number, end: number, reverse: boolean): EditorEdit {
  if (start === end && !reverse) return { from:start, to:end, text:'  ', start:start+2, end:start+2 };
  const from = value.lastIndexOf('\n', start - 1) + 1;
  const last = start === end ? end : end - 1;
  const next = value.indexOf('\n', last);
  const to = next < 0 ? value.length : next;
  const lines = value.slice(from, to).split('\n');
  let delta = 0, first = 0;
  const text = lines.map((line, i) => {
    const removed = reverse ? (/^(?: {1,2}|\t)/.exec(line)?.[0].length || 0) : 0;
    const change = reverse ? -removed : 2;
    delta += change;
    if (i === 0) first = reverse ? -Math.min(removed, start - from) : 2;
    return reverse ? line.slice(removed) : '  ' + line;
  }).join('\n');
  return { from, to, text, start:start+first, end: start === end ? start+first : end+delta };
}
