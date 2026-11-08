import { For, Show, lazy } from 'solid-js';
import {
  app, setApp, paged, selectedStackObj, stackDirty, stackServiceNames,
  resourcesReady, createStack, saveStack, stackAction, removeStack, deployStack, openSheet, t
} from '../store';
const TerminalPane = lazy(() => import('../components/TerminalPane'));
import YamlEditor from '../components/YamlEditor';

export default function Stacks() {
  const pg = () => paged('stacks', app.stacks);
  const sel = selectedStackObj;

  return (
    <div class="view-section wide">
      <div class="stacks-split">
        <div class="stack-list">
          <div class="stack-list-head">
            <button type="button" class="btn btn-primary" onClick={createStack}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
              {t('stack.new')}
            </button>
          </div>
          <div class="stack-list-body">
            <Show when={resourcesReady('stacks') && !pg().rows.length}>
              <div style="padding:var(--space-3);color:var(--muted);font-size:var(--text-sm)">{t('stack.emptyTitle')}</div>
            </Show>
            <For each={pg().rows}>{s => (
              <button type="button" class="stack-item" classList={{ active: app.selectedStack === s.id }}
                      onClick={() => { setApp('selectedStack', s.id); setApp('editorFullscreen', false); }}>
                <span class="stack-item-dot" classList={{ up: s.status === 'up' }} />
                <span class="stack-item-name">{s.name}</span>
              </button>
            )}</For>
          </div>
          <Show when={pg().pages > 1}>
            <div class="pagination">
              <button type="button" class="pg-btn" disabled={pg().page <= 1} onClick={() => setApp('page', 'stacks', pg().page - 1)}>‹</button>
              <span>{t('pg.info', { p: pg().page, n: pg().pages, t: pg().total })}</span>
              <button type="button" class="pg-btn" disabled={pg().page >= pg().pages} onClick={() => setApp('page', 'stacks', pg().page + 1)}>›</button>
            </div>
          </Show>
        </div>

        <Show when={sel()} fallback={<div class="workbench"><div class="wb-empty">{t('stack.noSelection')}</div></div>}>
          <div class="workbench">
            <div class="wb-head">
              <span class="wb-title">{sel()!.name}</span>
              <span class={`status-badge ${sel()!.status === 'up' ? 'status-running' : 'status-exited'}`}>
                {sel()!.status === 'unknown' ? t('data.unknown') : sel()!.status === 'up' ? t('status.running') : t('status.stopped')}
              </span>
              <div class="wb-actions"><Show when={sel()!.managed && !sel()!.loadError}>
                <button type="button" class="btn btn-secondary" onClick={() => openSheet('stackLogs', sel()!.id)}>{t('stack.viewServiceLogs')}</button>
                <button type="button" class="btn btn-secondary" disabled={sel()!.busy} onClick={() => saveStack(sel()!.id)}>{t('stack.saveDraft')}</button>
                <button type="button" class="btn btn-secondary" disabled={sel()!.busy} onClick={() => stackAction(sel()!.id)}>
                  {sel()!.status === 'up' ? t('act.stop') : t('act.start')}
                </button>
                <button type="button" class="btn btn-secondary" disabled={sel()!.busy} onClick={() => stackAction(sel()!.id, 'restart')}>{t('act.restart')}</button>
                <button type="button" class="btn btn-secondary" disabled={sel()!.busy} onClick={() => stackAction(sel()!.id, 'update')}>{t('stack.update')}</button>
                <button type="button" class="btn btn-secondary" disabled={sel()!.busy} onClick={() => stackAction(sel()!.id, 'down')}>{t('stack.down')}</button>
                <button type="button" class="btn btn-danger" disabled={sel()!.busy || app.actionBusy} onClick={() => removeStack(sel()!.id)}>{t('common.delete')}</button>
                <button type="button" class="btn btn-primary" disabled={sel()!.busy} onClick={() => deployStack(sel()!.id)}
                        title={stackDirty() ? t('editor.dirty') : t('editor.inSync')}>
                  {t('stack.deploy')}
                </button>
                <span class="dirty-badge" style={`display:${stackDirty() ? 'inline-flex' : 'none'}`}>{t('editor.dirty')}</span></Show>
              </div>
            </div>
            <div class="wb-services">
              <For each={stackServiceNames()}>{svc => (
                <button type="button" class="service-pill" title={t('stack.viewServiceLogs')}
                        onClick={() => openSheet('serviceLogs', sel()!.name + '||' + svc)}>
                  <span class="dot" classList={{ down: sel()!.status !== 'up' }} />
                  {svc}
                </button>
              )}</For>
            </div>
            <Show when={sel()!.loadError || sel()!.runtimeError || !sel()!.managed}><p class="auth-error">{sel()!.loadError || sel()!.runtimeError || t('stack.readonly')}</p></Show>
            <YamlEditor />
            <div class="wb-terminal" id="wb-terminal">
              <Show when={sel()!.deployLog.length}
                fallback={<div class="t-dim">$ {t('stack.termHint')}</div>}>
                <TerminalPane output={sel()!.deployLog.join('\n')} compact />
              </Show>
            </div>
          </div>
        </Show>
      </div>
    </div>
  );
}
