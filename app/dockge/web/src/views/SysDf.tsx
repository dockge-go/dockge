// 磁盘占用：docker system df 汇总 + 按类别清理 + 一键清理。
import { For, Show, createResource } from "solid-js";
import { RefreshCw, Trash2 } from "lucide-solid";
import { api } from "../api/api";
import { refresh, toast } from "../store/index";
import { confirmDialog } from "../components/Confirm";
import { SectionHeader, SpinnerBlock } from "../components/widgets";
import { errText } from "../api/format";

export function SysDf() {
  const [df, { refetch }] = createResource(() => api.df());

  const pruneFn = (type: string) => {
    if (type === "Images" || type === "镜像") return () => api.pruneImages();
    if (type === "Containers" || type === "容器") return () => api.pruneContainers();
    if (type === "Local Volumes" || type === "卷") return () => api.pruneVolumes();
    return null; // Build Cache 等暂无对应接口
  };

  const prune = async (type: string) => {
    const fn = pruneFn(type);
    if (!fn) {
      toast("该类别暂不支持在线清理", "info");
      return;
    }
    if (!(await confirmDialog("清理", `确定清理 ${type} 中未使用的数据？`))) return;
    try {
      await fn();
      toast(`已清理 ${type}`, "success");
      await Promise.all([refetch(), refresh(false)]);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  const pruneAll = async () => {
    if (!(await confirmDialog("一键清理", "将清理未使用的镜像、已停止容器与未使用卷。继续？"))) return;
    try {
      await Promise.all([api.pruneImages(), api.pruneContainers(), api.pruneVolumes()]);
      toast("已清理全部未使用数据", "success");
      await Promise.all([refetch(), refresh(false)]);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  return (
    <div class="view-section">
      <SectionHeader
        title="Disk Usage"
        subtitle="各类资源的磁盘占用"
        actions={
          <>
            <button class="btn btn-secondary" onClick={() => void refetch()}>
              <RefreshCw size={14} /> Refresh
            </button>
            <button class="btn btn-danger" onClick={() => void pruneAll()}>
              <Trash2 size={14} /> Prune All
            </button>
          </>
        }
      />
      <Show when={!df.loading} fallback={<SpinnerBlock />}>
        <table class="data-table">
          <thead>
            <tr>
              <th>Type</th>
              <th>Used</th>
              <th>Reclaimable</th>
              <th style={{ width: "100px" }}></th>
            </tr>
          </thead>
          <tbody>
            <For each={df()?.list ?? []}>
              {(it) => {
                const reclaimable = it.reclaimable !== "0B";
                return (
                  <tr style={{ cursor: "default" }}>
                    <td class="cell-main">{it.type}</td>
                    <td>
                      {it.size} <span class="cell-dim">· {it.count} items · {it.active} active</span>
                    </td>
                    <td style={{ color: reclaimable ? "var(--danger)" : "var(--muted)", "font-size": "14px" }}>
                      {it.reclaimable} reclaimable
                    </td>
                    <td>
                      <Show when={reclaimable && pruneFn(it.type)}>
                        <button class="btn btn-secondary" style={{ padding: "4px 12px", "font-size": "12px" }} onClick={() => void prune(it.type)}>
                          Prune
                        </button>
                      </Show>
                    </td>
                  </tr>
                );
              }}
            </For>
          </tbody>
        </table>
      </Show>
    </div>
  );
}
