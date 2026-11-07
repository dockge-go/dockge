import { onMount, Show, type JSX } from 'solid-js';
import { app, setApp, closeSheet, dialogCancel, user } from './store';
import Sidebar from './components/Sidebar';
import Topbar from './components/Topbar';
import AuthScreen from './components/AuthScreen';
import Sheet from './components/Sheet';
import Dialog from './components/Dialog';
import Toasts from './components/Toasts';

export default function App(props: { children?: JSX.Element }) {
  onMount(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { closeSheet(); dialogCancel(); }
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        const el = document.getElementById('global-search') as HTMLInputElement | null;
        if (el) el.focus();
      }
    };
    window.addEventListener('keydown', onKey);
  });

  return (
    <>
      <Show when={user()} fallback={<AuthScreen />}>
        <div class="app-shell">
          <Sidebar />
          <div class="main-content">
            <Topbar />
            <main class="content-area">{props.children}</main>
          </div>
        </div>
        <div class="sidebar-backdrop" classList={{ open: app.sidebarOpen }} onClick={() => setApp('sidebarOpen', false)} />
      </Show>
      <Sheet />
      <Dialog />
      <Toasts />
    </>
  );
}
