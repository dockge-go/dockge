// 磁盘占用：docker system df 汇总 + 按类别清理 + 一键清理。
import { For, Show, createResource } from "solid-js";
import { RefreshCw, Trash2 } from "lucide-solid";
import { api } from "../api/api";
import { refresh, toast } from "../store/index";
import { confirmDialog } from "../components/Confirm";
import { SectionHeader, SpinnerBlock } from "../components/widgets";
import { errText } from "../api/format";
import { t, type MsgKey } from "../i18n";

// docker system df 的 type 是稳定英文标识；展示层走词典，分发按原值匹配
const TYPE_LABEL: Record<string, MsgKey> = {
  Images: "sysdf.typeImages",
  Containers: "sysdf.typeContainers",
  "Local Volumes": "sysdf.typeVolumes",
  "Build Cache": "sysdf.typeBuildCache",
};

const typeLabel = (type: string) => (TYPE_LABEL[type] ? t(TYPE_LABEL[type]) : type);

export function SysDf() {
  const [df, { refetch }] = createResource(() => api.df());

  const pruneFn = (type: string) => {
    if (type === "Images") return () => api.pruneImages();
    if (type === "Containers") return () => api.pruneContainers();
    if (type === "Local Volumes") return () => api.pruneVolumes();
    return null; // Build Cache 等暂无对应接口
  };

  const prune = async (type: string) => {
    const fn = pruneFn(type);
    if (!fn) {
      toast(t("toast.unsupportedPrune"), "info");
      return;
    }
    const label = typeLabel(type);
    if (!(await confirmDialog(t("sysdf.pruneConfirm"), t("sysdf.pruneDesc", { type: label })))) return;
    try {
      await fn();
      toast(t("toast.pruneCategory", { type: label }), "success");
      await Promise.all([refetch(), refresh(false)]);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  const pruneAll = async () => {
    if (!(await confirmDialog(t("sysdf.pruneAllConfirm"), t("sysdf.pruneAllDesc")))) return;
    try {
      await Promise.all([api.pruneImages(), api.pruneContainers(), api.pruneVolumes()]);
      toast(t("toast.pruneAll"), "success");
      await Promise.all([refetch(), refresh(false)]);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  return (
    <div class="view-section">
      <SectionHeader
        title={t("nav.sysdf")}
        subtitle={t("sysdf.subtitle")}
        actions={
          <>
            <button class="btn btn-secondary" onClick={() => void refetch()}>
              <RefreshCw size={14} /> {t("common.refresh")}
            </button>
            <button class="btn btn-danger" onClick={() => void pruneAll()}>
              <Trash2 size={14} /> {t("sysdf.pruneAll")}
            </button>
          </>
        }
      />
      <Show when={!df.loading} fallback={<SpinnerBlock />}>
        <table class="data-table">
          <thead>
            <tr>
              <th>{t("th.type")}</th>
              <th>{t("th.used")}</th>
              <th>{t("th.reclaimable")}</th>
              <th style={{ width: "100px" }}></th>
            </tr>
          </thead>
          <tbody>
            <For each={df()?.list ?? []}>
              {(it) => {
                const reclaimable = it.reclaimable !== "0B";
                return (
                  <tr style={{ cursor: "default" }}>
                    <td class="cell-main">{typeLabel(it.type)}</td>
                    <td>
                      {it.size} <span class="cell-dim">· {it.count} {t("sysdf.items")} · {it.active} {t("sysdf.active")}</span>
                    </td>
                    <td style={{ color: reclaimable ? "var(--danger)" : "var(--muted)", "font-size": "14px" }}>
                      {it.reclaimable} {t("sysdf.reclaimable")}
                    </td>
                    <td>
                      <Show when={reclaimable && pruneFn(it.type)}>
                        <button class="btn btn-secondary" style={{ padding: "4px 12px", "font-size": "12px" }} onClick={() => void prune(it.type)}>
                          {t("sysdf.prune")}
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
