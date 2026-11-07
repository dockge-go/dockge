/* ─────────────────────────────────────────────────────────────────────────
 * i18n：zh 为基准语言，t(key, vars) 支持 {var} 插值，语言切换即时生效。
 * 首次进入：localStorage 无偏好时按浏览器语言（navigator.languages）检测，
 * 非中文环境默认英文 —— 登录/初始化页也可手动切换并记忆。
 * ─────────────────────────────────────────────────────────────────── */
import { createSignal } from 'solid-js';
import { zh } from './zh';
import { en } from './en';

export type Lang = 'zh' | 'en';

const LS_KEY = 'dockge.lang';

function detectLang(): Lang {
  try {
    const stored = localStorage.getItem(LS_KEY);
    if (stored === 'zh' || stored === 'en') return stored;
  } catch (e) { /* ignore */ }
  const langs: string[] = (navigator.languages && navigator.languages.length)
    ? Array.from(navigator.languages)
    : [navigator.language || ''];
  for (const l of langs) {
    if (!l) continue;
    if (l.toLowerCase().startsWith('zh')) return 'zh';
  }
  return 'en';
}

const [lang, setLangState] = createSignal<Lang>(detectLang());

export { lang };

export function setLang(l: Lang) {
  setLangState(l);
  try { localStorage.setItem(LS_KEY, l); } catch (e) { /* ignore */ }
  document.documentElement.lang = l === 'en' ? 'en-US' : 'zh-CN';
  document.title = t('app.title');
}

export function t(key: string, vars?: Record<string, string | number>): string {
  const dict = lang() === 'en' ? en : zh;
  let val: string | undefined = dict[key];
  if (val === undefined) val = zh[key];
  if (val === undefined) return key;
  if (vars) {
    for (const k of Object.keys(vars)) {
      val = val.split('{' + k + '}').join(String(vars[k]));
    }
  }
  return val;
}

/* 初始化 html lang / 标题（不落盘：保持“检测优先”直到用户显式选择） */
document.documentElement.lang = lang() === 'en' ? 'en-US' : 'zh-CN';
document.title = t('app.title');
