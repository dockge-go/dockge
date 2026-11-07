/* 运行时 inspect/stats 与栈文件的展示契约。 */
export interface PortMap {
  PrivatePort?: number; PublicPort?: number;
  Type?: string; IP?: string; Proto?: 'tcp' | 'udp';
}
export interface MountMap { source: string; dest: string; mode: string; type?: string; name?: string }
export type RestartPolicy = 'no' | 'always' | 'unless-stopped' | 'on-failure';

export interface Container {
  Id: string; Names: string[]; Image: string; State: string; ImageID: string;
  upHours?: number; exitCode?: number; exitMinAgo?: number;
  Created: number; Ports: PortMap[];
  system: boolean; _stats?: { cpuPct: number; cpuBar: number; memMB: number; memBar: number; netIO: string; blockIO: string };
  /* docker run 参数面 */
  Env?: string[];
  Mounts?: MountMap[];
  Networks?: string[]; NetworkIPs?: Record<string, string>; Labels?: Record<string, string>;
  RestartPolicy?: RestartPolicy;
  RestartCount?: number;
  Health?: 'starting' | 'healthy' | 'unhealthy';
  Cpus?: number;
  MemoryMB?: number;
  Entrypoint?: string;
  Cmd?: string;
}
export interface Image { Id: string; RepoTags: string[]; Repository: string; Size: number; Created: number; system: boolean; InUse: boolean; UsedBy?: {id:string; name:string}[]; RepoDigests?: string[] }
export interface Volume { Name: string; Driver: string; Mountpoint: string; Usage: string; system: boolean; UsedBy?: { id: string; name: string }[] }
export interface Network { ID: string; Name: string; Driver: string; Subnet: string; Containers?: { name: string; ip: string }[]; system: boolean }
export interface Stack {
  id: string; name: string; compose_path: string; status: 'up' | 'down' | 'unknown';
  compose: string; deployLog: string[]; managed: boolean; env: string; savedEnv: string; savedCompose: string; busy: boolean; loadError: string; runtimeError?: string;
}

export const defaultCompose = () => 'services:\n  app:\n    image: nginx:latest\n    ports:\n      - \"8080:80\"';
