import { For } from 'solid-js';
import { toasts } from '../store';

export default function Toasts() {
  return (
    <div class="toast-container">
      <For each={toasts()}>{x => <div class="toast" classList={{ [x.type]: true }}>{x.msg}</div>}</For>
    </div>
  );
}
