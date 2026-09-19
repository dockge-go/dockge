// i18n 核心：响应式语言切换（SolidJS signal），持久化 localStorage(dockge.lang)，默认 zh-CN。
// t() 读取 locale 信号 —— 在 JSX 表达式中调用时语言切换自动重渲染；
// 在事件回调（toast/confirm）中调用则取调用时刻的语言。
// 词典为扁平 key（继承设计定稿 crateman_web_index 的 key 约定），zh-CN 为基准语言。
import { createSignal } from "solid-js";
import zhCN from "./zh-CN";
import enUS from "./en-US";

export type LocaleKey = "zh-CN" | "en-US";
export type MsgKey = keyof typeof zhCN;

const LANG_KEY = "dockge.lang";

const dicts: Record<LocaleKey, Record<MsgKey, string>> = {
  "zh-CN": zhCN,
  "en-US": enUS,
};

// 兼容设计定稿时代存的 'zh' / 'en' 短码
function normalize(v: string | null): LocaleKey | null {
  if (v === "zh-CN" || v === "zh") return "zh-CN";
  if (v === "en-US" || v === "en") return "en-US";
  return null;
}

function initialLocale(): LocaleKey {
  try {
    const stored = normalize(localStorage.getItem(LANG_KEY));
    if (stored) return stored;
  } catch {
    // localStorage 不可用时用默认语言
  }
  return "zh-CN";
}

const [locale, setLocaleSignal] = createSignal<LocaleKey>(initialLocale());

function applyDocumentLocale() {
  document.documentElement.lang = locale();
  document.title = t("app.title");
}

export function useLocale(): LocaleKey {
  return locale();
}

export function setLocale(next: LocaleKey) {
  setLocaleSignal(next);
  try {
    localStorage.setItem(LANG_KEY, next);
  } catch {
    // 持久化失败不影响本次会话
  }
  applyDocumentLocale();
}

export function t(key: MsgKey, vars?: Record<string, string | number>): string {
  let val = dicts[locale()][key] ?? zhCN[key];
  if (vars) {
    for (const [k, v] of Object.entries(vars)) {
      val = val.replaceAll(`{${k}}`, String(v));
    }
  }
  return val;
}

// 模块加载即同步一次 <html lang> 与页面标题
applyDocumentLocale();
