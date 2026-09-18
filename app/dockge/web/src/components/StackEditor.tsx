// YAML 工作台编辑器：CodeMirror 6（语法高亮、行号、当前行、undo、Tab 缩进）。
// 深色控制台岛恒暗；compose/env 共用同一实例，切换文件时整体换 doc。
// 校验闭环被动呈现：后端草稿校验结果注入 lint 诊断（gutter/悬停/面板，
// 交互全部复用 CodeMirror 自带能力），工具栏指示检查中/通过/错误数。
import { Show, createEffect, onCleanup, onMount } from "solid-js";
import { AlertTriangle, Check, Loader2, Maximize2, Minimize2 } from "lucide-solid";
import { Compartment, EditorState } from "@codemirror/state";
import { EditorView, highlightActiveLine, highlightActiveLineGutter, keymap, lineNumbers } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { yaml } from "@codemirror/lang-yaml";
import { bracketMatching, defaultHighlightStyle, indentOnInput, syntaxHighlighting } from "@codemirror/language";
import { linter, lintGutter, setDiagnostics } from "@codemirror/lint";
import { t } from "../i18n";
import { toDiagnostics, type ValidationState } from "../lib/validate";

// 深色控制台岛主题：背景透明（由 .stack-editor-surface 提供），高亮用默认配色。
const consoleTheme = EditorView.theme(
  {
    "&": { backgroundColor: "transparent", color: "#e8e8ed", fontSize: "13px", height: "100%" },
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
  fullscreen: boolean;
  validation: ValidationState;
  output: string;
  onYamlChange: (value: string) => void;
  onEnvChange: (value: string) => void;
  onFullscreenChange: (value: boolean) => void;
}) {
  let host: HTMLDivElement | undefined;
  let view: EditorView | undefined;
  const languageConf = new Compartment();
  const readonlyConf = new Compartment();

  const content = () => (props.file === "compose" ? props.yaml : props.env);
  const result = () => (props.validation.status === "done" ? props.validation.result : null);

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
          keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
          indentOnInput(),
          bracketMatching(),
          syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
          // 空载 linter 启用 lint 状态，诊断经 setDiagnostics 由外部注入
          linter(() => []),
          lintGutter(),
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

  // 校验诊断注入：compose 文件显示后端结果；切到 .env 时清空
  // （同一实例换 doc，compose 的偏移量对 env 文本无意义）。
  createEffect(() => {
    const done = props.file === "compose" ? result() : null;
    const errors = done && !done.valid ? done.errors : [];
    if (view) view.dispatch(setDiagnostics(view.state, toDiagnostics(errors, props.yaml)));
  });

  const validationClass = () => (result()?.valid ? "valid" : props.validation.status === "checking" ? "checking" : "invalid");
  const validationText = () =>
    props.validation.status === "checking"
      ? t("editor.checking")
      : result()?.valid
        ? t("editor.validCompose")
        : t("editor.invalidCompose", { n: result()?.errors.length ?? 0 });

  return (
    <section class={`stack-editor ${props.fullscreen ? "editor-fullscreen" : ""}`}>
      <header class="stack-editor-toolbar">
        <span class={`editor-validation ${validationClass()}`}>
          {result()?.valid ? (
            <Check size={13} />
          ) : props.validation.status === "checking" ? (
            <Loader2 size={13} class="spin" />
          ) : (
            <AlertTriangle size={13} />
          )}
          {validationText()}
        </span>
        <span class="toolbar-spacer" />
        <button class="btn btn-ghost" onClick={() => props.onFullscreenChange(!props.fullscreen)}>
          <Show when={props.fullscreen} fallback={<Maximize2 size={13} />}><Minimize2 size={13} /></Show>
          {props.fullscreen ? t("editor.exitFullscreen") : t("editor.fullscreen")}
        </button>
      </header>
      <div class="stack-editor-surface" ref={host} />
      <footer class="stack-editor-status">
        <span>{props.file === "compose" ? "compose.yaml" : ".env"}</span>
        <span>{t("editor.lines", { n: content().split("\n").length })}</span>
        <Show when={props.readonly}><span>{t("editor.readonly")}</span></Show>
      </footer>
      <Show when={props.output}>
        <pre class="operation-output">{props.output}</pre>
      </Show>
    </section>
  );
}
