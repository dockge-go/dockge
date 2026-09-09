// 数据卷管理：列表（SSE 快照）、删除（确认）、清理未使用。
import { For, Show, createEffect, createMemo, createSignal } from "solid-js";
import { HardDrive, Trash2 } from "lucide-solid";
import { api } from "../api/api";
import { refresh, snapshot, toast } from "../store/index";
import { confirmDialog } from "../components/Confirm";
import { EmptyState, SectionHeader } from "../components/widgets";
import { errText } from "../api/format";
import { Pagination } from "../components/Pagination";
import { clampPage, paginate } from "../lib/pagination";
import { t } from "../i18n";

export function Volumes() {
  const [page, setPage] = createSignal(1);
  const rows = () => snapshot()?.volumes ?? [];
  const visibleRows = createMemo(() => paginate(rows(), page()));

  createEffect(() => setPage((value) => clampPage(value, rows().length)));

  const remove = async (name: string) => {
    if (!(await confirmDialog(t("vol.confirmRemove"), t("vol.confirmRemoveDesc", { name })))) return;
    try {
      await api.removeVolume(name);
      toast(t("toast.volumeDeleted"), "success");
      await refresh(false);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  const prune = async () => {
    if (!(await confirmDialog(t("vol.pruneTitle"), t("vol.pruneDesc")))) return;
    try {
      await api.pruneVolumes();
      toast(t("toast.pruneVolumes"), "success");
      await refresh(false);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  return (
    <div class="view-section">
      <SectionHeader
        title={t("vol.title")}
        subtitle={`${rows().length} volumes`}
        actions={
          <button class="btn-clear" onClick={() => void prune()}>
            {t("vol.clearUnused", { n: rows().length })}
          </button>
        }
      />
      <Show
        when={rows().length > 0}
        fallback={<EmptyState title={t("vol.empty")} icon={<HardDrive size={44} />} />}
      >
        <table class="data-table">
          <thead>
            <tr>
              <th>{t("net.detailName")}</th>
              <th>{t("net.detailDriver")}</th>
              <th style={{ width: "80px" }}></th>
            </tr>
          </thead>
          <tbody>
            <For each={visibleRows()}>
              {(v) => (
                <tr>
                  <td class="cell-main">{v.name}</td>
                  <td class="text-dim">{v.driver}</td>
                  <td class="row-actions-cell">
                    <div class="cell-actions">
                      <button
                        class="btn-icon"
                        style={{ color: "var(--danger)" }}
                        title={t("common.remove")}
                        onClick={() => void remove(v.name)}
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              )}
            </For>
          </tbody>
        </table>
        <Pagination page={page()} total={rows().length} onPageChange={setPage} />
      </Show>
    </div>
  );
}
