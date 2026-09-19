// YAML 工作台编辑器（上游 code-mirror 等价物）：CodeMirror 6，
// 语法高亮、行号、undo、Tab 缩进；深色控制台岛恒暗；compose/env 切换整体换 doc。
// 校验结果不在此呈现——上游在编辑器下方以纯文本展示（见 Compose.tsx）。
import { createEffect, onCleanup, onMount } from "solid-js";
import { Compartment, EditorState } from "@codemirror/state";
import { EditorView, highlightActiveLine, highlightActiveLineGutter, keymap, lineNumbers } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { yaml } from "@codemirror/lang-yaml";
import { bracketMatching, defaultHighlightStyle, indentOnInput, syntaxHighlighting } from "@codemirror/language";

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
          keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
          indentOnInput(),
          bracketMatching(),
          syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
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
