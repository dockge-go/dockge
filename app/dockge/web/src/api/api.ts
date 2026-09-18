// API 客户端：统一响应包（{code,message,data}）解析、JWT 注入与 401 处理。
// 接口契约与后端 app/dockge/api/v1 的 DTO 一一对应。

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
  role?: string; // admin / member
}

interface LoginData {
  accessToken: string;
  user: UserData;
}

export interface StackSummary {
  name: string;
  status: number; // 0 未知 / 1 未部署 / 2 已创建 / 3 运行中 / 4 已停止
  statusLabel: string;
  managed: boolean;
  composeFileName?: string;
  configFiles?: string;
}

interface StackContainer {
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

/** 镜像列表行（REST 与 SSE 帧同形状，对齐后端 v1.DockerImageData）。 */
export interface ImageRow {
  id: string;
  repo: string;
  tag: string;
  sizeBytes: number;
  createdAt: number;
}

interface VolumeRow {
  name: string;
  driver: string;
}

interface DockerInfo {
  version: string;
  os: string;
  arch: string;
  stacksTotal: number;
  stacksRunning: number;
  containersTotal: number;
  containersRunning: number;
  imagesTotal?: number;
}

export interface VersionSummary {
  version: string;
  apiVersion: string;
  os: string;
  arch: string;
}

export interface DockerStats {
  cpuUsage: number;
  memUsage: number;
  memTotalMB: number;
  memPercent: number;
  /** 非空 = 错误帧：stats_unavailable（数据源不可用，如非 Linux 平台） */
  error?: string;
}

interface DfCategory {
  type: string;
  count: number;
  active: number;
  size: string;
  reclaimable: string;
}

interface VersionCheck {
  latestVersion: string;
  currentVersion: string;
  hasUpdate: boolean;
}

export interface AuthConfig {
  mode: "jwt" | "proxy" | "oidc" | "disable";
  providers: Array<{ id: string; info: { label: string } }>;
  disableAuth: boolean;
}

export interface UserRow {
  id: number;
  username: string;
  nickname: string;
  role: string; // admin / member
  active: boolean;
  source: string; // local / proxy / oidc
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

/** 状态帧携带的实时资源计数（后端随 docker events 同帧推送；采集失败帧会省略）。 */
export interface ResourceCounts {
  containersTotal: number;
  containersRunning: number;
  stacksTotal: number;
  stacksRunning: number;
  imagesTotal: number;
}

export interface ContainerStatusFrame {
  containers: Array<Pick<ContainerRow, "id" | "state" | "status">>;
  counts?: ResourceCounts;
  images?: ImageRow[];
}

export type StackOp = "start" | "stop" | "restart" | "down" | "update";

/** /stacks/validate 的单条诊断（对齐 v1.StackValidateError）；line=0 表示无法定位行。 */
export interface ValidateError {
  line: number;
  message: string;
}

/** /stacks/validate 响应（对齐 v1.StackValidateResponse）。 */
export interface ValidateResult {
  valid: boolean;
  errors: ValidateError[];
}

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
  setup: (username: string, password: string) => request<LoginData>("POST", "/setup", { username, password }),
  needSetup: () => request<{ needSetup: boolean }>("GET", "/setup/need"),
  authConfig: () => request<AuthConfig>("GET", "/auth/config"),
  me: () => request<UserData>("GET", "/me"),
  changePassword: (oldPassword: string, newPassword: string) =>
    request<void>("PUT", "/me/password", { oldPassword, newPassword }),
  getDisableAuth: () => request<{ enabled: boolean }>("GET", "/me/disableauth"),
  toggleDisableAuth: (enable: boolean, currentPassword: string) =>
    request<{ enabled: boolean }>("POST", "/me/disableauth", { enable, currentPassword }),

  // 用户管理（admin）
  users: () => request<{ list: UserRow[] }>("GET", "/users"),
  createUser: (username: string, password: string, role: string) =>
    request<UserRow>("POST", "/users", { username, password, role }),
  setUserRole: (id: number, role: string) =>
    request<void>("PUT", `/users/${id}/role`, { role }),
  setUserActive: (id: number, active: boolean) =>
    request<void>("PUT", `/users/${id}/active`, { active }),
  removeUser: (id: number) => request<void>("DELETE", `/users/${id}`),

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
  validateStack: (yaml: string, env: string) =>
    request<ValidateResult>("POST", "/stacks/validate", { yaml, env }),

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
