// 服务器与运行时是两个数据来源；只消费契约字段，不展开运行时原始属性。
export interface HostMemory { total:number; used:number; available:number; percent:number }
export interface HostDisk extends HostMemory { path:string }
export interface HostSystem {
  source:string; sampledAt:string; hostname:string; os:string; kernel:string; arch:string;
  cpuModel:string; cpuCount:number|null; cpuPercent:number|null;
  memory:HostMemory|null; swap:HostMemory|null; disk:HostDisk|null;
  load:number[]|null; uptime:number|null; errors:Record<string,string>;
}
export function percentText(value:number|null|undefined):string|null {
  return value != null && Number.isFinite(value) && value >= 0 && value <= 100 ? value.toFixed(1) + '%' : null;
}
export function uptimeParts(value:number|null|undefined):{days:number;hours:number;minutes:number}|null {
  if (value == null || !Number.isFinite(value) || value < 0) return null;
  return {days:Math.floor(value/86400),hours:Math.floor(value%86400/3600),minutes:Math.floor(value%3600/60)};
}
// 加载失败不是空列表；服务器卡与运行时数量都必须保留未知状态。
export function resourceKnown(loading:boolean, errors:Record<string,string>, kind:string):boolean {
  return !loading && !errors.request && !errors.resources && !errors[kind];
}
export function systemInfoRows(host:HostSystem|null, info:Record<string,string>, runtime:string):[string,string|null][] {
  return [
    ['sysinfo.hostname',host?.hostname || null], ['sysinfo.os',host?.os || null],
    ['sysinfo.kernel',host?.kernel || null], ['sysinfo.arch',host?.arch || null],
    ['sysinfo.cpuModel',host?.cpuModel || null], ['sysinfo.cpus',host?.cpuCount != null ? String(host.cpuCount) : null],
    ['sysinfo.totalMemory',host?.memory ? String(host.memory.total) : null],
    ['sysinfo.runtime',runtime || null], ['sysinfo.runtimeVersion',info.ServerVersion || null]
  ];
}
