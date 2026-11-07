import { For, Show } from 'solid-js';
import { app, setApp, paged, openSheet, removeNetwork, clearUnused, t } from '../store';

export default function Networks() {
  const pg = () => paged('networks', app.networks);
  const unused = () => app.networks.filter(n => !n.system).length;

  return (
    <div class="view-section">
      <div class="section-header">
        <div>
          <h2 class="section-title">{t('net.title')}</h2>
          <p class="section-subtitle">{t('net.count', { n: app.networks.length })}</p>
        </div>
        <div class="section-actions">
          <button type="button" class="btn-clear" onClick={() => clearUnused('networks')}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" /></svg>
            {t('net.clearUnused', { n: unused() })}
          </button>
          <button type="button" class="btn btn-primary" onClick={() => openSheet('newNetwork')}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
            {t('net.new')}
          </button>
        </div>
      </div>

      <Show when={!app.networks.length}>
        <div class="empty-state"><div class="empty-state-title">{t('net.emptyTitle')}</div></div>
      </Show>

      <Show when={app.networks.length}>
        <div class="table-panel"><table class="data-table tbl-networks">
          <thead><tr>
            <th>{t('common.name')}</th><th>{t('common.driver')}</th><th>{t('common.subnet')}</th>
            <th>{t('nav.containers')}</th><th style="width:80px" />
          </tr></thead>
          <tbody>
            <For each={pg().rows}>{n => (
              <tr classList={{ 'row-system': n.system }} style="cursor:default">
                <td>
                  <div class="cell-name">
                    {n.Name}
                    <Show when={n.system}>
                      <span class="badge-system"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect x="3" y="11" width="18" height="11" rx="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" /></svg>{t('common.system')}</span>
                    </Show>
                  </div>
                  <div class="cell-meta">{n.ID.slice(0, 12)}</div>
                </td>
                <td><span style="font-size:var(--text-sm)">{n.Driver}</span></td>
                <td><span class="cell-sub">{n.Subnet}</span></td>
                <td><span style="font-size:var(--text-sm)">{t('common.containersCount', { n: n.Containers?.length ?? 0 })}</span></td>
                <td onClick={e => e.stopPropagation()}>
                  <Show when={!n.system}>
                    <button type="button" class="btn-icon" style="color:var(--danger)" title={t('common.remove')} onClick={() => removeNetwork(n.ID)}>
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" /></svg>
                    </button>
                  </Show>
                </td>
              </tr>
            )}</For>
          </tbody>
        </table></div>
        <Show when={pg().pages > 1}>
          <div class="pagination">
            <button type="button" class="pg-btn" disabled={pg().page <= 1} onClick={() => setApp('page', 'networks', pg().page - 1)}>‹</button>
            <span>{t('pg.info', { p: pg().page, n: pg().pages, t: pg().total })}</span>
            <button type="button" class="pg-btn" disabled={pg().page >= pg().pages} onClick={() => setApp('page', 'networks', pg().page + 1)}>›</button>
          </div>
        </Show>
      </Show>
    </div>
  );
}
