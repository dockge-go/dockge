// compose.yaml 结构化编辑：基于 yaml Document API 的服务/网络增删改。
// 走 Document 节点树而非 parse→stringify，注释在往返中原生保留
// （优于上游 copyYAMLComments 的行匹配恢复）。所有操作对非法 YAML 返回原文本。
import { isMap, isSeq, parseDocument, YAMLMap, type Document } from "yaml";

/** 服务可编辑字段（与上游容器配置表单一一对应）。 */
export interface ServiceFields {
  image?: string;
  ports?: string[];
  volumes?: string[];
  restart?: string;
  environment?: string[];
  dependsOn?: string[];
  networks?: string[];
  containerName?: string;
}

/** ServiceFields 键 → compose YAML 键（仅 depends_on 需要映射）。 */
const serviceYamlKeys: Record<keyof ServiceFields, string> = {
  image: "image",
  ports: "ports",
  volumes: "volumes",
  restart: "restart",
  environment: "environment",
  dependsOn: "depends_on",
  networks: "networks",
  containerName: "container_name",
};

function parse(text: string): Document.Parsed | null {
  try {
    const doc = parseDocument(text);
    return doc.errors.length > 0 ? null : doc;
  } catch {
    return null;
  }
}

function servicesMap(doc: Document.Parsed): YAMLMap | null {
  const node = doc.get("services", true);
  return isMap(node) ? node : null;
}

/** 列出全部服务名；非法 YAML 返回空。 */
export function listServices(text: string): string[] {
  const doc = parse(text);
  const services = doc && servicesMap(doc);
  if (!services) return [];
  return services.items.map((pair) => String(pair.key));
}

/** 新增服务骨架（上游默认 restart: unless-stopped）；重名时原样返回。 */
export function addService(text: string, name: string): string {
  const doc = parse(text);
  const services = doc && servicesMap(doc);
  if (!doc || !services || services.has(name)) return text;
  services.set(name, doc.createNode({ restart: "unless-stopped" }));
  return doc.toString({ lineWidth: 0 });
}

/** 删除服务；不存在时原样返回。 */
export function removeService(text: string, name: string): string {
  const doc = parse(text);
  const services = doc && servicesMap(doc);
  if (!doc || !services || !services.has(name)) return text;
  services.delete(name);
  return doc.toString({ lineWidth: 0 });
}

/** 更新服务字段：undefined 跳过、空数组删除该键、其余整体覆写。 */
export function updateService(text: string, name: string, fields: Partial<ServiceFields>): string {
  const doc = parse(text);
  const services = doc && servicesMap(doc);
  const serviceNode = services ? services.get(name, true) : undefined;
  const service = isMap(serviceNode) ? serviceNode : null;
  if (!doc || !service) return text;
  for (const [field, value] of Object.entries(fields) as Array<[keyof ServiceFields, ServiceFields[keyof ServiceFields]]>) {
    if (value === undefined) continue;
    const key = serviceYamlKeys[field];
    if (Array.isArray(value) && value.length === 0) {
      service.delete(key);
      continue;
    }
    service.set(key, doc.createNode(value));
  }
  return doc.toString({ lineWidth: 0 });
}

/** 读取服务当前字段（编辑表单初始值）；服务不存在返回空对象。 */
export function readService(text: string, name: string): ServiceFields {
  const doc = parse(text);
  const services = doc && servicesMap(doc);
  const node = services && services.get(name, true);
  if (!doc || !isMap(node)) return {};
  const result: ServiceFields = {};
  const scalar = (key: string): string | undefined => {
    const value = node.get(key, true);
    return value === undefined || value === null ? undefined : String(value);
  };
  const list = (key: string): string[] | undefined => {
    const value = node.get(key, true);
    if (!isSeq(value)) return undefined;
    return value.items.map((item) => String(item));
  };
  result.image = scalar("image");
  result.restart = scalar("restart");
  result.ports = list("ports");
  result.volumes = list("volumes");
  result.environment = list("environment");
  result.dependsOn = list("depends_on");
  result.networks = list("networks");
  result.containerName = scalar("container_name");
  return result;
}

/** 指定服务字段是否为表单不支持的写法——数组项含对象（长语法）或整体为
 *  字典（如 environment: K: v、depends_on: condition）。此时表单退位给
 *  YAML 编辑器：提供输入行会在保存时整体覆写，静默丢失字典内容。 */
export function hasLongSyntax(text: string, name: string, key: keyof ServiceFields): string | undefined {
  const doc = parse(text);
  const services = doc && servicesMap(doc);
  const node = services ? services.get(name, true) : undefined;
  if (!doc || !isMap(node)) return undefined;
  const value = node.get(serviceYamlKeys[key], true);
  if (isMap(value)) return "long";
  if (!isSeq(value)) return undefined;
  return value.items.some((item) => isMap(item)) ? "long" : undefined;
}

/** 顶层网络名列表。 */
export function listTopLevelNetworks(text: string): string[] {
  const doc = parse(text);
  const node = doc && doc.get("networks", true);
  if (!doc || !isMap(node)) return [];
  return node.items.map((pair) => String(pair.key));
}

/** 服务引用了但顶层 networks 未定义的网络名（compose 部署时才报
 *  "refers to undefined network ...: invalid compose project"，保存前提前拦截）。
 *  服务级 networks 兼容列表与字典写法、长语法 name 字段。 */
export function undefinedNetworks(text: string): string[] {
  const doc = parse(text);
  if (!doc) return [];
  const services = doc.get("services", true);
  if (!isMap(services)) return [];
  const defined = new Set(listTopLevelNetworks(text));
  const missing = new Set<string>();
  for (const pair of services.items) {
    const svc = pair.value;
    if (!isMap(svc)) continue;
    const refs = svc.get("networks", true);
    const names = isMap(refs)
      ? refs.items.map((p) => String(p.key))
      : isSeq(refs)
        ? refs.items.map((item) =>
            isMap(item) ? String(item.get("name") ?? "") : String(item ?? ""),
          )
        : null;
    if (!names) continue;
    for (const name of names) {
      if (name && !defined.has(name)) missing.add(name);
    }
  }
  return [...missing];
}

/** 服务引用了但顶层 volumes 未定义的具名卷（compose 部署时才报
 *  "refers to undefined volume ...: invalid compose project"，保存前提前拦截）。
 *  短语法 "name:/container"（bind 挂载 /path、./path 不需声明）与
 *  长语法 {type: volume, source: name} 均识别；匿名卷（单字段）无需声明。 */
export function undefinedVolumes(text: string): string[] {
  const doc = parse(text);
  if (!doc) return [];
  const services = doc.get("services", true);
  const volumes = doc.get("volumes", true);
  if (!isMap(services)) return [];
  const defined = new Set(isMap(volumes) ? volumes.items.map((p) => String(p.key)) : []);
  const missing = new Set<string>();
  for (const pair of services.items) {
    const svc = pair.value;
    if (!isMap(svc)) continue;
    const refs = svc.get("volumes", true);
    if (!isSeq(refs)) continue;
    for (const item of refs.items) {
      const src = isMap(item)
        ? (item.get("type") == null || String(item.get("type")) === "volume")
          ? String(item.get("source") ?? "")
          : ""
        : String(item ?? "").split(":")[0];
      // bind 挂载（/、./、../ 开头）与空源不要求顶层声明
      if (src && !src.startsWith("/") && !src.startsWith("./") && !src.startsWith("../") && !defined.has(src)) {
        missing.add(src);
      }
    }
  }
  return [...missing];
}

/** 格式化 compose YAML：统一缩进/规整流式写法（Document.toString，注释保留）。
 *  语法错误返回 null，由调用方提示——格式化不修复语法，只规整合法 YAML。 */
export function formatYaml(text: string): string | null {
  const doc = parse(text);
  if (!doc) return null;
  return doc.toString({ indentSeq: false }).trimEnd() + "\n";
}

/** 服务 depends_on 引用了不存在的服务名（删服务后残留引用时 compose 报
 *  "depends on undefined service"，保存前提前拦截）。
 *  兼容列表写法、字典写法与长语法 {service: name}。 */
export function undefinedDependsOn(text: string): string[] {
  const doc = parse(text);
  if (!doc) return [];
  const services = doc.get("services", true);
  if (!isMap(services)) return [];
  const names = new Set(services.items.map((p) => String(p.key)));
  const missing = new Set<string>();
  for (const pair of services.items) {
    const svc = pair.value;
    if (!isMap(svc)) continue;
    const refs = svc.get("depends_on", true);
    const deps = isMap(refs)
      ? refs.items.map((p) => String(p.key))
      : isSeq(refs)
        ? refs.items.map((item) =>
            isMap(item) ? String(item.get("service") ?? "") : String(item ?? ""),
          )
        : null;
    if (!deps) continue;
    for (const dep of deps) {
      if (dep && !names.has(dep)) missing.add(dep);
    }
  }
  return [...missing];
}

/** 确保挂载里的具名卷都在顶层 volumes 有定义（编辑表单的闭环写回）：
 *  bind 挂载（/、./、../ 开头）与匿名卷（无源）不需要声明，跳过；已定义不动。 */
export function ensureTopLevelVolumes(text: string, mounts: string[]): string {
  const doc = parse(text);
  if (!doc) return text;
  const existing = doc.get("volumes", true);
  let vols: YAMLMap;
  if (isMap(existing)) {
    vols = existing;
  } else {
    vols = new YAMLMap();
    doc.set("volumes", vols);
  }
  let changed = false;
  for (const mount of mounts) {
    const src = String(mount).split(":")[0].trim();
    if (!src || src.startsWith("/") || src.startsWith("./") || src.startsWith("../")) continue;
    if (vols.items.some((p) => String(p.key) === src)) continue;
    // 空对象而非 null：yaml 库会把 null 序列化成 "? key" 显式键形式，可读性差
    vols.set(src, doc.createNode({}));
    changed = true;
  }
  return changed ? doc.toString({ lineWidth: 0 }) : text;
}

/** 顶层网络条目（上游 NetworkInput 的内部/外部二分）。 */
interface NetworkEntry {
  name: string;
  external: boolean;
}

/** 读取顶层网络条目及其 external 标记。 */
export function listTopLevelNetworkEntries(text: string): NetworkEntry[] {
  const doc = parse(text);
  const node = doc && doc.get("networks", true);
  if (!doc || !isMap(node)) return [];
  return node.items.map((pair) => {
    const value = pair.value;
    const external = isMap(value) && String(value.get("external", true)) === "true";
    return { name: String(pair.key), external };
  });
}

/** 整体覆写顶层网络条目（空名跳过；空列表删除 networks 段）。 */
export function setTopLevelNetworkEntries(text: string, entries: NetworkEntry[]): string {
  const doc = parse(text);
  if (!doc) return text;
  const valid = entries.filter((e) => e.name.trim() !== "");
  if (valid.length === 0) {
    doc.delete("networks");
  } else {
    const value: Record<string, { external: true } | {}> = {};
    for (const e of valid) value[e.name] = e.external ? { external: true } : {};
    doc.set("networks", doc.createNode(value));
  }
  return doc.toString({ lineWidth: 0 });
}

/** 确保引用的网络都在顶层 networks 有定义（编辑表单选择网络的闭环写回）：
 *  未定义的名字补上——在本机网络集合（externalSet）中的标记 external: true，
 *  其余补普通定义；已有定义一律不动（尊重用户手写）。全部已定义时原样返回。 */
export function ensureTopLevelNetworks(text: string, names: string[], externalSet: Set<string>): string {
  const entries = listTopLevelNetworkEntries(text);
  const defined = new Set(entries.map((e) => e.name));
  let changed = false;
  for (const name of names) {
    if (!name || defined.has(name)) continue;
    entries.push({ name, external: externalSet.has(name) });
    defined.add(name);
    changed = true;
  }
  return changed ? setTopLevelNetworkEntries(text, entries) : text;
}

/** 顶层网络改名并同步替换全部服务引用——定义与引用必须一致，
 *  否则改名后留下未定义引用、部署被校验拦截。 */
export function renameTopLevelNetwork(text: string, oldName: string, newName: string): string {
  if (!oldName || !newName) return text;
  const out = mapNetworkRefs(text, (name) => (name === oldName ? newName : name));
  const entries = listTopLevelNetworkEntries(out);
  if (out === text && !entries.some((e) => e.name === oldName)) return text;
  return setTopLevelNetworkEntries(out, entries.map((e) => (e.name === oldName ? { ...e, name: newName } : e)));
}

/** 删除顶层网络并同步移除全部服务引用（服务 networks 清空则删该键）；
 *  长语法引用（对象项）不动，交由未定义网络校验提示。 */
export function removeTopLevelNetwork(text: string, name: string): string {
  if (!name) return text;
  const out = mapNetworkRefs(text, (ref) => (ref === name ? null : ref));
  const entries = listTopLevelNetworkEntries(out);
  if (out === text && !entries.some((e) => e.name === name)) return text;
  return setTopLevelNetworkEntries(out, entries.filter((e) => e.name !== name));
}

/** 对全部服务的 networks 引用做替换/删除（fn 返回 null 表示删除）；
 *  兼容短语法列表与字典写法；含长语法对象项的服务整体跳过（不动）。 */
function mapNetworkRefs(text: string, fn: (name: string) => string | null): string {
  const doc = parse(text);
  if (!doc) return text;
  const services = servicesMap(doc);
  if (!services) return text;
  let changed = false;
  for (const pair of services.items) {
    const svc = pair.value;
    if (!isMap(svc)) continue;
    const refs = svc.get("networks", true);
    if (isSeq(refs)) {
      if (refs.items.some((item) => isMap(item))) continue; // 长语法整体不动
      const next: string[] = [];
      for (const item of refs.items) {
        const mapped = fn(String(item));
        if (mapped) next.push(mapped);
      }
      if (next.length === refs.items.length && next.every((v, i) => v === String(refs.items[i]))) continue;
      changed = true;
      if (next.length === 0) svc.delete("networks");
      else svc.set("networks", doc.createNode(next));
    } else if (isMap(refs)) {
      const kept = new Map<string, unknown>();
      for (const p of refs.items) {
        const mapped = fn(String(p.key));
        if (mapped) kept.set(mapped, p.value);
      }
      if (kept.size === refs.items.length && [...refs.items].every((p, i) => String(p.key) === [...kept.keys()][i])) continue;
      changed = true;
      if (kept.size === 0) svc.delete("networks");
      else {
        const node = new YAMLMap();
        for (const [key, value] of kept) node.set(key, value as never);
        svc.set("networks", node);
      }
    }
  }
  return changed ? doc.toString({ lineWidth: 0 }) : text;
}

