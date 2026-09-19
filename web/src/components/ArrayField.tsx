// 上游 ArrayInput/ArraySelect 的等价物：字符串行列表（每行输入框/下拉 + 移除 ×）
// 与「添加 {名称}」按钮；未初始化（undefined）时只显示添加按钮。
// select 变体用于网络（选项来自 compose 顶层 networks 声明）。
import { For } from "solid-js";
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

export function ArraySelectField(props: {
  displayName: string;
  options: string[];
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
              <select
                class="form-select"
                aria-label={props.displayName}
                value={value}
                onChange={(e) => update(index(), e.currentTarget.value)}
              >
                <option value="">{t("form.selectNetwork")}</option>
                <For each={props.options}>{(option) => <option value={option}>{option}</option>}</For>
              </select>
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
