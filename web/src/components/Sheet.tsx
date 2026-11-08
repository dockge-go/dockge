import { For, Show, createEffect, lazy } from 'solid-js';
import {
  app, setApp, closeSheet, sheetTitle, imageHistoryLayers,
  createContainer, pullImage, createVolume, createNetwork, tagImage,
  logFiltered, logLevelClass, logScroll, toggleLogPause, clearLog, copyLog,
  t
} from '../store';

const TerminalPane = lazy(() => import('./TerminalPane'));

const FORM_KINDS = ['newContainer', 'pullImage', 'newVolume', 'newNetwork', 'tagImage'];
const VIEW_KINDS = ['logs', 'stackLogs', 'serviceLogs', 'terminal', 'history'];

export default function Sheet() {
  const kind = () => app.sheet.kind;
  const setForm = (k: string, v: string) => setApp('form', k, v);

  /* 打开表单 Sheet 即聚焦首个字段（仅精确指针设备，避免移动端弹软键盘） */
  createEffect(() => {
    const kind = app.sheet.kind;
    if (app.sheet.open && kind && FORM_KINDS.includes(kind) && matchMedia('(pointer: fine)').matches) {
      requestAnimationFrame(() => {
        const el = document.querySelector('.sheet-body .form-input') as HTMLInputElement | null;
        if (el) el.focus();
      });
    }
  });

  /* 日志流：过滤结果变化时若跟随则滚到底部 */
  createEffect(() => {
    if (kind() === 'logs' || kind() === 'stackLogs' || kind() === 'serviceLogs') {
      requestAnimationFrame(() => {
        if (app.logView.follow) {
          const el = document.getElementById('log-viewer');
          if (el) el.scrollTop = el.scrollHeight;
        }
      });
    }
  });

  return (
    <div class="sheet-overlay" classList={{ open: app.sheet.open }} aria-hidden={!app.sheet.open} inert={!app.sheet.open} onClick={e => { if (e.target === e.currentTarget) closeSheet(); }}>
      <div class="sheet" classList={{ 'sheet-stream': ['logs', 'stackLogs', 'serviceLogs', 'terminal'].includes(kind() || '') }} role="dialog" aria-modal="true" aria-label={sheetTitle()}>
        <div class="sheet-header">
          <h2 class="sheet-title">{sheetTitle()}</h2>
          <button type="button" class="btn-icon" disabled={app.actionBusy} onClick={closeSheet} aria-label={t('common.close')}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6 6 18" /><path d="m6 6 12 12" /></svg>
          </button>
        </div>
        <div class="sheet-body">
          <Show when={kind() === 'newContainer'}>
            <div class="form-group">
              <label class="form-label">{t('form.containerName')}</label>
              <input class="form-input" value={app.form.name || ''} placeholder="my-container" onInput={e => setForm('name', e.currentTarget.value)} />
            </div>
            <div class="form-group">
              <label class="form-label">{t('form.imageLabel')}</label>
              <input class="form-input" value={app.form.image || ''} placeholder="nginx:latest" list="img-list" onInput={e => setForm('image', e.currentTarget.value)} />
              <datalist id="img-list">
                <For each={app.images}>{i => <option value={i.RepoTags[0]} />}</For>
              </datalist>
            </div>
            <div class="form-row">
              <div class="form-group">
                <label class="form-label">{t('form.hostPort')}</label>
                <input class="form-input" value={app.form.hostPort || ''} placeholder="8080" inputmode="numeric" onInput={e => setForm('hostPort', e.currentTarget.value)} />
              </div>
              <div class="form-group">
                <label class="form-label">{t('form.containerPort')}</label>
                <input class="form-input" value={app.form.containerPort || ''} placeholder="80" inputmode="numeric" onInput={e => setForm('containerPort', e.currentTarget.value)} />
              </div>
            </div>
            <div class="form-group">
              <label class="form-label">{t('form.commandLabel')}</label>
              <input class="form-input" value={app.form.command || ''} placeholder='/bin/sh -c "echo hello"' onInput={e => setForm('command', e.currentTarget.value)} />
              <p class="form-help">{t('form.commandHelp')}</p>
            </div>
          </Show>

          <Show when={kind() === 'pullImage'}>
            <div class="form-group">
              <label class="form-label">{t('form.imageRef')}</label>
              <input class="form-input" value={app.form.imageRef || ''} placeholder="nginx:latest" onInput={e => setForm('imageRef', e.currentTarget.value)} />
              <p class="form-help">{t('form.imageRefHelp')}</p>
            </div>
          </Show>

          <Show when={kind() === 'newVolume'}>
            <div class="form-group">
              <label class="form-label">{t('form.volumeName')}</label>
              <input class="form-input" value={app.form.volumeName || ''} placeholder="my-data" onInput={e => setForm('volumeName', e.currentTarget.value)} />
            </div>
          </Show>

          <Show when={kind() === 'newNetwork'}>
            <div class="form-group">
              <label class="form-label">{t('form.networkName')}</label>
              <input class="form-input" value={app.form.networkName || ''} placeholder="my-network" onInput={e => setForm('networkName', e.currentTarget.value)} />
            </div>
            <div class="form-group">
              <label class="form-label">{t('form.driver')}</label>
              <select class="form-select" value={app.form.driver || 'bridge'} onInput={e => setForm('driver', e.currentTarget.value)}>
                <option value="bridge">bridge</option>
                <option value="host">host</option>
                <option value="overlay">overlay</option>
              </select>
            </div>
          </Show>

          <Show when={kind() === 'pullImage' && app.form.output}><TerminalPane output={app.form.output || ''} compact /></Show>
        <Show when={kind() === 'tagImage'}>
            <div class="form-group">
              <label class="form-label">{t('form.currentTag')}</label>
              <input class="form-input" readonly value={(app.images.find(x => x.Id === app.sheet.param)?.RepoTags[0]) || ''} />
            </div>
            <div class="form-group">
              <label class="form-label">{t('form.newTag')}</label>
              <input class="form-input" value={app.form.newTag || ''} placeholder="nginx:1.25-alpine" onInput={e => setForm('newTag', e.currentTarget.value)} />
            </div>
          </Show>

          <Show when={kind() === 'logs' || kind() === 'stackLogs' || kind() === 'serviceLogs'}>
            <div class="log-toolbar">
              <input aria-label={t('log.filterPh')} class="form-input log-filter" value={app.logView.filter} placeholder={t('log.filterPh')}
                     onInput={e => setApp('logView', 'filter', e.currentTarget.value)} />
              <button type="button" class="btn btn-secondary log-tool-btn" onClick={toggleLogPause}>
                {app.logView.paused ? t('log.resume') : t('log.pause')}
              </button>
              <button type="button" class="btn btn-secondary log-tool-btn" classList={{ active: app.logView.follow }}
                      onClick={() => { setApp('logView', 'follow', !app.logView.follow); if (app.logView.follow) { const el = document.getElementById('log-viewer'); if (el) el.scrollTop = el.scrollHeight; } }}>
                {t('log.follow')}
              </button>
              <button type="button" class="btn btn-secondary log-tool-btn" onClick={clearLog}>{t('log.clear')}</button>
              <button type="button" class="btn btn-secondary log-tool-btn" onClick={copyLog}>{t('log.copy')}</button>
              <span class="log-count">
                <Show when={app.logView.connection === 'live'}><span class="live-dot" /></Show>
                <span>{t('log.' + app.logView.connection)}{app.logView.paused ? ' · ' + t('log.pausedBadge') : ''}</span>
                <span>{t('log.lines', { n: logFiltered().length })}</span>
              </span>
            </div>
            <div class="log-only-viewer log-stream" id="log-viewer" onScroll={logScroll}>
              <For each={logFiltered()}>{l => <div class="log-line" classList={{ [logLevelClass(l.raw)]: true }}>{l.raw}</div>}</For>
              <Show when={!logFiltered().length}><div class="log-empty">{t('log.empty')}</div></Show>
            </div>
          </Show>

          <Show when={app.sheet.open && kind() === 'terminal' && app.sheet.param}>
            <TerminalPane containerID={app.sheet.param!} />
          </Show>

          <Show when={kind() === 'history'}>
            <table class="data-table">
              <thead><tr>
                <th>{t('th.layer')}</th><th>{t('th.command')}</th><th>{t('common.size')}</th>
              </tr></thead>
              <tbody>
                <For each={imageHistoryLayers()}>{l => (
                  <tr>
                    <td><span class="cell-meta" style="color:var(--accent)">{l.id}</span></td>
                    <td><span style="font-size:var(--text-sm)">{l.cmd}</span></td>
                    <td><span class="cell-sub">{l.size}</span></td>
                  </tr>
                )}</For>
              </tbody>
            </table>
          </Show>
        </div>
        <div class="sheet-footer">
          <Show when={FORM_KINDS.includes(kind() || '')}>
            <button type="button" class="btn btn-secondary" disabled={app.actionBusy} onClick={closeSheet}>{t('common.cancel')}</button>
            <Show when={kind() === 'newContainer'}>
              <button type="button" class="btn btn-primary" disabled={app.actionBusy} onClick={createContainer}>{t('btn.createStart')}</button>
            </Show>
            <Show when={kind() === 'pullImage'}>
              <button type="button" class="btn btn-primary" disabled={app.actionBusy} onClick={pullImage}>{t('btn.pull')}</button>
            </Show>
            <Show when={kind() === 'newVolume'}>
              <button type="button" class="btn btn-primary" disabled={app.actionBusy} onClick={createVolume}>{t('btn.create')}</button>
            </Show>
            <Show when={kind() === 'newNetwork'}>
              <button type="button" class="btn btn-primary" disabled={app.actionBusy} onClick={createNetwork}>{t('btn.create')}</button>
            </Show>


        <Show when={kind() === 'tagImage'}>
              <button type="button" class="btn btn-primary" disabled={app.actionBusy} onClick={() => tagImage(app.sheet.param!)}>{t('btn.tag')}</button>
            </Show>
          </Show>
          <Show when={VIEW_KINDS.includes(kind() || '')}>
            <button type="button" class="btn btn-secondary" disabled={app.actionBusy} onClick={closeSheet}>{t('common.close')}</button>
          </Show>
        </div>
      </div>
    </div>
  );
}
