// 全局状态：认证、REST 资源清单、容器状态 SSE、toast 与搜索联动。
import { createSignal } from "solid-js";
import {
  api,
  getToken,
  clearToken,
  setToken,
  setUnauthorizedHandler,
  type ContainerStatusFrame,
  type Snapshot,
  type UserData,
} from "../api/api";
import { errText } from "../api/format";
import { mergeContainerStatus } from "../lib/container-status";

// ---- toast ----

export interface ToastItem {
  id: number;
  message: string;
  type: "success" | "error" | "info";
}

const [toasts, setToasts] = createSignal<ToastItem[]>([]);
let toastSeq = 0;

export function toast(message: string, type: ToastItem["type"] = "info") {
  const id = ++toastSeq;
  setToasts((ts) => [...ts, { id, message, type }]);
  setTimeout(() => setToasts((ts) => ts.filter((t) => t.id !== id)), 3500);
}

export { toasts };

// ---- 认证 ----

const [user, setUser] = createSignal<UserData | null>(null);
const [authed, setAuthed] = createSignal(!!getToken());

export { user, authed };

setUnauthorizedHandler(() => {
  setAuthed(false);
  setUser(null);
});

export async function login(username: string, password: string) {
  const data = await api.login(username, password);
  if (data.tokenRequired) throw new Error("该账号已启用两步验证，暂不支持在此登录");
  setToken(data.accessToken);
  setUser(data.user);
  setAuthed(true);
  await refresh(false);
  startContainerStatusStream();
  return data.user;
}

export async function setup(username: string, password: string) {
  const data = await api.setup(username, password);
  setToken(data.accessToken);
  setUser(data.user);
  setAuthed(true);
  await refresh(false);
  startContainerStatusStream();
}

export function logout() {
  clearToken();
  setUser(null);
  setAuthed(false);
  stopContainerStatusStream();
}

/** 启动时校验本地 token，成功则预热用户与实时流。 */
export async function boot(): Promise<boolean> {
  if (!getToken()) return false;
  try {
    setUser(await api.me());
    setAuthed(true);
    await refresh(false);
    startContainerStatusStream();
    return true;
  } catch {
    clearToken();
    setAuthed(false);
    return false;
  }
}

// ---- REST 资源清单 + 容器状态流 ----

const [snapshot, setSnapshot] = createSignal<Snapshot | null>(null);
const [sseOn, setSseOn] = createSignal(false);

export { snapshot, sseOn };

const STATE_TOAST: Record<string, { label: string; type: ToastItem["type"] }> = {
  running: { label: "已进入运行状态", type: "success" },
  exited: { label: "已停止", type: "info" },
  paused: { label: "已暂停", type: "info" },
};

function applyContainerStatus(frame: ContainerStatusFrame) {
  const prev = snapshot();
  if (!prev) return;
  const changed = new Map(frame.containers.map((container) => [container.id, container.state]));
  for (const container of prev.containers) {
    const state = changed.get(container.id);
    if (state && state !== container.state) {
      const message = STATE_TOAST[state];
      if (message) toast(`${container.name} ${message.label}`, message.type);
    }
  }
  const containers = mergeContainerStatus(prev.containers, frame);
  setSnapshot({ ...prev, containers, docker: {
    ...prev.docker,
    containersTotal: containers.length,
    containersRunning: containers.filter((container) => container.state === "running").length,
  } });
}

let es: EventSource | null = null;

export function startContainerStatusStream() {
  if (es) return;
  es = new EventSource(`/v1/docker/containers/stream?token=${encodeURIComponent(getToken())}`);
  es.onopen = () => setSseOn(true);
  es.onmessage = (ev) => {
    if (!ev.data) return; // 心跳
    try {
      applyContainerStatus(JSON.parse(ev.data) as ContainerStatusFrame);
      setSseOn(true);
    } catch {
      // 忽略无法解析的帧
    }
  };
  es.onerror = () => {
    setSseOn(false);
    if (!getToken()) {
      // token 已清除：停止自动重连
      es?.close();
      es = null;
    }
  };
}

export function stopContainerStatusStream() {
  es?.close();
  es = null;
  setSseOn(false);
}

/** 手动刷新：并发拉取全部资源并合成一帧快照（toast 控制是否提示）。 */
export async function refresh(withToast = true) {
  try {
    const [info, stacks, containers, images, networks, volumes] = await Promise.all([
      api.info(),
      api.stacks(),
      api.containers(),
      api.images(),
      api.networks(),
      api.volumes(),
    ]);
    setSnapshot({
      stacks: stacks.list,
      docker: info,
      containers: containers.list,
      dockerOk: true,
      images: images.list,
      networks: networks.list,
      volumes: volumes.list,
    });
    if (withToast) toast("数据已刷新", "info");
  } catch (e) {
    if (withToast) toast(errText(e), "error");
  }
}

// ---- 搜索/跨页联动（全局搜索点击资源后由目标页消费并打开详情） ----

const [pendingContainer, setPendingContainer] = createSignal<string | null>(null);
const [pendingStack, setPendingStack] = createSignal<string | null>(null);
const [pendingNewStack, setPendingNewStack] = createSignal(false);

export { pendingContainer, setPendingContainer, pendingStack, setPendingStack, pendingNewStack, setPendingNewStack };
