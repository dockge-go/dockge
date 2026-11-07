import { For, Show } from 'solid-js';
import { app, setApp, paged, openSheet, removeVolume, clearUnused, volumeUsedBy, t } from '../store';

export default function Volumes() {
  const pg = () => paged('volumes', app.volumes);
  const unused = () => app.volumes.filter(v => !v.system && !v.UsedBy?.length).length;
  const usedBy = (name: string) => volumeUsedBy()[name] || [];

  return (
    <div class="view-section">
      <div class="section-header">
        <div>
          <h2 class="section-title">{t('vol.title')}</h2>
          <p class="section-subtitle">{t('vol.count', { n: app.volumes.length })}</p>
        </div>
        <div class="section-actions">
          <button type="button" class="btn-clear" onClick={() => clearUnused('volumes')}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" /></svg>
            {t('vol.clearUnused', { n: unused() })}
          </button>
          <button type="button" class="btn btn-primary" onClick={() => openSheet('newVolume')}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
            {t('vol.new')}
          </button>
        </div>
      </div>

      <Show when={!app.volumes.length}>
        <div class="empty-state"><div class="empty-state-title">{t('vol.emptyTitle')}</div></div>
      </Show>

      <Show when={app.volumes.length}>
        <div class="table-panel"><table class="data-table">
          <thead><tr>
            <th>{t('common.name')}</th><th>{t('common.driver')}</th><th>{t('common.usage')}</th><th>{t('vol.usedBy')}</th><th style="width:80px" />
          </tr></thead>
          <tbody>
            <For each={pg().rows}>{v => {
              const refs = () => usedBy(v.Name);
              return (
              <tr classList={{ 'row-system': v.system }} style="cursor:default">
                <td>
                  <div class="cell-name">
                    {v.Name}
                    <Show when={v.system}>
                      <span class="badge-system"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect x="3" y="11" width="18" height="11" rx="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" /></svg>{t('common.system')}</span>
                    </Show>
                  </div>
                </td>
                <td><span class="cell-sub">{v.Driver}</span></td>
                <td><span class="cell-sub">{v.Usage || t('data.unknown')}</span></td>
                <td><span class="cell-sub">{refs().length ? refs().map(r => r.name).join(', ') : '—'}</span></td>
                <td onClick={e => e.stopPropagation()}>
                  <Show when={!v.system}>
                    <button type="button" class="btn-icon" style="color:var(--danger)" title={t('common.remove')} onClick={() => removeVolume(v.Name)}>
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" /></svg>
                    </button>
                  </Show>
                </td>
              </tr>
              );
            }}</For>
          </tbody>
        </table></div>
        <Show when={pg().pages > 1}>
          <div class="pagination">
            <button type="button" class="pg-btn" disabled={pg().page <= 1} onClick={() => setApp('page', 'volumes', pg().page - 1)}>‹</button>
            <span>{t('pg.info', { p: pg().page, n: pg().pages, t: pg().total })}</span>
            <button type="button" class="pg-btn" disabled={pg().page >= pg().pages} onClick={() => setApp('page', 'volumes', pg().page + 1)}>›</button>
          </div>
        </Show>
      </Show>
    </div>
  );
}
