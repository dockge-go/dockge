// API 客户端：统一响应包（{code,message,data}）解析、JWT 注入与 401 处理。
// 接口契约与后端 app/dockge/api/v1 的 DTO 一一对应。

export const TOKEN_KEY = "crate_token";

export const getToken = () => localStorage.getItem(TOKEN_KEY) ?? "";
export const setToken = (t: string) => localStorage.setItem(TOKEN_KEY, t);
export const clearToken = () => localStorage.removeItem(TOKEN_KEY);

export class ApiError extends Error {
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
  twoFA?: boolean;
}

export interface LoginData {
  accessToken: string;
  user: UserData;
  tokenRequired?: boolean;
}

export interface StackSummary {
  name: string;
  status: number; // 0 未知 / 1 未部署 / 2 已创建 / 3 运行中 / 4 已停止
  statusLabel: string;
  managed: boolean;
  composeFileName?: string;
  configFiles?: string;
}

export interface StackContainer {
  id: string;
  name: string;
  service?: string;
  state: string;
  status: string;
}

export interface StackDetail extends StackSummary {
  yaml: string;
  env: string;
  containers: StackContainer[];
  urls?: string[];
}

export interface ContainerRow {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  ports: string;
  stack?: string;
}

export interface ContainerInspectData {
  Id: string;
  Created: string;
  Config?: { Image?: string; Env?: string[] };
  Mounts?: Array<{ Type?: string; Name?: string; Source?: string; Destination?: string; Mode?: string }>;
  NetworkSettings?: {
    IPAddress?: string;
    Networks?: Record<string, { IPAddress?: string }>;
  };
}

export interface NetworkInspectData {
  Name?: string;
  Driver?: string;
  Created?: string;
  IPAM?: { Config?: Array<{ Subnet?: string; Gateway?: string }> };
}

export interface ImageRow {
  id: string;
  repo: string;
  tag: string;
  size: string;
  created: string;
}

export interface VolumeRow {
  name: string;
  driver: string;
}

export interface DockerInfo {
  version: string;
  os: string;
  arch: string;
  stacksTotal: number;
  stacksRunning: number;
  containersTotal: number;
  containersRunning: number;
}

export interface VersionSummary {
  version: string;
  apiVersion: string;
  os: string;
  arch: string;
}

export interface ContainerStat {
  id: string;
  name: string;
  cpu: number;
  memPercent: number;
  memUsage: string;
}

export interface DockerStats {
  cpuUsage: number;
  memUsage: number;
  memTotalMB: number;
  memPercent: number;
  containers?: ContainerStat[];
}

export interface DfCategory {
  type: string;
  count: number;
  active: number;
  size: string;
  reclaimable: string;
}

export interface VersionCheck {
  latestVersion: string;
  currentVersion: string;
  hasUpdate: boolean;
}

/** REST 聚合后的前端资源快照。 */
export interface Snapshot {
  stacks: StackSummary[];
  docker: DockerInfo;
  containers: ContainerRow[];
  dockerOk: boolean;
  images: ImageRow[];
  networks: string[];
  volumes: VolumeRow[];
}

export interface ContainerStatusFrame {
  containers: Array<Pick<ContainerRow, "id" | "state" | "status">>;
}

export type StackOp = "start" | "stop" | "restart" | "down" | "update";

// ---- 请求核心 ----

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
    if (getToken()) {
      clearToken();
      unauthorizedHandler();
    }
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
  setup: (username: string, password: string) => request<LoginData>("POST", "/setup", { username, password }),
  needSetup: () => request<{ needSetup: boolean }>("GET", "/setup/need"),
  me: () => request<UserData>("GET", "/me"),
  changePassword: (oldPassword: string, newPassword: string) =>
    request<void>("PUT", "/me/password", { oldPassword, newPassword }),

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

  // docker 资源
  version: () => request<VersionSummary>("GET", "/docker/version"),
  info: () => request<DockerInfo>("GET", "/docker/info"),
  containers: () => request<{ list: ContainerRow[] }>("GET", "/docker/containers"),
  containerInspect: (id: string) =>
    request<ContainerInspectData>("GET", `/docker/containers/${encodeURIComponent(id)}/inspect`),
  containerAction: (id: string, action: "start" | "stop" | "restart") =>
    request<void>("POST", `/docker/containers/${encodeURIComponent(id)}/${action}`),
  removeContainer: (id: string) => request<void>("DELETE", `/docker/containers/${encodeURIComponent(id)}`),
  pruneContainers: () => request<void>("POST", "/docker/containers/prune"),

  images: () => request<{ list: ImageRow[] }>("GET", "/docker/images"),
  pullImage: (reference: string) => request<void>("POST", "/docker/images/pull", { reference }),
  removeImage: (id: string) => request<void>("DELETE", `/docker/images/${encodeURIComponent(id)}`),
  pruneImages: () => request<void>("POST", "/docker/images/prune"),

  networks: () => request<{ list: string[] }>("GET", "/docker/networks"),
  networkInspect: (name: string) =>
    request<NetworkInspectData>("GET", `/docker/networks/${encodeURIComponent(name)}`),
  createNetwork: (name: string, driver: string, subnet: string) =>
    request<void>("POST", "/docker/networks/create", { name, driver, subnet }),
  removeNetwork: (name: string) => request<void>("DELETE", `/docker/networks/${encodeURIComponent(name)}`),
  pruneNetworks: () => request<void>("POST", "/docker/networks/prune"),

  volumes: () => request<{ list: VolumeRow[] }>("GET", "/docker/volumes"),
  removeVolume: (name: string) => request<void>("DELETE", `/docker/volumes/${encodeURIComponent(name)}`),
  pruneVolumes: () => request<void>("POST", "/docker/volumes/prune"),

  df: () => request<{ list: DfCategory[] }>("GET", "/docker/df"),
  versionCheck: () => request<VersionCheck>("GET", "/version/check"),
  composerize: (dockerRunCommand: string) =>
    request<{ composeTemplate: string }>("POST", "/composerize", { dockerRunCommand }),

  // 设置
  globalEnv: () => request<{ globalENV: string }>("GET", "/settings/globalenv"),
  setGlobalEnv: (content: string) => request<void>("PUT", "/settings/globalenv", { content }),
};
