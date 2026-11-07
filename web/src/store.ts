/* ─────────────────────────────────────────────────────────────────────────
 * 全局单一数据源（solid-js/store + signals）
 * 资源数据、UI 状态（sheet/dialog/toast/搜索/日志流/终端）、认证会话全部在此。
 * 所有视图（侧栏徽标、仪表盘、列表、详情、compose 胶囊）均从本 store 派生 ——
 * 改一处，全局同步。数据可编辑并持久化到 localStorage（跨标签页同步）。
 * ─────────────────────────────────────────────────────────────────── */
import { createSignal, createEffect, createMemo, createRoot } from 'solid-js';
import { createStore, produce } from 'solid-js/store';
import { t, setLang, lang } from './i18n';
import { defaultData, defaultCompose, SERVICE_LOGS, Container, PortMap, Image, Volume, Network, Stack } from './lib/data';
import { ctrName, statusKind, formatBytes, timeAgo, genStats, uid, rand } from './lib/format';
import { AuthAPI, Session, UserPublic, passwordProblem } from './lib/auth';
import { yamlProblem, parseServices } from './lib/yaml';

const LS_DATA = 'dockge.spa.v1';
const LS_THEME = 'dockge.theme';
const PAGE_SIZE = 15;

/* ── 持久化装配 ── */
function loadData() {
  try {
    const d = JSON.parse(localStorage.getItem(LS_DATA) || 'null');
    if (!d || !Array.isArray(d.containers) || !Array.isArray(d.images)) return defaultData();
    return {
      containers: d.containers as Container[], images: d.images as Image[],
      volumes: (d.volumes || []) as Volume[], networks: (d.networks || []) as Network[],
      stacks: (d.stacks || []) as Stack[]
    };
  } catch (e) { return defaultData(); }
}
function loadExtras(): { socketPath?: string; liveEvents?: boolean } {
  try { return JSON.parse(localStorage.getItem(LS_DATA) || '{}') || {}; } catch (e) { return {}; }
}

/* ── 信号（会话 / 主题 / 搜索 / toast） ── */
export const [user, setUser] = createSignal<Session | null>(AuthAPI.session());
export const [noUsers, setNoUsers] = createSignal(AuthAPI.needSetup());
export const [userList, setUserList] = createSignal<UserPublic[]>(AuthAPI.listUsers());
/* 引导完成标志：AuthScreen 在 false 期间只渲染加载态，避免 needSetup 缓存初值误判分支（D-04） */
export const [authBooted, setAuthBooted] = createSignal(false);
export async function refreshAuthState() {
  try {
    await AuthAPI.refreshNeedSetup();
    setNoUsers(AuthAPI.needSetup());
  } catch (e) { /* 引导网络失败保持缓存值 */ }
  /* GET /v1/users 为 admin-only：member 固定调用会 403 弹无意义权限 toast，
   * member 的 userList 本就不可见——仅 admin 拉取（门控 per 02-CONTEXT）。 */
  if (user()?.role === 'admin') {
    try {
      await AuthAPI.refreshUsers();
      setUserList(AuthAPI.listUsers());
    } catch (e) { toast(authErrorText((e as Error).message), 'error'); }
  }
}
/* 应用启动引导（D-04）：刷新 needSetup 缓存后 AuthScreen 才做 setup/login 分支决策；
 * 调用点固定在模块作用域——组件挂载时序不定会漏接导致 authBooted 恒 false。 */
async function bootstrapAuth() {
  try {
    await AuthAPI.refreshNeedSetup();
    setNoUsers(AuthAPI.needSetup());
    if (user()?.role === 'admin') {
      await AuthAPI.refreshUsers();
      setUserList(AuthAPI.listUsers());
    }
  } catch (e) { /* 后端不可达：保持缓存默认，登录时错误经表单/toast 明细透出 */ }
  setAuthBooted(true);
}
void bootstrapAuth();

const storedTheme = (() => {
  try { const s = localStorage.getItem(LS_THEME); if (s === 'dark' || s === 'light') return s; } catch (e) { /* ignore */ }
  return 'auto';
})();
export const [theme, setThemeState] = createSignal<'auto' | 'dark' | 'light'>(storedTheme);
export function applyTheme(v: 'auto' | 'dark' | 'light') {
  const effective = v === 'auto'
    ? (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
    : v;
  document.documentElement.dataset.theme = effective;
}
export function setTheme(v: 'auto' | 'dark' | 'light') {
  setThemeState(v);
  try { localStorage.setItem(LS_THEME, v); } catch (e) { /* ignore */ }
  applyTheme(v);
}
applyTheme(storedTheme);
if (window.matchMedia) {
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (theme() === 'auto') applyTheme('auto');
  });
}

export const [toasts, setToasts] = createSignal<{ id: number; msg: string; type: string }[]>([]);
let toastSeq = 1;
export function toast(msg: string, type = 'info') {
  const id = toastSeq++;
  setToasts(v => [...v, { id, msg, type }]);
  setTimeout(() => setToasts(v => v.filter(x => x.id !== id)), 3500);
}

export const [searchQ, setSearchQ] = createSignal('');
export const [searchOpen, setSearchOpen] = createSignal(false);
export const [searchIdx, setSearchIdx] = createSignal(-1);

/* ── 数据 + UI 状态 store ── */
const init = loadData();
const extras = loadExtras();

export interface SheetState { open: boolean; kind: string | null; param: string | null }
export interface DialogState {
  open: boolean; mode: 'confirm' | 'prompt'; message: string; value: string;
  placeholder: string; okLabel: string | null; danger: boolean; requireValue: boolean;
  cb: ((value?: string) => void) | null;
}

export const [app, setApp] = createStore({
  containers: init.containers,
  images: init.images,
  volumes: init.volumes,
  networks: init.networks,
  stacks: init.stacks,
  socketPath: extras.socketPath != null ? extras.socketPath : '/var/run/docker.sock',
  liveEvents: extras.liveEvents != null ? !!extras.liveEvents : true,
  sys: { cpu: 34, mem: 62, disk: 45 },

  showAll: false,
  page: { containers: 1, images: 1, volumes: 1, networks: 1, stacks: 1 } as Record<string, number>,
  selectedStack: init.stacks.length ? init.stacks[0].id : null,
  detailId: null as string | null,
  detailTab: 'stats',
  editorFullscreen: false,
  sidebarOpen: false,

  dialog: { open: false, mode: 'confirm', message: '', value: '', placeholder: '', okLabel: null, danger: true, requireValue: true, cb: null } as DialogState,
  sheet: { open: false, kind: null, param: null } as SheetState,
  form: {} as Record<string, string>,
  authForm: { username: '', password: '', confirm: '', error: '', busy: false },
  pwForm: { current: '', next: '', confirm: '', error: '', busy: false },
  userForm: { username: '', password: '', role: 'member' },
  term: { lines: [] as { prompt?: string; text: string; isCmd: boolean }[], input: '', hist: [] as string[], histIdx: -1, host: '' },
  logView: { kind: null as string | null, param: null as string | null, lines: [] as { id: string; raw: string }[], filter: '', follow: true, paused: false, timer: 0 }
});

/* 首次载入：未部署过的栈视为已同步 */
init.stacks.forEach(s => { if (s.composeDeployed == null) setApp('stacks', x => x.id === s.id, 'composeDeployed', s.compose); });

/* ── 派生数据（UI 同步的根基） ── */
export const runningCount = createMemo(() => app.containers.filter(c => c.State === 'running').length);
export const stoppedCount = createMemo(() => app.containers.filter(c => c.State === 'exited').length);
export const pausedCount = createMemo(() => app.containers.filter(c => c.State === 'paused').length);
export const stoppedEditable = createMemo(() => app.containers.filter(c => c.State === 'exited' && !c.system).length);
export const recentContainers = createMemo(() => app.containers.slice(0, 5));
export const selectedStackObj = createMemo<Stack | null>(() => app.stacks.find(s => s.id === app.selectedStack) || null);
export const stackDirty = createMemo(() => {
  const s = selectedStackObj();
  return !!(s && s.composeDeployed != null && s.compose !== s.composeDeployed);
});
export const stackServiceNames = createMemo(() => {
  const s = selectedStackObj();
  return s ? parseServices(s.compose || '') : [];
});
const detail = createMemo(() => app.containers.find(c => c.Id === app.detailId) || null);
export { detail };
export const detailEnv = createMemo(() => {
  const c = detail();
  if (c && c.Env && c.Env.length) return c.Env;
  if (!c) return [];
  return ['PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin', 'CONTAINER_NAME=' + ctrName(c), 'LOG_LEVEL=info', 'APP_ENV=production'];
});
export const detailNets = createMemo(() => {
  const c = detail();
  if (c && c.Networks && c.Networks.length) {
    return c.Networks.map((name, i) => ({ name, ip: '172.2' + (i % 2 + 1) + '.0.' + (2 + i) }));
  }
  return [];
});
export const detailMounts = createMemo(() => {
  const c = detail();
  return (c && c.Mounts) || [];
});

/* ── 卷反向引用：从容器 Mounts 反查使用各命名卷的容器。
 * 选型 (b) 派生而非种子直填：容器 Mounts 是权威数据源，反向引用随之自动同步，
 * 避免两处重复维护导致不一致（种子里只有 postgres_data / redis_data 被挂载，
 * nginx_conf / app_uploads 保持“未使用”以演示空引用态）。 ── */
export const volumeUsedBy = createMemo<Record<string, { id: string; name: string }[]>>(() => {
  const map: Record<string, { id: string; name: string }[]> = {};
  app.volumes.forEach(v => { map[v.Name] = []; });
  app.containers.forEach(c => {
    (c.Mounts || []).forEach(m => {
      if (m.source && Object.prototype.hasOwnProperty.call(map, m.source)) {
        map[m.source].push({ id: c.Id, name: ctrName(c) });
      }
    });
  });
  return map;
});

/* ── 健康检查徽标（消费 Container.Health，文案走 i18n，配色复用现有 CSS 变量） ── */
export function healthBadge(h: Container['Health']): { label: string; badge: string; dot: string } | null {
  if (!h) return null;
  const map: Record<'starting' | 'healthy' | 'unhealthy', [string, string, string]> = {
    healthy:   ['health.healthy',   '--success', '--success-soft'],
    unhealthy: ['health.unhealthy', '--danger',  '--danger-soft'],
    starting:  ['health.starting',  '--warn',    '--warn-soft']
  };
  const [key, color, soft] = map[h];
  return {
    label: t(key),
    badge: `display:inline-flex;align-items:center;gap:6px;padding:2px 10px;border-radius:999px;font-size:var(--text-xs);font-weight:600;white-space:nowrap;margin-inline-start:6px;background:var(${soft});color:var(${color})`,
    dot: `width:6px;height:6px;border-radius:50%;flex-shrink:0;background:var(${color})`
  };
}

export const sysinfoRows = createMemo(() => [
  [t('sysinfo.apiVersion'), '3.4.4'], [t('sysinfo.version'), '5.2.2'], [t('sysinfo.kernel'), '6.6.12-linuxkit'],
  [t('sysinfo.os'), 'Linux'], [t('sysinfo.arch'), 'amd64'], [t('sysinfo.goVersion'), 'go1.21.4'],
  [t('sysinfo.podmanSocket'), '/run/podman/podman.sock'], [t('sysinfo.storageDriver'), 'overlay'],
  [t('sysinfo.graphRoot'), '/var/lib/containers/storage'], [t('sysinfo.containerRoot'), '/run/containers/storage'],
  [t('sysinfo.cgroupDriver'), 'systemd'], [t('sysinfo.cgroupVersion'), '2'],
  [t('sysinfo.cpus'), '8'], [t('sysinfo.totalMemory'), '16.00 GiB']
]);

export function paged<T>(key: string, arr: T[]): { rows: T[]; page: number; pages: number; total: number } {
  const total = arr.length;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const page = Math.min(Math.max(1, app.page[key] || 1), pages);
  setApp('page', key, page);
  return { rows: arr.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE), page, pages, total };
}
export const filteredContainers = createMemo(() => {
  const all = app.showAll;
  return app.containers.filter(c => all || c.State === 'running');
});

const imageRepoSet = createMemo(() => {
  const set: Record<string, 1> = {};
  app.containers.forEach(c => { set[c.Image] = 1; });
  return set;
});

/* ── 新建容器：动态行（端口 / 环境变量 / 挂载）── */
export interface FormPort { ip: string; host: string; ctn: string; proto: 'tcp' | 'udp' }
export interface FormEnv { k: string; v: string }
export interface FormMount { host: string; ctn: string; ro: boolean }
export const [cForm, setCForm] = createStore({
  ports: [] as FormPort[],
  envs: [] as FormEnv[],
  mounts: [] as FormMount[],
  network: 'bridge',
  restart: 'no' as 'no' | 'always' | 'unless-stopped' | 'on-failure',
  cpus: '',
  memory: '',
  entrypoint: ''
});
export function addFormPort() { setCForm('ports', p => [...p, { ip: '', host: '', ctn: '', proto: 'tcp' }]); }
export function removeFormPort(i: number) { setCForm('ports', p => p.filter((_, x) => x !== i)); }
export function addFormEnv() { setCForm('envs', e => [...e, { k: '', v: '' }]); }
export function removeFormEnv(i: number) { setCForm('envs', e => e.filter((_, x) => x !== i)); }
export function addFormMount() { setCForm('mounts', m => [...m, { host: '', ctn: '', ro: false }]); }
export function removeFormMount(i: number) { setCForm('mounts', m => m.filter((_, x) => x !== i)); }
export const sysdfRows = createMemo(() => {
  const usedImages = app.images.reduce((a, i) => a + (i.Size || 0), 0);
  const reclaimImages = app.images.filter(i => !imageRepoSet()[i.RepoTags[0]]).reduce((a, i) => a + (i.Size || 0), 0);
  const usedVol = app.volumes.reduce((a, v) => {
    const m = /([\d.]+)\s*(B|KB|MB|GB)/i.exec(v.Usage || '');
    return m ? a + parseFloat(m[1]) * ({ B: 1, KB: 1e3, MB: 1e6, GB: 1e9 } as Record<string, number>)[m[2].toUpperCase()] : a;
  }, 0);
  return [
    { key: 'images', label: t('img.title'), used: formatBytes(usedImages), reclaim: formatBytes(reclaimImages), count: app.images.length },
    { key: 'volumes', label: t('vol.title'), used: formatBytes(usedVol), reclaim: '0 B', count: app.volumes.length },
    { key: 'buildCache', label: t('sysdf.buildCache'), used: '1.1 GB', reclaim: '1.1 GB', count: 24 },
    { key: 'containers', label: t('ctn.title'), used: '48 MB', reclaim: '12 MB', count: app.containers.length }
  ];
});

/* ── 全局搜索 ── */
export interface SearchItem { group: string; name: string; meta: string; act: () => void }
export const searchResults = createMemo<SearchItem[]>(() => {
  const q = searchQ().trim().toLowerCase();
  if (!q) return [];
  const out: SearchItem[] = [];
  app.containers.forEach(c => {
    const name = ctrName(c);
    if (name.toLowerCase().includes(q) || c.Image.toLowerCase().includes(q) || c.Id.slice(0, 12).includes(q)) {
      out.push({ group: t('nav.containers'), name, meta: c.Image, act: () => { location.hash = ''; navigateToDetail(c.Id); } });
    }
  });
  app.images.forEach(img => {
    const tag = img.RepoTags[0] || '';
    if (tag.toLowerCase().includes(q) || (img.Repository || '').toLowerCase().includes(q)) {
      out.push({ group: t('nav.images'), name: tag || img.Id.slice(0, 12), meta: formatBytes(img.Size), act: () => { location.assign('#/images'); } });
    }
  });
  app.volumes.forEach(v => {
    if (v.Name.toLowerCase().includes(q)) {
      out.push({ group: t('nav.volumes'), name: v.Name, meta: v.Usage || '', act: () => { location.assign('#/volumes'); } });
    }
  });
  app.stacks.forEach(s => {
    if (s.name.toLowerCase().includes(q)) {
      out.push({ group: t('nav.stacks'), name: s.name, meta: s.status, act: () => { setApp('selectedStack', s.id); location.assign('#/stacks'); } });
    }
  });
  return out;
});
/* 容器详情跳转：保持 history 路由（非 hash），由 App 内统一处理 */
function navigateToDetail(id: string) {
  history.pushState(null, '', '/containers/' + id);
  window.dispatchEvent(new PopStateEvent('popstate'));
}
export function searchPick(item?: SearchItem) {
  if (item) item.act();
  setSearchQ('');
  setSearchOpen(false);
  setSearchIdx(-1);
}
export function searchKeydown(e: KeyboardEvent) {
  const n = searchResults().length;
  if (!n) return;
  if (e.key === 'ArrowDown') { e.preventDefault(); setSearchIdx(Math.min(searchIdx() + 1, n - 1)); }
  else if (e.key === 'ArrowUp') { e.preventDefault(); setSearchIdx(Math.max(searchIdx() - 1, 0)); }
  else if (e.key === 'Enter') { e.preventDefault(); searchPick(searchResults()[Math.max(searchIdx(), 0)]); }
  else if (e.key === 'Escape') { setSearchOpen(false); }
}

/* ── 对话框（iOS 风格 confirm / prompt） ── */
export function confirmDlg(message: string, cb: () => void, okLabel?: string) {
  setApp('dialog', { open: true, mode: 'confirm', message, value: '', placeholder: '', okLabel: okLabel || null, danger: true, requireValue: false, cb });
}
export function promptDlg(message: string, placeholder: string, cb: (v: string) => void, opts?: { value?: string; okLabel?: string; requireValue?: boolean }) {
  setApp('dialog', {
    open: true, mode: 'prompt', message, value: opts?.value || '', placeholder,
    okLabel: opts?.okLabel || null, danger: false, requireValue: opts?.requireValue !== false, cb
  });
}
export function dialogOk() {
  const d = { ...app.dialog };
  const v = (d.value || '').trim();
  if (d.mode === 'prompt' && d.requireValue && !v) { toast(t('toast.nameRequired'), 'error'); return; }
  setApp('dialog', 'open', false);
  if (d.cb) d.cb(d.mode === 'prompt' ? v : undefined);
}
export function dialogCancel() { setApp('dialog', 'open', false); setApp('dialog', 'cb', null); }

/* ── 认证动作 ── */
export function authErrorText(code: string): string {
  const map: Record<string, string> = {
    'bad-credentials': t('auth.badCredentials'),
    'locked': t('auth.locked'),
    'pw-short': t('auth.pwShort'),
    'pw-weak': t('auth.pwWeak'),
    'bad-username': t('auth.badUsername'),
    'already-setup': t('auth.alreadySetup'),
    'duplicate': t('auth.duplicateUser'),
    'last-admin': t('auth.lastAdmin'),
    'cannot-delete-self': t('auth.cannotDeleteSelf')
  };
  return map[code] || code;
}
export const pwChecks = createMemo(() => {
  const pw = app.authForm.password || '';
  return [
    { key: 'len', ok: pw.length >= 6, label: t('auth.pwRuleLen') },
    { key: 'letter', ok: /[A-Za-z]/.test(pw), label: t('auth.pwRuleLetter') },
    { key: 'digit', ok: /[0-9]/.test(pw), label: t('auth.pwRuleDigit') }
  ];
});
function afterAuth(sess: Session) {
  setUser(sess);
  refreshAuthState();
  setApp('authForm', { username: '', password: '', confirm: '', error: '', busy: false });
  toast(t('auth.welcome', { name: sess.username }), 'success');
  history.pushState(null, '', '/dashboard');
  window.dispatchEvent(new PopStateEvent('popstate'));
}
export async function doSetup() {
  const f = app.authForm;
  setApp('authForm', 'error', '');
  if (!f.username.trim()) { setApp('authForm', 'error', t('auth.badUsername')); return; }
  if (f.password !== f.confirm) { setApp('authForm', 'error', t('auth.pwMismatch')); return; }
  setApp('authForm', 'busy', true);
  try {
    const sess = await AuthAPI.setup(f.username.trim(), f.password);
    afterAuth(sess);
  } catch (e) {
    setApp('authForm', 'busy', false);
    setApp('authForm', 'error', authErrorText((e as Error).message));
  }
}
export async function doLogin() {
  const f = app.authForm;
  setApp('authForm', 'error', '');
  setApp('authForm', 'busy', true);
  try {
    const sess = await AuthAPI.login(f.username.trim(), f.password);
    afterAuth(sess);
  } catch (e) {
    setApp('authForm', 'busy', false);
    setApp('authForm', 'error', authErrorText((e as Error).message));
  }
}
export function doLogout() {
  AuthAPI.logout();
  setUser(null);
  closeSheet();
  dialogCancel();
  setApp('sidebarOpen', false);
  toast(t('auth.loggedOut'), 'info');
}
export async function changePassword() {
  const f = app.pwForm;
  setApp('pwForm', 'error', '');
  const cur = user();
  if (!cur) return;
  if (f.next !== f.confirm) { setApp('pwForm', 'error', t('auth.pwMismatch')); return; }
  setApp('pwForm', 'busy', true);
  try {
    await AuthAPI.changePassword(cur.username, f.current, f.next);
    setApp('pwForm', { current: '', next: '', confirm: '', error: '', busy: false });
    toast(t('auth.pwChanged'), 'success');
    /* 改密即失效全部旧会话：登出并要求重新登录（对齐后端 token 绑定） */
    AuthAPI.logout();
    setUser(null);
  } catch (e) {
    setApp('pwForm', 'busy', false);
    setApp('pwForm', 'error', authErrorText((e as Error).message));
  }
}
export async function createUser() {
  const f = app.userForm;
  if (!f.username.trim() || !f.password) { toast(t('toast.nameRequired'), 'error'); return; }
  try {
    await AuthAPI.createUser(f.username.trim(), f.password, f.role);
    refreshAuthState();
    toast(t('auth.userCreated', { name: f.username.trim() }), 'success');
    setApp('userForm', { username: '', password: '', role: 'member' });
    closeSheet();
  } catch (e) { toast(authErrorText((e as Error).message), 'error'); }
}
export function resetUserPassword(username: string) {
  promptDlg(t('auth.resetPwFor', { name: username }), '', v => {
    AuthAPI.resetUserPassword(username, v)
      .then(() => toast(t('auth.pwReset', { name: username }), 'success'))
      .catch(e => toast(authErrorText((e as Error).message), 'error'));
  }, { okLabel: t('common.ok') });
}
export function deleteUser(username: string) {
  const cur = user();
  if (!cur) return;
  if (username === cur.username) { toast(t('auth.cannotDeleteSelf'), 'error'); return; }
  confirmDlg(t('auth.confirmDeleteUser', { name: username }), async () => {
    try {
      /* deleteUser 已异步化（02-CONTEXT D-01 唯一签名偏离）：await 后错误直出后端 message */
      await AuthAPI.deleteUser(username);
      refreshAuthState();
      toast(t('auth.userDeleted', { name: username }), 'success');
    } catch (e) { toast(authErrorText((e as Error).message), 'error'); }
  });
}

/* ── Sheet ── */
export function openSheet(kind: string, param?: string) {
  setApp('form', {});
  setApp('sheet', { open: true, kind, param: param != null ? param : null });
  if (kind === 'terminal') {
    const c = app.containers.find(x => x.Id === param);
    setApp('term', { lines: [], input: '', hist: [], histIdx: -1, host: c ? ctrName(c) : String(param || '').slice(0, 12) });
  }
  if (kind === 'logs' || kind === 'stackLogs' || kind === 'serviceLogs') openLog(kind, param!);
}
export function closeSheet() {
  setApp('sheet', 'open', false);
  setApp('sheet', 'kind', null);
  setApp('sheet', 'param', null);
  stopLog();
}
export function sheetTitle(): string {
  const kind = app.sheet.kind, param = app.sheet.param;
  switch (kind) {
    case 'newContainer': return t('sheet.newContainer');
    case 'pullImage': return t('sheet.pullImage');
    case 'newVolume': return t('sheet.newVolume');
    case 'newNetwork': return t('sheet.newNetwork');
    case 'newUser': return t('auth.newUser');
    case 'copyToHost': return t('sheet.copyToHost');
    case 'tagImage': return t('sheet.tagImage');
    case 'logs': { const c = app.containers.find(x => x.Id === param); return t('sheet.logsTitle', { name: c ? ctrName(c) : '' }); }
    case 'terminal': { const c = app.containers.find(x => x.Id === param); return t('sheet.terminalTitle', { name: c ? ctrName(c) : '' }); }
    case 'stackLogs': { const s = app.stacks.find(x => x.id === param); return t('sheet.logsTitle', { name: s ? s.name : '' }); }
    case 'serviceLogs': { const p = String(param || '').split('||'); return t('sheet.logsTitle', { name: p[0] + ' / ' + p[1] }); }
    case 'history': { const img = app.images.find(x => x.Id === param); return t('sheet.historyTitle', { name: img ? (img.RepoTags[0] || img.Id.slice(0, 12)) : '' }); }
    default: return t('sheet.detail');
  }
}
export const imageHistoryLayers = [
  { id: 'sha256:' + rand(6), cmd: '/bin/sh -c #(nop) ADD file in /', size: '12 MB' },
  { id: 'sha256:' + rand(6), cmd: '/bin/sh -c #(nop)  CMD ["node"]', size: '4 KB' },
  { id: 'sha256:' + rand(6), cmd: 'RUN apk add --no-cache python3', size: '2.1 MB' },
  { id: 'sha256:' + rand(6), cmd: 'RUN apk add --no-cache nodejs', size: '5.4 MB' },
  { id: 'sha256:' + rand(6), cmd: 'FROM node:20-slim', size: '4.4 MB' }
];

/* ── 日志流（模拟 SSE：种子历史 + 周期追加，级别着色，过滤/跟随/暂停） ── */
function logTemplate(): string {
  const msgs: [string, string][] = [
    ['GET /api/health 200 ({n}ms)', 'INFO'],
    ['GET /api/metrics 200 ({n}ms)', 'INFO'],
    ['POST /api/orders 201 ({n}ms)', 'INFO'],
    ['connection accepted: client=172.18.0.{ip}', 'INFO'],
    ['cache hit ratio {n}% entries={m}', 'INFO'],
    ['GC pause {n}ms heap={m}MB', 'INFO'],
    ['slow query detected: {n}ms', 'WARN'],
    ['high memory usage: {n}%', 'WARN'],
    ['upstream timeout on /api ({n}ms), retrying', 'WARN'],
    ['connection reset by peer', 'ERROR'],
    ['unhandled exception in worker: ECONNREFUSED 127.0.0.1:5432', 'ERROR'],
    ['health check failed (attempt {n}/3)', 'ERROR']
  ];
  const pick = msgs[Math.floor(Math.random() * msgs.length)];
  const body = pick[0]
    .replace('{n}', String(Math.floor(Math.random() * 900 + 1)))
    .replace('{m}', String(Math.floor(Math.random() * 400 + 40)))
    .replace('{ip}', String(Math.floor(Math.random() * 20 + 2)));
  const ts = new Date().toISOString().slice(11, 19);
  return ts + ' ' + pick[1] + '  ' + body;
}
export function openLog(kind: string, param: string) {
  stopLog();
  const seeds: string[] = [];
  if (kind === 'logs') {
    const c = app.containers.find(x => x.Id === param);
    const svc = c ? (SERVICE_LOGS[ctrName(c)] || '') : '';
    seeds.push(...(svc ? svc.split('\n') : []));
  } else if (kind === 'stackLogs') {
    const st = app.stacks.find(x => x.id === param);
    if (st) parseServices(st.compose || '').forEach(name => {
      seeds.push(...(SERVICE_LOGS[name] || t('toast.noLogs', { name })).split('\n').map(l => '[' + name + '] ' + l));
    });
  } else if (kind === 'serviceLogs') {
    const p = String(param || '').split('||');
    seeds.push(...(SERVICE_LOGS[p[1]] || t('toast.noLogs', { name: p[1] })).split('\n').map(l => '[' + p[1] + '] ' + l));
  }
  const nowSec = Date.now() / 1000;
  const lines = seeds.map((l, i) => {
    const ts = new Date((nowSec - (seeds.length - i) * 2) * 1000).toISOString().slice(11, 19);
    const m = /^(\d{2}:\d{2}:\d{2})\s+(INFO|WARN|ERROR)\s+([\s\S]*)$/.exec(l)
      || /^(INFO|WARN|ERROR)\s+([\s\S]*)$/.exec(l);
    let raw = l;
    if (m && m.length === 4) raw = m[1] + ' ' + m[2] + '  ' + m[3];
    else if (m && m.length === 3) raw = ts + ' ' + m[1] + '  ' + m[2];
    return { id: 'seed-' + i, raw };
  });
  if (!lines.length) for (let k = 0; k < 8; k++) lines.push({ id: 'gen-' + k, raw: logTemplate() });
  setApp('logView', { kind, param, lines, filter: '', follow: true, paused: false, timer: 0 });
  const timer = window.setInterval(() => {
    if (app.logView.paused) return;
    let prefix = '';
    if (app.logView.kind === 'stackLogs') {
      const st = app.stacks.find(x => x.id === app.logView.param);
      const svcs = st ? parseServices(st.compose || '') : [];
      if (svcs.length) prefix = '[' + svcs[Math.floor(Math.random() * svcs.length)] + '] ';
    } else if (app.logView.kind === 'serviceLogs') {
      prefix = '[' + String(app.logView.param || '').split('||')[1] + '] ';
    }
    setApp('logView', 'lines', produce((arr: { id: string; raw: string }[]) => {
      arr.push({ id: 'l-' + Date.now(), raw: prefix + logTemplate() });
      if (arr.length > 500) arr.splice(0, arr.length - 500);
    }));
    scrollLogEnd();
  }, 1500);
  setApp('logView', 'timer', timer);
}
export function stopLog() {
  if (app.logView.timer) { clearInterval(app.logView.timer); setApp('logView', 'timer', 0); }
}
export const logFiltered = createMemo(() => {
  const q = app.logView.filter.trim().toLowerCase();
  if (!q) return app.logView.lines;
  return app.logView.lines.filter(l => l.raw.toLowerCase().includes(q));
});
export function logLevelClass(line: string): string {
  if (/\bERROR\b/.test(line)) return 'log-lv-error';
  if (/\bWARN\b/.test(line)) return 'log-lv-warn';
  return 'log-lv-info';
}
export function toggleLogPause() { setApp('logView', 'paused', !app.logView.paused); }
export function clearLog() { setApp('logView', 'lines', []); }
export function scrollLogEnd() {
  if (!app.logView.follow) return;
  const el = document.getElementById('log-viewer');
  if (el) el.scrollTop = el.scrollHeight;
}
export function logScroll() {
  const el = document.getElementById('log-viewer');
  if (!el) return;
  setApp('logView', 'follow', el.scrollHeight - el.scrollTop - el.clientHeight < 30);
}
export function copyLog() {
  const text = logFiltered().map(l => l.raw).join('\n');
  const done = (ok: boolean) => toast(ok ? t('log.copied') : t('log.copyFailed'), ok ? 'success' : 'error');
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(() => done(true), () => done(false));
    return;
  }
  try {
    const ta = document.createElement('textarea');
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand('copy');
    document.body.removeChild(ta);
    done(ok);
  } catch (e) { done(false); }
}

/* ── 容器操作 ── */
export function openDetail(id: string) {
  const c = app.containers.find(x => x.Id === id);
  if (!c) return;
  if (!c._stats) setApp('containers', x => x.Id === id, '_stats', genStats());
  setApp('detailId', id);
  setApp('detailTab', 'stats');
}
export function containerAction(id: string, action: 'start' | 'stop' | 'restart') {
  const c = app.containers.find(x => x.Id === id);
  if (!c) return;
  toast(t('toast.actionSent', { action: t('act.' + action), id: id.slice(0, 12) }), 'success');
  if (action === 'start') {
    setApp('containers', x => x.Id === id, 'State', 'running');
    if (!c.upHours) setApp('containers', x => x.Id === id, 'upHours', 1);
    setApp('containers', produce((arr: Container[]) => { delete (arr.find(x => x.Id === id) as Container)?._stats; }));
  } else if (action === 'stop') {
    setApp('containers', x => x.Id === id, 'State', 'exited');
    setApp('containers', x => x.Id === id, 'exitCode', 0);
    setApp('containers', x => x.Id === id, 'exitMinAgo', 1);
    setApp('containers', produce((arr: Container[]) => { delete arr.find(x => x.Id === id)?._stats; }));
  } else {
    setApp('containers', x => x.Id === id, 'State', 'running');
    if (!c.upHours) setApp('containers', x => x.Id === id, 'upHours', 1);
    setApp('containers', x => x.Id === id, '_stats', genStats());
  }
}
export function removeContainer(id: string) {
  confirmDlg(t('confirm.removeContainer'), () => {
    setApp('containers', arr => arr.filter(c => c.Id !== id));
    if (app.detailId === id) { history.pushState(null, '', '/containers'); window.dispatchEvent(new PopStateEvent('popstate')); }
    toast(t('toast.containerRemoved'), 'success');
  });
}
export function clearStopped() {
  const stopped = app.containers.filter(c => c.State === 'exited' && !c.system);
  if (!stopped.length) { toast(t('toast.noStopped'), 'info'); return; }
  confirmDlg(t('confirm.removeStopped', { n: stopped.length }), () => {
    setApp('containers', arr => arr.filter(c => !(c.State === 'exited' && !c.system)));
    toast(t('toast.removedCount', { n: stopped.length }), 'success');
  });
}
export function createContainer() {
  const f = app.form;
  if (!f.name || !f.image) { toast(t('toast.nameRequired'), 'error'); return; }
  const numOk = (v: string) => !v || /^\d+$/.test(v);
  if (cForm.ports.some(p => !numOk(p.host) || !numOk(p.ctn))) {
    toast(t('toast.invalidPort'), 'error'); return;
  }
  if (cForm.envs.some(e => !e.k.trim())) { toast(t('toast.envKeyRequired'), 'error'); return; }
  const ports: PortMap[] = cForm.ports
    .filter(p => p.ctn)
    .map(p => ({
      PrivatePort: +p.ctn,
      PublicPort: p.host ? +p.host : undefined,
      IP: p.ip || '0.0.0.0', Type: p.proto, Proto: p.proto
    }));
  const env = ['PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin',
    ...cForm.envs.filter(e => e.k.trim()).map(e => e.k.trim() + '=' + e.v)];
  const mounts = cForm.mounts.filter(m => m.host && m.ctn)
    .map(m => ({ source: m.host, dest: m.ctn, mode: m.ro ? 'ro' : 'rw' }));
  setApp('containers', produce((arr: Container[]) => {
    arr.unshift({
      Id: rand(12), Names: ['/' + f.name.trim()], Image: f.image.trim(), State: 'running',
      upHours: 1, Created: Date.now(), Ports: ports, system: false,
      Env: env, Mounts: mounts.length ? mounts : undefined,
      Networks: [cForm.network], RestartPolicy: cForm.restart,
      Cpus: cForm.cpus ? +cForm.cpus : undefined,
      MemoryMB: cForm.memory ? +cForm.memory : undefined,
      Entrypoint: f.entrypoint || undefined, Cmd: f.command || undefined
    });
  }));
  setCForm({ ports: [], envs: [], mounts: [], network: 'bridge', restart: 'no', cpus: '', memory: '', entrypoint: '' });
  toast(t('toast.createdContainer', { name: f.name.trim() }), 'success');
  closeSheet();
}
export function renameContainer(id: string) {
  const c = app.containers.find(x => x.Id === id);
  if (!c) return;
  promptDlg(t('prompt.renameCtn'), '', v => {
    setApp('containers', x => x.Id === id, 'Names', ['/' + v]);
    toast(t('toast.renamed', { name: v }), 'success');
  }, { value: ctrName(c), okLabel: t('common.ok') });
}
export function commitContainer(id: string) {
  const c = app.containers.find(x => x.Id === id);
  if (!c) return;
  promptDlg(t('prompt.imageTag'), c.Image + ':latest', v => {
    setApp('images', produce((arr: Image[]) => {
      arr.push({
        Id: 'sha256:' + rand(8), Repository: v.split(':')[0], RepoTags: [v],
        Size: Math.floor(Math.random() * 200 + 50) * 1000000, Created: Date.now(), system: false
      });
    }));
    toast(t('toast.committed', { name: v }), 'success');
  }, { value: c.Image + ':latest', okLabel: t('common.ok') });
}
export function exportContainer() {
  toast(t('toast.exporting'), 'info');
  setTimeout(() => toast(t('toast.exported'), 'success'), 1500);
}

/* ── 编排栈 ── */
export function createStack() {
  promptDlg(t('stack.createTitle'), t('stack.namePh'), name => {
    const s: Stack = {
      id: uid('stack'), name, compose_path: '/opt/stacks/' + name + '/compose.yaml',
      status: 'down', created: Date.now(), compose: defaultCompose(), composeDeployed: null, deployLog: []
    };
    setApp('stacks', produce((arr: Stack[]) => { arr.unshift(s); }));
    setApp('page', 'stacks', 1);
    setApp('selectedStack', s.id);
    toast(t('toast.createdStack', { name }), 'success');
  }, { okLabel: t('stack.create') });
}
export function renameStack(id: string) {
  const s = app.stacks.find(x => x.id === id);
  if (!s) return;
  promptDlg(t('prompt.renameStack'), '', v => {
    setApp('stacks', x => x.id === id, 'name', v);
    setApp('stacks', x => x.id === id, 'compose_path', '/opt/stacks/' + v + '/compose.yaml');
    toast(t('toast.stackRenamed', { name: v }), 'success');
  }, { value: s.name, okLabel: t('common.ok') });
}
export function stackAction(id: string) {
  const s = app.stacks.find(x => x.id === id);
  if (!s) return;
  if (s.status === 'up') {
    setApp('stacks', x => x.id === id, 'status', 'down');
    toast(t('toast.stackStopped', { name: s.name }), 'info');
  } else {
    setApp('stacks', x => x.id === id, 'status', 'up');
    toast(t('toast.stackStarted', { name: s.name }), 'success');
  }
}
export function removeStack(id: string) {
  confirmDlg(t('confirm.removeStack'), () => {
    setApp('stacks', arr => arr.filter(s => s.id !== id));
    if (app.selectedStack === id) setApp('selectedStack', null);
    toast(t('toast.stackRemoved'), 'success');
  });
}

/* 角色显示名（侧栏用户卡片 + 用户管理表格共用） */
export function roleLabel(role: string): string {
  return role === 'admin' ? t('auth.roleAdmin') : t('auth.roleMember');
}

export function saveSettings() {
  toast(t('toast.settingsSaved'), 'success');
}
export function deployStack(id: string) {
  const s = app.stacks.find(x => x.id === id);
  if (!s) return;
  const problem = yamlProblem(s.compose || '');
  if (problem) {
    toast(t('toast.yamlIssue') + (problem.line ? ' (line ' + problem.line + ')' : ''), 'error');
    return;
  }
  const svcs = parseServices(s.compose || '');
  if (!svcs.length) { toast(t('toast.yamlIssue'), 'error'); return; }
  const outLines = ['$ docker compose up -d', '[+] Running ' + (svcs.length + 1) + '/' + (svcs.length + 1), ' ⠿ Network ' + s.name + '_default  Created']
    .concat(svcs.map(x => ' ⠿ Container ' + s.name + '-' + x + '-1  Started'));
  setApp('stacks', x => x.id === id, 'deployLog', []);
  toast(t('toast.deployStarted'), 'info');
  outLines.forEach((line, i) => {
    setTimeout(() => {
      setApp('stacks', produce((arr: Stack[]) => {
        const st = arr.find(x => x.id === id);
        if (st) st.deployLog.push(line);
      }));
      const el = document.getElementById('wb-terminal');
      if (el) el.scrollTop = el.scrollHeight;
      if (i === outLines.length - 1) {
        setApp('stacks', produce((arr: Stack[]) => {
          const st = arr.find(x => x.id === id);
          if (st) { st.status = 'up'; st.composeDeployed = st.compose; }
        }));
        toast(t('toast.deployDone'), 'success');
      }
    }, 280 * (i + 1));
  });
}

/* ── 镜像 ── */
export function pullImage() {
  const f = app.form;
  if (!f.imageRef) { toast(t('toast.enterTag'), 'error'); return; }
  const ref = f.imageRef.trim();
  toast(t('toast.pullStarted', { name: ref }), 'info');
  setTimeout(() => {
    setApp('images', produce((arr: Image[]) => {
      arr.unshift({
        Id: 'sha256:' + rand(8), RepoTags: [ref], Repository: ref.split(':')[0],
        Size: Math.floor(Math.random() * 300 + 50) * 1000000, Created: Date.now(), system: false
      });
    }));
    toast(t('toast.pulled', { name: ref }), 'success');
  }, 1200);
  closeSheet();
}
export function tagImage(imgId: string) {
  const f = app.form;
  if (!f.newTag || !f.newTag.trim()) { toast(t('toast.enterTag'), 'error'); return; }
  const v = f.newTag.trim();
  setApp('images', produce((arr: Image[]) => {
    const img = arr.find(x => x.Id === imgId);
    if (img) { img.RepoTags.push(v); img.Repository = v.split(':')[0]; }
  }));
  toast(t('toast.tagged', { name: v }), 'success');
  closeSheet();
}
export function removeImage(id: string) {
  setApp('images', arr => arr.filter(i => i.Id !== id));
  toast(t('toast.imageRemoved'), 'success');
}
export function clearUnused(kind: 'images' | 'volumes' | 'networks') {
  const counts = {
    images: app.images.filter(x => !x.system).length,
    volumes: app.volumes.filter(x => !x.system).length,
    networks: app.networks.filter(x => !x.system).length
  };
  const n = counts[kind];
  const msgs = {
    images: { none: 'toast.noImages', confirm: 'confirm.removeImages', done: 'toast.imagesRemoved' },
    volumes: { none: 'toast.noVolumes', confirm: 'confirm.removeVolumes', done: 'toast.volumesRemoved' },
    networks: { none: 'toast.noNetworks', confirm: 'confirm.removeNetworks', done: 'toast.networksRemoved' }
  }[kind];
  if (!n) { toast(t(msgs.none), 'info'); return; }
  confirmDlg(t(msgs.confirm, { n }), () => {
    if (kind === 'images') setApp('images', arr => arr.filter(x => x.system));
    if (kind === 'volumes') setApp('volumes', arr => arr.filter(x => x.system));
    if (kind === 'networks') setApp('networks', arr => arr.filter(x => x.system));
    toast(t(msgs.done, { n }), 'success');
  });
}

/* ── 数据卷 / 网络 ── */
export function createVolume() {
  const f = app.form;
  if (!f.volumeName || !f.volumeName.trim()) { toast(t('toast.nameRequired'), 'error'); return; }
  const name = f.volumeName.trim();
  setApp('volumes', produce((arr: Volume[]) => {
    arr.push({ Name: name, Driver: 'local', Mountpoint: '/var/lib/docker/volumes/' + name, Usage: '0 B', system: false });
  }));
  toast(t('toast.createdVolume', { name }), 'success');
  closeSheet();
}
export function removeVolume(name: string) {
  const usedBy = (volumeUsedBy()[name] || []).map(u => u.name);
  if (usedBy.length) {
    confirmDlg(t('confirm.removeVolumeInUse', { name, names: usedBy.join(', ') }), () => {
      setApp('volumes', arr => arr.filter(v => v.Name !== name));
      toast(t('toast.volumeRemoved'), 'success');
    });
    return;
  }
  setApp('volumes', arr => arr.filter(v => v.Name !== name));
  toast(t('toast.volumeRemoved'), 'success');
}
export function createNetwork() {
  const f = app.form;
  if (!f.networkName || !f.networkName.trim()) { toast(t('toast.nameRequired'), 'error'); return; }
  const name = f.networkName.trim();
  setApp('networks', produce((arr: Network[]) => {
    arr.push({ ID: 'net' + rand(6), Name: name, Driver: f.driver || 'bridge', Subnet: '172.20.0.0/16', Containers: [], system: false });
  }));
  toast(t('toast.createdNetwork', { name }), 'success');
  closeSheet();
}
export function removeNetwork(id: string) {
  setApp('networks', arr => arr.filter(n => n.ID !== id));
  toast(t('toast.networkRemoved'), 'success');
}

/* ── 磁盘清理 ── */
export function sysdfPrune(label: string) {
  confirmDlg(t('confirm.pruneAll'), () => toast(t('toast.pruned', { v: label }), 'success'));
}
export function sysdfPruneAll() {
  confirmDlg(t('confirm.pruneAll'), () => toast(t('toast.prunedAll'), 'success'));
}

/* ── 设置 ── */
export function resetData() {
  confirmDlg(t('confirm.resetData'), () => {
    const d = defaultData();
    setApp('containers', d.containers);
    setApp('images', d.images);
    setApp('volumes', d.volumes);
    setApp('networks', d.networks);
    setApp('stacks', d.stacks);
    d.stacks.forEach(s => setApp('stacks', x => x.id === s.id, 'composeDeployed', s.compose));
    setApp('selectedStack', d.stacks.length ? d.stacks[0].id : null);
    setApp('detailId', null);
    history.pushState(null, '', '/dashboard');
    window.dispatchEvent(new PopStateEvent('popstate'));
    toast(t('toast.dataReset'), 'success');
  });
}

/* ── 模拟终端（bash 风格：历史 ↑↓、clear/exit、常见只读命令） ── */
function termFiles(): Record<string, string> {
  return {
    '/etc/os-release': 'PRETTY_NAME="Debian GNU/Linux 12"\nVERSION_ID="12"\nID=debian',
    '/etc/hostname': app.term.host,
    '/etc/hosts': '127.0.0.1\tlocalhost\n172.18.0.2\t' + app.term.host,
    '/proc/version': 'Linux version 6.6.12-linuxkit (mock)',
    '/proc/cpuinfo': 'processor\t: 0\nmodel name\t: x86_64\nprocessor\t: 1\nmodel name\t: x86_64'
  };
}
function termRun(cmd: string): string | null | false {
  const parts = cmd.split(/\s+/);
  const name = parts[0];
  const args = parts.slice(1);
  const arg = args.join(' ');
  const files = termFiles();
  const envC = app.containers.find(x => x.Id === app.sheet.param);
  const env = envC
    ? ['PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin', 'CONTAINER_NAME=' + ctrName(envC), 'LOG_LEVEL=info', 'APP_ENV=production']
    : ['PATH=/usr/bin'];
  switch (name) {
    case 'help': return t('term.help');
    case 'ls': return 'bin  boot  dev  etc  home  lib  lib64  media  mnt  opt  proc  root  run  sbin  srv  sys  tmp  usr  var';
    case 'pwd': return '/';
    case 'whoami': return 'root';
    case 'id': return 'uid=0(root) gid=0(root) groups=0(root)';
    case 'hostname': return app.term.host;
    case 'uname': return args.includes('-a') ? 'Linux ' + app.term.host + ' 6.6.12-linuxkit #1 SMP x86_64 GNU/Linux' : 'Linux';
    case 'env': return env.join('\n');
    case 'echo': return arg;
    case 'date': return new Date().toString();
    case 'uptime': return ' ' + new Date().toTimeString().slice(0, 8) + ' up 3 days, 22:15,  1 user,  load average: 0.12, 0.08, 0.05';
    case 'ps': return 'USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND\nroot         1  0.0  0.1  12345 6789 ?        Ss   10:23   0:01 /sbin/init\nroot        87  0.2  1.4  52310 18244 ?        S    10:24   2:31 node /app/server.js';
    case 'df': return 'Filesystem      Size  Used Avail Use% Mounted on\noverlay          50G   12G   36G  25% /\ntmpfs            64M     0   64M   0% /dev\nshm              64M     0   64M   0% /dev/shm';
    case 'free': return '              total        used        free      shared  buff/cache   available\nMem:          2.0Gi       512Mi       1.2Gi        64Mi       256Mi       1.4Gi\nSwap:            0B          0B          0B';
    case 'cat':
      if (!arg) return 'cat: missing operand';
      return files[arg] != null ? files[arg] : 'cat: ' + arg + ': No such file or directory';
    case 'history': return app.term.hist.map((h, i) => String(i + 1) + '  ' + h).join('\n') || '(empty)';
    case 'clear': return null;
    case 'exit': return false;
    default: return 'bash: ' + name + ': command not found';
  }
}
function termScroll() {
  const el = document.getElementById('sheet-terminal');
  if (el) el.scrollTop = el.scrollHeight;
}
export function termExec() {
  const raw = (app.term.input || '').trim();
  setApp('term', 'input', '');
  if (!raw) return;
  const prompt = 'root@' + app.term.host + ':/# ';
  setApp('term', 'lines', produce((arr: { prompt?: string; text: string; isCmd: boolean }[]) => { arr.push({ prompt, text: raw, isCmd: true }); }));
  setApp('term', 'hist', produce((h: string[]) => { h.push(raw); }));
  setApp('term', 'histIdx', app.term.hist.length);
  const out = termRun(raw);
  if (out === null) { setApp('term', 'lines', []); return; }
  if (out === false) { removeExecSession(); return; }
  if (out !== '') setApp('term', 'lines', produce((arr: { prompt?: string; text: string; isCmd: boolean }[]) => { arr.push({ text: out, isCmd: false }); }));
  termScroll();
}
export function termKey(e: KeyboardEvent) {
  if (e.isComposing) return;
  const t = app.term;
  if (e.key === 'ArrowUp') {
    e.preventDefault();
    if (t.hist.length) {
      setApp('term', 'histIdx', Math.max(0, t.histIdx - 1));
      setApp('term', 'input', t.hist[t.histIdx] || '');
    }
  } else if (e.key === 'ArrowDown') {
    e.preventDefault();
    if (t.hist.length) {
      setApp('term', 'histIdx', Math.min(t.hist.length, t.histIdx + 1));
      setApp('term', 'input', t.histIdx >= t.hist.length ? '' : t.hist[t.histIdx]);
    }
  } else if (e.key === 'l' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault();
    setApp('term', 'lines', []);
  } else if (e.key === 'c' && (e.ctrlKey || e.metaKey) && !t.input) {
    e.preventDefault();
    setApp('term', 'lines', produce((arr: { prompt?: string; text: string; isCmd: boolean }[]) => { arr.push({ prompt: 'root@' + t.host + ':/# ', text: '^C', isCmd: true }); }));
    termScroll();
  }
}
export function resizeTerminal() {
  promptDlg(t('prompt.termSize'), '80x24', v => {
    const m = /^(\d+)x(\d+)$/.exec(v);
    if (!m) { toast(t('toast.invalidSize'), 'error'); return; }
    toast(t('toast.termResized', { cols: m[1], rows: m[2] }), 'success');
  }, { value: '80x24', okLabel: t('common.ok'), requireValue: false });
}
export function removeExecSession() {
  toast(t('toast.sessionRemoved'), 'info');
  closeSheet();
}

/* ── 持久化（防抖 + 跨标签页同步） + 实时事件模拟 ── */
createRoot(() => {
  let persistTimer = 0;
  createEffect(() => {
    const replacer = (k: string, v: unknown) => (k && k.charAt(0) === '_' ? undefined : v);
    const snap = JSON.stringify({
      containers: app.containers, images: app.images, volumes: app.volumes,
      networks: app.networks, stacks: app.stacks,
      socketPath: app.socketPath, liveEvents: app.liveEvents
    }, replacer);
    clearTimeout(persistTimer);
    persistTimer = window.setTimeout(() => {
      try { localStorage.setItem(LS_DATA, snap); } catch (e) { /* ignore */ }
    }, 400);
  });
  window.addEventListener('storage', e => {
    if (e.key !== LS_DATA) return;
    try {
      const d = JSON.parse(e.newValue || 'null');
      if (!d || !Array.isArray(d.containers)) return;
      setApp('containers', d.containers);
      setApp('images', d.images);
      setApp('volumes', d.volumes || []);
      setApp('networks', d.networks || []);
      setApp('stacks', d.stacks || []);
      if (d.socketPath != null) setApp('socketPath', d.socketPath);
      if (d.liveEvents != null) setApp('liveEvents', !!d.liveEvents);
      toast(t('toast.refreshed'), 'info');
    } catch (err) { /* ignore */ }
  });

  setInterval(() => {
    if (!app.liveEvents) return;
    const drift = (v: number) => Math.max(5, Math.min(95, v + (Math.random() * 6 - 3)));
    setApp('sys', 'cpu', Math.round(drift(app.sys.cpu)));
    setApp('sys', 'mem', Math.round(drift(app.sys.mem)));
    setApp('sys', 'disk', Math.round(drift(app.sys.disk)));
    if (Math.random() < 0.3) return; /* 静默周期 */
    const pool = app.containers.filter(c => !c.system && c.State === 'running');
    if (!pool.length) return;
    const c = pool[Math.floor(Math.random() * pool.length)];
    const roll = Math.random();
    if (roll < 0.6) { /* health：仅刷新统计快照 */ }
    else if (roll < 0.9) {
      setApp('containers', produce((arr: Container[]) => {
        const x = arr.find(y => y.Id === c.Id);
        if (x) { x.upHours = x.upHours || 1; x._stats = genStats(); }
      }));
    } else {
      setApp('containers', produce((arr: Container[]) => {
        const x = arr.find(y => y.Id === c.Id);
        if (x) { x.State = 'exited'; x.exitCode = 137; x.exitMinAgo = 1; delete x._stats; }
      }));
    }
  }, 6000);
});

export { t, lang, setLang, ctrName, statusKind, formatBytes, timeAgo, passwordProblem };
export { statusLabel, statusText, portsText } from './lib/format';
