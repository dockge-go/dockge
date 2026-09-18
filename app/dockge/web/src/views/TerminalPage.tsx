// 容器终端页（上游 /terminal/:stack/:service/:type 复刻）：
// service 参数承载容器 ID/名称；标题「终端 - 服务 (栈)」；提供 bash/sh 切换。
import { A, useParams } from "@solidjs/router";

import { TerminalPane } from "../components/Terminal";
import { t } from "../i18n";

export function TerminalPage() {
  const params = useParams();
  const container = () => decodeURIComponent(params.service ?? "");
  const stack = () => decodeURIComponent(params.stack ?? "");
  const shell = () => (params.type === "sh" ? "sh" : "bash");
  const switchTo = () => (shell() === "bash" ? "sh" : "bash");

  return (
    <div>
      <h1 class="terminal-page-title">
        {t("compose.terminalTitle", { service: container(), stack: stack() })}
      </h1>
      <A
        class="btn btn-secondary"
        href={`/terminal/${encodeURIComponent(params.stack ?? "")}/${encodeURIComponent(params.service ?? "")}/${switchTo()}`}
      >
        {t("compose.switchShell", { shell: switchTo() })}
      </A>
      <div style={{ "margin-top": "12px" }}>
        <TerminalPane name={container()} type="exec" />
      </div>
    </div>
  );
}
