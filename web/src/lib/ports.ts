// 端口展示与链接（上游 parseDockerPort 等价物）：
// 宿主端口缺失时回落容器端口，443 用 https，其余 http，绝不产出 undefined。

export interface PortLike {
  hostPort?: number;
  containerPort: number;
}

/** 展示用端口：hostPort 缺失时回落 containerPort。 */
export function displayPort(port: PortLike): number {
  return port.hostPort ?? port.containerPort;
}

/** 端口链接：443 → https，其余 http；hostname 为空时由调用方回落 location.hostname。 */
export function portUrl(port: PortLike, hostname: string): string {
  const portNumber = displayPort(port);
  return `${portNumber === 443 ? "https" : "http"}://${hostname}:${portNumber}`;
}
