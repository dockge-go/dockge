/* 认证适配层：把 dockgy mock 实现替换为真实 /v1/* fetch（调用方接口不变）。
 * 冻结约定（02-CONTEXT D-01/D-03/D-05）：
 *  - 导出符号（Session/UserPublic/passwordProblem/AuthAPI）与 localStorage 键名不变；
 *    唯一有意偏离：deleteUser 异步化（Promise<boolean>）——错误明细铁律要求 await 后
 *    经 catch 直出后端 message，fire-and-forget 会静默丢失（store.ts 调用点已补 await）。
 *  - crate_token 存 {token,username,role,exp} JSON 包装：session() 保持同步签名读取；
 *    旧 mock 残留值（无 token 字段）经形状校验判无效，自动落登录/初始化页。
 *  - 后端错误包 {code,message} 映射回 mock 错误码；未命中的映射直出后端原文 message。
 *  - verify() 已删除（全仓零调用方）；'dockge.users' mock 存储不再读写。
 *  - needSetup()/listUsers() 同步签名靠模块级缓存维持；启动引导经 refreshNeedSetup/
 *    refreshUsers 更新缓存（见 store.ts bootstrapAuth）。 */

export interface Session { username: string; role: 'admin' | 'member'; exp: number }
export interface UserPublic { username: string; role: string; created: number }

const SESSION_KEY = 'crate_token';
const SESSION_TTL = 7 * 24 * 3600 * 1000;

/* 会话包装：localStorage 里的形状是前端内部自由度，后端只认 Authorization 头。 */
interface SessionEnvelope { token: string; username: string; role: 'admin' | 'member'; exp: number }

function readSession(): SessionEnvelope | null {
  try {
    const v = JSON.parse(localStorage.getItem(SESSION_KEY) || 'null');
    if (!v || typeof v.token !== 'string' || !v.token) return null; // 旧 mock 残留无 token → 判无效
    if (!v.username || typeof v.exp !== 'number' || v.exp < Date.now()) return null;
    return v;
  } catch (e) { return null; }
}
function writeSession(s: SessionEnvelope) {
  try { localStorage.setItem(SESSION_KEY, JSON.stringify(s)); } catch (e) { /* ignore */ }
}

/* 统一请求：成功（2xx 且 body.code===0）返回 body.data；失败 throw 后端原文 message
 * 或映射后的 mock 错误码（authErrorText 的 map[code]||code 天然支持直出）。 */
async function request(path: string, init?: RequestInit): Promise<any> {
  const res = await fetch(path, init);
  let body: any = null;
  try { body = await res.json(); } catch (e) { /* 非 JSON 响应按 HTTP 码处理 */ }
  if (!res.ok || !body || body.code !== 0) {
    throw new Error(mapErrorCode(res.status, body?.code, body?.message || ''));
  }
  return body.data;
}

/* 错误映射（per D-05）：已知语义映射回 mock 错误码供 i18n；未命中直出后端原文。 */
function mapErrorCode(httpStatus: number, _bodyCode: number | undefined, message: string): string {
  if (httpStatus === 401) return 'bad-credentials';
  if (httpStatus === 429) return 'locked';
  if (httpStatus === 409 && message.includes('最后一名管理员')) return 'last-admin';
  if (message.includes('不能删除当前登录账号')) return 'cannot-delete-self';
  if (httpStatus === 409) return message.includes('已完成初始化') ? 'already-setup' : 'duplicate';
  if (httpStatus === 400) {
    if (message.includes('至少6位')) return 'pw-short';
    if (message.includes('需包含字母和数字')) return 'pw-weak';
    if (message.includes('用户名不合法')) return 'bad-username';
  }
  return message || 'bad-credentials';
}

export function passwordProblem(pw: string): string | null {
  if (!pw || pw.length < 6) return 'short';
  if (!/[A-Za-z]/.test(pw) || !/[0-9]/.test(pw)) return 'weak';
  return null;
}

/* needSetup/listUsers 同步缓存：签名冻结，值由启动引导/调用后刷新。 */
let needSetupCache = true;
let usersCache: UserPublic[] = [];

async function authed(path: string, init?: RequestInit): Promise<any> {
  const s = readSession();
  if (!s) throw new Error('bad-credentials');
  return request(path, { ...init, headers: { ...(init?.headers || {}), Authorization: 'Bearer ' + s.token } });
}

function toSession(data: any): Session {
  const exp = Date.now() + SESSION_TTL;
  const s: SessionEnvelope = { token: data.accessToken, username: data.user.username, role: data.user.role, exp };
  writeSession(s);
  return { username: s.username, role: s.role, exp };
}

export const AuthAPI = {
  needSetup: (): boolean => needSetupCache,

  async refreshNeedSetup(): Promise<boolean> {
    const data = await request('/v1/setup/need');
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
    const data = await request('/v1/setup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    return toSession(data);
  },

  async login(username: string, password: string): Promise<Session> {
    const data = await request('/v1/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    return toSession(data);
  },

  logout() { try { localStorage.removeItem(SESSION_KEY); } catch (e) { /* ignore */ } },

  /* username 参数按 D-03 忽略（走 /v1/me/password 自身）；成功即清会话——
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

  listUsers(): UserPublic[] {
    return usersCache;
  },

  async refreshUsers(): Promise<void> {
    const data = await request('/v1/users');
    usersCache = (data as UserPublic[]) || [];
  },

  async createUser(username: string, password: string, role: string): Promise<boolean> {
    const p = passwordProblem(password);
    if (p) throw new Error('pw-' + p);
    if (!username || !/^[A-Za-z0-9_.-]{2,32}$/.test(username)) throw new Error('bad-username');
    await authed('/v1/users', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password, role }),
    });
    return true;
  },

  async resetUserPassword(username: string, nextPw: string): Promise<boolean> {
    const p = passwordProblem(nextPw);
    if (p) throw new Error('pw-' + p);
    await authed('/v1/users/' + encodeURIComponent(username) + '/password', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: nextPw }),
    });
    return true;
  },

  /* 唯一异步化偏差（见文件头）：错误需 await 后直出后端 message。 */
  async deleteUser(username: string): Promise<boolean> {
    await authed('/v1/users/' + encodeURIComponent(username), { method: 'DELETE' });
    return true;
  }
};
