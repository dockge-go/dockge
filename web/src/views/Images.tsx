import { For, Show } from 'solid-js';
import { app, setApp, paged, openSheet, removeImage, clearUnused, t, resourcesReady, resourceCount } from '../store';
import { formatBytes, timeAgo } from '../lib/format';

export default function Images() {
  const pg = () => paged('images', app.images);
  const unused = () => app.images.filter(i => !i.system && !i.InUse).length;

  return (
    <div class="view-section">
      <div class="section-header">
        <div>
          <h2 class="section-title">{t('img.title')}</h2>
          <p class="section-subtitle">{t('img.count', { n: resourceCount('images', app.images.length) })}</p>
        </div>
        <div class="section-actions">
          <button type="button" class="btn-clear" disabled={!resourcesReady('images') || !unused() || app.actionBusy} onClick={() => clearUnused('images')}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" /></svg>
            {t('img.clearUnused', { n: resourceCount('images', unused()) })}
          </button>
          <button type="button" class="btn btn-primary" onClick={() => openSheet('pullImage')}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
            {t('img.pull')}
          </button>
        </div>
      </div>

      <Show when={!resourcesReady('images')}><p class="form-help" role="status">{app.loading ? t('data.loading') : t('data.unknown')}</p></Show>

      <Show when={resourcesReady('images') && !app.images.length}>
        <div class="empty-state"><div class="empty-state-title">{t('img.emptyTitle')}</div></div>
      </Show>

      <Show when={resourcesReady('images') && app.images.length}>
        <div class="table-panel"><table class="data-table tbl-images">
          <thead><tr>
            <th>{t('common.repository')}</th><th>{t('common.tag')}</th><th>{t('common.size')}</th>
            <th>{t('common.created')}</th><th style="width:110px" />
          </tr></thead>
          <tbody>
            <For each={pg().rows}>{img => (
              <tr classList={{ 'row-system': img.system }} style="cursor:default">
                <td>
                  <div class="cell-name">
                    {img.Repository || (img.RepoTags[0] || '').split(':')[0]}
                    <Show when={img.system}>
                      <span class="badge-system"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect x="3" y="11" width="18" height="11" rx="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" /></svg>{t('common.system')}</span>
                    </Show>
                  </div>
                  <div class="cell-meta">{img.Id.slice(0, 12)}</div>
                </td>
                <td><span style="font-size:var(--text-sm)">{img.RepoTags[0] || '<missing>'}</span></td>
                <td><span class="cell-sub">{formatBytes(img.Size)}</span></td>
                <td><span class="cell-meta">{timeAgo(img.Created)}</span></td>
                <td onClick={e => e.stopPropagation()}>
                  <div style="display:flex;gap:2px">
                    <Show when={!img.system}>
                      <button type="button" class="btn-icon" title={t('img.history')} onClick={() => openSheet('history', img.Id)}>
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="10" /><polyline points="12 6 12 12 16 14" /></svg>
                      </button>
                      <button type="button" class="btn-icon" title={t('common.tag')} onClick={() => openSheet('tagImage', img.Id)}>
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z" /><line x1="7" y1="7" x2="7.01" y2="7" /></svg>
                      </button>
                      <button type="button" class="btn-icon" style="color:var(--danger)" title={t('common.remove')} onClick={() => removeImage(img.Id)}>
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" /></svg>
                      </button>
                    </Show>
                  </div>
                </td>
              </tr>
            )}</For>
          </tbody>
        </table></div>
        <Show when={pg().pages > 1}>
          <div class="pagination">
            <button type="button" class="pg-btn" disabled={pg().page <= 1} onClick={() => setApp('page', 'images', pg().page - 1)}>‹</button>
            <span>{t('pg.info', { p: pg().page, n: pg().pages, t: pg().total })}</span>
            <button type="button" class="pg-btn" disabled={pg().page >= pg().pages} onClick={() => setApp('page', 'images', pg().page + 1)}>›</button>
          </div>
        </Show>
      </Show>
    </div>
  );
}
