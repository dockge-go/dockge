// 部署自检（共享组件）：读 /v1/health（免鉴权），把运行时问题渲染为只读提示。
// 请求失败静默（不 toast）；无问题时仅在 always 模式渲染一行紧凑状态。
import { createSignal, For, onMount, Show } from "solid-js";

import { api } from "../api/api";
import { t, type MsgKey } from "../i18n";
import { deploymentProblems, type HealthRuntime } from "../lib/deployment-check";

// 问题 key → 修复提示 key（deploymentProblems 只会产出这两种）
function hintKeyOf(key: MsgKey): MsgKey {
  return key === "deploy.runtimeFail" ? "deploy.runtimeHint" : "deploy.stacksHint";
}

export function DeploymentCheck(props: { always?: boolean }) {
  const [runtime, setRuntime] = createSignal<HealthRuntime>();

  onMount(() => {
    api.health()
      .then((r) => setRuntime(r.runtime))
      .catch(() => {
        // 自检失败静默：只读提示不可用时不打扰（尤其 Setup 页）
      });
  });

  const problems = () => deploymentProblems(runtime());

  return (
    <Show
      when={problems().length > 0}
      fallback={
        <Show when={props.always && runtime()}>
          {(r) => (
            <div>
              <div class="settings-label">{t("deploy.title")}</div>
              <div style={{ display: "flex", gap: "8px", "align-items": "center", "margin-top": "6px" }}>
                <span class="status-pill active">{t("deploy.runtimeOk")}</span>
                <span class="form-help mono" style={{ margin: 0 }}>
                  {t("deploy.cli")}: {r().cli} · {t("deploy.stacksOk", { path: r().stacks.path })}
                </span>
              </div>
            </div>
          )}
        </Show>
      }
    >
      <div class="auth-error" role="alert" style={{ "text-align": "left" }}>
        <For each={problems()}>
          {(p, i) => (
            <div style={i() > 0 ? { "margin-top": "8px" } : undefined}>
              <div>{t(p.key, p.vars)}</div>
              <p class="form-help" style={{ "margin-top": "2px" }}>{t(hintKeyOf(p.key))}</p>
            </div>
          )}
        </For>
      </div>
    </Show>
  );
}
