// 系统信息：引擎版本、统计与 SSE 订阅数。
import { For, Show, createResource } from "solid-js";
import { RefreshCw } from "lucide-solid";
import { api, type VersionSummary } from "../api/api";
import { snapshot, sseOn } from "../store/index";
import { SectionHeader } from "../components/widgets";
import { errText } from "../api/format";
import { t } from "../i18n";

export function SysInfo() {
  const [version, { refetch }] = createResource<VersionSummary>(() => api.version());

  const rows = (): Array<[string, string]> => {
    const v = version();
    const snap = snapshot();
    const list: Array<[string, string]> = [
      ["Docker Version", v?.version ?? "…"],
      ["API Version", v?.apiVersion ?? "…"],
      ["OS", v?.os ?? "…"],
      ["Arch", v?.arch ?? "…"],
      ["Stacks Total", String(snap?.docker.stacksTotal ?? 0)],
      ["Stacks Running", String(snap?.docker.stacksRunning ?? 0)],
      ["Containers Total", String(snap?.docker.containersTotal ?? 0)],
      ["Containers Running", String(snap?.docker.containersRunning ?? 0)],
      ["Images", String(snap?.images.length ?? 0)],
      ["Volumes", String(snap?.volumes.length ?? 0)],
      ["Networks", String(snap?.networks.length ?? 0)],
      [t("container.status"), sseOn() ? t("sysinfo.connected") : t("sysinfo.disconnected")],
    ];
    return list;
  };

  return (
    <div class="view-section">
      <SectionHeader
        title={t("view.sysinfo")}
        subtitle={t("sysinfo.subtitle")}
        actions={
          <button class="btn btn-secondary" onClick={() => void refetch()}>
            <RefreshCw size={14} /> {t("common.refresh")}
          </button>
        }
      />
      <table class="data-table">
        <thead>
          <tr>
            <th>{t("th.key")}</th>
            <th>{t("th.value")}</th>
          </tr>
        </thead>
        <tbody>
          <For each={rows()}>
            {([k, v]) => (
              <tr style={{ cursor: "default" }}>
                <td class="text-dim" style={{ width: "200px" }}>{k}</td>
                <td class="mono">{v}</td>
              </tr>
            )}
          </For>
        </tbody>
      </table>
      <Show when={version.error}>
        <p class="text-dim" style={{ "margin-top": "12px" }}>{t("sysinfo.versionFailed", { msg: errText(version.error) })}</p>
      </Show>
    </div>
  );
}
