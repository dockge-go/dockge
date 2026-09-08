import { Show, createMemo } from "solid-js";
import { Check, Maximize2, Minimize2 } from "lucide-solid";

export function StackEditor(props: {
  file: "compose" | "env";
  yaml: string;
  env: string;
  readonly: boolean;
  fullscreen: boolean;
  output: string;
  onYamlChange: (value: string) => void;
  onEnvChange: (value: string) => void;
  onFullscreenChange: (value: boolean) => void;
}) {
  const content = () => (props.file === "compose" ? props.yaml : props.env);
  const lineNumbers = createMemo(() => content().split("\n").map((_, index) => index + 1).join("\n"));
  const valid = createMemo(() => props.yaml.split("\n").some((line) => line.trim() === "services:"));

  const update = (value: string) => {
    if (props.file === "compose") props.onYamlChange(value);
    else props.onEnvChange(value);
  };

  return (
    <section class={`stack-editor ${props.fullscreen ? "editor-fullscreen" : ""}`}>
      <header class="stack-editor-toolbar">
        <span class={`editor-validation ${valid() ? "valid" : "invalid"}`}>
          <Check size={13} /> {valid() ? "YAML 结构有效" : "缺少 services 段"}
        </span>
        <span class="toolbar-spacer" />
        <button class="btn btn-ghost" onClick={() => props.onFullscreenChange(!props.fullscreen)}>
          <Show when={props.fullscreen} fallback={<Maximize2 size={13} />}><Minimize2 size={13} /></Show>
          {props.fullscreen ? "退出全屏" : "全屏"}
        </button>
      </header>
      <div class="stack-editor-surface">
        <pre class="stack-editor-lines">{lineNumbers()}</pre>
        <textarea
          aria-label={props.file === "compose" ? "Compose YAML" : "环境变量"}
          spellcheck={false}
          readOnly={props.readonly}
          value={content()}
          onInput={(event) => update(event.currentTarget.value)}
        />
      </div>
      <footer class="stack-editor-status">
        <span>{props.file === "compose" ? "compose.yaml" : ".env"}</span>
        <span>{content().split("\n").length} 行</span>
        <Show when={props.readonly}><span>只读</span></Show>
      </footer>
      <Show when={props.output}>
        <pre class="operation-output">{props.output}</pre>
      </Show>
    </section>
  );
}
