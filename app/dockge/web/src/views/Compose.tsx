// 栈详情/新建页（上游复刻）：标题行（状态 pill + 操作按钮组双态）+ 双栏
// （左：容器卡片/合并日志；右：CodeMirror 编辑器 compose + .env 上下排列）。
// 校验闭环：防抖 2s 自动草稿校验 + 诊断注入 + 四态 pill（超越上游的 yamlError）。
import { For, Show, createEffect, createSignal, on, onCleanup, onMount } from "solid-js";
import { A, useBeforeLeave, useNavigate, useParams } from "@solidjs/router";
import { ChevronDown, ExternalLink, Play, RotateCw, Square, Terminal, Trash2 } from "lucide-solid";

import { api, type ContainerStat, type StackDetail, type StackOp } from "../api/api";
import { errText } from "../api/format";
import { confirmDialog } from "../components/Confirm";
import { StackEditor } from "../components/StackEditor";
import { DisplayTerminal, TerminalPane } from "../components/Terminal";
import { t } from "../i18n";
import type { ValidationState } from "../lib/validate";
import {
  addService,
  listServices,
  listTopLevelNetworks,
  readService,
  removeService,
  setTopLevelNetworks,
  updateService,
  type ServiceFields,
} from "../lib/yaml-edit";
import { draftYaml, refresh, setDraftYaml, toast } from "../store/index";

const STARTER_YAML = `services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
`;

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
  const [env, setEnv] = createSignal("");
  const [editMode, setEditMode] = createSignal(true);
  const [busy, setBusy] = createSignal(false);
  const [output, setOutput] = createSignal("");
  const [validation, setValidation] = createSignal<ValidationState>({ status: "checking" });
  const [loadedYaml, setLoadedYaml] = createSignal(STARTER_YAML);
  const [loadedEnv, setLoadedEnv] = createSignal("");
  const [downOpen, setDownOpen] = createSignal(false);
  const [stats, setStats] = createSignal<ContainerStat[]>([]);
  const [hostname, setHostname] = createSignal("");
  const [networks, setNetworks] = createSignal<string[]>([]);
  const [newService, setNewService] = createSignal("");
  const [openConfigs, setOpenConfigs] = createSignal<Set<string>>(new Set());
  const dirty = () => yaml() !== loadedYaml() || env() !== loadedEnv();

  // 未保存离开确认（上游 confirmLeaveStack 等价物）
  useBeforeLeave((e) => {
    if (!dirty() || e.defaultPrevented) return;
    if (!window.confirm(t("compose.leaveConfirm"))) e.preventDefault();
  });

  onMount(() => {
    api.primaryHostname().then((r) => setHostname(r.hostname)).catch(() => {});
    api.stackNetworks().then(setNetworks).catch(() => {});
    if (isNew()) {
      const draft = draftYaml();
      if (draft) {
        setYaml(draft);
        setLoadedYaml(draft);
        setDraftYaml(null);
      }
    }
  });

  // 路由参数变化（切换栈 / 新建保存后跳转）→ 加载对应栈
  createEffect(() => {
    const routeName = params.name;
    if (routeName) void load(decodeURIComponent(routeName));
  });

  const load = async (stackName: string) => {
    try {
      const data = await api.stack(stackName);
      setDetail(data);
      setName(data.name);
      setYaml(data.yaml);
      setEnv(data.env);
      setLoadedYaml(data.yaml);
      setLoadedEnv(data.env);
      setEditMode(false);
    } catch (error) {
      toast(errText(error), "error");
      navigate("/", { replace: true });
    }
  };

  // 校验闭环（被动）：compose/env 任一变更防抖 2s 后草稿校验；序号守卫丢弃过期响应
  let validateTimer: ReturnType<typeof setTimeout> | undefined;
  let validateSeq = 0;
  createEffect(
    on(
      () => [editMode(), yaml(), env()] as const,
      () => {
        validateSeq++;
        setValidation({ status: "checking" });
        clearTimeout(validateTimer);
        validateTimer = setTimeout(() => {
          const seq = validateSeq;
          api
            .validateStack(yaml(), env())
            .then((result) => {
              if (seq === validateSeq) setValidation({ status: "done", result });
            })
            .catch((error: unknown) => {
              if (seq === validateSeq) {
                setValidation({ status: "done", result: { valid: false, errors: [{ line: 0, message: errText(error) }] } });
              }
            });
        }, 2000);
      },
    ),
  );
  onCleanup(() => clearTimeout(validateTimer));

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
      setLoadedYaml(yaml());
      setLoadedEnv(env());
      return true;
    } catch (error) {
      setOutput(errText(error));
      toast(errText(error), "error");
      return false;
    }
  };

  const deploy = async () => {
    if (busy()) return;
    setBusy(true);
    try {
      const saved = await save();
      if (saved) await runOp(detail() ? (detail()?.status === 3 ? "update" : "start") : "start");
    } finally {
      setBusy(false);
    }
  };

  const runOp = async (op: StackOp) => {
    const stackName = detail()?.name ?? name().trim().toLowerCase();
    if (!stackName || busy()) return;
    setBusy(true);
    setOutput(`$ docker compose ${op}\n`);
    try {
      await api.stackOpStream(stackName, op, (chunk) => setOutput((prev) => prev + chunk));
      await refresh(false);
      await load(stackName);
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
    if (!serviceName) return;
    if (editingServices().includes(serviceName)) {
      toast(t("compose.containerExists"), "error");
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

  const patchService = (serviceName: string, fields: ServiceFields) => {
    setYaml(updateService(yaml(), serviceName, fields));
  };

  const toggleConfig = (serviceName: string) => {
    setOpenConfigs((prev) => {
      const next = new Set(prev);
      if (next.has(serviceName)) next.delete(serviceName);
      else next.add(serviceName);
      return next;
    });
  };

  const remove = async () => {
    const stackName = detail()?.name;
    if (!stackName) return;
    if (!(await confirmDialog(t("compose.confirmDelete"), t("compose.confirmDeleteDesc", { name: stackName })))) return;
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

  const discard = async () => {
    if (!(await confirmDialog(t("compose.discard"), t("compose.discardConfirm"), false))) return;
    if (isNew()) {
      setYaml(STARTER_YAML);
      setEnv("");
    } else if (detail()) {
      setYaml(detail()!.yaml);
      setEnv(detail()!.env);
    }
  };

  // 浏览态每 5s 轮询单容器资源占用（上游 dockerStats 等价物）
  createEffect(() => {
    const data = detail();
    if (!data || editMode()) return;
    const poll = async () => {
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
    setBusy(true);
    setOutput(`$ docker compose ${op} ${service}\n`);
    try {
      const result = await api.stackServiceOp(stackName, service, op);
      setOutput(`$ docker compose ${op} ${service}\n` + (result.output || ""));
      await load(stackName);
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
  const portUrl = (hostPort?: number) => `http://${hostname() || location.hostname}:${hostPort}`;

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
                <button class="btn btn-secondary" onClick={() => setEditMode(true)}>{t("compose.edit")}</button>
                <Show when={detail()?.status === 3} fallback={<button class="btn btn-primary" disabled={busy()} onClick={() => void runOp("start")}><Play size={14} /> {t("compose.start")}</button>}>
                  <button class="btn btn-secondary" disabled={busy()} onClick={() => void runOp("restart")}><RotateCw size={14} /> {t("compose.restart")}</button>
                </Show>
                <button class="btn btn-secondary" disabled={busy()} onClick={() => void runOp("update")}>{t("compose.update")}</button>
                <Show when={detail()?.status === 3}>
                  <button class="btn btn-secondary" disabled={busy()} onClick={() => void runOp("stop")}><Square size={14} /> {t("compose.stop")}</button>
                </Show>
                <button class="btn btn-danger" disabled={busy()} onClick={() => void remove()}>{t("compose.delete")}</button>
              </Show>
            }
          >
            <button class="btn btn-primary" disabled={busy()} onClick={() => void deploy()}>{t("compose.deploy")}</button>
            <button class="btn btn-secondary" disabled={busy()} onClick={() => void save().then((ok) => ok && toast(t("toast.draftSaved"), "success"))}>{t("compose.saveDraft")}</button>
            <Show when={!isNew()}>
              <button class="btn btn-secondary" disabled={busy() || !dirty()} onClick={() => void discard()}>{t("compose.discard")}</button>
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
                <ExternalLink size={11} style={{ "vertical-align": "-1px" }} /> {url}
              </a>
            )}
          </For>
        </div>
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
              <Show
                when={(detail()?.containers ?? []).length > 0}
                fallback={<p class="settings-desc">{managed() ? t("compose.noContainers") : ""}</p>}
              >
              <For each={detail()?.containers ?? []}>
                {(container) => {
                  const [imageName, imageTag] = splitImage(container.image ?? "");
                  const stat = () => statOf(container.name);
                  const running = () => container.state === "running" || container.state === "healthy";
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
                                  <a class="port-badge" href={portUrl(port.hostPort)} target="_blank" rel="noreferrer">
                                    {port.hostPort}:{port.containerPort}
                                  </a>
                                )}
                              </For>
                            </div>
                          </Show>
                          <Show when={stat()}>
                            <div class="container-stats">
                              <span>CPU {stat()!.cpuPerc}</span>
                              <span>MEM {stat()!.memUsage}</span>
                            </div>
                          </Show>
                        </div>
                        <div class="container-card-actions">
                          <Show when={!editing() && running()}>
                            <A class="btn-icon" aria-label={t("container.terminal")} href={`/terminal/${encodeURIComponent(params.name ?? "")}/${encodeURIComponent(container.id)}/bash`}>
                              <Terminal size={15} />
                            </A>
                          </Show>
                          <Show when={!editing() && serviceCount() > 1}>
                            <Show when={running()} fallback={<button class="btn-icon" aria-label={t("compose.start")} disabled={busy()} onClick={() => void runServiceOp(container.service ?? container.name, "start")}><Play size={14} /></button>}>
                              <button class="btn-icon" aria-label={t("compose.restart")} disabled={busy()} onClick={() => void runServiceOp(container.service ?? container.name, "restart")}><RotateCw size={14} /></button>
                              <button class="btn-icon" aria-label={t("compose.stop")} disabled={busy()} onClick={() => void runServiceOp(container.service ?? container.name, "stop")}><Square size={14} /></button>
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
                  onInput={(e) => setNewService(e.currentTarget.value)}
                />
                <button class="btn btn-secondary" type="submit" disabled={!newService().trim()}>
                  {t("compose.addContainer")}
                </button>
              </form>
              <For each={editingServices()} fallback={<p class="settings-desc">{t("compose.noContainers")}</p>}>
                {(serviceName) => (
                  <div class="container-card">
                    <div class="container-card-row">
                      <div class="container-card-main">
                        <h4>{serviceName}</h4>
                        <div class="container-meta mono">{readService(yaml(), serviceName).image ?? "-"}</div>
                      </div>
                      <div class="container-card-actions">
                        <button class="btn btn-sm btn-secondary" onClick={() => toggleConfig(serviceName)}>{t("compose.editConfig")}</button>
                        <button class="btn-icon danger" aria-label={t("compose.deleteContainer")} onClick={() => deleteContainer(serviceName)}>
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </div>
                    <Show when={openConfigs().has(serviceName)}>
                      <ServiceConfigForm
                        networks={networks()}
                        initial={readService(yaml(), serviceName)}
                        onSave={(fields) => patchService(serviceName, fields)}
                      />
                    </Show>
                  </div>
                )}
              </For>
            </Show>
          </div>

          <Show when={!editing() && !isNew() && detail()}>
            <div class="card">
              <h4 class="card-title">{t("compose.terminal")}</h4>
              <TerminalPane name={detail()!.name} type="compose-logs" />
            </div>
          </Show>
        </div>

        <div class="compose-editors">
          <div>
            <div class="editor-file-title">{detail()?.composeFileName || "compose.yaml"}</div>
            <StackEditor
              file="compose"
              yaml={yaml()}
              env={env()}
              readonly={!editing()}
              fullscreen={false}
              validation={validation()}
              output=""
              onYamlChange={setYaml}
              onEnvChange={setEnv}
              onFullscreenChange={() => {}}
            />
          </div>
          <Show when={editing()}>
            <div>
              <div class="editor-file-title">.env</div>
              <StackEditor
                file="env"
                yaml={yaml()}
                env={env()}
                readonly={false}
                fullscreen={false}
                validation={{ status: "done", result: { valid: true, errors: [] } }}
                output=""
                onYamlChange={setYaml}
                onEnvChange={setEnv}
                onFullscreenChange={() => {}}
              />
            </div>
          </Show>
          <Show when={editing()}>
            <TopNetworksCard yaml={yaml()} suggestions={networks()} onChange={setYaml} />
          </Show>
          <Show when={output()}>
            <div>
              <h4 class="card-title" style={{ "margin-bottom": "8px" }}>{t("compose.progress")}</h4>
              <DisplayTerminal content={output()} rows={8} />
            </div>
          </Show>
        </div>
      </div>
    </div>
  );
}

/** 编辑态服务配置表单（上游 Container.vue 编辑表单的等价物）：
 *  失焦即写回 YAML（Document API，注释保留）。列表字段每行一条。 */
function ServiceConfigForm(props: {
  networks: string[];
  initial: ServiceFields;
  onSave: (fields: ServiceFields) => void;
}) {
  const [image, setImage] = createSignal(props.initial.image ?? "");
  const [ports, setPorts] = createSignal((props.initial.ports ?? []).join("\n"));
  const [volumes, setVolumes] = createSignal((props.initial.volumes ?? []).join("\n"));
  const [restart, setRestart] = createSignal(props.initial.restart ?? "");
  const [env, setEnv] = createSignal((props.initial.environment ?? []).join("\n"));
  const [dependsOn, setDependsOn] = createSignal((props.initial.dependsOn ?? []).join("\n"));

  const lines = (value: string) =>
    value.split("\n").map((line) => line.trim()).filter(Boolean);

  const save = () =>
    props.onSave({
      image: image().trim() || undefined,
      ports: lines(ports()),
      volumes: lines(volumes()),
      restart: restart() || undefined,
      environment: lines(env()),
      dependsOn: lines(dependsOn()),
    });

  return (
    <div class="service-form" onFocusOut={save}>
      <div>
        <label class="form-label">{t("form.image")}</label>
        <input class="form-input mono" value={image()} onInput={(e) => setImage(e.currentTarget.value)} />
      </div>
      <div>
        <label class="form-label">{t("form.restartPolicy")}</label>
        <select class="form-select" value={restart()} onChange={(e) => { setRestart(e.currentTarget.value); save(); }}>
          <option value=""></option>
          <option value="always">{t("policy.always")}</option>
          <option value="unless-stopped">{t("policy.unlessStopped")}</option>
          <option value="on-failure">{t("policy.onFailure")}</option>
          <option value="no">{t("policy.no")}</option>
        </select>
      </div>
      <div>
        <label class="form-label">{t("form.ports")}</label>
        <textarea class="form-input mono" rows="2" placeholder="HOST:CONTAINER" value={ports()} onInput={(e) => setPorts(e.currentTarget.value)} />
      </div>
      <div>
        <label class="form-label">{t("form.volumes")}</label>
        <textarea class="form-input mono" rows="2" placeholder="HOST:CONTAINER" value={volumes()} onInput={(e) => setVolumes(e.currentTarget.value)} />
      </div>
      <div>
        <label class="form-label">{t("form.env")}</label>
        <textarea class="form-input mono" rows="2" placeholder="KEY=VALUE" value={env()} onInput={(e) => setEnv(e.currentTarget.value)} />
      </div>
      <div>
        <label class="form-label">{t("form.dependsOn")}</label>
        <textarea class="form-input mono" rows="2" value={dependsOn()} onInput={(e) => setDependsOn(e.currentTarget.value)} />
      </div>
      <Show when={props.networks.length > 0}>
        <p class="form-help">{t("compose.networksHint", { list: props.networks.join(" · ") })}</p>
      </Show>
    </div>
  );
}

/** 编辑态顶层网络卡（上游右栏 Networks 卡的等价物）。 */
function TopNetworksCard(props: { yaml: string; suggestions: string[]; onChange: (text: string) => void }) {
  const [input, setInput] = createSignal("");
  const current = () => listTopLevelNetworks(props.yaml);

  const add = (event: SubmitEvent) => {
    event.preventDefault();
    const name = input().trim();
    if (!name || current().includes(name)) return;
    props.onChange(setTopLevelNetworks(props.yaml, [...current(), name]));
    setInput("");
  };

  const remove = (name: string) => {
    props.onChange(setTopLevelNetworks(props.yaml, current().filter((item) => item !== name)));
  };

  return (
    <div class="card">
      <h4 class="card-title">{t("form.networks")}</h4>
      <form class="add-container" onSubmit={add}>
        <input class="form-input mono" list="network-suggestions" value={input()} onInput={(e) => setInput(e.currentTarget.value)} />
        <datalist id="network-suggestions">
          <For each={props.suggestions}>{(name) => <option value={name} />}</For>
        </datalist>
        <button class="btn btn-secondary" type="submit" disabled={!input().trim()}>{t("common.save")}</button>
      </form>
      <For each={current()} fallback={<p class="settings-desc">default</p>}>
        {(name) => (
          <div class="network-row">
            <span class="mono">{name}</span>
            <button class="btn-icon danger" aria-label={t("compose.delete")} onClick={() => remove(name)}>
              <Trash2 size={13} />
            </button>
          </div>
        )}
      </For>
    </div>
  );
}
