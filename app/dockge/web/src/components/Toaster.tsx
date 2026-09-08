import { For } from "solid-js";
import { toasts } from "../store/index";

export function Toaster() {
  return (
    <div class="toast-container" aria-live="polite">
      <For each={toasts()}>{(t) => <div class={`toast ${t.type}`}>{t.message}</div>}</For>
    </div>
  );
}
