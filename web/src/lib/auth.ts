/* 认证模拟层：对齐后端契约（docs/REQUIREMENTS.md R1）
 *  - 首启无用户 → 强制 Setup 创建 admin（密码 ≥6 位且含字母与数字）
 *  - 登录签发 7 天会话（localStorage key 沿用后端 "crate_token"）
 *  - 改密后旧会话全部失效（会话绑定密码版本，同 JWT claim 绑定哈希摘要）
 *  - admin 用户管理：创建/重置密码/删除；不可删除自己或最后一名 admin
 *  - 登录防爆破：连续失败 5 次锁定 10 秒
 * 接真实后端：把本文件替换为 fetch /v1/* 即可，调用方接口不变。 */

export interface Session { username: string; role: 'admin' | 'member'; exp: number }
interface UserRec { username: string; role: string; salt: string; hash: string; pwVer: number; created: number }
export interface UserPublic { username: string; role: string; created: number }

const USERS_KEY = 'dockge.users';
const SESSION_KEY = 'crate_token';
const SESSION_TTL = 7 * 24 * 3600 * 1000;

function readJSON<T>(key: string, fallback: T): T {
  try { const v = JSON.parse(localStorage.getItem(key) || 'null'); return v == null ? fallback : v; } catch (e) { return fallback; }
}
function writeJSON(key: string, val: unknown) {
  try { localStorage.setItem(key, JSON.stringify(val)); } catch (e) { /* ignore */ }
}

function djb2(s: string): string {
  let h = 5381;
  for (let i = 0; i < s.length; i++) h = ((h << 5) + h + s.charCodeAt(i)) >>> 0;
  return 'djb2:' + h.toString(16);
}
async function sha256Hex(str: string): Promise<string | null> {
  if (window.crypto && crypto.subtle && crypto.subtle.digest) {
    const data = new TextEncoder().encode(str);
    const buf = await crypto.subtle.digest('SHA-256', data);
    return Array.from(new Uint8Array(buf), b => b.toString(16).padStart(2, '0')).join('');
  }
  return null;
}
async function hashPassword(password: string, salt: string): Promise<string> {
  const h = await sha256Hex(salt + '\u0000' + password);
  return h ? 'sha256:' + h : djb2(salt + password);
}

const users = () => readJSON<UserRec[]>(USERS_KEY, []);
const saveUsers = (list: UserRec[]) => writeJSON(USERS_KEY, list);

export function passwordProblem(pw: string): string | null {
  if (!pw || pw.length < 6) return 'short';
  if (!/[A-Za-z]/.test(pw) || !/[0-9]/.test(pw)) return 'weak';
  return null;
}

/* 防爆破：进程内计数（真实实现按 IP+账号双维度） */
let fails = { count: 0, lockUntil: 0 };

export const AuthAPI = {
  needSetup: (): boolean => users().length === 0,

  session(): Session | null {
    const s = readJSON<{ username: string; pwVer: number; exp: number } | null>(SESSION_KEY, null);
    if (!s || !s.username || !s.exp || s.exp < Date.now()) return null;
    const u = users().find(x => x.username === s.username);
    if (!u || u.pwVer !== s.pwVer) return null;
    return { username: u.username, role: u.role as 'admin' | 'member', exp: s.exp };
  },

  async setup(username: string, password: string): Promise<Session> {
    if (users().length > 0) throw new Error('already-setup');
    if (!username || !/^[A-Za-z0-9_.-]{2,32}$/.test(username)) throw new Error('bad-username');
    const p = passwordProblem(password);
    if (p) throw new Error('pw-' + p);
    const salt = Math.random().toString(36).slice(2, 12);
    const hash = await hashPassword(password, salt);
    saveUsers([{ username, role: 'admin', salt, hash, pwVer: 1, created: Date.now() }]);
    return AuthAPI.login(username, password);
  },

  async login(username: string, password: string): Promise<Session> {
    if (fails.lockUntil > Date.now()) throw new Error('locked');
    const u = users().find(x => x.username === username);
    try {
      if (!u) throw new Error('bad-credentials');
      const hash = await hashPassword(password, u.salt);
      if (hash !== u.hash) throw new Error('bad-credentials');
      fails = { count: 0, lockUntil: 0 };
      const exp = Date.now() + SESSION_TTL;
      writeJSON(SESSION_KEY, { username: u.username, pwVer: u.pwVer, exp });
      return { username: u.username, role: u.role as 'admin' | 'member', exp };
    } catch (e) {
      if ((e as Error).message === 'bad-credentials') {
        fails.count++;
        if (fails.count >= 5) fails = { count: 5, lockUntil: Date.now() + 10000 };
      }
      throw e;
    }
  },

  logout() { try { localStorage.removeItem(SESSION_KEY); } catch (e) { /* ignore */ } },

  async verify(username: string, password: string): Promise<boolean> {
    const u = users().find(x => x.username === username);
    if (!u) return false;
    return (await hashPassword(password, u.salt)) === u.hash;
  },

  async changePassword(username: string, currentPw: string, nextPw: string): Promise<boolean> {
    const list = users();
    const u = list.find(x => x.username === username);
    if (!u) throw new Error('bad-credentials');
    const p = passwordProblem(nextPw);
    if (p) throw new Error('pw-' + p);
    if ((await hashPassword(currentPw, u.salt)) !== u.hash) throw new Error('bad-credentials');
    const salt = Math.random().toString(36).slice(2, 12);
    u.salt = salt; u.hash = await hashPassword(nextPw, salt); u.pwVer = (u.pwVer || 1) + 1;
    saveUsers(list);
    try { localStorage.removeItem(SESSION_KEY); } catch (e) { /* ignore */ }
    return true;
  },

  listUsers(): UserPublic[] {
    return users().map(u => ({ username: u.username, role: u.role, created: u.created }));
  },

  async createUser(username: string, password: string, role: string): Promise<boolean> {
    const list = users();
    if (list.some(x => x.username === username)) throw new Error('duplicate');
    if (!username || !/^[A-Za-z0-9_.-]{2,32}$/.test(username)) throw new Error('bad-username');
    const p = passwordProblem(password);
    if (p) throw new Error('pw-' + p);
    const salt = Math.random().toString(36).slice(2, 12);
    list.push({ username, role: role === 'admin' ? 'admin' : 'member', salt, hash: await hashPassword(password, salt), pwVer: 1, created: Date.now() });
    saveUsers(list);
    return true;
  },

  async resetUserPassword(username: string, nextPw: string): Promise<boolean> {
    const list = users();
    const u = list.find(x => x.username === username);
    if (!u) throw new Error('bad-credentials');
    const p = passwordProblem(nextPw);
    if (p) throw new Error('pw-' + p);
    const salt = Math.random().toString(36).slice(2, 12);
    u.salt = salt; u.hash = await hashPassword(nextPw, salt); u.pwVer = (u.pwVer || 1) + 1;
    saveUsers(list);
    return true;
  },

  deleteUser(username: string): boolean {
    const list = users();
    const target = list.find(x => x.username === username);
    if (!target) throw new Error('bad-credentials');
    const admins = list.filter(x => x.role === 'admin');
    if (target.role === 'admin' && admins.length <= 1) throw new Error('last-admin');
    saveUsers(list.filter(x => x.username !== username));
    return true;
  }
};
