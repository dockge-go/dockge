// 镜像页：列表（使用中标记）+ 批量勾选删除 + 手动拉取（流式进度终端）。
// 上游无此页（有意偏差）：本机镜像的轻量管理入口。
import { For, Show, createSignal, onMount } from "solid-js";
import { CloudDownload, Trash2 } from "lucide-solid";

import { api, type ImageItem } from "../api/api";
import { errText } from "../api/format";
import { confirmDialog } from "../components/Confirm";
import { DisplayTerminal } from "../components/Terminal";
import { t } from "../i18n";
import { toast } from "../store/index";

/** 镜像的删除/拉取引用：tag 为 <none> 时只能用 ID。 */
function refOf(img: ImageItem): string {
  return img.tag === "<none>" ? img.id : `${img.repository}:${img.tag}`;
}

export function Images() {
  const [images, setImages] = createSignal<ImageItem[]>([]);
  const [selected, setSelected] = createSignal<Set<string>>(new Set<string>());
  const [query, setQuery] = createSignal("");
  const [pullRef, setPullRef] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [output, setOutput] = createSignal("");

  const load = async () => {
    try {
      setImages((await api.images()).list);
      setSelected(new Set<string>());
    } catch (error) {
      toast(errText(error), "error");
    }
  };
  onMount(() => void load());

  // 镜像名模糊查询（repository:tag 子串，忽略大小写）
  const filtered = () => {
    const q = query().trim().toLowerCase();
    if (!q) return images();
    return images().filter((img) => `${img.repository}:${img.tag}`.toLowerCase().includes(q));
  };

  const toggle = (ref: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(ref)) next.delete(ref);
      else next.add(ref);
      return next;
    });

  const del = async () => {
    if (busy() || selected().size === 0) return;
    const count = selected().size;
    if (!(await confirmDialog(t("images.confirmDelete"), t("images.confirmDeleteDesc", { count })))) return;
    setBusy(true);
    try {
      const result = await api.deleteImages([...selected()]);
      setOutput(result.output || "");
      await load();
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
    } finally {
      setBusy(false);
    }
  };

  const pull = async () => {
    const ref = pullRef().trim();
    if (!ref || busy()) return;
    setBusy(true);
    setOutput(`$ docker pull ${ref}\n`);
    try {
      await api.pullImageStream(ref, (chunk) => setOutput((prev) => prev + chunk));
      await load();
    } catch (error) {
      setOutput((prev) => prev + `\n[error] ${errText(error)}\n`);
      toast(errText(error), "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <div class="img-toolbar">
        <input
          class="form-input mono"
          placeholder={t("images.pullPlaceholder")}
          aria-label={t("images.pullPlaceholder")}
          value={pullRef()}
          onInput={(e) => setPullRef(e.currentTarget.value)}
          onKeyDown={(e) => e.key === "Enter" && void pull()}
        />
        <button class="btn btn-primary" disabled={busy() || !pullRef().trim()} onClick={() => void pull()}>
          <CloudDownload size={14} /> {t("images.pull")}
        </button>
        <span class="img-toolbar-spacer" />
        <input
          class="form-input img-search"
          placeholder={t("images.search")}
          aria-label={t("images.search")}
          value={query()}
          onInput={(e) => setQuery(e.currentTarget.value)}
        />
        <button class="btn btn-danger" disabled={busy() || selected().size === 0} onClick={() => void del()}>
          <Trash2 size={14} /> {t("images.deleteSelected", { count: selected().size })}
        </button>
      </div>

      <Show when={output()}>
        <div class="compose-progress">
          <DisplayTerminal content={output()} rows={8} />
        </div>
      </Show>

      <div class="card">
        <Show when={filtered().length > 0} fallback={<p class="empty-hint">{t("images.empty")}</p>}>
          <table class="img-table">
            <thead>
              <tr>
                <th></th>
                <th>{t("images.name")}</th>
                <th>{t("images.id")}</th>
                <th>{t("images.size")}</th>
                <th>{t("images.created")}</th>
                <th>{t("images.status")}</th>
              </tr>
            </thead>
            <tbody>
              <For each={filtered()}>
                {(img) => {
                  const ref = () => refOf(img);
                  const checked = () => selected().has(ref());
                  return (
                    <tr>
                      <td>
                        <input
                          type="checkbox"
                          aria-label={ref()}
                          checked={checked()}
                          onChange={() => toggle(ref())}
                        />
                      </td>
                      <td class="mono">{ref()}</td>
                      <td class="mono muted">{img.id}</td>
                      <td>{img.size}</td>
                      <td>{img.createdSince}</td>
                      <td>
                        <span class={`status-pill ${img.inUse ? "active" : "exited"}`}>
                          {img.inUse ? t("images.inUse") : t("images.unused")}
                        </span>
                      </td>
                    </tr>
                  );
                }}
              </For>
            </tbody>
          </table>
        </Show>
      </div>
    </div>
  );
}
