// 镜像管理：列表（SSE 快照）、拉取（Sheet）、删除（确认）、清理未使用。
import { For, Show, createEffect, createMemo, createSignal } from "solid-js";
import { Download, Image, Plus, Trash2 } from "lucide-solid";
import { api } from "../api/api";
import { refresh, snapshot, toast } from "../store/index";
import { confirmDialog } from "../components/Confirm";
import { Sheet } from "../components/Sheet";
import { EmptyState, SectionHeader } from "../components/widgets";
import { errText, shortId } from "../api/format";
import { Pagination } from "../components/Pagination";
import { clampPage, paginate } from "../lib/pagination";
import { t } from "../i18n";

export function Images() {
  const [pullOpen, setPullOpen] = createSignal(false);
  const [reference, setReference] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [page, setPage] = createSignal(1);

  const rows = () => snapshot()?.images ?? [];
  const visibleRows = createMemo(() => paginate(rows(), page()));

  createEffect(() => setPage((value) => clampPage(value, rows().length)));

  const pull = async () => {
    if (!reference().trim() || busy()) return;
    setBusy(true);
    try {
      await api.pullImage(reference().trim());
      toast(t("toast.imagePulled", { ref: reference() }), "success");
      setPullOpen(false);
      setReference("");
      await refresh(false);
    } catch (e) {
      toast(errText(e), "error");
    } finally {
      setBusy(false);
    }
  };

  const remove = async (id: string) => {
    if (!(await confirmDialog(t("common.delete"), t("img.confirmRemoveDesc", { id: shortId(id) })))) return;
    try {
      await api.removeImage(id);
      toast(t("toast.imageRemoved"), "success");
      await refresh(false);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  const prune = async () => {
    if (!(await confirmDialog(t("img.pruneTitle"), t("img.pruneDesc")))) return;
    try {
      await api.pruneImages();
      toast(t("toast.pruneImages"), "success");
      await refresh(false);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  return (
    <div class="view-section">
      <SectionHeader
        title={t("img.title")}
        subtitle={`${rows().length} images`}
        actions={
          <>
            <button class="btn-clear" onClick={() => void prune()}>
              {t("img.clearUnused", { n: rows().length })}
            </button>
            <button class="btn btn-primary" onClick={() => setPullOpen(true)}>
              <Plus size={14} /> {t("img.pull")}
            </button>
          </>
        }
      />
      <Show
        when={rows().length > 0}
        fallback={<EmptyState title={t("img.empty")} desc={t("img.emptyDesc")} icon={<Image size={44} />} />}
      >
        <table class="data-table">
          <thead>
            <tr>
              <th>Repository</th>
              <th>{t("common.tag")}</th>
              <th>{t("common.size")}</th>
              <th>{t("common.created")}</th>
              <th style={{ width: "80px" }}></th>
            </tr>
          </thead>
          <tbody>
            <For each={visibleRows()}>
              {(img) => (
                <tr>
                  <td>
                    <div class="cell-main">{img.repo}</div>
                    <div class="cell-sub">{shortId(img.id)}</div>
                  </td>
                  <td>{img.tag || "<none>"}</td>
                  <td class="text-dim">{img.size}</td>
                  <td class="cell-dim">{img.created}</td>
                  <td class="row-actions-cell">
                    <div class="cell-actions">
                      <button
                        class="btn-icon"
                        style={{ color: "var(--danger)" }}
                        title={t("common.remove")}
                        onClick={() => void remove(img.id)}
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

      <Sheet
        open={pullOpen()}
        onOpenChange={setPullOpen}
        title={t("img.pull")}
        footer={
          <>
            <button class="btn btn-secondary" onClick={() => setPullOpen(false)}>{t("common.cancel")}</button>
            <button class="btn btn-primary" disabled={busy() || !reference().trim()} onClick={() => void pull()}>
              <Download size={14} /> {busy() ? t("img.pulling") : t("img.pull")}
            </button>
          </>
        }
      >
        <div class="form-group">
          <label class="form-label">{t("img.reference")}</label>
          <input
            class="form-input"
            placeholder="nginx:latest"
            autofocus
            value={reference()}
            onInput={(e) => setReference(e.currentTarget.value)}
          />
          <p class="form-help">{t("img.referenceHelp")}</p>
        </div>
      </Sheet>
    </div>
  );
}
