import { Show, createEffect } from 'solid-js';
import { app, setApp, dialogOk, dialogCancel, t } from '../store';

/* macOS 警告框：默认按钮响应回车 —— 打开即聚焦确认按钮（仅精确指针设备，避免弹软键盘） */
export default function Dialog() {
  let okBtn: HTMLButtonElement | undefined;
  createEffect(() => {
    if (app.dialog.open && okBtn && matchMedia('(pointer: fine)').matches) {
      okBtn.focus();
    }
  });

  return (
    <div class="dialog-overlay" classList={{ open: app.dialog.open }} role="alertdialog" aria-modal="true"
         onClick={e => { if (e.target === e.currentTarget) dialogCancel(); }}>
      <div class="dialog-alert">
        <div class="dialog-message">{app.dialog.message}</div>
        <Show when={app.dialog.mode === 'prompt'}>
          <input class="form-input dialog-input" value={app.dialog.value} placeholder={app.dialog.placeholder}
                 onInput={e => setApp('dialog', 'value', e.currentTarget.value)}
                 onKeyDown={e => { if (e.key === 'Enter') dialogOk(); if (e.key === 'Escape') dialogCancel(); }} />
        </Show>
        <div class="dialog-actions">
          <button type="button" class="dialog-btn" onClick={dialogCancel}>{t('common.cancel')}</button>
          <button type="button" class="dialog-btn" ref={okBtn} classList={{ destructive: app.dialog.danger }} onClick={dialogOk}>
            {app.dialog.okLabel || t('common.ok')}
          </button>
        </div>
      </div>
    </div>
  );
}
