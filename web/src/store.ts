import { LogReplay, type LogRecord } from './lib/log-replay';
/* 资源与界面状态；资源操作以服务端响应为准。 */
import { createSignal, createEffect, createMemo, createRoot } from 'solid-js';
import { createStore, produce } from 'solid-js/store';
import { t, setLang, lang } from './i18n';
import { defaultCompose, Container, Image, Volume, Network, Stack } from './lib/data';
import { ctrName, statusKind, formatBytes, timeAgo } from './lib/format';
import { AuthAPI, Session, passwordProblem } from './lib/auth';
import { yamlProblem, parseServices } from './lib/yaml';
import { api, stream, events, terminalPath } from './lib/api';
import { LogBuffer } from './lib/log-buffer';
import { mergeStackRefresh, mergeStackListRefresh } from './lib/stack-refresh';
import type { HostSystem } from './lib/system';

const LS_THEME = 'dockge.theme';
const PAGE_SIZE = 15;
/* ── 信号（会话 / 主题 / 搜索 / toast） ── */
export const [user, setUser] = createSignal<Session | null>(null);
export const [noUsers, setNoUsers] = createSignal(AuthAPI.needSetup());
export const [authMode, setAuthMode] = createSignal<'local' | 'oidc'>('local');
/* 引导完成标志：AuthScreen 在 false 期间只渲染加载态，避免 needSetup 缓存初值误判分支 */
export const [authBooted, setAuthBooted] = createSignal(false);
export const [authBootError, setAuthBootError] = createSignal('');
export async function refreshAuthState() {
  try {
    await AuthAPI.refreshNeedSetup();
    setNoUsers(AuthAPI.needSetup());
  } catch (e) { toast((e as Error).message, 'error'); }
}

/* 应用启动引导：刷新 needSetup 缓存后 AuthScreen 才做 setup/login 分支决策；
 * 调用点固定在模块作用域——组件挂载时序不定会漏接导致 authBooted 恒 false。 */
export async function bootstrapAuth() {
  setAuthBooted(false); setAuthBootError('');
  try {
    setAuthMode(await AuthAPI.mode());
    await AuthAPI.refreshNeedSetup();
    setNoUsers(AuthAPI.needSetup());
    const url = new URL(window.location.href);
    if (authMode() === 'oidc' && url.searchParams.get('oidc') === '1') {
      // 票据只领取一次；先移除标记，失败后的刷新才能回到正常登录入口。
      url.searchParams.delete('oidc');
      history.replaceState(null, '', url.pathname + url.search + url.hash);
      AuthAPI.logout(); setUser(null);
      afterAuth(await AuthAPI.oidcSession());
    } else {
      try { setUser(await AuthAPI.validateSession()); }
      catch (e) { setUser(null); toast((e as Error).message, 'error'); }
    }
    setAuthBooted(true);
  } catch (e) {
    setAuthBootError((e as Error).message);
  }
}

export function doOIDCLogin() { window.location.assign('/v1/auth/oidc/login'); }


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


export interface SheetState { open: boolean; kind: string | null; param: string | null }
export interface DialogState {
  open: boolean; mode: 'confirm' | 'prompt'; message: string; value: string;
  placeholder: string; okLabel: string | null; danger: boolean; requireValue: boolean;
  cb: ((value?: string) => void) | null;
}

export const [app, setApp] = createStore({
  containers: [] as Container[],
  images: [] as Image[],
  volumes: [] as Volume[],
  networks: [] as Network[],
  stacks: [] as Stack[],
  runtime: '', info: {} as Record<string, string>, disk: [] as DiskRow[],
  errors: {} as Record<string, string>, loading: true, refreshedAt: 0, actionBusy: false,
  version: '', deployment: '',
  system: null as HostSystem | null, systemReceivedAt: 0, systemError: '',

  showAll: false,
  page: { containers: 1, images: 1, volumes: 1, networks: 1, stacks: 1 } as Record<string, number>,
  selectedStack: null as string | null,
  detailId: null as string | null,
  detailTab: 'stats',
  editorFullscreen: false,
  sidebarOpen: false,

  dialog: { open: false, mode: 'confirm', message: '', value: '', placeholder: '', okLabel: null, danger: true, requireValue: true, cb: null } as DialogState,
  sheet: { open: false, kind: null, param: null } as SheetState,
  form: {} as Record<string, string>,
  authForm: { username: '', password: '', confirm: '', error: '', busy: false },
  pwForm: { current: '', next: '', confirm: '', error: '', busy: false },
  logView: { kind: null as string | null, param: null as string | null, lines: [] as { id: string; raw: string }[], filter: '', follow: true, paused: false, timer: 0, connection: 'ended' }
});

/* ── 派生数据（UI 同步的根基） ── */
export const runningCount = createMemo(() => app.containers.filter(c => c.State === 'running').length);
export const stoppedCount = createMemo(() => app.containers.filter(c => c.State === 'exited').length);
export const pausedCount = createMemo(() => app.containers.filter(c => c.State === 'paused').length);
export const stoppedEditable = createMemo(() => app.containers.filter(c => ['exited','dead','created'].includes(c.State) && !c.system).length);
export const recentContainers = createMemo(() => app.containers.slice(0, 5));
export const selectedStackObj = createMemo<Stack | null>(() => app.stacks.find(s => s.id === app.selectedStack) || null);
export const stackDirty = createMemo(() => {
  const s = selectedStackObj();
  return !!(s && (s.compose !== s.savedCompose || s.env !== s.savedEnv));
});
export const stackServiceNames = createMemo(() => {
  const s = selectedStackObj();
  return s ? parseServices(s.compose || '') : [];
});
const detail = createMemo(() => app.containers.find(c => c.Id === app.detailId) || null);
export { detail };
export const detailEnv = createMemo(() => detail()?.Env || []);
export const detailNets = createMemo(() => (detail()?.Networks || []).map(name => ({ name, ip: detail()?.NetworkIPs?.[name] || t('data.unknown') })));
export const detailMounts = createMemo(() => {
  const c = detail();
  return (c && c.Mounts) || [];
});

/* 卷引用与后端 inspect 快照一致，bind 不计作命名卷。 */
export const volumeUsedBy = createMemo<Record<string, { id: string; name: string }[]>>(() =>
  Object.fromEntries(app.volumes.map(v => [v.Name, v.UsedBy || []]))
);

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

export const sysdfRows = createMemo(() => app.disk.map(row => ({ ...row, label: t(({ images: 'img.title', containers: 'ctn.title', volumes: 'vol.title', buildCache: 'sysdf.buildCache' } as Record<string,string>)[row.key] || row.key) })));

/* ── 全局搜索 ── */
export interface SearchItem { group: string; name: string; meta: string; act: () => void }
export const searchResults = createMemo<SearchItem[]>(() => {
  const q = searchQ().trim().toLowerCase();
  if (!q) return [];
  const out: SearchItem[] = [];
  app.containers.forEach(c => {
    const name = ctrName(c);
    if (name.toLowerCase().includes(q) || c.Image.toLowerCase().includes(q) || c.Id.slice(0, 12).includes(q)) {
      out.push({ group: t('nav.containers'), name, meta: c.Image, act: () => { navigateToDetail(c.Id); } });
    }
  });
  app.images.forEach(img => {
    const tag = img.RepoTags[0] || '';
    if (tag.toLowerCase().includes(q) || (img.Repository || '').toLowerCase().includes(q)) {
      out.push({ group: t('nav.images'), name: tag || img.Id.slice(0, 12), meta: formatBytes(img.Size), act: () => { navigate('/images'); } });
    }
  });
  app.volumes.forEach(v => {
    if (v.Name.toLowerCase().includes(q)) {
      out.push({ group: t('nav.volumes'), name: v.Name, meta: v.Usage || '', act: () => { navigate('/volumes'); } });
    }
  });
  app.stacks.forEach(s => {
    if (s.name.toLowerCase().includes(q)) {
      out.push({ group: t('nav.stacks'), name: s.name, meta: s.status, act: () => { setApp('selectedStack', s.id); navigate('/stacks'); } });
    }
  });
  app.networks.forEach(n => { if (n.Name.toLowerCase().includes(q)) out.push({ group: t('nav.networks'), name: n.Name, meta: n.Driver, act: () => navigate('/networks') }); });
  return out;
});
function navigate(path: string) { history.pushState(null, '', path); window.dispatchEvent(new PopStateEvent('popstate')); }
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
  if (!app.dialog.open) return;
  const d = { ...app.dialog };
  const v = (d.value || '').trim();
  if (d.mode === 'prompt' && d.requireValue && !v) { toast(t('toast.nameRequired'), 'error'); return; }
  setApp('dialog', 'open', false);
  setApp('dialog', 'cb', null);
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
    'already-setup': t('auth.alreadySetup')
  };
  return map[code] || code;
}
export function passwordChecks(pw: string) {
  return [
    { key: 'len', ok: pw.length >= 6, label: t('auth.pwRuleLen') },
    { key: 'letter', ok: /[A-Za-z]/.test(pw), label: t('auth.pwRuleLetter') },
    { key: 'digit', ok: /[0-9]/.test(pw), label: t('auth.pwRuleDigit') }
  ];
}
export const pwChecks = createMemo(() => passwordChecks(app.authForm.password));
export const changePwChecks = createMemo(() => passwordChecks(app.pwForm.next));
function afterAuth(sess: Session) {
  setUser(sess);
  void refreshData();
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
  endDataSession();
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
    doLogout();
  } catch (e) {
    setApp('pwForm', 'busy', false);
    setApp('pwForm', 'error', authErrorText((e as Error).message));
  }
}
/* ── Sheet ── */
export function openSheet(kind: string, param?: string) {
  if (app.actionBusy) return;
  closeSheet();
  setApp('form', {});
  setApp('sheet', { open: true, kind, param: param ?? null });
  if (kind === 'history') { setImageHistoryLayers([]); const token = AuthAPI.token(); void api<HistoryLayer[]>('/resources/images/' + encodeURIComponent(param!) + '/history').then(rows => { if (token === AuthAPI.token() && app.sheet.kind === kind && app.sheet.param === param) setImageHistoryLayers(rows); }).catch(e => toast((e as Error).message, 'error')); }
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
    case 'tagImage': return t('sheet.tagImage');
    case 'logs': { const c = app.containers.find(x => x.Id === param); return t('sheet.logsTitle', { name: c ? ctrName(c) : '' }); }
    case 'terminal': { const c = app.containers.find(x => x.Id === param); return t('sheet.terminalTitle', { name: c ? ctrName(c) : '' }); }
    case 'stackLogs': { const s = app.stacks.find(x => x.id === param); return t('sheet.logsTitle', { name: s ? s.name : '' }); }
    case 'serviceLogs': { const p = String(param || '').split('||'); return t('sheet.logsTitle', { name: p[0] + ' / ' + p[1] }); }
    case 'history': { const img = app.images.find(x => x.Id === param); return t('sheet.historyTitle', { name: img ? (img.RepoTags[0] || img.Id.slice(0, 12)) : '' }); }
    default: return t('sheet.detail');
  }
}
interface HistoryLayer { id: string; cmd: string; size: string }
export const [imageHistoryLayers, setImageHistoryLayers] = createSignal<HistoryLayer[]>([]);
let logController: AbortController | null = null;
let resetLogBuffer = () => {};
let logGeneration = 0, logSeq = 0;
export function openLog(kind: string, param: string) {
  stopLog(); setPausedLog([]);
  setApp('logView', { kind, param, lines: [], filter: '', follow: true, paused: false, timer: 0, connection: 'connecting' });
  const generation = logGeneration;
  const buffer = new LogBuffer(), replay = new LogReplay(); resetLogBuffer = () => buffer.clear();
  const append = (text: string) => {
    const rows = buffer.push(text);
    setApp('logView', 'lines', rows.map(raw => ({ id: String(++logSeq), raw })));
    requestAnimationFrame(scrollLogEnd);
  };
  const connect = () => {
    if (generation !== logGeneration || !user()) return;
    let name = param, type = 'container-logs'; const extra: Record<string,string> = {};
    if (kind === 'stackLogs') { name = app.stacks.find(s => s.id === param)?.name || param; type = 'compose-logs'; }
    if (kind === 'serviceLogs') { const p = param.split('||'); name = p[0]; extra.service = p[1]; type = 'compose-logs'; }
    const controller = new AbortController(); logController = controller;
    const since = replay.begin();
    if (since) { extra.since = since; extra.tail = 'all'; }
    void events(terminalPath(name, type, extra), ({event, data}) => {
      if (generation !== logGeneration) return;
      if (event === 'ready') setApp('logView', 'connection', 'live');
      if (event === 'log') append(replay.push(data as LogRecord));
      if (event === 'error') throw new Error(String(data));
    }, controller.signal).catch(e => {
      if (generation === logGeneration && !controller.signal.aborted) append('\n' + (e as Error).message + '\n');
    }).finally(() => {
      if (generation !== logGeneration) return;
      setApp('logView', 'connection', 'ended');
      setApp('logView', 'timer', window.setTimeout(connect, 3000));
    });
  };
  connect();
}
export function stopLog() {
  ++logGeneration; clearTimeout(app.logView.timer);
  logController?.abort(); logController = null;
  setApp('logView', 'connection', 'ended');
}
export const logFiltered = createMemo(() => {
  const q = app.logView.filter.trim().toLowerCase();
  const lines = app.logView.paused ? pausedLog() : app.logView.lines;
  if (!q) return lines;
  return lines.filter(l => l.raw.toLowerCase().includes(q));
});
export function logLevelClass(line: string): string {
  if (/\bERROR\b/.test(line)) return 'log-lv-error';
  if (/\bWARN\b/.test(line)) return 'log-lv-warn';
  return 'log-lv-info';
}
const [pausedLog, setPausedLog] = createSignal<{ id:string; raw:string }[]>([]);
export function toggleLogPause() { setPausedLog(app.logView.lines.slice()); setApp('logView', 'paused', !app.logView.paused); }
export function clearLog() { resetLogBuffer(); setPausedLog([]); setApp('logView', 'lines', []); }
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

/* 所有操作等待真实结果；失败只报错误，不提前伪造本地状态。 */
export function openDetail(id: string) { setApp('detailId', id); setApp('detailTab', 'stats'); }
async function perform(work: () => Promise<unknown>, message?: string): Promise<boolean> {
  if (app.actionBusy) return false;
  const token = AuthAPI.token(); setApp('actionBusy', true);
  try {
    await work();
    if (token !== AuthAPI.token()) return false;
    if (message) toast(message, 'success');
    return true;
  } catch (e) { if (token === AuthAPI.token()) toast((e as Error).message, 'error'); return false; }
  finally { if (token === AuthAPI.token()) { await freshData(); if (token === AuthAPI.token()) setApp('actionBusy', false); } }
}
const action = (kind: string, id: string, op: string, value?: string) => api<{output:string}>(`/resources/${kind}/${encodeURIComponent(id)}/${op}`, { method: 'POST', body: JSON.stringify({ value }) });
export function containerAction(id: string, op: 'start'|'stop'|'restart') { void perform(() => action('containers', id, op)); }
export function removeContainer(id: string) { const c=app.containers.find(c=>c.Id===id); confirmDlg(t('confirm.deleteContainerScope', {name:c ? ctrName(c) : id}), () => { void perform(() => action('containers', id, 'delete'), t('toast.containerRemoved')); }); }
async function removeBatch(kind: string, ids: string[]) {
  const failures: string[] = []; let removed = 0;
  for (const id of ids) { try { await action(kind,id,'delete'); removed++; } catch(e) { failures.push(id + ': ' + (e as Error).message); } }
  if (failures.length) throw new Error(t('data.batchResult', {removed, failed:failures.length}) + '\n' + failures.join('\n'));
  toast(t('data.batchResult', {removed, failed:0}), 'success');
}
export function clearStopped() {
  const ids = app.containers.filter(c => ['exited','dead','created'].includes(c.State) && !c.system).map(c => c.Id);
  confirmDlg(t('confirm.removeStopped', { n: ids.length }), () => { void perform(() => removeBatch('containers', ids)); });
}
export async function createContainer() {
  const f = { ...app.form };
  if (await perform(() => api('/resources/containers', { method:'POST', body: JSON.stringify({name:f.name?.trim(), image:f.image?.trim(), hostPort:f.hostPort || '', containerPort:f.containerPort || '', command:f.command || ''}) }), t('toast.createdContainer', {name:f.name || ''}))) closeSheet();
}
export function renameContainer(id: string) { promptDlg(t('prompt.renameCtn'), '', v => { void perform(() => action('containers', id, 'rename', v)); }, {value: ctrName(app.containers.find(c => c.Id === id)!)}); }
export function commitContainer(id: string) { promptDlg(t('prompt.imageTag'), '', v => { void perform(() => action('containers', id, 'commit', v)); }); }
export function createStack() {
  promptDlg(t('stack.createTitle'), t('stack.namePh'), async name => {
    if (await perform(() => api('/stacks', { method:'POST', body: JSON.stringify({name, yaml:defaultCompose(), env:''}) }), t('toast.createdStack', {name}))) setApp('selectedStack', name);
  });
}
export function stackAction(id: string, op?: string) {
  const s = app.stacks.find(s => s.id === id); if (s) void runStack(id, op || (s.status === 'up' ? 'stop' : 'start'), false);
}
export async function removeStack(id: string) {
  try {
    const token = AuthAPI.token();
    const plan = await api<DeletePlan>('/stacks/' + encodeURIComponent(id) + '/delete-plan');
    if (token !== AuthAPI.token()) return;
    if (plan.blockers.length) { toast(plan.blockers.join('\n'), 'error'); return; }
    const summary = t('stack.deleteScope', { directory:plan.directory, containers:plan.containers.length, volumes:plan.volumes.length, networks:plan.networks.length, images:plan.images.length }) + (plan.retained.length ? '\n' + t('stack.retained') + '\n' + plan.retained.join('\n') : '');
    confirmDlg(summary, () => { void perform(async () => {
      const result = await api<{output:string}>('/stacks/' + encodeURIComponent(id), {method:'DELETE'});
      if (token === AuthAPI.token() && result.output) toast(result.output, 'info');
    }, t('toast.stackRemoved')); });
  } catch(e) { toast((e as Error).message, 'error'); }
}
interface DeletePlan { directory:string; containers:string[]; volumes:string[]; networks:string[]; images:string[]; retained:string[]; blockers:string[] }
async function saveDraft(id: string, yaml: string, env: string) {
  await api('/stacks/' + encodeURIComponent(id), { method:'PUT', body:JSON.stringify({name:id, yaml, env}) });
}
export async function saveStack(id: string) {
  const s = app.stacks.find(s => s.id === id); if (!s || !s.managed || s.busy) return;
  const submitted = s.compose, env = s.env;
  setApp('stacks', s => s.id === id, 'busy', true);
  const token = AuthAPI.token();
  try { await saveDraft(id, submitted, env); if (token === AuthAPI.token()) { setApp('stacks', s => s.id === id, {savedCompose:submitted, savedEnv:env}); toast(t('stack.saved'), 'success'); } }
  catch(e) { if (token === AuthAPI.token()) toast((e as Error).message, 'error'); }
  finally { if (token === AuthAPI.token()) { setApp('stacks', s => s.id === id, 'busy', false); await freshData(); } }
}
const streams = new Set<AbortController>();
async function runStack(id: string, op: string, save: boolean) {
  const s = app.stacks.find(s => s.id === id); if (!s || !s.managed || s.busy || s.loadError) return;
  const submitted = s.compose, env = s.env, token = AuthAPI.token();
  if (save) { const problem = yamlProblem(submitted); if (problem) { toast(problem.msg, 'error'); return; } }
  const controller = new AbortController(); streams.add(controller);
  setApp('stacks', s => s.id === id, {busy:true, deployLog:[]});
  let output = '';
  try {
    if (save) { await saveDraft(id, submitted, env); if (token === AuthAPI.token()) setApp('stacks', s => s.id === id, {savedCompose:submitted, savedEnv:env}); }
    await stream('/stacks/' + encodeURIComponent(id) + '/' + op, text => {
      if (token !== AuthAPI.token()) return;
      output += text; setApp('stacks', s => s.id === id, 'deployLog', output.split('\n'));
      requestAnimationFrame(() => { const el = document.getElementById('wb-terminal'); if (el) el.scrollTop = el.scrollHeight; });
    }, undefined, controller.signal);
    if (token === AuthAPI.token()) { toast(t('toast.deployDone'), 'success'); }
  } catch(e) { if (token === AuthAPI.token()) toast((e as Error).message, 'error'); }
  finally { streams.delete(controller); if (token === AuthAPI.token()) { setApp('stacks', s => s.id === id, 'busy', false); await freshData(); } }
}
export function deployStack(id: string) { void runStack(id, 'start', true); }

/* 镜像、卷、网络操作都由运行时决定结果；禁止 force/prune 绕过共享引用。 */
export async function pullImage() {
  const ref = app.form.imageRef?.trim(); if (!ref) return;
  const token = AuthAPI.token(), controller = new AbortController(); streams.add(controller); setApp('form', 'output', '');
  try { await perform(() => stream('/images/pull', text => { if (token === AuthAPI.token()) setApp('form', 'output', old => (old || '') + text); }, {image:ref}, controller.signal), t('toast.pulled', {name:ref})); }
  finally { streams.delete(controller); }
}
export async function tagImage(id: string) {
  if (await perform(() => action('images', id, 'tag', app.form.newTag?.trim()), t('toast.tagged', {name:app.form.newTag || ''}))) closeSheet();
}
export function removeImage(id: string) { const image=app.images.find(i=>i.Id===id); if (image?.InUse) { toast(t('data.imageUsedBy', {names:(image.UsedBy || []).map(c=>c.name+' ('+c.id.slice(0,12)+')').join(', ')}), 'error'); return; } confirmDlg(t('data.deleteImageScope', {name:image?.RepoTags.join(', ') || id}), () => { void perform(() => action('images', id, 'delete'), t('toast.imageRemoved')); }); }
export function clearUnused(kind: 'images'|'volumes'|'networks') {
  if (app.errors[kind] || app.errors.containers) { toast(app.errors[kind] || app.errors.containers, 'error'); return; }
  const ids = kind === 'images' ? app.images.filter(i => !i.system && !i.InUse).map(i => i.Id) : kind === 'volumes' ? app.volumes.filter(v => !v.system && !v.UsedBy?.length).map(v => v.Name) : app.networks.filter(n => !n.system && !n.Containers?.length).map(n => n.ID);
  confirmDlg(t('data.deleteUnused', { n:ids.length, kind:t('nav.' + kind) }), () => { void perform(() => removeBatch(kind, ids)); });
}
export async function createVolume() {
  if (await perform(() => api('/resources/volumes', {method:'POST', body:JSON.stringify({name:app.form.volumeName?.trim()})}), t('toast.createdVolume', {name:app.form.volumeName || ''}))) closeSheet();
}
export function removeVolume(name: string) { void perform(() => action('volumes', name, 'delete'), t('toast.volumeRemoved')); }
export async function createNetwork() {
  if (await perform(() => api('/resources/networks', {method:'POST', body:JSON.stringify({name:app.form.networkName?.trim(), driver:app.form.driver || 'bridge'})}), t('toast.createdNetwork', {name:app.form.networkName || ''}))) closeSheet();
}
export function removeNetwork(id: string) { void perform(() => action('networks', id, 'delete'), t('toast.networkRemoved')); }
export function sysdfPrune(key: string) {
  if (key === 'buildCache') confirmDlg(t('sysdf.cacheConfirm'), () => { void perform(async () => { const r=await action('buildCache','unused','delete'); toast(r.output || t('sysdf.cacheDone'), 'success'); }); });
  else if (key === 'containers') clearStopped();
  else if (key === 'images' || key === 'volumes' || key === 'networks') clearUnused(key);
}

interface DiskRow {key:string; used:string; reclaim:string; count:number; reclaimBytes:number; reclaimKnown:boolean}
interface Snapshot { containers:Container[]; images:Image[]; volumes:Volume[]; networks:Network[]; info:Record<string,string>; disk:DiskRow[]; errors:Record<string,string>; runtime:string }
interface StackDTO {name:string; status:number; managed:boolean; configFiles?:string; yaml?:string; env?:string; runtimeError?:string; loadError?:string}
interface LiveSnapshot { resources:Snapshot; stacks:StackDTO[]; health:{version:string; runtime:{cli:string; compose:string; ready:boolean; error?:string; stacks:{path:string; writable:boolean; error?:string}}} }
let refreshGeneration = 0;
let refreshPromise: Promise<void> | null = null;
let liveController: AbortController | null = null;
let liveTimer = 0;
let settleRefresh: (() => void) | null = null;
async function freshData() { stopLiveData(); await refreshData(); }
function stopLiveData() {
  ++refreshGeneration; clearTimeout(liveTimer); liveController?.abort(); liveController = null;
  settleRefresh?.(); settleRefresh = null; refreshPromise = null;
}
function applySnapshot(snapshot: LiveSnapshot) {
  const r = snapshot.resources, errors = {...r.errors}, h = snapshot.health;
  setApp({containers:r.containers || [], images:r.images || [], volumes:r.volumes || [], networks:r.networks || [], info:r.info, disk:r.disk, runtime:r.runtime});
  setApp('version', h.version);
  setApp('deployment', h.runtime.cli + ' / ' + h.runtime.compose + ' · ' + h.runtime.stacks.path);
  if (!h.runtime.ready) errors.runtime = h.runtime.error || t('data.unknown');
  if (!h.runtime.stacks.writable) errors.stacksDirectory = h.runtime.stacks.error || t('data.unknown');
  if (!errors.stacks) {
    const rows = mergeStackListRefresh(snapshot.stacks.map(d => {
      const incoming: Stack = {id:d.name, name:d.name, compose_path:d.configFiles || '', status:stackStatus(d.status), compose:d.yaml || '', savedCompose:d.yaml || '', env:d.env || '', savedEnv:d.env || '', managed:d.managed, deployLog:[], busy:false, loadError:d.loadError || '', runtimeError:d.runtimeError || ''};
      return incoming;
    }), app.stacks);
    setApp('stacks',rows);
    if (!rows.some(s => s.id === app.selectedStack)) setApp('selectedStack',rows[0]?.id || null);
  }
  setApp('errors',errors); setApp('refreshedAt',Date.now()); setApp('loading',false);
}
// 手动刷新与操作完成都重连同一个 SSE；不并行保留 HTTP 轮询。
export function refreshData(): Promise<void> {
  if (!user() || document.visibilityState === 'hidden') return Promise.resolve();
  if (refreshPromise) return refreshPromise;
  stopLiveData();
  const token = AuthAPI.token(), generation = refreshGeneration, controller = new AbortController();
  liveController = controller;
  refreshPromise = new Promise<void>(resolve => { settleRefresh = resolve; });
  const initial = refreshPromise;
  const settle = () => { settleRefresh?.(); settleRefresh = null; refreshPromise = null; };
  void events('/events',({event,data}) => {
    if (generation !== refreshGeneration || token !== AuthAPI.token()) return;
    if (event === 'snapshot') { applySnapshot(data as LiveSnapshot); settle(); }
    if (event === 'system') setApp({system:data as HostSystem,systemReceivedAt:Date.now(),systemError:''});
    if (event === 'error') throw new Error(String(data));
  },controller.signal).catch(e => {
    if (generation === refreshGeneration && !controller.signal.aborted) setApp({errors:{request:(e as Error).message},loading:false,system:null,systemError:(e as Error).message});
  }).finally(() => {
    if (generation !== refreshGeneration) return;
    // 正常 EOF 也表示订阅已失联，不能继续展示为有效实时快照。
    setApp('errors','request',app.errors.request || t('data.reconnecting'));
    setApp({system:null,systemReceivedAt:0,loading:false}); settle(); liveController = null;
    liveTimer = window.setTimeout(() => { if (user()) void refreshData(); },3000);
  });
  return initial;
}
function stackStatus(n: number): Stack['status'] { return n === 3 ? 'up' : n === 0 ? 'unknown' : 'down'; }
function endDataSession() {
  stopLiveData();
  streams.forEach(c => c.abort()); streams.clear();
  setApp({system:null, systemReceivedAt:0, systemError:'', containers:[], images:[], volumes:[], networks:[], stacks:[], info:{}, disk:[], errors:{}, selectedStack:null, detailId:null, loading:true, actionBusy:false, version:'', deployment:'', refreshedAt:0, form:{}, pwForm:{current:'',next:'',confirm:'',error:'',busy:false}});
  setImageHistoryLayers([]);
}
void bootstrapAuth().then(() => { if (user()) void refreshData(); });
document.addEventListener('visibilitychange', () => { if (document.visibilityState === 'hidden') stopLiveData(); else if (user()) void refreshData(); });
window.addEventListener('storage', e => {
 if (e.key !== 'crate_token') return;
 closeSheet(); dialogCancel(); endDataSession(); setToasts([]);
 setUser(AuthAPI.session());
 if (user()) void refreshAuthState().then(() => refreshData());
});

export { t, lang, setLang, ctrName, statusKind, formatBytes, timeAgo, passwordProblem };
export { statusLabel, statusText, portsText } from './lib/format';
