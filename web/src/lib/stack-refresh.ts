import type { Stack } from './data';

// 在请求完成时合并当前草稿，而非使用请求开始时的旧快照。
export function mergeStackRefresh(incoming: Stack, current?: Stack, before?: Stack): Stack {
  if (!current) return incoming;
  const editing = !!incoming.loadError || current.busy || current.compose !== current.savedCompose || current.env !== current.savedEnv || !!before && (current.compose !== before.compose || current.savedCompose !== before.savedCompose || current.env !== before.env);
  return {
    ...incoming,
    ...(editing ? { compose: current.compose, savedCompose: current.savedCompose, env: current.env, savedEnv: current.savedEnv } : {}),
    busy: current.busy,
    deployLog: current.deployLog,
  };
}

// 每轮只建立一次索引，避免逐栈扫描旧列表；草稿规则仍由单栈合并统一处理。
export function mergeStackListRefresh(incoming: Stack[], current: readonly Stack[]): Stack[] {
  const byId = new Map<string, Stack>();
  for (const stack of current) {
    if (!byId.has(stack.id)) byId.set(stack.id, stack);
  }
  return incoming.map(stack => mergeStackRefresh(stack, byId.get(stack.id)));
}
