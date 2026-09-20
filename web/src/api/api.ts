// API 客户端：统一响应包（{code,message,data}）解析、JWT 注入与 401 处理。
// 接口契约与后端 api/v1 的 DTO 一一对应。
// 一比一复刻上游页面集：仅保留栈/设置/认证/composerize 相关接口。

const TOKEN_KEY = "crate_token";

// remember=false 时 token 走 sessionStorage（关闭标签页即失效），对齐上游 Remember me
export const getToken = () => localStorage.getItem(TOKEN_KEY) ?? sessionStorage.getItem(TOKEN_KEY) ?? "";
export const setToken = (t: string, remember = false) => {
  (remember ? localStorage : sessionStorage).setItem(TOKEN_KEY, t);
};
export const clearToken = () => {
  localStorage.removeItem(TOKEN_KEY);
  sessionStorage.removeItem(TOKEN_KEY);
};

class ApiError extends Error {
  constructor(
    public code: number,
    message: string,
  ) {
    super(message);
  }
}

let unauthorizedHandler: () => void = () => {};
export const setUnauthorizedHandler = (fn: () => void) => {
  unauthorizedHandler = fn;
};

// ---- DTO（对齐 v1 包） ----

export interface UserData {
  id: number;
  username: string;
  nickname: string;
  role?: string; // 单用户软件，恒为 admin（字段为兼容存量数据保留）
}

interface LoginData {
  accessToken: string;
  user: UserData;
}

export interface StackSummary {
  name: string;
  status: number; // 0 未知 / 1 未部署 / 2 已创建 / 3 运行中 / 4 已停止
  managed: boolean;
  composeFileName?: string;
  configFiles?: string;
}

interface StackContainer {
  id: string;
  name: string;
  service?: string;
  image?: string;
  state: string;
  status: string;
  ports?: Array<{ hostIP?: string; hostPort?: number; containerPort: number; protocol?: string }>;
}

export interface StackDetail extends StackSummary {
  yaml: string;
  env: string;
  containers: StackContainer[];
  urls?: string[];
}

/** 单容器即时资源占用（对齐 v1.ContainerStat，字段同 docker stats --format json）。 */
export interface ContainerStat {
  name: string;
  cpuPerc: string;
  memUsage: string;
  memPerc: string;
  netIO: string;
  blockIO: string;
}

export type StackOp = "start" | "stop" | "restart" | "down" | "update";

/** 镜像列表行（对齐 v1.ImageData）。 */
export interface ImageItem {
  id: string;
  repository: string;
  tag: string;
  size: string;
  createdSince: string;
  inUse: boolean;
}

/** 容器运行时自检结果（/v1/health data.runtime，免鉴权）。 */
export interface HealthRuntimeInfo {
  cli: string;
  compose: string;
  ready: boolean;
  error?: string;
  stacks: { path: string; mounted: boolean };
}

interface HealthData {
  status: string;
  version: string;
  runtime: HealthRuntimeInfo;
}

// ---- 请求核心 ----

/** 流式 POST：响应体逐块回调（进度终端）；非 2xx 按统一错误包解析。 */
async function postStream(path: string, body: unknown, onChunk: (text: string) => void): Promise<void> {
  const headers: Record<string, string> = {};
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const res = await fetch(`/v1${path}`, {
    method: "POST",
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!res.ok || !res.body) {
    const payload = (await res.json().catch(() => null)) as { message?: string } | null;
    throw new ApiError(res.status, payload?.message || `请求失败（HTTP ${res.status}）`);
  }
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    onChunk(decoder.decode(value, { stream: true }));
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const res = await fetch(`/v1${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (res.status === 401) {
    const payload = (await res.json().catch(() => null)) as { message?: string } | null;
    if (getToken()) clearToken();
    unauthorizedHandler();
    throw new ApiError(401, payload?.message || "登录失效，请重新登录");
  }
  const payload = (await res.json().catch(() => null)) as
    | { code?: number; message?: string; data?: T }
    | null;
  if (!payload) throw new ApiError(res.status, `请求失败（HTTP ${res.status}）`);
  if (payload.code !== undefined && payload.code !== 0) {
    throw new ApiError(payload.code, payload.message || "请求失败");
  }
  return payload.data as T;
}

// ---- API ----

export const api = {
  // 认证
  login: (username: string, password: string) => request<LoginData>("POST", "/login", { username, password }),
  /** 免登录模式（disableAuth）下的自动登录；未开启时后端返回 401。 */
  autoLogin: () => request<LoginData>("POST", "/auto-login"),
  setup: (username: string, password: string) => request<LoginData>("POST", "/setup", { username, password }),
  needSetup: () => request<{ needSetup: boolean }>("GET", "/setup/need"),
  me: () => request<UserData>("GET", "/me"),
  changePassword: (oldPassword: string, newPassword: string) =>
    request<void>("PUT", "/me/password", { oldPassword, newPassword }),
  getDisableAuth: () => request<{ enabled: boolean }>("GET", "/me/disableauth"),
  toggleDisableAuth: (enable: boolean, currentPassword: string) =>
    request<{ enabled: boolean }>("POST", "/me/disableauth", { enable, currentPassword }),

  // 栈
  stacks: () => request<{ list: StackSummary[] }>("GET", "/stacks"),
  stack: (name: string) => request<StackDetail>("GET", `/stacks/${encodeURIComponent(name)}`),
  createStack: (name: string, yaml: string, env: string) =>
    request<void>("POST", "/stacks", { name, yaml, env }),
  saveStack: (name: string, yaml: string, env: string) =>
    request<void>("PUT", `/stacks/${encodeURIComponent(name)}`, { name, yaml, env }),
  deleteStack: (name: string) => request<void>("DELETE", `/stacks/${encodeURIComponent(name)}`),
  stackOp: (name: string, op: StackOp) =>
    request<{ output: string }>("POST", `/stacks/${encodeURIComponent(name)}/${op}`),
  /** 流式栈操作：compose 输出逐块实时回调（进度终端）。 */
  stackOpStream: (name: string, op: StackOp, onChunk: (text: string) => void): Promise<void> =>
    postStream(`/stacks/${encodeURIComponent(name)}/${op}`, undefined, onChunk),
  stackNetworks: () => request<string[]>("GET", "/stacks/networks"),

  // 镜像
  images: () => request<{ list: ImageItem[] }>("GET", "/images"),
  /** 流式拉取镜像：docker pull 输出逐块实时回调（进度终端）。 */
  pullImageStream: (ref: string, onChunk: (text: string) => void): Promise<void> =>
    postStream("/images/pull", { image: ref }, onChunk),
  deleteImages: (refs: string[]) =>
    request<{ output: string }>("POST", "/images/delete", { images: refs }),
  stackServiceOp: (name: string, service: string, op: "start" | "stop" | "restart") =>
    request<{ output: string }>("POST", `/stacks/${encodeURIComponent(name)}/services/${encodeURIComponent(service)}/${op}`),
  stackStats: (name: string) =>
    request<ContainerStat[]>("GET", `/stacks/${encodeURIComponent(name)}/stats`),
  /** 部署自检（免鉴权）：Setup 页与设置页共用。 */
  health: () => request<HealthData>("GET", "/health"),
  composerize: (dockerRunCommand: string) =>
    request<{ composeTemplate: string }>("POST", "/composerize", { dockerRunCommand }),

  // 设置
  globalEnv: () => request<{ globalENV: string }>("GET", "/settings/globalenv"),
  setGlobalEnv: (content: string) => request<void>("PUT", "/settings/globalenv", { content }),
  primaryHostname: () => request<{ hostname: string }>("GET", "/settings/primaryhostname"),
  setPrimaryHostname: (hostname: string) =>
    request<void>("PUT", "/settings/primaryhostname", { hostname }),
};
