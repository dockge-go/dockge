// 右侧滑出面板：Kobalte Dialog 封装（原型 .sheet-overlay/.sheet）。
import { Dialog } from "@kobalte/core/dialog";
import { X } from "lucide-solid";
import { Show, type JSX } from "solid-js";
import { t } from "../i18n";

export function Sheet(props: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  children: JSX.Element;
  footer?: JSX.Element;
}) {
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay class="sheet-overlay" />
        <Dialog.Content class="sheet">
          <div class="sheet-header">
            <Dialog.Title class="sheet-title">{props.title}</Dialog.Title>
            <Dialog.CloseButton class="btn-icon" aria-label={t("common.close")}>
              <X size={18} />
            </Dialog.CloseButton>
          </div>
          <div class="sheet-body">{props.children}</div>
          <Show when={props.footer}>
            <div class="sheet-footer">{props.footer}</div>
          </Show>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog>
  );
}
