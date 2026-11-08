import { For, Show } from 'solid-js';
import { A, useNavigate } from '@solidjs/router';
import {
  app, runningCount, stoppedCount, pausedCount, recentContainers, openSheet, createStack,
  ctrName, statusKind, statusLabel, statusText, portsText, t, resourcesReady, resourceCount
} from '../store';
import { useSystemMetrics } from '../hooks/system';
import { percentText, uptimeParts } from '../lib/system';
import { formatBytes } from '../lib/format';
import SystemStatus from '../components/SystemStatus';

export default function Dashboard() {
  const nav = useNavigate();
  const metrics = useSystemMetrics();
  const host = metrics.system;
  const unknown = () => t('data.unknown');
  const duration = () => { const parts = uptimeParts(host()?.uptime); return parts ? t('sysinfo.duration', parts) : unknown(); };
  const usage = (kind:'memory'|'disk') => { const value=host()?.[kind];return value ? formatBytes(value.used) + ' / ' + formatBytes(value.total) : unknown(); };
  const rows = () => [
    {key:'cpu',label:t('res.cpu'),value:host()?.cpuPercent,detail:host()?.cpuCount != null ? t('sysinfo.logicalCPUs',{n:host()!.cpuCount!}) : unknown()},
    {key:'mem',label:t('res.memory'),value:host()?.memory?.percent,detail:usage('memory')},
    {key:'disk',label:t('res.disk'),value:host()?.disk?.percent,detail:usage('disk')}
  ];
  const total = () => app.containers.length || 1;
  const pct = (n: number) => `width:${(n / total() * 100).toFixed(1)}%`;

  return (
    <div class="view-section">
      <div class="stats-grid">
        <button type="button" class="stat-card" onClick={() => nav('/containers')}>
          <div class="stat-card-label">{t('dash.labelRunning')}</div>
          <div class="stat-card-value running">{resourceCount('containers', runningCount())}</div>
        </button>
        <button type="button" class="stat-card" onClick={() => nav('/containers')}>
          <div class="stat-card-label">{t('dash.labelStopped')}</div>
          <div class="stat-card-value stopped">{resourceCount('containers', stoppedCount())}</div>
        </button>
        <button type="button" class="stat-card" onClick={() => nav('/containers')}>
          <div class="stat-card-label">{t('dash.labelPaused')}</div>
          <div class="stat-card-value warning">{resourceCount('containers', pausedCount())}</div>
        </button>
        <button type="button" class="stat-card" onClick={() => nav('/images')}>
          <div class="stat-card-label">{t('dash.labelImages')}</div>
          <div class="stat-card-value">{resourceCount('images', app.images.length)}</div>
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
          <Show when={resourcesReady('containers')} fallback={<p class="form-help">{unknown()}</p>}>
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
          </Show>
          <dl class="host-facts resource-counts">
            <div><dt>{t('nav.stacks')}</dt><dd>{resourceCount('stacks', app.stacks.length)}</dd></div>
            <div><dt>{t('nav.volumes')}</dt><dd>{resourceCount('volumes', app.volumes.length)}</dd></div>
            <div><dt>{t('nav.networks')}</dt><dd>{resourceCount('networks', app.networks.length)}</dd></div>
          </dl>
          <A href="/sysdf" class="host-info-link">{t('sysinfo.runtimeDisk')}</A>
        </div>
        <div class="chart-card host-card">
          <div class="host-card-heading"><div class="chart-card-title">{t('dash.resourceUsage')}</div><A href="/sysinfo" class="host-info-link">{t('nav.sysinfo')}</A></div>
          <SystemStatus metrics={metrics} />
          <For each={rows()}>{row => <div class="host-resource">
            <div class="resource-row">
              <span class="resource-label">{row.label}</span>
              <div class="resource-track" role="meter" aria-label={row.label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={percentText(row.value) ? row.value ?? undefined : undefined} aria-valuetext={percentText(row.value) || unknown()}><div class={`resource-fill ${row.key}`} style={{width:percentText(row.value) || '0%'}} /></div>
              <span class="resource-val">{percentText(row.value) || (row.key==='cpu' && host()?.cpuCount && !host()?.errors.cpu ? t('sysinfo.warming') : unknown())}</span>
            </div>
            <div class="host-resource-detail">{row.detail}<Show when={row.key==='disk'}> · {t(host()?.source==='local' ? 'sysinfo.rootPartition' : 'sysinfo.stackPartition', {path:host()?.disk?.path || '/opt/stacks'})}</Show></div>
          </div>}</For>
          <dl class="host-facts">
            <div><dt>{t('sysinfo.load')}</dt><dd>{host()?.load?.map(v=>v.toFixed(2)).join(' / ') || unknown()}</dd></div>
            <div><dt>{t('sysinfo.uptime')}</dt><dd>{duration()}</dd></div>
            <div><dt>{t('sysinfo.swap')}</dt><dd>{host()?.swap ? host()!.swap!.total === 0 ? t('sysinfo.swapDisabled') : formatBytes(host()!.swap!.used) + ' / ' + formatBytes(host()!.swap!.total) : unknown()}</dd></div>
          </dl>
        </div>
      </div>

      <div class="section-header">
        <div>
          <h2 class="section-title">{t('dash.recentContainers')}</h2>
          <p class="section-subtitle">{t('common.totalRunning', { total: resourceCount('containers', app.containers.length), running: resourceCount('containers', runningCount()) })}</p>
        </div>
        <A href="/containers" class="btn btn-secondary">{t('common.viewAll')}</A>
      </div>
      <Show when={resourcesReady('containers')} fallback={<p class="form-help" role="status">{app.loading ? t('data.loading') : unknown()}</p>}>
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
      </Show>
    </div>
  );
}
