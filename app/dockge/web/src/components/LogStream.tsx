// 日志流：SSE 逐行追加到滚动视图（容器 /v1/docker/containers/:id/logs、
// 栈 /v1/stacks/:name/logs/stream），EventSource 以 ?token= 认证。
import { onCleanup, onMount, createSignal } from "solid-js";
import { getToken } from "../api/api";

type LogStreamProps =
  | { containerId: string; stackName?: never }
  | { stackName: string; containerId?: never };

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
    const base = props.containerId !== undefined
      ? `/v1/docker/containers/${encodeURIComponent(props.containerId)}/logs`
      : `/v1/stacks/${encodeURIComponent(props.stackName)}/logs/stream`;
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
