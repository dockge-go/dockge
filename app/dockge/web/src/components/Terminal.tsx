// xterm.js 终端面板：连接 /v1/terminal/{name}/{type} WebSocket（exec / compose-logs）。
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

export function TerminalPane(props: { name: string; type: "exec" | "compose-logs" }) {
  let host!: HTMLDivElement;

  onMount(() => {
    const term = createTerm(20);
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);
    requestAnimationFrame(() => fit.fit());

    const ro = new ResizeObserver(() => fit.fit());
    ro.observe(host);

    let ws: WebSocket | null = null;
    const send = (data: string) => {
      if (ws && ws.readyState === WebSocket.OPEN) ws.send(data);
    };

    const proto = location.protocol === "https:" ? "wss" : "ws";
    const tail = props.type === "compose-logs" ? "?tail=200" : "";
    ws = new WebSocket(
      `${proto}://${location.host}/v1/terminal/${encodeURIComponent(props.name)}/${props.type}${tail ? `${tail}&token=` : "?token="}${encodeURIComponent(getToken())}`,
    );
    ws.onmessage = (ev) => term.write(String(ev.data));
    ws.onclose = () => term.write(`\r\n\x1b[33m${t("term.sessionEnded")}\x1b[0m\r\n`);
    ws.onopen = () =>
      send(JSON.stringify({ type: "resize", data: { rows: term.rows, cols: term.cols } }));
    term.onData((d) => {
      if (props.type === "compose-logs") return;
      send(d);
    });
    term.onResize(({ rows, cols }) =>
      send(JSON.stringify({ type: "resize", data: { rows, cols } })),
    );

    onCleanup(() => {
      ro.disconnect();
      ws?.close();
      term.dispose();
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
