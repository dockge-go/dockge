import { For, Show } from 'solid-js';
import { sysdfRows, sysdfPrune, sysdfPruneAll, t } from '../store';

export default function SysDf() {
  return (
    <div class="view-section">
      <div class="section-header">
        <div>
          <h2 class="section-title">{t('nav.sysdf')}</h2>
          <p class="section-subtitle">{t('sysdf.subtitle')}</p>
        </div>
        <button type="button" class="btn btn-danger" onClick={sysdfPruneAll}>{t('sysdf.pruneAll')}</button>
      </div>
      <div class="table-panel"><table class="data-table">
        <thead><tr>
          <th>{t('th.type')}</th><th>{t('th.used')}</th><th>{t('th.reclaimable')}</th><th style="width:100px" />
        </tr></thead>
        <tbody>
          <For each={sysdfRows()}>{row => (
            <tr>
              <td class="cell-name">{row.label}</td>
              <td><span style="font-size:var(--text-sm)">{row.used}</span> <span class="cell-meta">{t('sysdf.items', { n: row.count })}</span></td>
              <td>
                <span style={`font-size:var(--text-sm);color:${row.reclaim !== '0 B' ? 'var(--danger)' : 'var(--muted)'}`}>
                  {t('sysdf.reclaimable', { v: row.reclaim })}
                </span>
              </td>
              <td>
                <Show when={row.reclaim !== '0 B'}>
                  <button type="button" class="btn btn-secondary" style="padding:4px 12px;font-size:var(--text-xs)" onClick={() => sysdfPrune(row.label)}>
                    {t('sysdf.prune')}
                  </button>
                </Show>
              </td>
            </tr>
          )}</For>
        </tbody>
      </table></div>
    </div>
  );
}
