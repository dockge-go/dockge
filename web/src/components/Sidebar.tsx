import { A, useLocation } from '@solidjs/router';
import { app, setApp, runningCount, user, doLogout, t } from '../store';

interface NavItem { route: string; label: string; icon: string; badge?: 'containers' | 'stacks' | 'images' }
interface NavGroup { group: string; items: NavItem[] }

const NAV: NavGroup[] = [
  {
    group: 'nav.group.overview',
    items: [{ route: '/dashboard', label: 'nav.dashboard', icon: 'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z' }]
  },
  {
    group: 'nav.group.resources',
    items: [
      { route: '/containers', label: 'nav.containers', badge: 'containers', icon: 'M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16zM3.27 6.96 12 12.01l8.73-5.05M12 22.08V12' },
      { route: '/stacks', label: 'nav.stacks', badge: 'stacks', icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5' },
      { route: '/images', label: 'nav.images', badge: 'images', icon: 'M2 3h20a0 0 0 0 1 0 0v14a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V3zM8 21h8M12 17v4M7 8h2m2 0h2m2 0h2M7 11h10' },
      { route: '/volumes', label: 'nav.volumes', icon: 'M12 5c4.97 0 9 1.34 9 3s-4.03 3-9 3-9-1.34-9-3 4.03-3 9-3zM3 5v14c0 1.66 4.03 3 9 3s9-1.34 9-3V5' },
      { route: '/networks', label: 'nav.networks', icon: 'M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zM2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z' }
    ]
  },
  {
    group: 'nav.group.system',
    items: [
      { route: '/sysinfo', label: 'nav.sysinfo', icon: 'M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zM12 16v-4M12 8h.01' },
      { route: '/sysdf', label: 'nav.sysdf', icon: 'M21.21 15.89A10 10 0 1 1 8 2.83M22 12A10 10 0 0 0 12 2v10z' },
      { route: '/settings', label: 'nav.settings', icon: 'M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6zM19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9c.14.62.7 1.06 1.34 1.06H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z' }
    ]
  }
];

export default function Sidebar() {
  const loc = useLocation();
  const active = (route: string) => {
    const p = loc.pathname;
    if (route === '/containers') return p === '/containers' || p.startsWith('/containers/');
    if (route === '/dashboard') return p === '/' || p === '/dashboard';
    return p === route;
  };
  const badgeText = (b?: string) => {
    if (b === 'containers') return runningCount() + '/' + app.containers.length;
    if (b === 'stacks') return String(app.stacks.length);
    if (b === 'images') return String(app.images.length);
    return '';
  };

  return (
    <aside class="sidebar" classList={{ open: app.sidebarOpen }} aria-label={t('aria.mainNav')}>
      <div class="sidebar-brand">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <rect x="2" y="3" width="20" height="14" rx="2" /><path d="M8 21h8M12 17v4" /><path d="M7 8h2m2 0h2m2 0h2M7 11h10" />
        </svg>
        <span class="sidebar-brand-name">Dockge</span>
      </div>
      <nav class="sidebar-nav">
        {NAV.map(g => (
          <div class="nav-group">
            <div class="nav-group-label">{t(g.group)}</div>
            {g.items.map(item => (
              <A href={item.route} class="nav-item" classList={{ active: active(item.route) }}
                 onClick={() => setApp('sidebarOpen', false)}>
                <svg class="nav-item-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
                  <path d={item.icon} />
                </svg>
                <span>{t(item.label)}</span>
                {item.badge && <span class="nav-item-badge">{badgeText(item.badge)}</span>}
              </A>
            ))}
          </div>
        ))}
      </nav>
      <div class="sidebar-footer">
        <div class="engine-badge" style="margin-bottom:var(--space-2)">
          <span class="engine-dot" style={{background:app.errors.runtime || app.errors.info || !app.runtime ? 'var(--meta)' : 'var(--success)'}} />
          <span>{app.runtime || t('data.unknown')} · {app.info.ServerVersion || t('data.unknown')}</span>
        </div>
        <div class="user-chip">
          <span class="user-avatar">{(user()?.username || '?').slice(0, 1).toUpperCase()}</span>
          <span class="user-chip-name">
            <span class="user-chip-username">{user()?.username}</span>
            <span class="user-chip-role">{t('auth.roleAdmin')}</span>
          </span>
          <button type="button" class="btn-icon" style="padding:4px" title={t('auth.logout')} aria-label={t('auth.logout')} onClick={doLogout}>
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" /><polyline points="16 17 21 12 16 7" /><line x1="21" y1="12" x2="9" y2="12" /></svg>
          </button>
        </div>
      </div>
    </aside>
  );
}
