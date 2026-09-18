// 日志流：SSE 逐行追加到滚动视图（/v1/docker/containers/:id/logs），EventSource 以 ?token= 认证。
import { onCleanup, onMount, createSignal } from "solid-js";
import { getToken } from "../api/api";

type LogStreamProps = { containerId: string };

export function LogStream(props: LogStreamProps) {
  let pre: HTMLPreElement | undefined;
  const [lines, setLines] = createSignal<string[]>([]);

  onMount(() => {
    const push = (line: string) => {
      setLines((ls) => [...ls.slice(-499), line]);
      requestAnimationFrame(() => {
        if (pre) pre.scrollTop = pre.scrollHeight;
      });
    };
    const base = `/v1/docker/containers/${encodeURIComponent(props.containerId)}/logs`;
    const es = new EventSource(`${base}?tail=200&token=${encodeURIComponent(getToken())}`);
    es.onmessage = (ev) => {
      if (ev.data) push(ev.data);
    };
    onCleanup(() => es.close());
  });

  return (
    <pre class="log-viewer dark tall" ref={pre}>
      {lines().join("\n")}
    </pre>
  );
}
