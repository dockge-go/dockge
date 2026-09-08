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
      toast(`镜像 ${reference()} 已拉取`, "success");
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
    if (!(await confirmDialog("删除镜像", `确定删除镜像 ${shortId(id)}？使用中的镜像会跳过。`))) return;
    try {
      await api.removeImage(id);
      toast("镜像已删除", "success");
      await refresh(false);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  const prune = async () => {
    if (!(await confirmDialog("清理未使用镜像", "将删除所有未被容器使用的镜像，释放磁盘空间。继续？"))) return;
    try {
      await api.pruneImages();
      toast("已清理未使用镜像", "success");
      await refresh(false);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  return (
    <div class="view-section">
      <SectionHeader
        title="Images"
        subtitle={`${rows().length} images`}
        actions={
          <>
            <button class="btn-clear" onClick={() => void prune()}>
              清空未使用镜像
            </button>
            <button class="btn btn-primary" onClick={() => setPullOpen(true)}>
              <Plus size={14} /> Pull Image
            </button>
          </>
        }
      />
      <Show
        when={rows().length > 0}
        fallback={<EmptyState title="暂无镜像" desc="拉取一个镜像开始使用。" icon={<Image size={44} />} />}
      >
        <table class="data-table">
          <thead>
            <tr>
              <th>Repository</th>
              <th>Tag</th>
              <th>Size</th>
              <th>Created</th>
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
                        title="Remove"
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
        title="Pull Image"
        footer={
          <>
            <button class="btn btn-secondary" onClick={() => setPullOpen(false)}>Cancel</button>
            <button class="btn btn-primary" disabled={busy() || !reference().trim()} onClick={() => void pull()}>
              <Download size={14} /> {busy() ? "拉取中…" : "Pull"}
            </button>
          </>
        }
      >
        <div class="form-group">
          <label class="form-label">镜像引用</label>
          <input
            class="form-input"
            placeholder="nginx:latest"
            autofocus
            value={reference()}
            onInput={(e) => setReference(e.currentTarget.value)}
          />
          <p class="form-help">例如 nginx:latest、ubuntu:22.04、ghcr.io/owner/repo:tag</p>
        </div>
      </Sheet>
    </div>
  );
}
