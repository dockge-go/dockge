// 上游 ArrayInput/ArraySelect 的等价物：字符串行列表（每行输入框/下拉 + 移除 ×）
// 与「添加 {名称}」按钮；未初始化（undefined）时只显示添加按钮。
// select 变体用于受限选项（网络、depends_on 等）。
import { For, createEffect } from "solid-js";
import { X } from "lucide-solid";

import { t } from "../i18n";

export function ArrayField(props: {
  displayName: string;
  placeholder?: string;
  rows: string[] | undefined;
  onChange: (rows: string[] | undefined) => void;
}) {
  const add = () => props.onChange([...(props.rows ?? []), ""]);
  const remove = (index: number) => {
    const next = [...(props.rows ?? [])];
    next.splice(index, 1);
    props.onChange(next);
  };
  const update = (index: number, value: string) => {
    const next = [...(props.rows ?? [])];
    next[index] = value;
    props.onChange(next);
  };

  return (
    <div>
      <ul class="array-field-list">
        <For each={props.rows ?? []}>
          {(value, index) => (
            <li>
              <input
                class="form-input"
                type="text"
                placeholder={props.placeholder ?? ""}
                aria-label={props.displayName}
                value={value}
                onInput={(e) => update(index(), e.currentTarget.value)}
              />
              <button class="btn-icon danger" aria-label={t("common.close")} onClick={() => remove(index())}>
                <X size={14} />
              </button>
            </li>
          )}
        </For>
      </ul>
      <button class="btn btn-sm btn-secondary" type="button" onClick={add}>
        {t("form.addListItem", { name: props.displayName })}
      </button>
    </div>
  );
}

/** 单行下拉。Solid 中 <select value> 的 prop 设置早于 <For> 渲染 options，
 *  值会落空（表现为"选完立刻弹回占位"）；经 ref + effect 在 DOM 更新后同步。 */
function RowSelect(props: {
  value: string;
  options: string[];
  displayName: string;
  placeholder: string;
  onPick: (value: string) => void;
}) {
  let el!: HTMLSelectElement;
  createEffect(() => {
    el.value = props.value;
  });
  return (
    <select
      ref={el}
      class="form-select"
      aria-label={props.displayName}
      onChange={(e) => props.onPick(e.currentTarget.value)}
    >
      <option value="">{props.placeholder}</option>
      <For each={props.options}>{(option) => <option value={option}>{option}</option>}</For>
    </select>
  );
}

export function ArraySelectField(props: {
  displayName: string;
  options: string[];
  rows: string[] | undefined;
  onChange: (rows: string[] | undefined) => void;
  placeholder?: string;
}) {
  const add = () => props.onChange([...(props.rows ?? []), ""]);
  const remove = (index: number) => {
    const next = [...(props.rows ?? [])];
    next.splice(index, 1);
    props.onChange(next);
  };
  const update = (index: number, value: string) => {
    const next = [...(props.rows ?? [])];
    next[index] = value;
    props.onChange(next);
  };
  const placeholder = () => props.placeholder ?? t("form.selectNetwork");

  return (
    <div>
      <ul class="array-field-list">
        <For each={props.rows ?? []}>
          {(value, index) => (
            <li>
              <RowSelect
                value={value}
                options={props.options}
                displayName={props.displayName}
                placeholder={placeholder()}
                onPick={(picked) => update(index(), picked)}
              />
              <button class="btn-icon danger" aria-label={t("common.close")} onClick={() => remove(index())}>
                <X size={14} />
              </button>
            </li>
          )}
        </For>
      </ul>
      <button class="btn btn-sm btn-secondary" type="button" onClick={add}>
        {t("form.addListItem", { name: props.displayName })}
      </button>
    </div>
  );
}
