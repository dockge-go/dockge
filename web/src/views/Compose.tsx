// 栈详情/新建页（上游 Compose.vue 复刻）：标题行（状态 pill + 按钮组双态）+
// URL 徽章 + 进度终端 + 双栏（左：容器卡/合并日志；右：compose/.env 编辑器）。
// 校验与上游一致：客户端 YAML 解析，错误以纯文本呈现在编辑器下方（首次 3s 防抖）。
import { For, Show, createEffect, createSignal, on, onCleanup, onMount, untrack } from "solid-js";
import { A, useBeforeLeave, useNavigate, useParams } from "@solidjs/router";
import {
  ChevronDown,
  ChevronUp,
  CloudDownload,
  Pencil,
  Play,
  Rocket,
  RotateCw,
  Save,
  Square,
  Terminal,
  Trash2,
  WandSparkles,
  X,
} from "lucide-solid";

import { parseDocument } from "yaml";

import { api, type ContainerStat, type StackDetail, type StackOp } from "../api/api";
import { errText } from "../api/format";
import { ArrayField, ArraySelectField } from "../components/ArrayField";
import { confirmDialog } from "../components/Confirm";
import { StackEditor } from "../components/StackEditor";
import { DisplayTerminal, TerminalPane } from "../components/Terminal";
import { ImageCombo } from "../components/ImageCombo";
import { t } from "../i18n";
import { displayPort, portUrl } from "../lib/ports";
import {
  addService,
  ensureTopLevelNetworks,
  ensureTopLevelVolumes,
  formatYaml,
  hasLongSyntax,
  listServices,
  listTopLevelNetworkEntries,
  listTopLevelNetworks,
  readService,
  removeService,
  removeTopLevelNetwork,
  renameTopLevelNetwork,
  setTopLevelNetworkEntries,
  undefinedDependsOn,
  undefinedNetworks,
  undefinedVolumes,
  updateService,
  type ServiceFields,
} from "../lib/yaml-edit";
import { draftYaml, refresh, setDraftYaml, toast } from "../store/index";

const STARTER_YAML = `services:
  nginx:
    image: nginx:latest
    restart: unless-stopped
    ports:
      - "8080:80"
`;
const STARTER_ENV = "# VARIABLE=value #comment";

function statusClass(status: number): string {
  if (status === 3) return "active";
  if (status === 4) return "exited";
  return "";
}

function statusLabel(status: number): string {
  if (status === 3) return t("home.active");
  if (status === 4 || status === 2) return t("home.exited");
  return t("home.inactive");
}

/** 镜像引用拆分为 name:tag（tag 缺省 latest，与上游一致）。 */
function splitImage(image: string): [string, string] {
  const idx = image.lastIndexOf(":");
  if (idx <= image.lastIndexOf("/")) return [image, "latest"];
  return [image.slice(0, idx), image.slice(idx + 1) || "latest"];
}

/** 上游 yamlToJSON 的校验部分：解析失败或 services 非对象时返回错误消息；
 *  另拦截服务引用未定义网络/具名卷（否则直到 compose 部署才报 invalid compose project）。 */
function composeYamlError(text: string): string {
  try {
    const doc = parseDocument(text);
    if (doc.errors.length > 0) throw doc.errors[0];
    const config = (doc.toJS() ?? {}) as { services?: unknown };
    const services = config.services ?? {};
    if (Array.isArray(services) || typeof services !== "object") {
      throw new Error("Services must be an object");
    }
    const missingNets = undefinedNetworks(text);
    if (missingNets.length > 0) {
      throw new Error(t("compose.undefinedNetworks", { names: missingNets.join(", ") }));
    }
    const missingVols = undefinedVolumes(text);
    if (missingVols.length > 0) {
      throw new Error(t("compose.undefinedVolumes", { names: missingVols.join(", ") }));
    }
    const missingDeps = undefinedDependsOn(text);
    if (missingDeps.length > 0) {
      throw new Error(t("compose.undefinedDependsOn", { names: missingDeps.join(", ") }));
    }
    return "";
  } catch (e) {
    return e instanceof Error ? e.message : String(e);
  }
}

/** URL 徽章展示形（上游：host + pathname(去尾斜杠) + search，解析失败回退原文）。 */
function urlDisplay(url: string): string {
  try {
    const obj = new URL(url);
    const pathname = obj.pathname === "/" ? "" : obj.pathname;
    return obj.host + pathname + obj.search;
  } catch {
    return url;
  }
}

/** 端口去重（IPv4/IPv6 双栈会产出重复的 Publisher 条目）。 */
function dedupePorts(ports: Array<{ hostPort?: number; containerPort: number }>) {
  const seen = new Set<string>();
  return ports.filter((port) => {
    const key = `${port.hostPort}:${port.containerPort}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

export function Compose() {
  const params = useParams();
  const navigate = useNavigate();
  const isNew = () => params.name === undefined;

  const [detail, setDetail] = createSignal<StackDetail | null>(null);
  const [name, setName] = createSignal("");
  const [yaml, setYaml] = createSignal(STARTER_YAML);
  const [env, setEnv] = createSignal(STARTER_ENV);
  const [editMode, setEditMode] = createSignal(true);
  const [busy, setBusy] = createSignal(false);
  const [output, setOutput] = createSignal("");
  const [yamlError, setYamlError] = createSignal("");
  // 日志流连接状态（实时指示）
  const [logState, setLogState] = createSignal<"live" | "ended">("live");
  const [downOpen, setDownOpen] = createSignal(false);
  const [stats, setStats] = createSignal<ContainerStat[]>([]);
  const [hostname, setHostname] = createSignal("");
  const [networks, setNetworks] = createSignal<string[]>([]);
  const [localImages, setLocalImages] = createSignal<string[]>([]);
  // 本地镜像列表供编辑表单 image 下拉选择（排序后同仓库多 tag 相邻，便于切换版本）；
  // 编辑表单每次展开下拉时经 refreshImages 重拉（5s 去抖）——镜像页/部署拉取的新镜像立即可见
  let imagesFetchedAt = 0;
  const refreshImages = () => {
    if (Date.now() - imagesFetchedAt < 5_000) return;
    imagesFetchedAt = Date.now();
    api.images()
      .then((r) => setLocalImages(
        r.list
          .map((img) => (img.tag === "<none>" ? img.id : `${img.repository}:${img.tag}`))
          .sort(),
      ))
      .catch(() => {});
  };
  const [newService, setNewService] = createSignal("");
  const [openConfigs, setOpenConfigs] = createSignal<Set<string>>(new Set());
  // 展开统计详情的容器名集合（上游 DockerStat 等价物）
  const [openStats, setOpenStats] = createSignal<Set<string>>(new Set());

  // 离开确认（上游 exitConfirm）：编辑态即确认，不按 dirty 判断
  useBeforeLeave((e) => {
    if (!editing() || e.defaultPrevented) return;
    if (!window.confirm(t("compose.leaveConfirm"))) e.preventDefault();
  });

  // Escape 关闭「停止并置于非活动状态」下拉（对齐上游 Bootstrap dropdown 行为）
  createEffect(() => {
    if (!downOpen()) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setDownOpen(false);
    };
    document.addEventListener("keydown", onKey);
    onCleanup(() => document.removeEventListener("keydown", onKey));
  });

  onMount(() => {
    api.primaryHostname().then((r) => setHostname(r.hostname)).catch(() => {});
    api.stackNetworks().then(setNetworks).catch(() => {});
    refreshImages();
  });

  // 路由参数变化（切换栈 / 进出新建页）→ 清理上一栈的会话状态再加载。
  // output（操作进度终端）、展开集合、服务输入若不清理，会原样挂在下一个栈上。
  createEffect(() => {
    const routeName = params.name;
    setOutput("");
    setOpenStats(new Set<string>());
    setOpenConfigs(new Set<string>());
    setNewService("");
    setDownOpen(false);
    if (routeName) {
      void load(decodeURIComponent(routeName));
      return;
    }
    // 新建页：重置为初始模板；composerize 草稿在此消费（untrack：草稿变化不重触发本 effect）
    setDetail(null);
    setName("");
    setYaml(untrack(() => draftYaml()) ?? STARTER_YAML);
    setEnv(STARTER_ENV);
    setEditMode(true);
    untrack(() => setDraftYaml(null));
  });

  const load = async (stackName: string) => {
    try {
      const data = await api.stack(stackName);
      setDetail(data);
      setName(data.name);
      setYaml(data.yaml);
      setEnv(data.env);
      setEditMode(false);
    } catch (error) {
      toast(errText(error), "error");
      navigate("/", { replace: true });
    }
  };

  // 上游 yamlCodeChange：解析成功即清空错误；失败时若已有错误立即更新，否则 3s 防抖首次呈现
  let yamlErrorTimer: ReturnType<typeof setTimeout> | undefined;
  createEffect(
    on(
      () => yaml(),
      () => {
        const message = composeYamlError(yaml());
        clearTimeout(yamlErrorTimer);
        if (!message) {
          setYamlError("");
          return;
        }
        if (yamlError()) {
          setYamlError(message);
          return;
        }
        yamlErrorTimer = setTimeout(() => setYamlError(message), 3000);
      },
    ),
  );
  onCleanup(() => clearTimeout(yamlErrorTimer));

  const save = async (): Promise<boolean> => {
    if (isNew()) {
      const stackName = name().trim().toLowerCase();
      if (!/^[a-z0-9_-]+$/.test(stackName)) {
        toast(t("compose.nameHelp"), "error");
        return false;
      }
      try {
        await api.createStack(stackName, yaml(), env());
        await refresh(false);
        navigate(`/compose/${encodeURIComponent(stackName)}`, { replace: true });
        return true;
      } catch (error) {
        setOutput(errText(error));
        toast(errText(error), "error");
        return false;
      }
    }
    const data = detail();
    if (!data?.managed) return false;
    try {
      await api.saveStack(data.name, yaml(), env());
      return true;
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
      return false;
    }
  };

  /** compose 执行类操作（部署/启动/更新）的前置校验门：现算校验
   *  （编辑器下方错误有 3s 防抖，不能作为拦截依据），语法/未定义引用
   *  在此拦下，避免保存成功后 compose 才报 invalid compose project。
   *  返回 true = 校验未过，调用方应中止。 */
  const yamlGate = (): boolean => {
    const message = composeYamlError(yaml());
    if (!message) return false;
    setYamlError(message);
    toast(message, "error");
    return true;
  };

  const deploy = async () => {
    if (busy()) return;
    // 上游 deployStack：名称为空时以首个服务名（或其 container_name）命名
    if (isNew() && !name().trim()) {
      const services = listServices(yaml());
      if (services.length === 0) {
        toast(t("compose.noServices"), "error");
        return;
      }
      const first = readService(yaml(), services[0]);
      const derived = (first.containerName || services[0]).toLowerCase();
      setName(derived);
    }
    if (yamlGate()) return;
    setBusy(true);
    try {
      const saved = await save();
      if (saved) await runOp(detail() ? (detail()?.status === 3 ? "update" : "start") : "start");
    } finally {
      setBusy(false);
    }
  };

  /** 格式化当前 compose（统一缩进/规整流式写法，注释保留）；语法错误提示且不动内容。 */
  const format = () => {
    const formatted = formatYaml(yaml());
    if (formatted === null) {
      toast(t("compose.formatSyntaxError"), "error");
      return;
    }
    setYaml(formatted);
  };

  const runOp = async (op: StackOp) => {
    // 注意不检查 busy()：deploy 链路在 save 期间保持 busy，
    // 若在此早退会导致「部署只存草稿、从不执行 compose」（按钮自身的
    // disabled=busy 已承担防重入）。
    const stackName = detail()?.name ?? name().trim().toLowerCase();
    if (!stackName) return;
    // start/update 会创建容器（compose 需完整合法的 project 定义）；
    // stop/restart/down 只作用于已存在容器，不拦。
    if ((op === "start" || op === "update") && yamlGate()) return;
    setBusy(true);
    setOutput(`$ docker compose ${op}\n`);
    try {
      await api.stackOpStream(stackName, op, (chunk) => setOutput((prev) => prev + chunk));
      await refresh(false);
      // 操作期间用户可能已切到别的栈（busy 只禁按钮，侧栏链接仍可点）：
      // 仅当路由仍在本栈时才重载，否则闭包里的旧栈名会覆盖当前视图
      if (decodeURIComponent(params.name ?? "") === stackName) await load(stackName);
    } catch (error) {
      setOutput((prev) => prev + `\n[error] ${errText(error)}\n`);
      toast(errText(error), "error");
    } finally {
      setBusy(false);
    }
  };

  // ---- 编辑态：服务增删改（经 yaml Document API 写回，注释保留）----

  const editingServices = () => (editing() ? listServices(yaml()) : []);

  const addContainer = async (event: SubmitEvent) => {
    event.preventDefault();
    const serviceName = newService().trim();
    if (editingServices().includes(serviceName)) {
      toast(t("compose.containerExists"), "error");
      return;
    }
    if (!serviceName) {
      toast(t("compose.containerNameEmpty"), "error");
      return;
    }
    setYaml(addService(yaml(), serviceName));
    setNewService("");
  };

  const deleteContainer = (serviceName: string) => {
    setYaml(removeService(yaml(), serviceName));
    setOpenConfigs((prev) => {
      const next = new Set(prev);
      next.delete(serviceName);
      return next;
    });
  };

  const patchService = (serviceName: string, fields: Partial<ServiceFields>) => {
    let next = updateService(yaml(), serviceName, fields);
    // 闭环：表单选网络/输具名卷必补顶层定义，否则部署被未定义引用校验拦下
    if (fields.networks) next = ensureTopLevelNetworks(next, fields.networks, new Set(networks()));
    if (fields.volumes) next = ensureTopLevelVolumes(next, fields.volumes);
    setYaml(next);
  };

  const toggleConfig = (serviceName: string) => {
    setOpenConfigs((prev) => {
      const next = new Set(prev);
      if (next.has(serviceName)) next.delete(serviceName);
      else next.add(serviceName);
      return next;
    });
  };

  const toggleStats = (containerName: string) => {
    setOpenStats((prev) => {
      const next = new Set(prev);
      if (next.has(containerName)) next.delete(containerName);
      else next.add(containerName);
      return next;
    });
  };

  const remove = async () => {
    const stackName = detail()?.name;
    if (!stackName) return;
    if (!(await confirmDialog(t("compose.confirmDelete"), t("compose.confirmDeleteDesc")))) return;
    setOutput("$ docker compose down\n");
    try {
      await api.deleteStack(stackName);
      await refresh(false);
      toast(t("toast.deleted", { name: stackName }), "success");
      navigate("/", { replace: true });
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
    }
  };

  // 上游 discardStack：重载内容并退出编辑态（无确认弹窗）
  const discard = () => {
    if (isNew()) {
      setYaml(STARTER_YAML);
      setEnv(STARTER_ENV);
    } else if (detail()) {
      setYaml(detail()!.yaml);
      setEnv(detail()!.env);
    }
    setEditMode(false);
  };

  // 浏览态每 5s 轮询单容器资源占用（上游 dockerStats 等价物）。
  // docker stats 每次约 1s（采样窗口固有开销），页面不可见时暂停轮询以免空耗。
  createEffect(() => {
    const data = detail();
    if (!data || editMode()) return;
    const poll = async () => {
      if (document.hidden) return;
      try {
        setStats(await api.stackStats(data.name));
      } catch {
        // 静默：下一轮自愈
      }
    };
    void poll();
    const timer = setInterval(poll, 5000);
    onCleanup(() => clearInterval(timer));
  });

  const runServiceOp = async (service: string, op: "start" | "stop" | "restart") => {
    const stackName = detail()?.name;
    if (!stackName || busy()) return;
    // 单服务 start 即 up（需合法 project 定义），同 runOp 拦截
    if (op === "start" && yamlGate()) return;
    setBusy(true);
    setOutput(`$ docker compose ${op} ${service}\n`);
    try {
      const result = await api.stackServiceOp(stackName, service, op);
      setOutput(`$ docker compose ${op} ${service}\n` + (result.output || ""));
      if (decodeURIComponent(params.name ?? "") === stackName) await load(stackName);
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
    } finally {
      setBusy(false);
    }
  };

  const managed = () => isNew() || detail()?.managed === true;
  const editing = () => editMode() && managed();
  const serviceCount = () => detail()?.containers.length ?? 0;
  const statOf = (containerName: string) => stats().find((s) => s.name === containerName);

  return (
    <div>
      <div class="compose-header">
        <h1>
          <Show when={!isNew()} fallback={<span>{t("compose.newTitle")}</span>}>
            <span class={`status-pill ${statusClass(detail()?.status ?? 0)}`}>{statusLabel(detail()?.status ?? 0)}</span>
            <span>{params.name ? decodeURIComponent(params.name) : ""}</span>
          </Show>
        </h1>
        <div class="compose-actions">
          <Show
            when={editing()}
            fallback={
              <Show when={managed()}>
                <button class="btn btn-secondary" onClick={() => setEditMode(true)}><Pencil size={14} /> {t("compose.edit")}</button>
                <Show when={detail()?.status === 3} fallback={<button class="btn btn-primary" disabled={busy()} onClick={() => void runOp("start")}><Play size={14} /> {t("compose.start")}</button>}>
                  <button class="btn btn-secondary" disabled={busy()} onClick={() => void runOp("restart")}><RotateCw size={14} /> {t("compose.restart")}</button>
                </Show>
                <button class="btn btn-secondary" disabled={busy()} onClick={() => void runOp("update")}><CloudDownload size={14} /> {t("compose.update")}</button>
                <Show when={detail()?.status === 3}>
                  <button class="btn btn-secondary" disabled={busy()} onClick={() => void runOp("stop")}><Square size={14} /> {t("compose.stop")}</button>
                </Show>
                <button class="btn btn-danger" disabled={busy()} onClick={() => void remove()}><Trash2 size={14} /> {t("compose.delete")}</button>
              </Show>
            }
          >
            <button class="btn btn-primary" disabled={busy()} onClick={() => void deploy()}><Rocket size={14} /> {t("compose.deploy")}</button>
            <button class="btn btn-secondary" disabled={busy()} onClick={() => void save().then((ok) => ok && toast(t("toast.draftSaved"), "success"))}><Save size={14} /> {t("compose.saveDraft")}</button>
            <Show when={!isNew()}>
              <button class="btn btn-secondary" disabled={busy()} onClick={discard}>{t("compose.discard")}</button>
            </Show>
          </Show>
          <Show when={managed()}>
            <div class="dropdown">
              <button class="btn btn-secondary" aria-label={t("compose.down")} onClick={() => setDownOpen((v) => !v)}>
                <ChevronDown size={14} />
              </button>
              <Show when={downOpen()}>
                <div class="dropdown-menu">
                  <button onClick={() => { setDownOpen(false); void runOp("down"); }}>
                    <Square size={14} /> {t("compose.down")}
                  </button>
                </div>
              </Show>
            </div>
          </Show>
        </div>
      </div>

      <Show when={detail()?.urls?.length}>
        <div class="url-badges">
          <For each={detail()?.urls ?? []}>
            {(url) => (
              <a class="url-badge" href={url} target="_blank" rel="noreferrer">
                {urlDisplay(url)}
              </a>
            )}
          </For>
        </div>
      </Show>

      <Show when={output()}>
        <div class="compose-progress">
          <DisplayTerminal content={output()} rows={8} />
        </div>
      </Show>

      <Show when={!isNew() && detail() && !detail()?.managed && !busy()}>
        <p class="settings-desc">{t("compose.unmanaged")}</p>
      </Show>

      <div class="compose-grid">
        <div>
          <Show when={isNew()}>
            <div class="card">
              <label class="form-label" for="compose-name">{t("compose.name")}</label>
              <input id="compose-name" class="form-input" value={name()} placeholder={t("compose.namePlaceholder")} onInput={(e) => setName(e.currentTarget.value.toLowerCase())} />
              <p class="form-help">{t("compose.nameHelp")}</p>
            </div>
          </Show>

          <div class="card" style={{ "margin-top": isNew() ? "20px" : "0" }}>
            <h4 class="card-title">{t("compose.containers")}</h4>
            <Show when={editing()} fallback={
              <Show when={(detail()?.containers ?? []).length > 0}>
              <For each={detail()?.containers ?? []}>
                {(container) => {
                  const [imageName, imageTag] = splitImage(container.image ?? "");
                  const stat = () => statOf(container.name);
                  const running = () => container.state === "running" || container.state === "healthy";
                  // 上游：restart/stop 对 unhealthy 容器同样可用
                  const operable = () => running() || container.state === "unhealthy";
                  return (
                    <div class="container-card">
                      <div class="container-card-row">
                        <div class="container-card-main">
                          <h4>{container.service || container.name}</h4>
                          <div class="container-meta mono">
                            {imageName}:<span class="tag">{imageTag}</span>
                          </div>
                          <div>
                            <span class={`status-pill ${running() ? "active" : container.state === "unhealthy" ? "exited" : ""}`}>{container.state}</span>
                          </div>
                          <Show when={!editing() && (container.ports?.length ?? 0) > 0}>
                            <div class="container-ports">
                              <For each={dedupePorts(container.ports ?? [])}>
                                {(port) => (
                                  <a class="port-badge" href={portUrl(port, hostname() || location.hostname)} target="_blank" rel="noreferrer">
                                    {displayPort(port)}
                                  </a>
                                )}
                              </For>
                            </div>
                          </Show>
                          <Show when={stat()}>
                            <div class="container-stats-row">
                              <Show when={!openStats().has(container.name)}>
                                <span class="container-stats">CPU: {stat()!.cpuPerc}</span>
                                <span class="container-stats">MEM: {stat()!.memUsage}</span>
                              </Show>
                              <span class="stats-toggle">
                                <button
                                  class="btn-icon"
                                  aria-label={t("stats.detail")}
                                  aria-expanded={openStats().has(container.name)}
                                  onClick={() => toggleStats(container.name)}
                                >
                                  <Show when={openStats().has(container.name)} fallback={<ChevronDown size={14} />}>
                                    <ChevronUp size={14} />
                                  </Show>
                                </button>
                              </span>
                            </div>
                          </Show>
                          <Show when={stat() && openStats().has(container.name)}>
                            <div class="stat-detail">
                              <div class="stat-detail-title">{stat()!.name}</div>
                              <div class="stat-detail-grid">
                                <div>
                                  <div class="stat-label">{t("stats.cpu")}</div>
                                  <div>{stat()!.cpuPerc}</div>
                                </div>
                                <div>
                                  <div class="stat-label">{t("stats.memory")}</div>
                                  <div>{stat()!.memUsage} ({stat()!.memPerc})</div>
                                </div>
                                <div>
                                  <div class="stat-label">{t("stats.networkIO")}</div>
                                  <div>{stat()!.netIO}</div>
                                </div>
                                <div>
                                  <div class="stat-label">{t("stats.blockIO")}</div>
                                  <div>{stat()!.blockIO}</div>
                                </div>
                              </div>
                            </div>
                          </Show>
                        </div>
                        <div class="container-card-actions">
                          <Show when={!editing() && running()}>
                            <A class="btn btn-sm btn-secondary" href={`/terminal/${encodeURIComponent(params.name ?? "")}/${encodeURIComponent(container.id)}/bash`}>
                              <Terminal size={14} /> Bash
                            </A>
                          </Show>
                          <Show when={!editing() && serviceCount() > 1}>
                            <Show when={operable()} fallback={<button class="btn btn-sm btn-primary" disabled={busy()} onClick={() => void runServiceOp(container.service ?? container.name, "start")}><Play size={14} /> {t("compose.start")}</button>}>
                              <button class="btn btn-sm btn-secondary" disabled={busy()} onClick={() => void runServiceOp(container.service ?? container.name, "restart")}><RotateCw size={14} /> {t("compose.restart")}</button>
                              <button class="btn btn-sm btn-secondary" disabled={busy()} onClick={() => void runServiceOp(container.service ?? container.name, "stop")}><Square size={14} /> {t("compose.stop")}</button>
                            </Show>
                          </Show>
                        </div>
                      </div>
                    </div>
                  );
                }}
              </For>
              </Show>
            }>
              {/* 编辑态：服务来自 YAML（上游 jsonConfig 等价物），配置表单写回保留注释 */}
              <form class="add-container" onSubmit={(e) => void addContainer(e)}>
                <input
                  class="form-input"
                  value={newService()}
                  placeholder={t("compose.addContainerName")}
                  aria-label={t("compose.addContainerName")}
                  onInput={(e) => setNewService(e.currentTarget.value)}
                />
                <button class="btn btn-primary" type="submit">
                  {t("compose.addContainer")}
                </button>
              </form>
              <For each={editingServices()}>
                {(serviceName) => (
                  <div class="container-card">
                    <div class="container-card-row">
                      <div class="container-card-main">
                        <h4>{serviceName}</h4>
                        <div class="container-meta mono">{readService(yaml(), serviceName).image ?? "-"}</div>
                      </div>
                      <div class="container-card-actions">
                        <button class="btn btn-sm btn-secondary" onClick={() => toggleConfig(serviceName)}><Pencil size={14} /> {t("compose.edit")}</button>
                        <button class="btn btn-sm btn-danger" onClick={() => deleteContainer(serviceName)}>
                          <Trash2 size={14} /> {t("compose.deleteContainer")}
                        </button>
                      </div>
                    </div>
                    <Show when={openConfigs().has(serviceName)}>
                      <ServiceConfigForm
                        yaml={yaml()}
                        serviceName={serviceName}
                        services={editingServices()}
                        hostNetworks={networks()}
                        localImages={localImages()}
                        onImagesOpen={refreshImages}
                        onChange={(fields) => patchService(serviceName, fields)}
                      />
                    </Show>
                  </div>
                )}
              </For>
            </Show>
          </div>

        </div>

        <div class="compose-editors">
          <div>
            <div class="editor-file-row">
              <h4 class="card-title editor-file-title">{detail()?.composeFileName || "compose.yaml"}</h4>
              <Show when={editing()}>
                <button class="btn btn-sm btn-secondary" onClick={format}>
                  <WandSparkles size={14} /> {t("compose.format")}
                </button>
              </Show>
            </div>
            <StackEditor
              file="compose"
              yaml={yaml()}
              env={env()}
              readonly={!editing()}
              onYamlChange={setYaml}
              onEnvChange={setEnv}
            />
            <Show when={editing() && yamlError()}>
              <div class="yaml-error">{yamlError()}</div>
            </Show>
          </div>
          <Show when={editing()}>
            <div>
              <h4 class="card-title editor-file-title">.env</h4>
              <StackEditor
                file="env"
                yaml={yaml()}
                env={env()}
                readonly={false}
                onYamlChange={setYaml}
                onEnvChange={setEnv}
              />
            </div>
          </Show>
          <Show when={editing()}>
            <NetworksCard yaml={yaml()} externalOptions={networks()} onChange={setYaml} />
          </Show>
        </div>

        <Show when={!editing() && !isNew() && detail()}>
          <div class="card compose-log-terminal">
            <h4 class="card-title">
              {t("compose.logs")}
              <span class={`log-state ${logState()}`}>{logState() === "live" ? t("compose.live") : t("compose.ended")}</span>
            </h4>
            <TerminalPane name={detail()!.name} type="compose-logs" onState={setLogState} />
          </div>
        </Show>
      </div>
    </div>
  );
}

/** 编辑态服务配置表单（上游 Container.vue config 的等价物）：
 *  字段变更即写回 YAML（Document API，注释保留）；列表字段为 ArrayInput 行样式。
 *  image 输入框旁：仅当当前镜像在本地有同仓库其它 tag 时出现版本下拉（切换版本）；
 *  networks 下拉 = compose 顶层 ∪ 本机网络（选择即自动补顶层定义）；
 *  depends_on 下拉选同栈其他服务。 */
function ServiceConfigForm(props: {
  yaml: string;
  serviceName: string;
  services: string[];
  hostNetworks: string[];
  localImages: string[];
  onImagesOpen: () => void;
  onChange: (fields: Partial<ServiceFields>) => void;
}) {
  const initial = () => readService(props.yaml, props.serviceName);
  // 网络选项：compose 顶层定义 ∪ 本机 docker 网络（去重，compose 内优先）
  const networkOptions = () => [...new Set([...listTopLevelNetworks(props.yaml), ...props.hostNetworks])];
  // 依赖选项：同栈其他服务（排除自身，避免自依赖）
  const dependOptions = () => props.services.filter((s) => s !== props.serviceName);
  // 长语法（数组项为对象）时上游提示改用 YAML 编辑器
  const long = (key: keyof ServiceFields) => hasLongSyntax(props.yaml, props.serviceName, key) !== undefined;

  const patch = (fields: Partial<ServiceFields>) => props.onChange(fields);

  return (
    <div class="service-form">
      <div class="form-block">
        <label class="form-label" for={`service-image-${props.serviceName}`}>{t("form.image")}</label>
        <ImageCombo
          id={`service-image-${props.serviceName}`}
          value={initial().image ?? ""}
          images={props.localImages}
          onOpen={() => props.onImagesOpen()}
          onChange={(image) => patch({ image })}
        />
      </div>
      <div class="form-block">
        <label class="form-label">{t("form.ports")}</label>
        <Show when={!long("ports")} fallback={<p class="form-help">{t("form.longSyntax")}</p>}>
        <ArrayField
          displayName={t("form.ports")}
          placeholder="HOST:CONTAINER"
          rows={initial().ports}
          onChange={(rows) => patch({ ports: rows ?? [] })}
        />
        </Show>
      </div>
      <div class="form-block">
        <label class="form-label">{t("form.volumes")}</label>
        <Show when={!long("volumes")} fallback={<p class="form-help">{t("form.longSyntax")}</p>}>
        <ArrayField
          displayName={t("form.volumes")}
          placeholder="HOST:CONTAINER"
          rows={initial().volumes}
          onChange={(rows) => patch({ volumes: rows ?? [] })}
        />
        </Show>
      </div>
      <div class="form-block">
        <label class="form-label">{t("form.restartPolicy")}</label>
        <select
          class="form-select"
          value={initial().restart ?? ""}
          onChange={(e) => patch({ restart: e.currentTarget.value || undefined })}
        >
          <option value=""></option>
          <option value="always">{t("policy.always")}</option>
          <option value="unless-stopped">{t("policy.unlessStopped")}</option>
          <option value="on-failure">{t("policy.onFailure")}</option>
          <option value="no">{t("policy.no")}</option>
        </select>
      </div>
      <div class="form-block">
        <label class="form-label">{t("form.env")}</label>
        <Show when={!long("environment")} fallback={<p class="form-help">{t("form.longSyntax")}</p>}>
        <ArrayField
          displayName={t("form.env")}
          placeholder="KEY=VALUE"
          rows={initial().environment}
          onChange={(rows) => patch({ environment: rows ?? [] })}
        />
        </Show>
      </div>
      <div class="form-block">
        <label class="form-label">{t("form.networks")}</label>
        <Show when={!long("networks")} fallback={<p class="form-help">{t("form.longSyntax")}</p>}>
        <ArraySelectField
          displayName={t("form.networks")}
          options={networkOptions()}
          rows={initial().networks}
          onChange={(rows) => patch({ networks: rows ?? [] })}
        />
        </Show>
      </div>
      <div class="form-block">
        <label class="form-label">{t("form.dependsOn")}</label>
        <Show when={!long("dependsOn")} fallback={<p class="form-help">{t("form.longSyntax")}</p>}>
        <ArraySelectField
          displayName={t("form.dependsOn")}
          options={dependOptions()}
          placeholder={t("form.selectService")}
          rows={initial().dependsOn}
          onChange={(rows) => patch({ dependsOn: rows ?? [] })}
        />
        </Show>
      </div>
    </div>
  );
}

/** 编辑态网络卡（上游 NetworkInput 的等价物）：内部网络行内改名/增删 +
 *  外部网络开关（勾选写入 external: true）。 */
function NetworksCard(props: { yaml: string; externalOptions: string[]; onChange: (text: string) => void }) {
  const entries = () => listTopLevelNetworkEntries(props.yaml);
  const internal = () => entries().filter((e) => !e.external);
  const external = () => entries().filter((e) => e.external);

  const write = (next: Array<{ name: string; external: boolean }>) =>
    props.onChange(setTopLevelNetworkEntries(props.yaml, next));

  // 改名/删除经同步函数：顶层定义与服务级引用一起动，保持一致（否则留下未定义引用）
  const renameInternal = (index: number, name: string) => {
    const old = internal()[index];
    if (!old || !name.trim()) return;
    props.onChange(renameTopLevelNetwork(props.yaml, old.name, name.trim()));
  };

  const removeInternal = (index: number) => {
    const old = internal()[index];
    if (!old) return;
    props.onChange(removeTopLevelNetwork(props.yaml, old.name));
  };

  const addInternal = () => write([...internal(), { name: "", external: false }, ...external()]);

  const toggleExternal = (name: string, on: boolean) => {
    const rest = entries().filter((e) => e.name !== name);
    write(on ? [...rest, { name, external: true }] : rest);
  };

  const selected = (name: string) => external().some((e) => e.name === name);

  return (
    <div class="card">
      <h4 class="card-title">{t("form.networks")}</h4>
      <h5>{t("networks.internal")}</h5>
      <ul class="array-field-list">
        <For each={internal()}>
          {(entry, index) => (
            <li>
              <input
                class="form-input mono"
                placeholder={t("networks.namePlaceholder")}
                aria-label={t("networks.namePlaceholder")}
                value={entry.name}
                onInput={(e) => renameInternal(index(), e.currentTarget.value)}
              />
              <button class="btn-icon danger" aria-label={t("common.close")} onClick={() => removeInternal(index())}>
                <X size={14} />
              </button>
            </li>
          )}
        </For>
      </ul>
      <button class="btn btn-sm btn-secondary" type="button" onClick={addInternal}>
        {t("networks.addInternal")}
      </button>
      <h5 style={{ "margin-top": "16px" }}>{t("networks.external")}</h5>
      <Show when={props.externalOptions.length === 0} fallback={
        <For each={props.externalOptions}>
          {(name) => (
            <label class="network-switch">
              <input type="checkbox" checked={selected(name)} onChange={(e) => toggleExternal(name, e.currentTarget.checked)} />
              {name}
            </label>
          )}
        </For>
      }>
        <p class="settings-desc">{t("networks.none")}</p>
      </Show>
    </div>
  );
}
