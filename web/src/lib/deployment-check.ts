// 部署自检（纯函数，无 DOM）：把 /v1/health 的运行时状态映射为「问题列表」。
// MsgKey 仅作类型导入，运行时不加载 i18n 模块（node --test 下无 document）。
import type { MsgKey } from "../i18n";

export type HealthRuntime = {
  cli: string;
  compose: string;
  ready: boolean;
  error?: string;
  stacks: { path: string; mounted: boolean };
};

export type DeploymentProblem = { key: MsgKey; vars?: Record<string, string> };

export function deploymentProblems(runtime: HealthRuntime | undefined): DeploymentProblem[] {
  if (!runtime) return [];
  const problems: DeploymentProblem[] = [];
  if (!runtime.ready) {
    problems.push({ key: "deploy.runtimeFail", vars: { error: runtime.error ?? "" } });
  }
  if (!runtime.stacks.mounted) {
    problems.push({ key: "deploy.stacksFail", vars: { path: runtime.stacks.path } });
  }
  return problems;
}