import { For, Show, createEffect, createMemo, createSignal, onMount } from "solid-js";
import { Play, RotateCw, Square, Trash2 } from "lucide-solid";

import { api, type ContainerInspectData } from "../api/api";
import { confirmDialog } from "../components/Confirm";
import { ContainerDetail } from "../components/ContainerDetail";
import { Pagination } from "../components/Pagination";
import { EmptyState, LiveDot, SectionHeader, StatusBadge } from "../components/widgets";
import { errText, shortId } from "../api/format";
import { clampPage, paginate } from "../lib/pagination";
import { pendingContainer, refresh, setPendingContainer, snapshot, toast } from "../store/index";

export function Containers() {
  const [showAll, setShowAll] = createSignal(false);
  const [page, setPage] = createSignal(1);
  const [detailId, setDetailId] = createSignal<string | null>(null);
  const [inspect, setInspect] = createSignal<ContainerInspectData | null>(null);
  const [busy, setBusy] = createSignal(false);

  const rows = createMemo(() => {
    const containers = snapshot()?.containers ?? [];
    return showAll() ? containers : containers.filter((container) => container.state === "running");
  });
  const visibleRows = createMemo(() => paginate(rows(), page()));
  const current = createMemo(() => (snapshot()?.containers ?? []).find((container) => container.id === detailId()));
  const stoppedCount = createMemo(() => (snapshot()?.containers ?? []).filter((container) => container.state === "exited").length);

  createEffect(() => setPage((value) => clampPage(value, rows().length)));
  createEffect(() => {
    if (detailId() && !current()) setDetailId(null);
  });

  const openDetail = async (id: string) => {
    setDetailId(id);
    setInspect(null);
    try {
      setInspect(await api.containerInspect(id));
    } catch (error) {
      toast(errText(error), "error");
    }
  };

  onMount(() => {
    const pending = pendingContainer();
    if (!pending) return;
    setPendingContainer(null);
    void openDetail(pending);
  });

  const action = async (id: string, value: "start" | "stop" | "restart") => {
    if (busy()) return;
    setBusy(true);
    try {
      await api.containerAction(id, value);
      toast(`${shortId(id)} 已发送${value}指令`, "success");
      await refresh(false);
    } catch (error) {
      toast(errText(error), "error");
    } finally {
      setBusy(false);
    }
  };

  const remove = async (id: string) => {
    if (!(await confirmDialog("删除容器", `确定强制删除容器 ${shortId(id)}？此操作不可恢复。`))) return;
    try {
      await api.removeContainer(id);
      setDetailId(null);
      toast("容器已删除", "success");
      await refresh(false);
    } catch (error) {
      toast(errText(error), "error");
    }
  };

  const pruneStopped = async () => {
    if (!stoppedCount()) return toast("没有已停止的容器", "info");
    if (!(await confirmDialog("清空已停止容器", `将删除 ${stoppedCount()} 个已停止容器，继续？`))) return;
    try {
      await api.pruneContainers();
      toast("已清理停止的容器", "success");
      await refresh(false);
    } catch (error) {
      toast(errText(error), "error");
    }
  };

  return (
    <div class="view-section workspace-view">
      <SectionHeader
        title={<>Containers <LiveDot /></>}
        subtitle={`${snapshot()?.docker.containersTotal ?? 0} total · ${snapshot()?.docker.containersRunning ?? 0} running · SSE`}
        actions={
          <>
            <button class="btn-clear" onClick={() => void pruneStopped()}>清空已停止 ({stoppedCount()})</button>
            <label class="filter-toggle">
              <input type="checkbox" checked={showAll()} onChange={(event) => { setShowAll(event.currentTarget.checked); setPage(1); }} />
              显示全部
            </label>
          </>
        }
      />

      <div class="master-detail" classList={{ "has-detail": Boolean(current()) }}>
        <section class="master-pane" aria-label="容器列表">
          <Show when={rows().length > 0} fallback={<EmptyState title={showAll() ? "暂无容器" : "没有运行中的容器"} />}>
            <div class="resource-list">
              <For each={visibleRows()}>
                {(container) => (
                  <article
                    class="resource-row"
                    classList={{ selected: detailId() === container.id }}
                    tabindex="0"
                    onClick={() => void openDetail(container.id)}
                    onKeyDown={(event) => event.key === "Enter" && void openDetail(container.id)}
                  >
                    <div class="resource-row-main">
                      <strong>{container.name}</strong>
                      <span class="mono">{shortId(container.id)} · {container.image}</span>
                    </div>
                    <StatusBadge state={container.state} />
                    <p>{container.ports || "无公开端口"}</p>
                    <div class="resource-row-actions" onClick={(event) => event.stopPropagation()}>
                      <Show when={container.state === "running"} fallback={<button class="btn-icon" aria-label="启动" onClick={() => void action(container.id, "start")}><Play size={14} /></button>}>
                        <button class="btn-icon" aria-label="停止" onClick={() => void action(container.id, "stop")}><Square size={14} /></button>
                      </Show>
                      <button class="btn-icon" aria-label="重启" onClick={() => void action(container.id, "restart")}><RotateCw size={14} /></button>
                      <button class="btn-icon danger" aria-label="删除" onClick={() => void remove(container.id)}><Trash2 size={14} /></button>
                    </div>
                  </article>
                )}
              </For>
            </div>
            <Pagination page={page()} total={rows().length} onPageChange={setPage} />
          </Show>
        </section>

        <Show when={current()} fallback={<div class="detail-placeholder"><p>选择一个容器查看状态、日志和终端</p></div>}>
          {(container) => (
            <ContainerDetail
              container={container()}
              inspect={inspect()}
              busy={busy()}
              onBack={() => setDetailId(null)}
              onAction={(value) => void action(container().id, value)}
              onRemove={() => void remove(container().id)}
            />
          )}
        </Show>
      </div>
    </div>
  );
}
