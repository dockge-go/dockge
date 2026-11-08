import { useLocation } from '@solidjs/router';
import { createEffect, on, onMount, onCleanup, For, Show, type JSX } from 'solid-js';
import { app, setApp, closeSheet, dialogCancel, user, refreshData, t } from './store';
import Sidebar from './components/Sidebar';
import Topbar from './components/Topbar';
import AuthScreen from './components/AuthScreen';
import Sheet from './components/Sheet';
import Dialog from './components/Dialog';
import Toasts from './components/Toasts';

export default function App(props: { children?: JSX.Element }) {
  const location = useLocation();
  // 路由切换释放检视流，不重建概览连接，也不取消后台部署任务。
  createEffect(on(() => location.pathname, () => {
    if (['logs', 'stackLogs', 'serviceLogs', 'terminal', 'history'].includes(app.sheet.kind || '')) closeSheet();
  }, {defer:true}));
  onMount(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof Element && e.target.closest('.terminal-pane.interactive')) return;
      if (e.key === 'Escape') { closeSheet(); dialogCancel(); }
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        const el = document.getElementById('global-search') as HTMLInputElement | null;
        if (el) el.focus();
      }
    };
    window.addEventListener('keydown', onKey);
    onCleanup(() => window.removeEventListener('keydown', onKey));
  });

  return (
    <>
      <Show when={user()} fallback={<AuthScreen />}>
        <div class="app-shell">
          <Sidebar />
          <div class="main-content">
            <Topbar />
            <main class="content-area">
              <div class="data-source" style="margin-bottom:var(--space-3);font-size:var(--text-xs);color:var(--muted)">
                {app.loading ? t('data.loading') : t('data.source', {runtime:app.runtime || t('data.unknown'), time:app.refreshedAt ? new Date(app.refreshedAt).toLocaleTimeString() : '—'})}
                <button type="button" class="btn btn-ghost" disabled={app.loading} onClick={() => void refreshData()}>{t('common.refresh')}</button>
                <For each={Object.entries(app.errors)}>{([source, message]) => <p class="auth-error" style="white-space:pre-wrap">{source}: {message}</p>}</For>
              </div>
              {props.children}</main>
          </div>
        </div>
        <div class="sidebar-backdrop" classList={{ open: app.sidebarOpen }} onClick={() => setApp('sidebarOpen', false)} />
      </Show>
      <Show when={user()}><Sheet /><Dialog /></Show>
      <Toasts />
    </>
  );
}
