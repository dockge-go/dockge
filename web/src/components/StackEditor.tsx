// YAML 工作台编辑器（上游 code-mirror 等价物）：CodeMirror 6，
// 语法高亮、行号、undo、Tab 缩进、键名补全、Ctrl+F 搜索、语法错误行内标注；
// 深色控制台岛恒暗；compose/env 切换整体换 doc。
// 引用类错误（未定义网络/卷/依赖）不在此呈现——上游在编辑器下方以纯文本展示（见 Compose.tsx）。
import { createEffect, onCleanup, onMount } from "solid-js";
import { Compartment, EditorState } from "@codemirror/state";
import { EditorView, highlightActiveLine, highlightActiveLineGutter, keymap, lineNumbers } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { bracketMatching, defaultHighlightStyle, indentOnInput, syntaxHighlighting } from "@codemirror/language";
import { autocompletion, type CompletionContext, type CompletionResult } from "@codemirror/autocomplete";
import { searchKeymap } from "@codemirror/search";
import { linter, type Diagnostic } from "@codemirror/lint";
import { yaml } from "@codemirror/lang-yaml";

import { composeDefects } from "../lib/yaml-edit";

/** compose 键名补全：按当前行缩进区分层级——顶层（services/networks/volumes）
 *  与服务内键（image/ports/restart 等）；restart 补全常用取值。 */
const TOP_KEYS = ["services", "networks", "volumes", "name"];
const SERVICE_KEYS = [
  "image", "container_name", "restart", "ports", "volumes", "environment",
  "depends_on", "networks", "healthcheck", "command", "entrypoint", "env_file",
  "labels", "build", "user", "working_dir", "privileged", "profiles",
];
const RESTART_VALUES = ["always", "unless-stopped", "on-failure", "no"];

function composeCompletion(ctx: CompletionContext): CompletionResult | null {
  const line = ctx.state.doc.lineAt(ctx.pos);
  const before = line.text.slice(0, ctx.pos - line.from);
  // 仅在行首起的裸键名处补全（值/列表项等场景不触发）
  if (!/^\s*[\w-]*$/.test(before)) return null;
  const word = ctx.matchBefore(/[\w-]+/);
  const indent = before.length - before.trimStart().length;
  // 层级按 2 空格缩进：0 = 顶层（services/networks/...），4 = 服务内键（image/ports/...）
  if (indent !== 0 && indent !== 4) return null;
  const options = (indent === 0 ? TOP_KEYS : SERVICE_KEYS).map((label) => ({
    label,
    type: indent === 0 ? "keyword" : "property",
    detail: indent === 0 ? "顶层键" : "服务配置",
  }));
  return { from: word ? word.from : ctx.pos, options, validFor: /^[\w-]*$/ };
}

function restartValueCompletion(ctx: CompletionContext): CompletionResult | null {
  const line = ctx.state.doc.lineAt(ctx.pos);
  const m = line.text.slice(0, ctx.pos - line.from).match(/^(\s*restart:\s*)(\w*)$/);
  if (!m) return null;
  const from = line.from + m[1].length;
  return {
    from,
    options: RESTART_VALUES.map((label) => ({ label, type: "enum" })),
    validFor: /^\w*$/,
  };
}

/** 语法错误行内标注（引用类缺陷在编辑器下方呈现，不重复标）。 */
const syntaxLinter = linter(
  (view): Diagnostic[] => {
    const d = composeDefects(view.state.doc.toString());
    if (!d.syntax || d.syntaxLine < 1) return [];
    const line = view.state.doc.line(Math.min(d.syntaxLine, view.state.doc.lines));
    return [{ from: line.from, to: line.to, severity: "error", message: d.syntax }];
  },
  { delay: 300 },
);

// 深色控制台岛主题：背景透明（由 .stack-editor-surface 提供），高亮用默认配色。
const consoleTheme = EditorView.theme(
  {
    "&": { backgroundColor: "transparent", color: "#e8e8ed", fontSize: "14px", height: "100%" },
    ".cm-scroller": { fontFamily: '"SF Mono", ui-monospace, "JetBrains Mono", Menlo, Consolas, monospace', lineHeight: "1.65" },
    ".cm-gutters": { backgroundColor: "transparent", color: "#8e8e93", border: "none", opacity: "0.6" },
    ".cm-activeLine": { backgroundColor: "rgba(255,255,255,0.05)" },
    ".cm-activeLineGutter": { backgroundColor: "transparent", color: "#e8e8ed", opacity: "1" },
    ".cm-content": { caretColor: "#16a34a", paddingBottom: "16px" },
    "&.cm-focused": { outline: "none" },
    ".cm-cursor, .cm-dropCursor": { borderLeftColor: "#16a34a" },
    ".cm-selectionBackground": { backgroundColor: "rgba(41,151,255,0.25)" },
  },
  { dark: true },
);

export function StackEditor(props: {
  file: "compose" | "env";
  yaml: string;
  env: string;
  readonly: boolean;
  onYamlChange: (value: string) => void;
  onEnvChange: (value: string) => void;
}) {
  let host: HTMLDivElement | undefined;
  let view: EditorView | undefined;
  const languageConf = new Compartment();
  const readonlyConf = new Compartment();

  const content = () => (props.file === "compose" ? props.yaml : props.env);

  onMount(() => {
    if (!host) return;
    view = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: content(),
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightActiveLine(),
          history(),
          keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap, indentWithTab]),
          indentOnInput(),
          bracketMatching(),
          syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
          autocompletion({ override: [composeCompletion, restartValueCompletion] }),
          syntaxLinter,
          consoleTheme,
          languageConf.of(props.file === "compose" ? yaml() : []),
          readonlyConf.of(EditorState.readOnly.of(props.readonly)),
          EditorView.updateListener.of((update) => {
            if (!update.docChanged) return;
            const value = update.state.doc.toString();
            if (props.file === "compose") props.onYamlChange(value);
            else props.onEnvChange(value);
          }),
        ],
      }),
    });
  });

  onCleanup(() => view?.destroy());

  // 外部内容变化（打开别的栈 / composerize 转换落入）→ 整体替换 doc；
  // 自身输入引起的变化经比较后跳过，避免回环。
  createEffect(() => {
    const value = content();
    if (view && view.state.doc.toString() !== value) {
      view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } });
    }
  });

  // compose ↔ env 切换：换语言高亮（doc 已由上面的 effect 同步）
  createEffect(() => {
    const file = props.file;
    if (!view) return;
    view.dispatch({
      effects: languageConf.reconfigure(file === "compose" ? yaml() : []),
    });
  });

  // 只读态（外部栈）热切换
  createEffect(() => {
    const ro = props.readonly;
    if (!view) return;
    view.dispatch({ effects: readonlyConf.reconfigure(EditorState.readOnly.of(ro)) });
  });

  return (
    <section class="stack-editor">
      <div class="stack-editor-surface" ref={host} />
    </section>
  );
}
