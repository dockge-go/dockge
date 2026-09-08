import zhCN from './zh-CN';
import enUS from './en-US';

export const locales = { 'zh-CN': zhCN, 'en-US': enUS } as const;
export type LocaleKey = keyof typeof locales;
export type MsgType = typeof zhCN;

let current: LocaleKey = ('zh-CN' in locales ? 'zh-CN' : 'en-US') as LocaleKey;

export function useLocale(): LocaleKey { return current; }
export function setLocale(locale: LocaleKey) { current = locale; }
export function t<K extends keyof MsgType>(key: K): MsgType[K] {
  return (locales[current] as Record<string, unknown>)[key] as MsgType[K];
}
