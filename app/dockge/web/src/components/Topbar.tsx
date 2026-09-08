// 顶栏：汉堡（移动端）+ 页面标题 + 全局搜索（下拉 + 键盘导航 + Cmd/Ctrl+K）+ 刷新。
import { For, Show, createMemo, createSignal, onCleanup, onMount } from "solid-js";
import { useLocation, useNavigate } from "@solidjs/router";
import { Menu, RefreshCw, Search } from "lucide-solid";
import { refresh, snapshot, setPendingContainer, setPendingStack } from "../store/index";

const TITLES: Record<string, string> = {
  "/": "Dashboard",
  "/containers": "Containers",
  "/stacks": "Stacks",
  "/images": "Images",
  "/volumes": "Volumes",
  "/networks": "Networks",
  "/sysinfo": "System Info",
  "/sysdf": "Disk Usage",
  "/settings": "Settings",
};

interface SearchHit {
  group: string;
  name: string;
  meta: string;
  go: () => void;
}

export function Topbar(props: { onHamburger: () => void }) {
  const navigate = useNavigate();
  const location = useLocation();
  const [query, setQuery] = createSignal("");
  const [open, setOpen] = createSignal(false);
  const [activeIdx, setActiveIdx] = createSignal(-1);
  let inputRef: HTMLInputElement | undefined;

  const title = () => {
    const seg = "/" + (location.pathname.split("/")[1] ?? "");
    return TITLES[seg] ?? "Dockge";
  };

  const results = createMemo<SearchHit[]>(() => {
    const q = query().trim().toLowerCase();
    if (!q) return [];
    const snap = snapshot();
    if (!snap) return [];
    const hits: SearchHit[] = [];
    for (const c of snap.containers) {
      if (
        c.name.toLowerCase().includes(q) ||
        c.image.toLowerCase().includes(q) ||
        c.id.toLowerCase().includes(q)
      ) {
        hits.push({
          group: "Containers",
          name: c.name,
          meta: c.image,
          go: () => {
            navigate("/containers");
            setPendingContainer(c.id);
          },
        });
      }
    }
    for (const s of snap.stacks) {
      if (s.name.toLowerCase().includes(q)) {
        hits.push({
          group: "Stacks",
          name: s.name,
          meta: s.statusLabel,
          go: () => {
            navigate("/stacks");
            setPendingStack(s.name);
          },
        });
      }
    }
    for (const img of snap.images) {
      const ref = `${img.repo}:${img.tag}`;
      if (ref.toLowerCase().includes(q) || img.id.toLowerCase().includes(q)) {
        hits.push({ group: "Images", name: ref, meta: img.size, go: () => navigate("/images") });
      }
    }
    for (const v of snap.volumes) {
      if (v.name.toLowerCase().includes(q)) {
        hits.push({ group: "Volumes", name: v.name, meta: v.driver, go: () => navigate("/volumes") });
      }
    }
    return hits;
  });

  const grouped = createMemo(() => {
    const map = new Map<string, SearchHit[]>();
    for (const hit of results()) {
      const list = map.get(hit.group) ?? [];
      list.push(hit);
      map.set(hit.group, list);
    }
    return [...map.entries()];
  });

  const pick = (hit: SearchHit) => {
    hit.go();
    setOpen(false);
    setQuery("");
    inputRef?.blur();
  };

  const onKeydown = (e: KeyboardEvent) => {
    if (!open() || results().length === 0) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIdx((i) => Math.min(i + 1, results().length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIdx((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter" && activeIdx() >= 0) {
      e.preventDefault();
      pick(results()[activeIdx()]);
    } else if (e.key === "Escape") {
      setOpen(false);
      inputRef?.blur();
    }
  };

  onMount(() => {
    const global = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        inputRef?.focus();
      }
    };
    document.addEventListener("keydown", global);
    onCleanup(() => document.removeEventListener("keydown", global));
  });

  return (
    <header class="topbar">
      <button class="hamburger" aria-label="切换导航" aria-expanded="true" onClick={props.onHamburger}>
        <Menu size={20} />
      </button>
      <h1 class="topbar-title">{title()}</h1>
      <div class="topbar-spacer" />
      <div class="topbar-actions">
        <div class="search-box">
          <Search size={16} class="search-box-icon" />
          <input
            ref={inputRef}
            type="search"
            placeholder="搜索…"
            aria-label="全局搜索"
            autocomplete="off"
            value={query()}
            onInput={(e) => {
              setQuery(e.currentTarget.value);
              setOpen(!!e.currentTarget.value.trim());
              setActiveIdx(-1);
            }}
            onKeyDown={onKeydown}
            onBlur={() => setTimeout(() => setOpen(false), 150)}
          />
          <Show when={open() && results().length > 0}>
            <div class="search-dropdown">
              <For each={grouped()}>
                {([group, hits]) => (
                  <div class="search-group">
                    <div class="search-group-label">{group}</div>
                    <For each={hits}>
                      {(hit) => (
                        <div
                          class="search-result-item"
                          classList={{ active: results().indexOf(hit) === activeIdx() }}
                          onMouseDown={(e) => e.preventDefault()}
                          onClick={() => pick(hit)}
                        >
                          <span class="sr-name">{hit.name}</span>
                          <span class="sr-meta">{hit.meta}</span>
                        </div>
                      )}
                    </For>
                  </div>
                )}
              </For>
            </div>
          </Show>
        </div>
        <button class="btn-icon" aria-label="刷新" title="Refresh" onClick={() => void refresh()}>
          <RefreshCw size={18} />
        </button>
      </div>
    </header>
  );
}
