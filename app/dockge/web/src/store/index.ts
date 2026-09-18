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
import { t } from "../i18n";

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
const [authed, setAuthed] = createSignal(false);

export { user, authed };

setUnauthorizedHandler(() => {
  setAuthed(false);
  setUser(null);
});

/** 建立/复位会话的公共收尾：token 入库、先拉全量快照（保证 SSE 初帧到达时
 * snapshot 已就绪、不被丢弃），再预热用户与实时流。 */
async function completeLogin(data: { accessToken: string; user: UserData }) {
  setToken(data.accessToken);
  setUser(data.user);
  setAuthed(true);
  await refresh(false);
  startContainerStatusStream();
}

/** 密码登录：建立会话并预热用户与实时流。 */
export async function login(username: string, password: string) {
  const data = await api.login(username, password);
  completeLogin(data);
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
  stopContainerStatusStream();
}

/** 启动时校验会话（localStorage token 或 httpOnly cookie），成功则预热用户与实时流。 */
export async function boot(): Promise<boolean> {
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

const STATE_TOAST: Record<string, { labelKey: "toast.sseRunning" | "toast.sseExited" | "toast.ssePaused"; type: ToastItem["type"] }> = {
  running: { labelKey: "toast.sseRunning", type: "success" },
  exited: { labelKey: "toast.sseExited", type: "info" },
  paused: { labelKey: "toast.ssePaused", type: "info" },
};

/** 精准增量：仅刷新容器列表与仪表盘计数（新容器由 SSE 帧发现时调用，
 * 不触碰栈/镜像/网络/卷等其他资源，避免全局重拉）。 */
async function refreshContainers() {
  const prev = snapshot();
  if (!prev) return;
  try {
    const [containers, info] = await Promise.all([api.containers(), api.info()]);
    setSnapshot({ ...prev, containers: containers.list, docker: info });
  } catch {
    // 静默失败：下一帧/下次全量刷新会自愈
  }
}

// SSE 状态帧只携带 id/state/status：列表里没有的新容器无法由帧补全字段，
// 检测到未知 id 时节流触发精准容器刷新（5s 节流：事件驱动帧本身已防抖，
// 无风暴风险；新容器应在秒级入列而非等用户手动刷新）。
let lastUnknownContainerRefresh = 0;

function applyContainerStatus(frame: ContainerStatusFrame) {
  const prev = snapshot();
  if (!prev) return;
  const known = new Set(prev.containers.map((container) => container.id));
  const hasUnknown = frame.containers.some((container) => !known.has(container.id));
  if (hasUnknown && Date.now() - lastUnknownContainerRefresh > 5_000) {
    lastUnknownContainerRefresh = Date.now();
    void refreshContainers();
  }
  const changed = new Map(frame.containers.map((container) => [container.id, container.state]));
  for (const container of prev.containers) {
    const state = changed.get(container.id);
    if (state && state !== container.state) {
      const message = STATE_TOAST[state];
      if (message) toast(`${container.name} ${t(message.labelKey)}`, message.type);
    }
  }
  const containers = mergeContainerStatus(prev.containers, frame);
  const images = frame.images;
  const c = frame.counts;
  setSnapshot({
    ...prev,
    containers,
    // 镜像列表与计数同帧同源：帧带全量镜像时整体替换，徽标与列表一起删/一起留。
    ...(images ? { images } : {}),
    docker: {
      ...prev.docker,
      containersTotal: c?.containersTotal ?? containers.length,
      containersRunning: c?.containersRunning ?? containers.filter((container) => container.state === "running").length,
      ...(c ? { stacksTotal: c.stacksTotal, stacksRunning: c.stacksRunning } : {}),
      imagesTotal: c?.imagesTotal ?? (images ? images.length : prev.docker.imagesTotal),
    },
  });
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
    if (withToast) toast(t("toast.refreshed"), "info");
  } catch (e) {
    if (withToast) toast(errText(e), "error");
  }
}

// ---- 搜索/跨页联动（全局搜索点击资源后由目标页消费并打开详情） ----

const [pendingContainer, setPendingContainer] = createSignal<string | null>(null);
const [pendingStack, setPendingStack] = createSignal<string | null>(null);
const [pendingNewStack, setPendingNewStack] = createSignal(false);

export { pendingContainer, setPendingContainer, pendingStack, setPendingStack, pendingNewStack, setPendingNewStack };
