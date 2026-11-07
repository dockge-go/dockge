import { For } from 'solid-js';
import { app, refreshData, t, formatBytes } from '../store';
import { useSystemMetrics } from '../hooks/system';
import { systemInfoRows } from '../lib/system';
import SystemStatus from '../components/SystemStatus';

export default function SysInfo() {
  const metrics=useSystemMetrics();
  const rows=()=>systemInfoRows(metrics.system(),app.info,app.runtime);
  return (
    <div class="view-section">
      <div class="section-header">
        <div>
          <h2 class="section-title">{t('nav.sysinfo')}</h2>
          <p class="section-subtitle">{t('sysinfo.subtitle')}</p>
        </div>
        <button type="button" class="btn btn-secondary" onClick={() => {metrics.refresh();void refreshData();}}>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="23 4 23 10 17 10" /><polyline points="1 20 1 14 7 14" /><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" /></svg>
          {t('common.refresh')}
        </button>
      </div>
      <SystemStatus metrics={metrics} />
      <div class="table-panel">
        <table class="data-table kv-table system-summary">
          <thead><tr><th>{t('th.key')}</th><th>{t('th.value')}</th></tr></thead>
          <tbody>
            <For each={rows()}>{row => (
              <tr>
                <td class="cell-sub">{t(row[0])}</td>
                <td>{row[1] === null ? t('data.unknown') : row[0] === 'sysinfo.totalMemory' ? formatBytes(Number(row[1])) : row[1]}</td>
              </tr>
            )}</For>
          </tbody>
        </table>
      </div>
    </div>
  );
}
