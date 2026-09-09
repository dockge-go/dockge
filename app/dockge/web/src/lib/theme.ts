// 明暗主题：data-theme 写在 <html> 上，首帧前由 index.html 内联脚本预置（防 FOUC）。
// 手动选择持久化到 localStorage(dockge.theme)；未手动选择时实时跟随系统 prefers-color-scheme。
// 终端 / YAML 编辑器 / 日志等控制台区域不随主题切换（保持深色），由 CSS 变量保证。
import { createSignal } from "solid-js";

export type Theme = "light" | "dark";

const KEY = "dockge.theme";

function currentDomTheme(): Theme {
  return document.documentElement.dataset.theme === "dark" ? "dark" : "light";
}

const [theme, setThemeSignal] = createSignal<Theme>(currentDomTheme());

function apply(next: Theme) {
  document.documentElement.dataset.theme = next;
  setThemeSignal(next);
}

export function useTheme(): Theme {
  return theme();
}

export function toggleTheme() {
  const next: Theme = currentDomTheme() === "dark" ? "light" : "dark";
  try {
    localStorage.setItem(KEY, next);
  } catch {
    // 持久化失败不影响本次会话
  }
  apply(next);
}

// 首次访问（未手动选择）时，系统主题变化实时跟随
if (window.matchMedia) {
  const media = window.matchMedia("(prefers-color-scheme: dark)");
  media.addEventListener("change", () => {
    let manual: string | null = null;
    try {
      manual = localStorage.getItem(KEY);
    } catch {
      // 忽略，视为未手动选择
    }
    if (manual !== "dark" && manual !== "light") {
      apply(media.matches ? "dark" : "light");
    }
  });
}
