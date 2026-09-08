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
      toast("网络已创建", "success");
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
    if (!(await confirmDialog("删除网络", `确定删除网络 ${n}？仅未使用的网络可删除。`))) return;
    try {
      await api.removeNetwork(n);
      toast("网络已删除", "success");
      await refresh(false);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  const prune = async () => {
    if (!(await confirmDialog("清理未使用网络", "将删除所有未被容器使用的自定义网络。继续？"))) return;
    try {
      await api.pruneNetworks();
      toast("已清理未使用网络", "success");
      await refresh(false);
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  return (
    <div class="view-section">
      <SectionHeader
        title="Networks"
        subtitle={`${rows().length} networks`}
        actions={
          <>
            <button class="btn-clear" onClick={() => void prune()}>
              清空未使用网络
            </button>
            <button class="btn btn-primary" onClick={() => setCreateOpen(true)}>
              <Plus size={14} /> New Network
            </button>
          </>
        }
      />
      <Show
        when={rows().length > 0}
        fallback={<EmptyState title="暂无网络" icon={<Network size={44} />} />}
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
                        title="Remove"
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
        footer={<button class="btn btn-secondary" onClick={() => setDetailName(null)}>Close</button>}
      >
        <Show when={detail()} fallback={<SpinnerBlock />}>
          <div class="detail-grid">
            <div class="od-field"><span class="od-label">Name</span><span class="od-value">{detail()?.Name}</span></div>
            <div class="od-field"><span class="od-label">Driver</span><span class="od-value">{detail()?.Driver ?? "—"}</span></div>
            <div class="od-field"><span class="od-label">Subnet</span><span class="od-value">{detail()?.IPAM?.Config?.[0]?.Subnet ?? "—"}</span></div>
            <div class="od-field"><span class="od-label">Gateway</span><span class="od-value">{detail()?.IPAM?.Config?.[0]?.Gateway ?? "—"}</span></div>
          </div>
        </Show>
      </Sheet>

      {/* 新建网络 */}
      <Sheet
        open={createOpen()}
        onOpenChange={setCreateOpen}
        title="New Network"
        footer={
          <>
            <button class="btn btn-secondary" onClick={() => setCreateOpen(false)}>Cancel</button>
            <button class="btn btn-primary" disabled={busy() || !name().trim()} onClick={() => void create()}>Create</button>
          </>
        }
      >
        <div class="form-group">
          <label class="form-label">网络名称</label>
          <input class="form-input" placeholder="my-network" value={name()} onInput={(e) => setName(e.currentTarget.value)} />
        </div>
        <div class="form-group">
          <label class="form-label">Driver</label>
          <select class="form-select" value={driver()} onChange={(e) => setDriver(e.currentTarget.value)}>
            <option value="bridge">bridge</option>
            <option value="host">host</option>
            <option value="overlay">overlay</option>
          </select>
        </div>
        <div class="form-group">
          <label class="form-label">子网（可选）</label>
          <input class="form-input" placeholder="172.20.0.0/16" value={subnet()} onInput={(e) => setSubnet(e.currentTarget.value)} />
        </div>
      </Sheet>
    </div>
  );
}
