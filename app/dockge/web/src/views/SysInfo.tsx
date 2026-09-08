// 系统信息：引擎版本、统计与 SSE 订阅数。
import { For, Show, createResource } from "solid-js";
import { RefreshCw } from "lucide-solid";
import { api, type VersionSummary } from "../api/api";
import { snapshot, sseOn } from "../store/index";
import { SectionHeader } from "../components/widgets";
import { errText } from "../api/format";

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
      ["容器状态流", sseOn() ? "已连接 (SSE)" : "未连接"],
    ];
    return list;
  };

  return (
    <div class="view-section">
      <SectionHeader
        title="System Info"
        subtitle="引擎与运行环境信息"
        actions={
          <button class="btn btn-secondary" onClick={() => void refetch()}>
            <RefreshCw size={14} /> Refresh
          </button>
        }
      />
      <table class="data-table">
        <thead>
          <tr>
            <th>Key</th>
            <th>Value</th>
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
        <p class="text-dim" style={{ "margin-top": "12px" }}>版本信息获取失败：{errText(version.error)}</p>
      </Show>
    </div>
  );
}
