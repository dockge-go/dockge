// 单框镜像 combobox：可自由输入镜像引用（填空），聚焦或点箭头展开本地镜像
// 下拉（输入即时过滤，点击回填）——输入与选择合一，不拆两个控件。
// 每次展开经 onOpen 请求最新本地列表（数据由页面持有，含去抖），镜像页拉取后立即可见。
import { For, Show, createSignal } from "solid-js";
import { ChevronDown } from "lucide-solid";

import { t } from "../i18n";

export function ImageCombo(props: {
  value: string;
  images: string[];
  id?: string;
  onOpen?: () => void;
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = createSignal(false);
  const [filter, setFilter] = createSignal("");

  // 子串过滤（忽略大小写）；当前仓库的其它 tag 排前，便于版本切换
  const options = () => {
    const q = filter().trim().toLowerCase();
    const repo = props.value.trim().replace(/:[^:/]+$/, "");
    const list = q ? props.images.filter((ref) => ref.toLowerCase().includes(q)) : props.images;
    return [...list]
      .sort((a, b) =>
        Number(b.toLowerCase().startsWith(repo + ":")) - Number(a.toLowerCase().startsWith(repo + ":")))
      .slice(0, 50);
  };

  const pick = (ref: string) => {
    props.onChange(ref);
    setFilter("");
    setOpen(false);
  };

  const expand = (next: boolean) => {
    if (next) props.onOpen?.();
    setOpen(next);
  };

  return (
    <div class="image-combo">
      <input
        id={props.id}
        class="form-input mono"
        value={props.value}
        onFocus={() => { setFilter(""); expand(true); }}
        onBlur={() => setOpen(false)}
        onInput={(e) => {
          setFilter(e.currentTarget.value);
          setOpen(true);
          props.onChange(e.currentTarget.value);
        }}
        onKeyDown={(e) => e.key === "Escape" && setOpen(false)}
      />
      <button
        type="button"
        class="image-combo-toggle"
        aria-label="toggle"
        tabIndex={-1}
        onMouseDown={(e) => {
          e.preventDefault(); // 不抢输入框焦点，仅切换面板
          setFilter("");
          expand(!open());
        }}
      >
        <ChevronDown size={14} />
      </button>
      <Show when={open()}>
        <ul class="image-combo-panel" onMouseDown={(e) => e.preventDefault()}>
          <Show
            when={options().length > 0}
            fallback={
              <li class="image-combo-hint">
                {filter().trim() ? t("form.imageNotLocal") : t("images.empty")}
              </li>
            }
          >
            <For each={options()}>
              {(ref) => (
                <li>
                  <button type="button" class="image-combo-option mono" onClick={() => pick(ref)}>{ref}</button>
                </li>
              )}
            </For>
          </Show>
        </ul>
      </Show>
    </div>
  );
}
