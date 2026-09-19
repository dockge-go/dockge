// 明暗主题：偏好三态（light/dark/auto，对齐上游 Appearance 设置）。
// data-theme 写在 <html> 上，首帧前由 index.html 内联脚本预置（防 FOUC，
// 该脚本对非 light/dark 的存储值——含 auto——回退 prefers-color-scheme）。
// 偏好持久化 localStorage(dockge.theme)；auto 时实时跟随系统。
import { createSignal } from "solid-js";

type ThemePref = "auto" | "light" | "dark";
export type Theme = "light" | "dark";

const KEY = "dockge.theme";

function storedPref(): ThemePref {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "light" || v === "dark" || v === "auto") return v;
  } catch {
    // localStorage 不可用时按默认
  }
  return "auto";
}

const [pref, setPrefSignal] = createSignal<ThemePref>(storedPref());

function apply(next: Theme) {
  document.documentElement.dataset.theme = next;
  // 对齐上游：theme-color meta 跟随主题（移动端浏览器地址栏着色）
  document.querySelector('meta[name="theme-color"]')?.setAttribute("content", next === "dark" ? "#0d1117" : "#5cdd8b");
}

export function useThemePref(): ThemePref {
  return pref();
}

/** 设置主题偏好并持久化；auto 立即按系统解析。 */
export function setThemePref(next: ThemePref) {
  setPrefSignal(next);
  try {
    localStorage.setItem(KEY, next);
  } catch {
    // 持久化失败不影响本次会话
  }
  if (next === "auto") {
    apply(window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
  } else {
    apply(next);
  }
}

// auto 偏好下系统主题变化实时跟随；手动偏好不受影响
if (window.matchMedia) {
  const media = window.matchMedia("(prefers-color-scheme: dark)");
  media.addEventListener("change", () => {
    if (storedPref() === "auto") {
      apply(media.matches ? "dark" : "light");
    }
  });
}
