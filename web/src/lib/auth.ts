/* /v1 认证接口与浏览器会话存储。 */
import { t } from '../i18n';
import { requests } from './request-scope';

export interface Session { username: string; role: 'admin'; exp: number }

const SESSION_KEY = 'crate_token';

/* 会话包装：localStorage 里的形状是前端内部自由度，后端只认 Authorization 头。 */
interface SessionEnvelope { token: string; username: string; role: 'admin'; exp: number }

function readSession(): SessionEnvelope | null {
  try {
    const v = JSON.parse(localStorage.getItem(SESSION_KEY) || 'null');
    if (!v || typeof v.token !== 'string' || !v.token) return null; // 无服务端 token 的旧缓存不视作已登录
    if (!v.username || typeof v.exp !== 'number' || v.exp < Date.now()) return null;
    return v;
  } catch (e) { return null; }
}
function writeSession(s: SessionEnvelope) {
  try { localStorage.setItem(SESSION_KEY, JSON.stringify(s)); } catch (e) { /* ignore */ }
}

/* 统一请求：成功（2xx 且 body.code===0）返回 body.data；失败 throw 后端原文 message
 * 并保留后端错误明细。 */
async function request<T = unknown>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try { res = await fetch(path, init); }
  catch (e) { throw new Error(`${path}: ${(e as Error).message}`); }
  let body: { code: number; message?: string; data: T } | null = null;
  try { body = await res.json(); } catch (e) { /* 非 JSON 响应按 HTTP 码处理 */ }
  if (!res.ok || !body || body.code !== 0) {
    throw new Error(mapErrorCode(res.status, body?.code, body?.message || ''));
  }
  return body.data;
}

/* 后端 message 优先，不用泛化文案覆盖可定位的原因。 */
function mapErrorCode(httpStatus: number, _bodyCode: number | undefined, message: string): string {
  return message || `HTTP ${httpStatus}: 认证请求未返回错误详情`;
}

export function passwordProblem(pw: string): string | null {
  if (!pw || pw.length < 6) return 'short';
  if (!/[A-Za-z]/.test(pw) || !/[0-9]/.test(pw)) return 'weak';
  return null;
}

/* 初始化状态只来自服务端，不从本地账号缓存推断。 */
let needSetupCache = true;

async function authed<T = unknown>(path: string, init?: RequestInit): Promise<T> {
  const s = readSession();
  if (!s) throw new Error('bad-credentials');
  return requests.run(signal => request<T>(path, { ...init, signal, headers: { ...(init?.headers || {}), Authorization: 'Bearer ' + s.token } }), init?.signal);
}

interface LoginData { accessToken: string; expiresAt: number; user: { username: string; role: 'admin' } }
function toSession(data: LoginData): Session {
  const exp = data.expiresAt;
  if (!Number.isFinite(exp) || exp <= Date.now()) throw new Error(t('auth.invalidExpiry'));
  const s: SessionEnvelope = { token: data.accessToken, username: data.user.username, role: data.user.role, exp };
  requests.cancel();
  writeSession(s);
  return { username: s.username, role: s.role, exp };
}

export const AuthAPI = {
  token: (): string | null => readSession()?.token || null,
  needSetup: (): boolean => needSetupCache,

  async refreshNeedSetup(): Promise<boolean> {
    const data = await request<{ needSetup: boolean }>('/v1/setup/need');
    needSetupCache = !!data.needSetup;
    return needSetupCache;
  },

  session(): Session | null {
    const s = readSession();
    if (!s) return null;
    return { username: s.username, role: s.role, exp: s.exp };
  },

  async setup(username: string, password: string): Promise<Session> {
    if (!username || !/^[A-Za-z0-9_.-]{2,32}$/.test(username)) throw new Error('bad-username');
    const p = passwordProblem(password);
    if (p) throw new Error('pw-' + p);
    const data = await request<LoginData>('/v1/setup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    return toSession(data);
  },

  async login(username: string, password: string): Promise<Session> {
    const data = await request<LoginData>('/v1/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    return toSession(data);
  },

  logout() { requests.cancel(); try { localStorage.removeItem(SESSION_KEY); } catch (e) { /* ignore */ } },

  /* username 参数忽略（走 /v1/me/password 自身）；成功即清会话——
   * 服务端 JWT 摘要绑定已使旧 token 失效，前端同步清包装回登录页。 */
  async changePassword(_username: string, currentPw: string, nextPw: string): Promise<boolean> {
    const p = passwordProblem(nextPw);
    if (p) throw new Error('pw-' + p);
    await authed('/v1/me/password', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ oldPassword: currentPw, newPassword: nextPw }),
    });
    try { localStorage.removeItem(SESSION_KEY); } catch (e) { /* ignore */ }
    return true;
  },

  async mode(): Promise<'local' | 'oidc'> {
    const data = await request<{ mode: 'local' | 'oidc' }>('/v1/auth/config');
    if (data.mode !== 'local' && data.mode !== 'oidc') throw new Error(t('auth.invalidMode'));
    return data.mode;
  },

  async oidcSession(): Promise<Session> {
    const data = await request<LoginData>('/v1/auth/oidc/session', {method: 'POST'});
    return toSession(data);
  },

  async validateSession(): Promise<Session | null> {
    const s = readSession();
    if (!s) return null;
    try {
      const data = await authed<{username: string}>('/v1/me');
      if (s.token !== this.token()) return null;
      const verified = {...s, username:data.username, role:'admin' as const};
      writeSession(verified);
      return {username:verified.username, role:verified.role, exp:verified.exp};
    } catch (e) {
      if (s.token !== this.token()) return null;
      this.logout();
      throw e;
    }
  }
};
