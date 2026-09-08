// Promise 风格确认框：Kobalte AlertDialog 封装（原型 confirm() 的等价物）。
import { AlertDialog } from "@kobalte/core/alert-dialog";
import { createSignal } from "solid-js";

interface ConfirmReq {
  title: string;
  desc: string;
  danger: boolean;
  resolve: (v: boolean) => void;
}

const [req, setReq] = createSignal<ConfirmReq | null>(null);

export function confirmDialog(title: string, desc: string, danger = true): Promise<boolean> {
  return new Promise((resolve) => setReq({ title, desc, danger, resolve }));
}

export function ConfirmHost() {
  const close = (v: boolean) => {
    req()?.resolve(v);
    setReq(null);
  };
  return (
    <AlertDialog open={!!req()} onOpenChange={(o) => !o && close(false)}>
      <AlertDialog.Portal>
        <AlertDialog.Overlay class="confirm-overlay" />
        <AlertDialog.Content class="confirm-modal">
          <AlertDialog.Title class="confirm-title">{req()?.title}</AlertDialog.Title>
          <AlertDialog.Description class="confirm-desc">{req()?.desc}</AlertDialog.Description>
          <div class="confirm-actions">
            <AlertDialog.CloseButton class="btn btn-secondary">取消</AlertDialog.CloseButton>
            <button
              class={`btn ${req()?.danger ? "btn-danger" : "btn-primary"}`}
              onClick={() => close(true)}
            >
              确认
            </button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Portal>
    </AlertDialog>
  );
}
