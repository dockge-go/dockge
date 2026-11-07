import { For, Show } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import {
  app, setApp, paged, filteredContainers, runningCount, stoppedEditable, openDetail,
  clearStopped, openSheet, containerAction, t, healthBadge
} from '../store';
import { ctrName, statusKind, statusLabel, statusText, portsText } from '../lib/format';

/* 重启策略 → i18n 键；文案随语言切换即时生效 */
const RESTART_KEYS: Record<string, string> = {
  no: 'restart.no', always: 'restart.always',
  'unless-stopped': 'restart.unless-stopped', 'on-failure': 'restart.on-failure'
};
function restartText(c: { RestartPolicy?: string; RestartCount?: number }): string {
  const base = c.RestartPolicy ? t(RESTART_KEYS[c.RestartPolicy] || 'restart.no') : t('common.none');
  return c.RestartCount != null ? base + ' · ' + t('restart.count', { n: c.RestartCount }) : base;
}

function SystemBadge() {
  return (
    <span class="badge-system">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect x="3" y="11" width="18" height="11" rx="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" /></svg>
      {t('common.system')}
    </span>
  );
}

export default function Containers() {
  const nav = useNavigate();
  const pg = () => paged('containers', filteredContainers());

  return (
    <div class="view-section">
      <div class="section-header">
        <div>
          <h2 class="section-title">{t('ctn.title')}<span class="live-dot" /></h2>
          <p class="section-subtitle">{t('common.totalRunning', { total: app.containers.length, running: runningCount() })}</p>
        </div>
        <div class="section-actions">
          <button type="button" class="btn-clear" onClick={clearStopped}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" /></svg>
            {t('ctn.clearStopped', { n: stoppedEditable() })}
          </button>
          <div class="seg" role="tablist" aria-label="filter">
            <button type="button" classList={{ active: !app.showAll }}
                    onClick={() => { setApp('showAll', false); setApp('page', 'containers', 1); }}>{t('seg.running')}</button>
            <button type="button" classList={{ active: app.showAll }}
                    onClick={() => { setApp('showAll', true); setApp('page', 'containers', 1); }}>{t('seg.all')}</button>
          </div>
          <button type="button" class="btn btn-primary" onClick={() => openSheet('newContainer')}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
            {t('ctn.new')}
          </button>
        </div>
      </div>

      <Show when={!pg().rows.length}>
        <div class="empty-state">
          <div class="empty-state-title">{app.showAll ? t('ctn.emptyAll') : t('ctn.emptyRunning')}</div>
          <Show when={!app.showAll}>
            <p class="empty-state-desc">{t('ctn.emptyHint')}</p>
            <button type="button" class="btn btn-secondary" onClick={() => { setApp('showAll', true); setApp('page', 'containers', 1); }}>
              {t('seg.all')}
            </button>
          </Show>
        </div>
      </Show>

      <Show when={pg().rows.length}>
        <div class="table-panel">
        <table class="data-table tbl-containers">
          <thead><tr>
            <th>{t('common.name')}</th><th>{t('common.status')}</th><th>{t('common.image')}</th>
            <th>{t('common.ports')}</th><th>{t('common.uptime')}</th><th>{t('restart.policy')}</th><th style="width:130px">{t('common.actions')}</th>
          </tr></thead>
          <tbody>
            <For each={pg().rows}>{c => (
              <tr classList={{ clickable: !c.system, 'row-system': c.system }} tabindex="0"
                  onClick={() => { if (!c.system) { openDetail(c.Id); nav('/containers/' + c.Id); } }}
                  onKeyDown={e => { if (e.key === 'Enter' && !c.system) nav('/containers/' + c.Id); }}>
                <td>
                  <div class="cell-name">{ctrName(c)} <Show when={c.system}><SystemBadge /></Show></div>
                  <div class="cell-meta">{c.Id.slice(0, 12)}</div>
                </td>
                <td>
                  <span class={`status-badge status-${statusKind(c)}`}>{statusLabel(c)}</span>
                  <Show when={healthBadge(c.Health)} keyed>
                    {hb => (
                      <span style={hb.badge}><span style={hb.dot} />{hb.label}</span>
                    )}
                  </Show>
                  <Show when={c.State === 'running'}><span class="live-dot" title={t('common.live')} /></Show>
                </td>
                <td><span>{c.Image}</span></td>
                <td><span class="cell-meta">{portsText(c) || '—'}</span></td>
                <td><span class="cell-meta">{statusText(c)}</span></td>
                <td><span>{restartText(c)}</span></td>
                <td onClick={e => e.stopPropagation()}>
                  <div style="display:flex;gap:2px">
                    <Show when={!c.system} fallback={<SystemBadge />}>
                      <button type="button" class="btn-icon" style="color:var(--accent)" title={t('tab.logs')} onClick={() => openSheet('logs', c.Id)}>
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /><line x1="16" y1="13" x2="8" y2="13" /><line x1="16" y1="17" x2="8" y2="17" /></svg>
                      </button>
                      <button type="button" class="btn-icon" style="color:var(--success)" title={t('tab.exec')} onClick={() => openSheet('terminal', c.Id)}>
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="4 17 10 11 4 5" /><line x1="12" y1="19" x2="20" y2="19" /></svg>
                      </button>
                      <Show when={c.State === 'running'} fallback={
                        <button type="button" class="btn-icon" title={t('act.start')} onClick={() => containerAction(c.Id, 'start')}>
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3" /></svg>
                        </button>}>
                        <button type="button" class="btn-icon" title={t('act.stop')} onClick={() => containerAction(c.Id, 'stop')}>
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="4" width="4" height="16" /><rect x="14" y="4" width="4" height="16" /></svg>
                        </button>
                      </Show>
                    </Show>
                  </div>
                </td>
              </tr>
            )}</For>
          </tbody>
        </table>
        </div>
      </Show>

      <Show when={pg().pages > 1}>
        <div class="pagination">
          <button type="button" class="pg-btn" disabled={pg().page <= 1} onClick={() => setApp('page', 'containers', pg().page - 1)} aria-label={t('pg.prev')}>‹</button>
          <span>{t('pg.info', { p: pg().page, n: pg().pages, t: pg().total })}</span>
          <button type="button" class="pg-btn" disabled={pg().page >= pg().pages} onClick={() => setApp('page', 'containers', pg().page + 1)} aria-label={t('pg.next')}>›</button>
        </div>
      </Show>
    </div>
  );
}
