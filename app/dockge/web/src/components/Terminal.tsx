// xterm.js 终端面板：连接 /v1/terminal/{name}/{type} WebSocket（exec / compose-logs）。
// resize 以 JSON 控制消息上报；compose-logs 只读不回传输入。
import { onCleanup, onMount } from "solid-js";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { getToken } from "../api/api";
import { t } from "../i18n";

export function TerminalPane(props: { name: string; type: "exec" | "compose-logs" }) {
  let host!: HTMLDivElement;

  onMount(() => {
    const term = new Terminal({
      fontSize: 12,
      cursorBlink: true,
      fontFamily: '"SF Mono", ui-monospace, "JetBrains Mono", Menlo, Consolas, monospace',
      theme: {
        background: "#1d1d1f",
        foreground: "#e8e8ed",
        cursor: "#16a34a",
      },
    });
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
