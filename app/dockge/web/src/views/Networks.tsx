// 网络管理：列表（SSE 快照）、详情（inspect）、创建、删除、清理未使用。
import { For, Show, createEffect, createMemo, createSignal } from "solid-js";
import { Network, Plus, Trash2 } from "lucide-solid";
import { api, type NetworkInspectData } from "../api/api";
import { refresh, snapshot, toast } from "../store/index";
import { confirmDialog } from "../components/Confirm";
import { Sheet } from "../components/Sheet";
import { EmptyState, SectionHeader, SpinnerBlock } from "../components/widgets";
import { errText } from "../api/format";
import { Pagination } from "../components/Pagination";
import { clampPage, paginate } from "../lib/pagination";
import { t } from "../i18n";

export function Networks() {
  const [detailName, setDetailName] = createSignal<string | null>(null);
  const [detail, setDetail] = createSignal<NetworkInspectData | null>(null);
  const [createOpen, setCreateOpen] = createSignal(false);
  const [name, setName] = createSignal("");
  const [driver, setDriver] = createSignal("bridge");
  const [subnet, setSubnet] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [page, setPage] = createSignal(1);

  const rows = () => snapshot()?.networks ?? [];
  const visibleRows = createMemo(() => paginate(rows(), page()));

  createEffect(() => setPage((value) => clampPage(value, rows().length)));

  const openDetail = (n: string) => {
    setDetailName(n);
    setDetail(null);
    api
      .networkInspect(n)
      .then(setDetail)
      .catch((e) => toast(errText(e), "error"));
  };

  const create = async () => {
    if (!name().trim() || busy()) return;
    setBusy(true);
    try {
      await api.createNetwork(name().trim(), driver(), subnet().trim());
      toast(t("toast.networkCreated"), "success");
      setCreateOpen(false);
      setName("");
      setSubnet("");
      await refresh(false);
    } catch (e) {
      toast(errText(e), "error");
    } finally {
      setBusy(false);
    }
  };

  const remove = async (n: string) => {
    if (!(await confirmDialog(t("net.confirmRemove"), t("net.confirmRemoveDesc", { name: n })))) return;
    try {
      await api.removeNetwork(n);
      toast(t("toast.networkDeleted"), "success");
      await refresh(false);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  const prune = async () => {
    if (!(await confirmDialog(t("net.pruneTitle"), t("net.pruneDesc")))) return;
    try {
      await api.pruneNetworks();
      toast(t("toast.pruneNetworks"), "success");
      await refresh(false);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  return (
    <div class="view-section">
      <SectionHeader
        title={t("net.title")}
        subtitle={`${rows().length} networks`}
        actions={
          <>
            <button class="btn-clear" onClick={() => void prune()}>
              {t("net.clearUnused", { n: rows().length })}
            </button>
            <button class="btn btn-primary" onClick={() => setCreateOpen(true)}>
              <Plus size={14} /> {t("net.new")}
            </button>
          </>
        }
      />
      <Show
        when={rows().length > 0}
        fallback={<EmptyState title={t("net.empty")} icon={<Network size={44} />} />}
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
              {(n) => (
                <tr onClick={() => openDetail(n)} tabindex="0" onKeyDown={(e) => e.key === "Enter" && openDetail(n)}>
                  <td class="cell-main">{n}</td>
                  <td class="text-dim">
                    {detailName() === n && detail()?.Driver ? detail()?.Driver : "bridge"}
                  </td>
                  <td class="row-actions-cell" onClick={(e) => e.stopPropagation()}>
                    <div class="cell-actions">
                      <button
                        class="btn-icon"
                        style={{ color: "var(--danger)" }}
                        title={t("common.remove")}
                        onClick={() => void remove(n)}
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

      {/* 网络详情 */}
      <Sheet
        open={!!detailName()}
        onOpenChange={(o) => !o && setDetailName(null)}
        title={detailName() ?? ""}
        footer={<button class="btn btn-secondary" onClick={() => setDetailName(null)}>{t("common.close")}</button>}
      >
        <Show when={detail()} fallback={<SpinnerBlock />}>
          <div class="detail-grid">
            <div class="od-field"><span class="od-label">{t("net.detailName")}</span><span class="od-value">{detail()?.Name}</span></div>
            <div class="od-field"><span class="od-label">{t("net.detailDriver")}</span><span class="od-value">{detail()?.Driver ?? "—"}</span></div>
            <div class="od-field"><span class="od-label">{t("net.detailSubnet")}</span><span class="od-value">{detail()?.IPAM?.Config?.[0]?.Subnet ?? "—"}</span></div>
            <div class="od-field"><span class="od-label">{t("net.detailGateway")}</span><span class="od-value">{detail()?.IPAM?.Config?.[0]?.Gateway ?? "—"}</span></div>
          </div>
        </Show>
      </Sheet>

      {/* 新建网络 */}
      <Sheet
        open={createOpen()}
        onOpenChange={setCreateOpen}
        title={t("net.new")}
        footer={
          <>
            <button class="btn btn-secondary" onClick={() => setCreateOpen(false)}>{t("common.cancel")}</button>
            <button class="btn btn-primary" disabled={busy() || !name().trim()} onClick={() => void create()}>{t("stack.create")}</button>
          </>
        }
      >
        <div class="form-group">
          <label class="form-label">{t("net.networkName")}</label>
          <input class="form-input" placeholder="my-network" value={name()} onInput={(e) => setName(e.currentTarget.value)} />
        </div>
        <div class="form-group">
          <label class="form-label">{t("common.driver")}</label>
          <select class="form-select" value={driver()} onChange={(e) => setDriver(e.currentTarget.value)}>
            <option value="bridge">bridge</option>
            <option value="host">host</option>
            <option value="overlay">overlay</option>
          </select>
        </div>
        <div class="form-group">
          <label class="form-label">{t("net.subnet")}</label>
          <input class="form-input" placeholder="172.20.0.0/16" value={subnet()} onInput={(e) => setSubnet(e.currentTarget.value)} />
        </div>
      </Sheet>
    </div>
  );
}
