// 容器终端页（上游 /terminal/:stack/:service/:type 复刻）：
// service 参数承载容器 ID/名称，type 为 bash/sh（均走 exec WS）。
import { A, useParams } from "@solidjs/router";
import { ArrowLeft } from "lucide-solid";

import { TerminalPane } from "../components/Terminal";
import { t } from "../i18n";

export function TerminalPage() {
  const params = useParams();
  const container = () => decodeURIComponent(params.service ?? "");

  return (
    <div>
      <div class="compose-header">
        <h1>
          <A href={params.stack ? `/compose/${params.stack}` : "/"} class="btn-icon" aria-label={t("nav.home")}>
            <ArrowLeft size={16} />
          </A>
          <span class="mono" style={{ "font-size": "22px" }}>{container()}</span>
        </h1>
      </div>
      <div class="card">
        <TerminalPane name={container()} type="exec" />
      </div>
    </div>
  );
}
