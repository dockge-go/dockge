import { Show, createMemo, onCleanup, onMount } from 'solid-js';
import { useLocation } from '@solidjs/router';
import {
  app, setApp, user, toast, t, lang, setLang, theme, setTheme,
  searchQ, setSearchQ, searchOpen, setSearchOpen, searchIdx, setSearchIdx,
  searchResults, searchPick, searchKeydown, ctrName, detail
} from '../store';

/* 顶栏标题：详情页显示容器名，其余按路由字典 */
function useTopbarTitle(): () => string {
  const loc = useLocation();
  return createMemo(() => {
    if (loc.pathname.startsWith('/containers/') && detail()) return ctrName(detail()!);
    const map: Record<string, string> = {
      '/': 'nav.dashboard', '/dashboard': 'nav.dashboard', '/containers': 'nav.containers',
      '/stacks': 'nav.stacks', '/images': 'nav.images', '/volumes': 'nav.volumes',
      '/networks': 'nav.networks', '/sysinfo': 'nav.sysinfo', '/sysdf': 'nav.sysdf', '/settings': 'nav.settings'
    };
    return t(map[loc.pathname] || 'nav.dashboard');
  });
}

export default function Topbar() {
  const title = useTopbarTitle();
  let searchWrap: HTMLDivElement | undefined;

  onMount(() => {
    const onDocClick = (e: MouseEvent) => {
      if (searchWrap && !searchWrap.contains(e.target as Node)) setSearchOpen(false);
    };
    document.addEventListener('mousedown', onDocClick);
    onCleanup(() => document.removeEventListener('mousedown', onDocClick));
  });

  return (
    <header class="topbar">
      <button type="button" class="hamburger" aria-expanded={app.sidebarOpen}
              aria-label={t('aria.toggleNav')}
              onClick={() => setApp('sidebarOpen', !app.sidebarOpen)}>
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="3" y1="6" x2="21" y2="6" /><line x1="3" y1="12" x2="21" y2="12" /><line x1="3" y1="18" x2="21" y2="18" /></svg>
      </button>
      <h1 class="topbar-title">{title()}</h1>
      <div class="topbar-spacer" />
      <div class="topbar-actions">
        <div class="search-box" ref={searchWrap}>
          <svg class="search-box-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="11" cy="11" r="8" /><path d="m21 21-4.3-4.3" /></svg>
          <input id="global-search" type="search" value={searchQ()} autocomplete="off"
                 placeholder={t('common.search')} aria-label={t('aria.search')}
                 onInput={e => { setSearchQ(e.currentTarget.value); setSearchIdx(-1); setSearchOpen(true); }}
                 onFocus={() => setSearchOpen(true)}
                 onKeyDown={searchKeydown} />
          <Show when={searchOpen() && searchResults().length > 0}>
            <div class="search-dropdown">
              {searchResults().map((r, i) => (
                <div>
                  <Show when={i === 0 || searchResults()[i - 1].group !== r.group}>
                    <div class="search-group-label">{r.group}</div>
                  </Show>
                  <button type="button" class="search-result-item" classList={{ active: i === searchIdx() }}
                          onMouseDown={e => { e.preventDefault(); searchPick(r); }}>
                    <span class="sr-name">{r.name}</span>
                    <span class="sr-meta">{r.meta}</span>
                  </button>
                </div>
              ))}
            </div>
          </Show>
        </div>
        <div class="lang-switch" role="group" aria-label={t('aria.lang')}>
          <button type="button" class="lang-btn" classList={{ active: lang() === 'zh' }} onClick={() => setLang('zh')}>中</button>
          <button type="button" class="lang-btn" classList={{ active: lang() === 'en' }} onClick={() => setLang('en')}>EN</button>
        </div>
        <button type="button" class="btn-icon" onClick={() => setTheme(theme() === 'dark' ? 'light' : 'dark')} aria-label={t('aria.theme')}>
          <Show when={theme() === 'dark'}
            fallback={<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" /></svg>}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="5" /><line x1="12" y1="1" x2="12" y2="3" /><line x1="12" y1="21" x2="12" y2="23" /><line x1="4.22" y1="4.22" x2="5.64" y2="5.64" /><line x1="18.36" y1="18.36" x2="19.78" y2="19.78" /><line x1="1" y1="12" x2="3" y2="12" /><line x1="21" y1="12" x2="23" y2="12" /><line x1="4.22" y1="19.78" x2="5.64" y2="18.36" /><line x1="18.36" y1="5.64" x2="19.78" y2="4.22" /></svg>
          </Show>
        </button>
        <button type="button" class="btn-icon topbar-refresh" onClick={() => toast(t('toast.refreshed'), 'info')}
                aria-label={t('common.refresh')} title={t('common.refresh')}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="23 4 23 10 17 10" /><polyline points="1 20 1 14 7 14" /><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" /></svg>
        </button>
        <Show when={user()}>
          <span class="topbar-user">{user()!.username}</span>
        </Show>
      </div>
    </header>
  );
}
