import { For, Show } from 'solid-js';
import {
  app, setApp, authMode, changePassword, changePwChecks, refreshData, t
} from '../store';

const cardStyle = 'background:var(--panel);border-radius:var(--radius-card);box-shadow:var(--shadow-card);padding:var(--space-5)';

export default function Settings() {
  return (
    <div class="view-section">
      <h2 class="section-title" style="margin-bottom:var(--space-6)">{t('nav.settings')}</h2>
      <div style="max-width:560px;display:flex;flex-direction:column;gap:var(--space-6)">

        <div style={cardStyle}>
          <h3 style="margin:0 0 var(--space-4);font-size:var(--text-base);font-weight:600">{t('set.engineConnection')}</h3>
          <p class="form-help">{app.deployment || t('data.unknown')}</p>
          <p class="form-help">{t('set.deploymentHelp')}</p>
          <button type="button" class="btn btn-secondary" disabled={app.loading} onClick={() => void refreshData()}>{t('common.refresh')}</button>
        </div>

        <Show when={authMode() === 'oidc'}>
          <div style={cardStyle}>
            <h3>{t('auth.accountSection')}</h3>
            <p class="form-help">{t('auth.oidcAccount')}</p>
          </div>
        </Show>
        <Show when={authMode() === 'local'}>
        {/* 账户与安全：修改密码 */}
        <div style={cardStyle}>
          <h3 style="margin:0 0 var(--space-4);font-size:var(--text-base);font-weight:600">{t('auth.accountSection')}</h3>
          <div class="form-group">
            <label class="form-label">{t('auth.currentPassword')}</label>
            <input class="form-input" type="password" autocomplete="current-password" value={app.pwForm.current}
                   onInput={e => setApp('pwForm', 'current', e.currentTarget.value)} />
          </div>
          <div class="form-group">
            <label class="form-label">{t('auth.newPassword')}</label>
            <input class="form-input" type="password" autocomplete="new-password" value={app.pwForm.next}
                   onInput={e => setApp('pwForm', 'next', e.currentTarget.value)} />
          </div>
          <div class="form-group">
            <label class="form-label">{t('auth.confirmPassword')}</label>
            <input class="form-input" type="password" autocomplete="new-password" value={app.pwForm.confirm}
                   onInput={e => setApp('pwForm', 'confirm', e.currentTarget.value)}
                   onKeyDown={e => { if (e.key === 'Enter') changePassword(); }} />
          </div>
          <Show when={app.pwForm.next}>
            <ul class="pw-checklist">
              <For each={changePwChecks()}>{c => (
                <li classList={{ ok: c.ok }}><span class="pw-check-dot">{c.ok ? '✓' : '·'}</span><span>{c.label}</span></li>
              )}</For>
            </ul>
          </Show>
          <Show when={app.pwForm.error}><p class="auth-error">{app.pwForm.error}</p></Show>
          <p class="form-help">{t('auth.pwHint')}</p>
          <button type="button" class="btn btn-primary" disabled={app.pwForm.busy} onClick={changePassword}>{t('auth.submitPw')}</button>
        </div>

        </Show>

        <div style="background:var(--surface);border:1px solid var(--border-soft);border-radius:var(--radius-lg);padding:var(--space-5)">
          <h3 style="margin:0 0 var(--space-3);font-size:var(--text-base);font-weight:600">{t('set.about')}</h3>
          <div style="font-size:var(--text-sm);color:var(--fg-2);line-height:1.8">
            <div><strong>Dockge</strong> — {t('app.title').split('—')[1]?.trim() || ''}</div>
            <div style="margin-top:var(--space-3);color:var(--muted)">{t('set.version')}: {app.version || t('data.unknown')}</div>
          <Show when={app.version && app.version !== import.meta.env.VITE_APP_VERSION}><p class="form-error">{t('set.versionMismatch', {front:import.meta.env.VITE_APP_VERSION, back:app.version})}</p></Show>
          </div>
        </div>
      </div>
    </div>
  );
}
