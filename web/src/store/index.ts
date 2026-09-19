// 全局状态：认证、栈清单与 toast。栈状态经 refresh() 拉取
// （上游为 socket 推送，本实现为请求时刷新 + 操作后刷新）。
import { createSignal } from "solid-js";
import {
  api,
  clearToken,
  setToken,
  setUnauthorizedHandler,
  type StackSummary,
  type UserData,
} from "../api/api";
import { errText } from "../api/format";
import { t } from "../i18n";

// ---- toast ----

interface ToastItem {
  id: number;
  message: string;
  type: "success" | "error" | "info";
}

const [toasts, setToasts] = createSignal<ToastItem[]>([]);
let toastSeq = 0;

// 上游对齐：错误 toast 持久显示（点击关闭），成功/信息 20s 自动消失
export function toast(message: string, type: ToastItem["type"] = "info") {
  const id = ++toastSeq;
  setToasts((ts) => [...ts, { id, message, type }]);
  if (type !== "error") {
    setTimeout(() => dismissToast(id), 20_000);
  }
}

/** 点击关闭（持久错误 toast 的唯一退出途径）。 */
export function dismissToast(id: number) {
  setToasts((ts) => ts.filter((item) => item.id !== id));
}

export { toasts };

// ---- 认证 ----

const [user, setUser] = createSignal<UserData | null>(null);
const [authed, setAuthed] = createSignal(false);

export { user, authed };

setUnauthorizedHandler(() => {
  setAuthed(false);
  setUser(null);
});

/** 建立/复位会话的公共收尾：token 入库并拉取栈清单。 */
async function completeLogin(data: { accessToken: string; user: UserData }, remember = false) {
  setToken(data.accessToken, remember);
  setUser(data.user);
  setAuthed(true);
  await refresh(false);
}

/** 密码登录：建立会话。 */
export async function login(username: string, password: string, remember = false) {
  const data = await api.login(username, password);
  completeLogin(data, remember);
  return data.user;
}

export async function setup(username: string, password: string) {
  const data = await api.setup(username, password);
  completeLogin(data);
}

export function logout() {
  clearToken();
  setUser(null);
  setAuthed(false);
}

/** 启动时校验会话（localStorage/sessionStorage token），成功则预热栈清单；
 * 会话失效时尝试免登录模式（上游 autoLogin 等价物），仍失败才回到登录页。 */
export async function boot(): Promise<boolean> {
  try {
    setUser(await api.me());
    setAuthed(true);
    await refresh(false);
    return true;
  } catch {
    clearToken();
    setAuthed(false);
  }
  try {
    const data = await api.autoLogin();
    await completeLogin(data, true);
    return true;
  } catch {
    return false;
  }
}

// ---- 栈清单 ----

const [snapshot, setSnapshot] = createSignal<{ stacks: StackSummary[] } | null>(null);

export { snapshot };

/** 手动刷新：拉取栈清单（toast 控制是否提示）。 */
export async function refresh(withToast = true) {
  try {
    const stacks = await api.stacks();
    setSnapshot({ stacks: stacks.list });
    if (withToast) toast(t("toast.refreshed"), "info");
  } catch (e) {
    if (withToast) toast(errText(e), "error");
  }
}

// ---- 跨页交接：首页 docker-run 转换结果落入 /compose 新建编辑器 ----

const [draftYaml, setDraftYaml] = createSignal<string | null>(null);

export { draftYaml, setDraftYaml };
