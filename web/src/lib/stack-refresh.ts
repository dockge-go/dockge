import type { Stack } from './data';

// 在请求完成时合并当前草稿，而非使用请求开始时的旧快照。
export function mergeStackRefresh(incoming: Stack, current?: Stack, before?: Stack): Stack {
  if (!current) return incoming;
  const editing = current.busy || current.compose !== current.savedCompose || !!before && (current.compose !== before.compose || current.savedCompose !== before.savedCompose || current.env !== before.env);
  return {
    ...incoming,
    ...(editing ? { compose: current.compose, savedCompose: current.savedCompose, env: current.env } : {}),
    busy: current.busy,
    deployLog: current.deployLog,
    composeDeployed: current.composeDeployed,
  };
}
