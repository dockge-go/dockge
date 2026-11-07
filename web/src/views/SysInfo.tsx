import { For } from 'solid-js';
import { sysinfoRows, toast, t } from '../store';

export default function SysInfo() {
  return (
    <div class="view-section">
      <div class="section-header">
        <div>
          <h2 class="section-title">{t('nav.sysinfo')}</h2>
          <p class="section-subtitle">{t('sysinfo.subtitle')}</p>
        </div>
        <button type="button" class="btn btn-secondary" onClick={() => toast(t('toast.infoRefreshed'), 'success')}>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="23 4 23 10 17 10" /><polyline points="1 20 1 14 7 14" /><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" /></svg>
          {t('common.refresh')}
        </button>
      </div>
      <div class="table-panel">
        <table class="data-table kv-table">
          <thead><tr><th>{t('th.key')}</th><th>{t('th.value')}</th></tr></thead>
          <tbody>
            <For each={sysinfoRows()}>{row => (
              <tr>
                <td style="width:220px" class="cell-sub">{row[0]}</td>
                <td style="font-family:var(--font-mono);font-weight:500">{row[1]}</td>
              </tr>
            )}</For>
          </tbody>
        </table>
      </div>
    </div>
  );
}
