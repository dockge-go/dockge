// xterm.js 终端面板：连接 WebSocket 终端流。
// - /v1/terminal/{name}/{type}（exec 容器 shell / compose-logs 栈日志）
// - /v1/console/terminal（宿主 shell，type="console"）
// resize 以 JSON 控制消息上报；compose-logs 只读不回传输入。
// DisplayTerminal 为纯展示实例（操作进度输出等，无连接、无输入）。
import { createEffect, onCleanup, onMount } from "solid-js";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { getToken } from "../api/api";
import { t } from "../i18n";

const MONO = '"JetBrains Mono", ui-monospace, "SF Mono", Menlo, Consolas, monospace';

function createTerm(rows: number, blink = true) {
  return new Terminal({
    fontSize: 14,
    cursorBlink: blink,
    fontFamily: MONO,
    theme: { background: "#0d1117", foreground: "#e6edf3" },
    rows,
  });
}

/** 只读日志终端的额外选项：
 *  - disableStdin：日志流不接受输入，显式禁用避免「敲键盘没反应」的误解；
 *  - convertEol：日志经普通管道输出（仅 LF），xterm 需据此把光标回到行首，
 *    否则会出现「阶梯状错位」（PTY 模式由终端驱动补 CR，无需此项）。
 */
const READ_ONLY_OPTIONS = { disableStdin: true, cursorBlink: false, convertEol: true } as const;

export function TerminalPane(props: {
  name: string;
  type: "exec" | "compose-logs" | "console";
  /** 连接状态回调：live=流已建立，ended=会话结束（供日志卡显示实时指示）。 */
  onState?: (state: "live" | "ended") => void;
}) {
  let host!: HTMLDivElement;
  let term: Terminal;
  let ws: WebSocket | null = null;

  onMount(() => {
    const readOnly = props.type === "compose-logs";
    term = createTerm(20, !readOnly);
    if (readOnly) {
      term.options.disableStdin = READ_ONLY_OPTIONS.disableStdin;
      term.options.convertEol = READ_ONLY_OPTIONS.convertEol;
    }
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);
    requestAnimationFrame(() => fit.fit());

    const ro = new ResizeObserver(() => fit.fit());
    ro.observe(host);

    const send = (data: string) => {
      if (ws && ws.readyState === WebSocket.OPEN) ws.send(data);
    };
    term.onData((d) => {
      if (props.type === "compose-logs") return;
      send(d);
    });
    term.onResize(({ rows, cols }) =>
      send(JSON.stringify({ type: "resize", data: { rows, cols } })),
    );

    // 上游对齐：选中文本即复制；右键粘贴（交互终端，日志流无输入通道）
    term.onSelectionChange(async () => {
      const sel = term.getSelection();
      if (!sel) return;
      try {
        await navigator.clipboard?.writeText(sel);
      } catch {
        // 非安全上下文（如局域网 HTTP）无剪贴板 API 或权限被拒：忽略
      }
    });
    term.element?.addEventListener("contextmenu", async (e) => {
      if (props.type === "compose-logs") return;
      e.preventDefault();
      try {
        const text = await navigator.clipboard?.readText();
        if (text) term.paste(text);
      } catch {
        // 同上：无剪贴板 API 时忽略
      }
    });

    // 键盘输入必须落在终端的隐藏输入区：进入页面即聚焦，点击终端区也聚焦。
    // 否则用户直接敲键盘毫无反应（表现为「终端不能输入」）。
    const focusTerm = () => term.focus();
    if (!readOnly) {
      requestAnimationFrame(() => term.focus());
      host.addEventListener("click", focusTerm);
    }

    onCleanup(() => {
      ro.disconnect();
      host.removeEventListener("click", focusTerm);
      ws?.close();
      term.dispose();
    });
  });

  // WebSocket 随 name/type 变化重连（切栈/切容器时终端实例复用、连接与内容刷新）；
  // 主动切换不写「会话已结束」，仅清屏等待新会话输出。
  createEffect(() => {
    const name = props.name;
    const kind = props.type;
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const tail = kind === "compose-logs" ? "&tail=200" : "";
    const base = kind === "console"
      ? `${proto}://${location.host}/v1/console/terminal`
      : `${proto}://${location.host}/v1/terminal/${encodeURIComponent(name)}/${kind}`;
    const sock = new WebSocket(`${base}?token=${encodeURIComponent(getToken())}${tail}`);
    ws = sock;
    term.write("\x1b[2J\x1b[H");
    sock.onmessage = (ev) => term.write(String(ev.data));
    // 一次性守卫：close 事件可能触发多次（如服务端断开+本地 close），只写一次结束消息
    let ended = false;
    sock.onclose = () => {
      if (ended) return;
      ended = true;
      props.onState?.("ended");
      term.write(`\r\n\x1b[33m${t("term.sessionEnded")}\x1b[0m\r\n`);
    };
    sock.onopen = () => {
      props.onState?.("live");
      sock.send(JSON.stringify({ type: "resize", data: { rows: term.rows, cols: term.cols } }));
    };
    onCleanup(() => {
      sock.onclose = null;
      sock.close();
      if (ws === sock) ws = null;
    });
  });

  return <div class="terminal-host" ref={(element) => { host = element; }} />;
}

/** 只读展示终端（上游进度终端的等价物）：内容变化时整体重写。 */
export function DisplayTerminal(props: { content: string; rows?: number }) {
  let host!: HTMLDivElement;
  let term: Terminal | undefined;
  const rows = () => props.rows ?? 8;

  onMount(() => {
    term = createTerm(rows(), false);
    // 进度内容经管道输出（仅 LF，无 PTY 补 CR），需 convertEol 回行首，
    // 否则阶梯状错位（同 READ_ONLY_OPTIONS 的说明）。
    term.options.convertEol = true;
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);
    requestAnimationFrame(() => fit.fit());
    const ro = new ResizeObserver(() => fit.fit());
    ro.observe(host);
    onCleanup(() => {
      ro.disconnect();
      term?.dispose();
      term = undefined;
    });
  });

  createEffect(() => {
    term?.write("\x1b[2J\x1b[H" + props.content);
  });

  return <div class="terminal-host progress" ref={(element) => { host = element; }} />;
}
