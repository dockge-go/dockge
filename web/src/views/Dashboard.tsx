import { For, Show } from 'solid-js';
import { A, useNavigate } from '@solidjs/router';
import {
  app, runningCount, stoppedCount, pausedCount, recentContainers, openSheet, createStack,
  ctrName, statusKind, statusLabel, statusText, portsText, t
} from '../store';

export default function Dashboard() {
  const nav = useNavigate();
  const total = () => app.containers.length || 1;
  const pct = (n: number) => `width:${(n / total() * 100).toFixed(1)}%`;

  return (
    <div class="view-section">
      <div class="stats-grid">
        <button type="button" class="stat-card" onClick={() => nav('/containers')}>
          <div class="stat-card-label">{t('dash.labelRunning')}</div>
          <div class="stat-card-value running">{runningCount()}</div>
        </button>
        <button type="button" class="stat-card" onClick={() => nav('/containers')}>
          <div class="stat-card-label">{t('dash.labelStopped')}</div>
          <div class="stat-card-value stopped">{stoppedCount()}</div>
        </button>
        <button type="button" class="stat-card" onClick={() => nav('/containers')}>
          <div class="stat-card-label">{t('dash.labelPaused')}</div>
          <div class="stat-card-value warning">{pausedCount()}</div>
        </button>
        <button type="button" class="stat-card" onClick={() => nav('/images')}>
          <div class="stat-card-label">{t('dash.labelImages')}</div>
          <div class="stat-card-value">{app.images.length}</div>
        </button>
      </div>

      <div style="display:flex;gap:var(--space-3);flex-wrap:wrap;margin-bottom:var(--space-6)">
        <button type="button" class="btn-pill btn-pill-primary"
                onClick={() => { nav('/containers'); setTimeout(() => openSheet('newContainer'), 80); }}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
          {t('dash.newContainer')}
        </button>
        <button type="button" class="btn-pill btn-pill-secondary" onClick={() => { nav('/stacks'); setTimeout(() => createStack(), 80); }}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
          {t('dash.newStack')}
        </button>
      </div>

      <div class="charts-grid">
        <div class="chart-card">
          <div class="chart-card-title">{t('dash.containerStatus')}</div>
          <div class="bar-stacked">
            <div class="bar-seg running" style={pct(runningCount())} />
            <div class="bar-seg paused" style={pct(pausedCount())} />
            <div class="bar-seg exited" style={pct(stoppedCount())} />
          </div>
          <div class="chart-legend">
            <span class="chart-legend-item"><span class="chart-legend-dot" style="background:var(--success)" />{t('legend.running', { n: runningCount() })}</span>
            <span class="chart-legend-item"><span class="chart-legend-dot" style="background:var(--warn)" />{t('legend.paused', { n: pausedCount() })}</span>
            <span class="chart-legend-item"><span class="chart-legend-dot" style="background:var(--meta)" />{t('legend.stopped', { n: stoppedCount() })}</span>
          </div>
        </div>
        <div class="chart-card">
          <div class="chart-card-title">{t('dash.resourceUsage')}</div>
          <div class="resource-row"><span class="resource-label">{t('res.cpu')}</span><div class="resource-track"><div class="resource-fill cpu" style={`width:${app.sys.cpu}%`} /></div><span class="resource-val">{app.sys.cpu}%</span></div>
          <div class="resource-row"><span class="resource-label">{t('res.memory')}</span><div class="resource-track"><div class="resource-fill mem" style={`width:${app.sys.mem}%`} /></div><span class="resource-val">{app.sys.mem}%</span></div>
          <div class="resource-row"><span class="resource-label">{t('res.disk')}</span><div class="resource-track"><div class="resource-fill disk" style={`width:${app.sys.disk}%`} /></div><span class="resource-val">{app.sys.disk}%</span></div>
        </div>
      </div>

      <div class="section-header">
        <div>
          <h2 class="section-title">{t('dash.recentContainers')}</h2>
          <p class="section-subtitle">{t('common.totalRunning', { total: app.containers.length, running: runningCount() })}</p>
        </div>
        <A href="/containers" class="btn btn-secondary">{t('common.viewAll')}</A>
      </div>
      <div class="table-panel"><table class="data-table tbl-containers">
        <thead><tr>
          <th>{t('common.name')}</th><th>{t('common.status')}</th><th>{t('common.image')}</th>
          <th>{t('common.ports')}</th><th>{t('common.uptime')}</th>
        </tr></thead>
        <tbody>
          <For each={recentContainers()}>{c => (
            <tr classList={{ clickable: !c.system, 'row-system': c.system }} tabindex="0"
                onClick={() => !c.system && nav('/containers/' + c.Id)}
                onKeyDown={e => { if (e.key === 'Enter' && !c.system) nav('/containers/' + c.Id); }}>
              <td>
                <div class="cell-name">
                  {ctrName(c)}
                  <Show when={c.system}><span class="badge-system"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect x="3" y="11" width="18" height="11" rx="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" /></svg>{t('common.system')}</span></Show>
                </div>
                <div class="cell-meta">{c.Id.slice(0, 12)}</div>
              </td>
              <td>
                <span class={`status-badge status-${statusKind(c)}`}>{statusLabel(c)}</span>
                <Show when={c.State === 'running'}><span class="live-dot" title={t('common.live')} /></Show>
              </td>
              <td><span class="cell-sub">{c.Image}</span></td>
              <td><span class="cell-meta">{portsText(c) || '—'}</span></td>
              <td><span class="cell-meta">{statusText(c)}</span></td>
            </tr>
          )}</For>
        </tbody>
      </table></div>
    </div>
  );
}
