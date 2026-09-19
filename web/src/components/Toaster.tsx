import { For } from "solid-js";
import { dismissToast, toasts } from "../store/index";

export function Toaster() {
  return (
    <div class="toast-container" aria-live="polite">
      <For each={toasts()}>
        {(t) => (
          <div
            class={`toast ${t.type}`}
            role={t.type === "error" ? "alert" : "status"}
            onClick={() => dismissToast(t.id)}
          >
            {t.message}
          </div>
        )}
      </For>
    </div>
  );
}
