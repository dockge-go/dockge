import { For, Show, createEffect, createMemo, createSignal, onCleanup, onMount } from "solid-js";
import { Play, RotateCw, Square, Trash2 } from "lucide-solid";

import { api, type ContainerInspectData } from "../api/api";
import { confirmDialog } from "../components/Confirm";
import { ContainerDetail } from "../components/ContainerDetail";
import { Pagination } from "../components/Pagination";
import { EmptyState, LiveDot, SectionHeader, StatusBadge } from "../components/widgets";
import { errText, shortId } from "../api/format";
import { clampPage, paginate } from "../lib/pagination";
import { listNav } from "../lib/listnav";
import { pendingContainer, refresh, setPendingContainer, snapshot, toast } from "../store/index";
import { t } from "../i18n";

export function Containers() {
  const [showAll, setShowAll] = createSignal(false);
  const [page, setPage] = createSignal(1);
  const [detailId, setDetailId] = createSignal<string | null>(null);
  const [inspect, setInspect] = createSignal<ContainerInspectData | null>(null);
  const [busy, setBusy] = createSignal(false);
  let listHost: HTMLDivElement | undefined;

  onMount(() => {
    if (listHost) onCleanup(listNav(listHost));
  });

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
      toast(t("toast.actionSent", { id: shortId(id), action: value }), "success");
      await refresh(false);
    } catch (error) {
      toast(errText(error), "error");
    } finally {
      setBusy(false);
    }
  };

  const remove = async (id: string) => {
    if (!(await confirmDialog(t("common.delete"), t("ctn.confirmRemoveDesc", { id: shortId(id) })))) return;
    try {
      await api.removeContainer(id);
      setDetailId(null);
      toast(t("toast.containerDeleted"), "success");
      await refresh(false);
    } catch (error) {
      toast(errText(error), "error");
    }
  };

  const pruneStopped = async () => {
    if (!stoppedCount()) return toast(t("ctn.noStopped"), "info");
    if (!(await confirmDialog(t("ctn.clearStopped", { n: stoppedCount() }), t("ctn.pruneStoppedDesc", { n: stoppedCount() })))) return;
    try {
      await api.pruneContainers();
      toast(t("toast.pruneStopped"), "success");
      await refresh(false);
    } catch (error) {
      toast(errText(error), "error");
    }
  };

  return (
    <div class="view-section workspace-view">
      <SectionHeader
        title={<>{t("view.containers")} <LiveDot /></>}
        subtitle={t("res.subtitle", { total: snapshot()?.docker.containersTotal ?? 0, running: snapshot()?.docker.containersRunning ?? 0 })}
        actions={
          <>
            <button class="btn-clear" onClick={() => void pruneStopped()}>{t("ctn.clearStopped", { n: stoppedCount() })}</button>
            <label class="filter-toggle">
              <input type="checkbox" checked={showAll()} onChange={(event) => { setShowAll(event.currentTarget.checked); setPage(1); }} />
              {t("ctn.showAll")}
            </label>
          </>
        }
      />

      <div class="master-detail" classList={{ "has-detail": Boolean(current()) }}>
        <section class="master-pane" aria-label={t("aria.containerList")}>
          <Show when={rows().length > 0} fallback={<EmptyState title={showAll() ? t("ctn.noEmpty") : t("ctn.noRunning")} />}>
            <div class="resource-list" ref={listHost}>
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
                    <p>{container.ports || t("ctn.noPublicPort")}</p>
                    <div class="resource-row-actions" onClick={(event) => event.stopPropagation()}>
                      <Show when={container.state === "running"} fallback={<button class="btn-icon" aria-label={t("act.start")} onClick={() => void action(container.id, "start")}><Play size={14} /></button>}>
                        <button class="btn-icon" aria-label={t("act.stop")} onClick={() => void action(container.id, "stop")}><Square size={14} /></button>
                      </Show>
                      <button class="btn-icon" aria-label={t("act.restart")} onClick={() => void action(container.id, "restart")}><RotateCw size={14} /></button>
                      <button class="btn-icon danger" aria-label={t("common.delete")} onClick={() => void remove(container.id)}><Trash2 size={14} /></button>
                    </div>
                  </article>
                )}
              </For>
            </div>
            <Pagination page={page()} total={rows().length} onPageChange={setPage} />
          </Show>
        </section>

        <Show when={current()} fallback={<div class="detail-placeholder"><p>{t("ctn.selectHint")}</p></div>}>
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
