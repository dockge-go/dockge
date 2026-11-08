import { For, Show, createEffect } from 'solid-js';
import { useNavigate, useParams } from '@solidjs/router';
import {
  app, setApp, resourcesReady, detail, detailEnv, detailNets, detailMounts, openDetail,
  openSheet, containerAction, removeContainer, renameContainer, commitContainer, t, healthBadge
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

export default function ContainerDetail() {
  const params = useParams();
  const nav = useNavigate();
  /* 路由参数 → store.detailId；容器不存在时回列表 */
  createEffect(() => {
    const id = params.id;
    if (!id || !resourcesReady('containers')) return;
    if (!app.containers.some(x => x.Id === id)) { nav('/containers', { replace: true }); return; }
    if (app.detailId !== id) openDetail(id);
  });

  const c = detail;
  return (
    <Show when={c()}>
      <div class="view-section">
        <div class="detail-head">
        <div class="crumb-row">
          <button type="button" class="crumb" onClick={() => nav('/containers')}>‹ {t('nav.containers')}</button>
          <span class="crumb-sep">/</span>
        </div>
          <span class="detail-name">{ctrName(c()!)}</span>
          <span class={`status-badge status-${statusKind(c()!)}`}>{statusLabel(c()!)}</span>
          <Show when={healthBadge(c()!.Health)} keyed>
            {hb => (
              <span style={hb.badge}><span style={hb.dot} />{hb.label}</span>
            )}
          </Show>
          <Show when={c()!.State === 'running'}><span class="live-dot" /></Show>
          <div class="detail-tools">
            <button type="button" class="btn btn-secondary" onClick={() => openSheet('logs', c()!.Id)}>{t('tab.logs')}</button>
            <button type="button" class="btn btn-secondary" disabled={c()!.State !== 'running' || c()!.system} onClick={() => openSheet('terminal', c()!.Id)}>{t('tab.exec')}</button>
            <Show when={c()!.State === 'running'} fallback={
              <button type="button" class="btn btn-primary" onClick={() => containerAction(app.detailId!, 'start')}>{t('act.start')}</button>}>
              <button type="button" class="btn btn-secondary" onClick={() => containerAction(app.detailId!, 'stop')}>{t('act.stop')}</button>
            </Show>
            <button type="button" class="btn btn-secondary" onClick={() => containerAction(app.detailId!, 'restart')}>{t('act.restart')}</button>
            <button type="button" class="btn btn-secondary" onClick={() => renameContainer(app.detailId!)}>{t('ctn.rename')}</button>
            <button type="button" class="btn btn-danger" onClick={() => removeContainer(app.detailId!)}>{t('common.remove')}</button>
          </div>
        </div>

        <div class="summary-grid">
          <div class="summary-cell"><div class="summary-cell-label">{t('common.image')}</div><div class="summary-cell-value">{c()!.Image}</div></div>
          <div class="summary-cell"><div class="summary-cell-label">{t('common.ports')}</div><div class="summary-cell-value" style="font-family:var(--font-mono);font-size:var(--text-xs)">{portsText(c()!) || t('common.none')}</div></div>
          <div class="summary-cell"><div class="summary-cell-label">{t('common.uptime')}</div><div class="summary-cell-value">{statusText(c()!)}</div></div>
          <div class="summary-cell"><div class="summary-cell-label">{t('common.created')}</div><div class="summary-cell-value">{new Date(c()!.Created).toLocaleString()}</div></div>
          <div class="summary-cell"><div class="summary-cell-label">{t('restart.policy')}</div><div class="summary-cell-value">{restartText(c()!)}</div></div>
        </div>

        <div class="tabs-bar">
          <div class="seg" role="tablist">
            <button type="button" classList={{ active: app.detailTab === 'stats' }} onClick={() => setApp('detailTab', 'stats')}>{t('tab.stats')}</button>
            <button type="button" classList={{ active: app.detailTab === 'env' }} onClick={() => setApp('detailTab', 'env')}>{t('tab.env')}</button>
            <button type="button" classList={{ active: app.detailTab === 'net' }} onClick={() => setApp('detailTab', 'net')}>{t('tab.net')}</button>
            <button type="button" classList={{ active: app.detailTab === 'mounts' }} onClick={() => setApp('detailTab', 'mounts')}>{t('tab.mounts')}</button>
          </div>
          <button type="button" class="btn btn-secondary" style="height:26px;font-size:var(--text-xs)" onClick={() => commitContainer(app.detailId!)}>{t('ctn.commit')}</button>
        </div>

        <Show when={app.detailTab === 'stats'}>
          <div class="stat-grid">
            <div class="stat-cell">
              <div class="stat-cell-label">{t('res.cpu')}</div>
              <div class="stat-cell-value">{c()!._stats ? c()!._stats!.cpuPct + '%' : t('data.unknown')}</div>
              <div class="stat-bar-track"><div class="stat-bar-fill cpu" style={`width:${c()!._stats ? c()!._stats!.cpuBar : 0}%`} /></div>
              <div class="stat-detail">{c()!.Cpus ? c()!.Cpus + ' CPU' : t('data.noLimit')}</div>
            </div>
            <div class="stat-cell">
              <div class="stat-cell-label">{t('res.memory')}</div>
              <div class="stat-cell-value">{c()!._stats ? c()!._stats!.memMB + ' MB' : t('data.unknown')}</div>
              <div class="stat-bar-track"><div class="stat-bar-fill mem" style={`width:${c()!._stats ? c()!._stats!.memBar : 0}%`} /></div>
              <div class="stat-detail">{c()!.MemoryMB ? c()!.MemoryMB + ' MB' : t('data.noLimit')}</div>
            </div>
            <div class="stat-cell">
              <div class="stat-cell-label">{t('stat.netIO')}</div>
              <div class="stat-cell-value" style="font-size:var(--text-lg)">{c()!._stats?.netIO || t('data.unknown')}</div>
              <div class="stat-io-row"><span style="color:var(--muted)">{t('stat.rx')}</span><span>{c()!._stats?.netIO?.split('/')[0]?.trim() || t('data.unknown')}</span></div>
              <div class="stat-io-row"><span style="color:var(--muted)">{t('stat.tx')}</span><span>{c()!._stats?.netIO?.split('/')[1]?.trim() || t('data.unknown')}</span></div>
            </div>
            <div class="stat-cell">
              <div class="stat-cell-label">{t('stat.blockIO')}</div>
              <div class="stat-cell-value" style="font-size:var(--text-lg)">{c()!._stats?.blockIO || t('data.unknown')}</div>
              <div class="stat-io-row"><span style="color:var(--muted)">{t('stat.read')}</span><span>{c()!._stats?.blockIO?.split('/')[0]?.trim() || t('data.unknown')}</span></div>
              <div class="stat-io-row"><span style="color:var(--muted)">{t('stat.write')}</span><span>{c()!._stats?.blockIO?.split('/')[1]?.trim() || t('data.unknown')}</span></div>
            </div>
          </div>
        </Show>

        <Show when={app.detailTab === 'env'}>
          <div class="env-box">
            <For each={detailEnv()}>{e => {
              const i = e.indexOf('=');
              return (
                <div class="env-row">
                  <span class="env-key">{i > 0 ? e.slice(0, i) : e}</span>
                  <span class="env-val"> = {i > 0 ? e.slice(i + 1) : ''}</span>
                </div>
              );
            }}</For>
          </div>
        </Show>

        <Show when={app.detailTab === 'net'}>
          <For each={detailNets()}>{n => (
            <div class="net-row">
              <div><strong>{n.name}</strong><span style="color:var(--muted);margin-inline-start:var(--space-3);font-family:var(--font-mono);font-size:var(--text-xs)">{n.ip}</span></div>
            </div>
          )}</For>
        </Show>

        <Show when={app.detailTab === 'mounts'}>
          <For each={detailMounts()}>{m => (
            <div class="mount-row">
              <div><strong>{m.source}</strong></div>
              <div style="color:var(--muted);font-size:var(--text-xs);font-family:var(--font-mono)">{m.dest} · {m.mode}</div>
            </div>
          )}</For>
        </Show>
      </div>
    </Show>
  );
}
