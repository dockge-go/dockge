import { For, Show } from 'solid-js';
import {
  app, setApp, user, userList, roleLabel, changePassword, pwChecks,
  openSheet, resetUserPassword, deleteUser, saveSettings, resetData, t
} from '../store';

const cardStyle = 'background:var(--panel);border-radius:var(--radius-card);box-shadow:var(--shadow-card);padding:var(--space-5)';

export default function Settings() {
  return (
    <div class="view-section">
      <h2 class="section-title" style="margin-bottom:var(--space-6)">{t('nav.settings')}</h2>
      <div style="max-width:560px;display:flex;flex-direction:column;gap:var(--space-6)">

        <div style={cardStyle}>
          <h3 style="margin:0 0 var(--space-4);font-size:var(--text-base);font-weight:600">{t('set.engineConnection')}</h3>
          <div class="form-group">
            <label class="form-label">{t('set.socketPath')}</label>
            <input class="form-input" value={app.socketPath} placeholder="/var/run/docker.sock"
                   onInput={e => setApp('socketPath', e.currentTarget.value)} />
            <p class="form-help">{t('set.socketHelp')}</p>
          </div>
          <button type="button" class="btn btn-primary" onClick={saveSettings}>{t('set.saveReconnect')}</button>
        </div>

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
              <For each={pwChecks()}>{c => (
                <li classList={{ ok: c.ok }}><span class="pw-check-dot">{c.ok ? '✓' : '·'}</span><span>{c.label}</span></li>
              )}</For>
            </ul>
          </Show>
          <Show when={app.pwForm.error}><p class="auth-error">{app.pwForm.error}</p></Show>
          <p class="form-help">{t('auth.pwHint')}</p>
          <button type="button" class="btn btn-primary" disabled={app.pwForm.busy} onClick={changePassword}>{t('auth.submitPw')}</button>
        </div>

        {/* 用户管理（仅管理员） */}
        <Show when={user()?.role === 'admin'}>
          <div style={cardStyle}>
            <div class="section-header" style="margin-bottom:var(--space-4)">
              <h3 style="margin:0;font-size:var(--text-base);font-weight:600">{t('auth.userMgmt')}</h3>
              <button type="button" class="btn btn-primary" onClick={() => openSheet('newUser')}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
                {t('auth.newUser')}
              </button>
            </div>
            <div class="table-panel"><table class="data-table">
              <thead><tr>
                <th>{t('auth.username')}</th><th>{t('auth.role')}</th><th>{t('common.created')}</th><th style="width:120px" />
              </tr></thead>
              <tbody>
                <For each={userList()}>{u => (
                  <tr style="cursor:default">
                    <td class="cell-name">{u.username}</td>
                    <td><span class={`status-badge ${u.role === 'admin' ? 'status-running' : 'status-exited'}`}>{roleLabel(u.role)}</span></td>
                    <td><span class="cell-meta">{new Date(u.created).toLocaleDateString()}</span></td>
                    <td>
                      <div style="display:flex;gap:2px">
                        <button type="button" class="btn-icon" title={t('auth.changePw')} onClick={() => resetUserPassword(u.username)}>
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect x="3" y="11" width="18" height="11" rx="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" /></svg>
                        </button>
                        <Show when={u.username !== user()?.username}>
                          <button type="button" class="btn-icon" style="color:var(--danger)" title={t('common.delete')} onClick={() => deleteUser(u.username)}>
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" /></svg>
                          </button>
                        </Show>
                      </div>
                    </td>
                  </tr>
                )}</For>
              </tbody>
            </table></div>
          </div>
        </Show>

        <div style={cardStyle}>
          <h3 style="margin:0 0 var(--space-4);font-size:var(--text-base);font-weight:600">{t('set.simulation')}</h3>
          <label class="switch-row" style="margin-bottom:var(--space-2)">
            <span class="switch"><input type="checkbox" checked={app.liveEvents} onChange={e => setApp('liveEvents', e.currentTarget.checked)} /></span>
            {t('set.liveEvents')}
          </label>
          <p class="form-help">{t('set.liveEventsHelp')}</p>
        </div>

        <div style={cardStyle}>
          <h3 style="margin:0 0 var(--space-4);font-size:var(--text-base);font-weight:600">{t('set.dataSection')}</h3>
          <p class="form-help" style="margin:0 0 var(--space-3)">{t('set.resetDataHelp')}</p>
          <button type="button" class="btn btn-danger" onClick={resetData}>{t('set.resetData')}</button>
        </div>

        <div style="background:var(--surface);border:1px solid var(--border-soft);border-radius:var(--radius-lg);padding:var(--space-5)">
          <h3 style="margin:0 0 var(--space-3);font-size:var(--text-base);font-weight:600">{t('set.about')}</h3>
          <div style="font-size:var(--text-sm);color:var(--fg-2);line-height:1.8">
            <div><strong>Dockge</strong> — {t('app.title').split('—')[1]?.trim() || ''}</div>
            <div>{t('set.aboutLine2')}</div>
            <div style="margin-top:var(--space-3);color:var(--muted)">{t('set.aboutLine3')}</div>
          </div>
        </div>
      </div>
    </div>
  );
}
