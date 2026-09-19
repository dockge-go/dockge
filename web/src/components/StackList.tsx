// 栈列表侧栏（上游复刻）：搜索框 + 状态 pill + 栈名；外部栈半透明。
// 直接订阅 store 的 snapshot（与首页统计相同的响应式路径）。
import { For, Show, createMemo, createSignal } from "solid-js";
import { A, useLocation } from "@solidjs/router";
import { Search, X } from "lucide-solid";

import { t } from "../i18n";
import { snapshot } from "../store/index";

function statusClass(status: number): string {
  // 0 未知 / 1 未部署 / 2 已创建 / 3 运行中 / 4 已停止
  if (status === 3) return "active";
  if (status === 4) return "exited";
  return "";
}

function statusLabel(status: number): string {
  if (status === 3) return t("home.active");
  if (status === 4 || status === 2) return t("home.exited");
  return t("home.inactive");
}

// 上游排序权重：运行中 > 已退出 > 已创建 > 未部署/未知
function statusRank(status: number): number {
  if (status === 3) return 0;
  if (status === 4) return 1;
  if (status === 2) return 2;
  return 3;
}

export function StackList() {
  const [query, setQuery] = createSignal("");
  const location = useLocation();
  const selected = createMemo(() => decodeURIComponent(location.pathname.replace(/^\/compose\//, "")));
  const stacks = createMemo(() => snapshot()?.stacks ?? []);

  // 上游排序：托管栈优先 → 状态（运行中 > 已退出 > 已创建）→ 名称字典序
  const sorted = createMemo(() =>
    [...stacks()].sort((a, b) =>
      Number(b.managed) - Number(a.managed) ||
      statusRank(a.status) - statusRank(b.status) ||
      a.name.localeCompare(b.name),
    ),
  );

  const visible = createMemo(() => {
    const q = query().trim().toLowerCase();
    return q ? sorted().filter((s) => s.name.toLowerCase().includes(q)) : sorted();
  });

  return (
    <div>
      <div class="stacklist-search">
        <Search size={15} class="icon" />
        <input
          value={query()}
          placeholder={t("stacklist.searchPlaceholder")}
          aria-label={t("stacklist.searchPlaceholder")}
          onInput={(e) => setQuery(e.currentTarget.value)}
        />
        <Show when={query()}>
          <button class="btn-icon" style={{ position: "absolute", right: "4px", top: "4px", border: "none" }} aria-label={t("common.close")} onClick={() => setQuery("")}>
            <X size={14} />
          </button>
        </Show>
      </div>
      <div class="stacklist-items">
        <Show
          when={visible().length > 0}
          fallback={
            <A class="stacklist-empty" href="/compose">
              {t("stacklist.empty")}
            </A>
          }
        >
          <For each={visible()}>
            {(stack) => (
              <A class={`stacklist-item ${stack.managed ? "" : "external"}`} classList={{ selected: selected() === stack.name }} href={`/compose/${encodeURIComponent(stack.name)}`}>
                <span class={`status-pill ${statusClass(stack.status)}`}>{statusLabel(stack.status)}</span>
                <span class="stacklist-name" title={stack.name}>{stack.name}</span>
              </A>
            )}
          </For>
        </Show>
      </div>
    </div>
  );
}
