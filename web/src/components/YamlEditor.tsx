/* YAML 编辑器（编排栈核心）
 *  - 高亮覆盖层：透明 textarea 叠在着色 <pre> 上，字体度量一致，滚动双同步
 *  - 行号槽与内容区滚动同步；状态栏 Ln/Col · 行数 · 语法状态
 *  - Tab 两空格；Enter 继承缩进（键值行 +2、列表续行）；输入法合成中不拦截
 *  - js-yaml 实时校验：问题面板（行号 + 消息，点击跳转）
 *  - 格式化（确认提示会移除注释）；插入服务骨架（自动定位 services 块） */
import { Show, createMemo, createSignal } from 'solid-js';
import { app, setApp, selectedStackObj, confirmDlg, toast, t } from '../store';
import { hlYaml } from '../lib/highlight';
import { yamlProblem, parseServices, normalizeYaml } from '../lib/yaml';

export default function YamlEditor() {
  let ta: HTMLTextAreaElement | undefined;
  let pre: HTMLPreElement | undefined;
  let gutter: HTMLDivElement | undefined;
  const [cursorLn, setCursorLn] = createSignal(1);
  const [cursorCol, setCursorCol] = createSignal(1);

  const stack = selectedStackObj;
  const code = createMemo(() => stack()?.compose || '');
  const setCode = (v: string) => {
    const s = stack();
    if (s) setApp('stacks', x => x.id === s.id, 'compose', v);
  };
  const highlighted = createMemo(() => hlYaml(code()));
  const lineCount = createMemo(() => code().split('\n').length);
  const gutterText = createMemo(() => {
    const out: string[] = [];
    for (let i = 1; i <= lineCount(); i++) out.push(String(i));
    return out.join('\n');
  });
  const problem = createMemo(() => yamlProblem(code()));
  const valid = createMemo(() => problem() === null);

  const updateCursor = () => {
    if (!ta) return;
    const before = ta.value.slice(0, ta.selectionStart || 0);
    const lines = before.split('\n');
    setCursorLn(lines.length);
    setCursorCol(lines[lines.length - 1].length + 1);
  };
  const onScroll = () => {
    if (!ta) return;
    if (pre) { pre.scrollTop = ta.scrollTop; pre.scrollLeft = ta.scrollLeft; }
    if (gutter) gutter.scrollTop = ta.scrollTop;
  };
  const onKeydown = (e: KeyboardEvent) => {
    if (e.isComposing || !ta) return;
    if (e.key === 'Tab') {
      e.preventDefault();
      insertAtCaret('  ');
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const el = ta;
      const before = el.value.slice(0, el.selectionStart);
      const after = el.value.slice(el.selectionEnd);
      const curLine = before.slice(before.lastIndexOf('\n') + 1);
      let indent = (/^\s*/.exec(curLine) || [''])[0];
      if (/:\s*$/.test(curLine)) indent += '  ';
      else if (/^\s*-\s+\S/.test(curLine) && !/:\s/.test(curLine)) indent = (/^\s*-\s+/.exec(curLine) || [''])[0];
      const pos = el.selectionStart + ('\n' + indent).length;
      el.value = before + '\n' + indent + after;
      el.setSelectionRange(pos, pos);
      setCode(el.value);
      updateCursor();
    }
  };
  const insertAtCaret = (text: string) => {
    if (!ta) return;
    const before = ta.value.slice(0, ta.selectionStart);
    const after = ta.value.slice(ta.selectionEnd);
    const pos = ta.selectionStart + text.length;
    ta.value = before + text + after;
    ta.setSelectionRange(pos, pos);
    setCode(ta.value);
    updateCursor();
  };
  const jumpTo = (line: number | null) => {
    if (!line || !ta) return;
    const lines = code().split('\n');
    let pos = 0;
    for (let i = 0; i < Math.min(line - 1, lines.length); i++) pos += lines[i].length + 1;
    ta.focus();
    ta.setSelectionRange(pos, pos + (lines[line - 1]?.length || 0));
    const lh = ta.scrollHeight / Math.max(1, lineCount());
    ta.scrollTop = Math.max(0, (line - 4) * lh);
    onScroll();
    updateCursor();
  };
  const validate = () => {
    if (valid()) { toast(t('toast.yamlValid'), 'success'); return; }
    toast(t('toast.yamlIssue'), 'error');
    if (problem()?.line) jumpTo(problem()!.line);
  };
  const format = () => {
    if (!valid()) { toast(t('editor.fixFirst'), 'error'); return; }
    confirmDlg(t('editor.formatConfirm'), () => {
      try {
        setCode(normalizeYaml(code()));
        toast(t('toast.formatApplied'), 'success');
      } catch (e) {
        toast(t('toast.yamlIssue'), 'info');
      }
    });
  };
  const insertService = () => {
    const s = stack();
    if (!s) return;
    const lines = code().split('\n');
    let inSvc = false, at = lines.length;
    for (let i = 0; i < lines.length; i++) {
      if (/^services:\s*$/.test(lines[i])) { inSvc = true; continue; }
      if (inSvc && /^\S/.test(lines[i])) { at = i; break; }
    }
    const existing: Record<string, 1> = {};
    parseServices(s.compose || '').forEach(x => { existing[x] = 1; });
    let name = 'svc', n = 1;
    while (existing[name]) { n++; name = 'svc' + n; }
    lines.splice(at, 0, '  ' + name + ':', '    image: nginx:latest', '    restart: unless-stopped');
    setCode(lines.join('\n'));
    toast(t('editor.serviceInserted', { name }), 'success');
  };

  return (
    <div class="wb-editor">
      <div classList={{ 'editor-fullscreen': app.editorFullscreen }} style="display:flex;flex-direction:column;flex:1;min-height:0">
        <div class="editor-toolbar">
          <span class="editor-status" classList={{ 'is-ok': valid(), 'is-bad': !valid() }}>
            <span>{valid() ? '✓' : '✕'}</span>
            <span>{valid() ? t('editor.valid') : t('editor.invalid')}</span>
          </span>
          <span class="toolbar-sep" />
          <button type="button" class="btn btn-ghost" onClick={validate}>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="20 6 9 17 4 12" /></svg>
            {t('stack.validate')}
          </button>
          <button type="button" class="btn btn-ghost" onClick={format}>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="21" y1="10" x2="3" y2="10" /><line x1="21" y1="6" x2="3" y2="6" /><line x1="21" y1="14" x2="3" y2="14" /><line x1="21" y1="18" x2="3" y2="18" /></svg>
            {t('stack.format')}
          </button>
          <button type="button" class="btn btn-ghost" onClick={insertService}>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
            {t('editor.insertService')}
          </button>
          <button type="button" class="btn btn-ghost" onClick={() => setApp('editorFullscreen', !app.editorFullscreen)}>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="15 3 21 3 21 9" /><polyline points="9 21 3 21 3 15" /><line x1="21" y1="3" x2="14" y2="10" /><line x1="3" y1="21" x2="10" y2="14" /></svg>
            {t('stack.fullscreen')}
          </button>
        </div>

        <Show when={!valid()}>
          <div class="editor-problems">
            <button type="button" class="editor-problem" onClick={() => jumpTo(problem()!.line)}>
              <span class="ep-mark">✕</span>
              <span class="ep-line">{problem()?.line ? t('editor.lineN', { n: problem()!.line! }) : ''}</span>
              <span class="ep-msg">{problem()?.msg}</span>
            </button>
          </div>
        </Show>

        <div class="editor-surface">
          <div class="editor-lines editor-metric" ref={gutter}>{gutterText()}</div>
          <div class="editor-code">
            <pre class="editor-hl editor-metric" ref={pre} aria-hidden="true"><code innerHTML={highlighted()} /></pre>
            <textarea class="editor-input editor-metric" ref={ta} spellcheck={false} wrap="off"
                      value={code()}
                      onInput={e => { setCode(e.currentTarget.value); updateCursor(); }}
                      onScroll={onScroll}
                      onClick={updateCursor}
                      onKeyUp={updateCursor}
                      onKeyDown={onKeydown} />
          </div>
        </div>

        <div class="editor-statusbar">
          <span>{t('editor.pos', { ln: cursorLn(), col: cursorCol() })}</span>
          <span>{t('editor.lineCount', { n: lineCount() })}</span>
          <span>YAML</span>
          <span>UTF-8</span>
          <span class="sb-right" classList={{ 'is-ok': valid(), 'is-bad': !valid() }}>
            {valid() ? t('editor.valid') : t('editor.invalid')}
          </span>
        </div>
      </div>
    </div>
  );
}
