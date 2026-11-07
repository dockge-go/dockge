/* YAML 子集着色（逐行词法，覆盖 compose 文件）：
 * 输出 HTML，供编辑器覆盖层 <code innerHTML> 渲染；调用方负责文本域本身不可见。 */

function escHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function hlTail(t: string): string {
  if (!t) return '';
  const m = t.match(/^(\s+)([\s\S]*)$/);
  if (!m) return escHtml(t);
  const rest = m[2];
  if (!rest) return escHtml(m[1]);
  if (rest[0] === '#') return escHtml(m[1]) + '<span class="syn-comment">' + escHtml(rest) + '</span>';
  return escHtml(m[1]) + hlValue(rest);
}

function hlValue(rest: string): string {
  if (!rest) return '';
  const m = rest.match(/^("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')([\s\S]*)$/)
    || rest.match(/^(&\S+|\*\S+|!!?\S+|>[+-]?|[|][+-]?)([\s\S]*)$/);
  if (m) {
    const cls = /^["']/.test(m[1]) ? 'syn-str' : 'syn-key';
    return '<span class="' + cls + '">' + escHtml(m[1]) + '</span>' + hlTail(m[2]);
  }
  const w = rest.match(/^(\S+)([\s\S]*)$/);
  if (!w) return escHtml(rest);
  const isNumLike = /^(?:true|false|null|yes|no|on|off|-?\d+(?:\.\d+)?)$/i.test(w[1]);
  return (isNumLike ? '<span class="syn-num">' + escHtml(w[1]) + '</span>' : escHtml(w[1])) + hlTail(w[2]);
}

function hlLine(line: string): string {
  /* 注释切分：不在引号内的 # 视为注释起点 */
  let inS = false, inD = false, cut = -1;
  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (ch === "'" && !inD) inS = !inS;
    else if (ch === '"' && !inS) inD = !inD;
    else if (ch === '#' && !inS && !inD && (i === 0 || line[i - 1] === ' ' || line[i - 1] === '\t')) { cut = i; break; }
  }
  const code = cut >= 0 ? line.slice(0, cut) : line;
  const comment = cut >= 0 ? line.slice(cut) : '';

  const m = code.match(/^(\s*)((?:-\s+)?)([\s\S]*)$/) as [string, string, string, string] | null;
  let html = escHtml(m ? m[1] : '');
  if (m && m[2]) html += '<span class="syn-punct">' + escHtml(m[2]) + '</span>';
  const body = m ? m[3] : '';
  if (body) {
    /* key: value —— 冒号后是空格或行尾才算键 */
    const km = body.match(/^("[^"]*"|'[^']*'|[^:\s][^:]*?)(:(?:\s|$))([\s\S]*)$/);
    if (km) {
      html += '<span class="syn-key">' + escHtml(km[1]) + '</span>'
        + '<span class="syn-punct">' + escHtml(km[2]) + '</span>' + hlValue(km[3]);
    } else {
      html += hlValue(body);
    }
  }
  if (comment) html += '<span class="syn-comment">' + escHtml(comment) + '</span>';
  return html;
}

export function hlYaml(src: string): string {
  const lines = String(src == null ? '' : src).split('\n');
  let out = lines.map(l => hlLine(l) || '').join('\n');
  if (String(src || '').slice(-1) === '\n') out += '\n';
  return out;
}
