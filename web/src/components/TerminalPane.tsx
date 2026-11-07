import { createEffect, createSignal, onCleanup, onMount, Show } from 'solid-js';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { socketURL } from '../lib/api';
import { t } from '../i18n';

/* 与上游相同的终端仿真：直接处理 ANSI、UTF-8 字节、键盘/粘贴和 PTY 尺寸。 */
export default function TerminalPane(props: { output?: string; containerID?: string; compact?: boolean }) {
  let host!: HTMLDivElement;
  let terminal: Terminal | undefined, fit: FitAddon | undefined;
  let socket: WebSocket | undefined, resize: ResizeObserver | undefined, themeObserver: MutationObserver | undefined;
  let written = '', disposed = false;
  const [shell, setShell] = createSignal('sh');
  const [attempt, setAttempt] = createSignal(0);
  const [connection, setConnection] = createSignal('connecting');
  const sendSize = () => {
    if (socket?.readyState === WebSocket.OPEN && terminal) socket.send(JSON.stringify({ type: 'resize', data: { cols: Math.min(500, terminal.cols), rows: Math.min(500, terminal.rows) } }));
  };
  const fitSize = () => { if (!disposed && host.clientWidth > 0 && host.clientHeight > 0) { fit?.fit(); sendSize(); } };
  const applyColors = () => {
    if (!terminal) return;
    const style = getComputedStyle(host), color = (name: string) => style.getPropertyValue(name).trim();
    terminal.options.theme = {
      background: color('--panel-2'), foreground: color('--fg'), cursor: color('--accent'),
      cursorAccent: color('--panel-2'), selectionBackground: color('--accent-soft'),
      black: color('--fg'), brightBlack: color('--muted'), red: color('--danger'), brightRed: color('--danger'),
      green: color('--terminal-green'), brightGreen: color('--terminal-green'), yellow: color('--terminal-yellow'), brightYellow: color('--terminal-yellow'),
      blue: color('--accent'), brightBlue: color('--accent'), magenta: color('--syntax-string'), brightMagenta: color('--syntax-string'),
      cyan: color('--syntax-key'), brightCyan: color('--syntax-key'), white: color('--fg-2'), brightWhite: color('--fg')
    };
  };
  onMount(() => {
    terminal = new Terminal({ fontFamily: getComputedStyle(host).getPropertyValue('--font-mono'), fontSize: 13, cursorBlink: !!props.containerID,
      convertEol: true, scrollback: 5000, screenReaderMode: true, disableStdin: !props.containerID });
    fit = new FitAddon(); terminal.loadAddon(fit); terminal.open(host); applyColors();
    terminal.textarea?.setAttribute('aria-label', t('term.input'));
    resize = new ResizeObserver(fitSize); resize.observe(host); fitSize();
    themeObserver = new MutationObserver(applyColors); themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    terminal.onResize(sendSize);
    terminal.onData(data => { if (socket?.readyState === WebSocket.OPEN) socket.send(data); });
    createEffect(() => {
      const output = props.output || '';
      if (props.containerID || !terminal) return;
      if (!output.startsWith(written)) { terminal.reset(); written = ''; }
      const next = output.slice(written.length); written = output;
      if (next) terminal.write(next);
    });
    createEffect(() => {
      const id = props.containerID, selectedShell = shell(); attempt();
      if (!id || !terminal) return;
      socket?.close(); terminal.reset(); setConnection('connecting');
      try {
        const ws = new WebSocket(socketURL(id, 'container-exec', { shell: selectedShell })); socket = ws; ws.binaryType = 'arraybuffer';
        ws.onopen = () => { if (socket === ws && !disposed) { setConnection('live'); fitSize(); terminal?.focus(); } };
        ws.onmessage = e => { if (socket === ws && !disposed) terminal?.write(e.data instanceof ArrayBuffer ? new Uint8Array(e.data) : String(e.data)); };
        ws.onerror = () => { if (socket === ws && !disposed) { setConnection('ended'); terminal?.writeln('\r\n' + t('term.connectionFailed')); } };
        ws.onclose = () => { if (socket === ws && !disposed) { setConnection('ended'); terminal?.writeln('\r\n' + t('log.ended')); } };
      } catch (e) { setConnection('ended'); terminal.writeln((e as Error).message); }
    });
  });
  onCleanup(() => { disposed = true; socket?.close(); resize?.disconnect(); themeObserver?.disconnect(); terminal?.dispose(); });
  return <div class="terminal-pane" classList={{ compact: props.compact, interactive: !!props.containerID }}>
    <Show when={props.containerID}>
      <div class="terminal-toolbar">
        <label class="form-label">Shell <select class="form-select" value={shell()} onChange={e => setShell(e.currentTarget.value)}><option value="sh">sh</option><option value="bash">bash</option></select></label>
        <span class="cell-meta" role="status">{t('log.' + connection())}</span>
        <button type="button" class="btn btn-secondary" disabled={connection() !== 'ended'} onClick={() => setAttempt(n => n + 1)}>{t('term.reconnect')}</button>
      </div>
    </Show>
    <div class="terminal-surface" ref={host} role="region" aria-label={props.containerID ? t('term.input') : t('term.output')} />
  </div>;
}
