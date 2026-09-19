// compose.yaml 结构化编辑：基于 yaml Document API 的服务/网络增删改。
// 走 Document 节点树而非 parse→stringify，注释在往返中原生保留
// （优于上游 copyYAMLComments 的行匹配恢复）。所有操作对非法 YAML 返回原文本。
import { isMap, isSeq, parseDocument, type Document, type YAMLMap } from "yaml";

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

/** 指定服务字段是否为「长语法」（数组项含对象）——上游此时提示改用 YAML 编辑器。 */
export function hasLongSyntax(text: string, name: string, key: keyof ServiceFields): string | undefined {
  const doc = parse(text);
  const services = doc && servicesMap(doc);
  const node = services ? services.get(name, true) : undefined;
  if (!doc || !isMap(node)) return undefined;
  const value = node.get(serviceYamlKeys[key], true);
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
    const value: Record<string, null | { external: true }> = {};
    for (const e of valid) value[e.name] = e.external ? { external: true } : null;
    doc.set("networks", doc.createNode(value));
  }
  return doc.toString({ lineWidth: 0 });
}

