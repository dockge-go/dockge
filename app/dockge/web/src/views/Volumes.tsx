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

export function Volumes() {
  const [page, setPage] = createSignal(1);
  const rows = () => snapshot()?.volumes ?? [];
  const visibleRows = createMemo(() => paginate(rows(), page()));

  createEffect(() => setPage((value) => clampPage(value, rows().length)));

  const remove = async (name: string) => {
    if (!(await confirmDialog("删除数据卷", `确定删除卷 ${name}？其中的数据将丢失。`))) return;
    try {
      await api.removeVolume(name);
      toast("卷已删除", "success");
      await refresh(false);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  const prune = async () => {
    if (!(await confirmDialog("清理未使用卷", "将删除所有未被容器引用的数据卷，数据不可恢复。继续？"))) return;
    try {
      await api.pruneVolumes();
      toast("已清理未使用卷", "success");
      await refresh(false);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  return (
    <div class="view-section">
      <SectionHeader
        title="Volumes"
        subtitle={`${rows().length} volumes`}
        actions={
          <button class="btn-clear" onClick={() => void prune()}>
            清空未使用卷
          </button>
        }
      />
      <Show
        when={rows().length > 0}
        fallback={<EmptyState title="暂无数据卷" icon={<HardDrive size={44} />} />}
      >
        <table class="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Driver</th>
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
                        title="Remove"
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
