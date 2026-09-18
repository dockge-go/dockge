// 登录页：用户名密码 → 会话建立 → 仪表盘；
// OIDC/proxy 模式按 /v1/auth/config 渲染对应入口；未安装时引导至 /setup。
import { createSignal, For, onMount, Show } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { LogIn } from "lucide-solid";
import { api, getToken, type AuthConfig } from "../api/api";
import { login } from "../store/index";
import { errText } from "../api/format";
import { t } from "../i18n";

export function Login() {
  const navigate = useNavigate();
  const [username, setUsername] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [error, setError] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [authConfig, setAuthConfig] = createSignal<AuthConfig | null>(null);

  onMount(async () => {
    if (getToken()) {
      navigate("/", { replace: true });
      return;
    }
    try {
      const setupResult = await api.needSetup();
      if (setupResult.needSetup) {
        navigate("/setup", { replace: true });
        return;
      }
      const cfg = await api.authConfig();
      setAuthConfig(cfg);
    } catch {
      // 探测失败不阻塞登录表单
    }
    // OIDC 回调失败会 302 回本页并携带 oidc_error，展示后清理地址栏
    const oidcError = new URLSearchParams(location.search).get("oidc_error");
    if (oidcError) {
      setError(t("login.oidcFailed", { msg: oidcError }));
      history.replaceState(null, "", "/login");
    }
  });

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (busy()) return;
    setBusy(true);
    setError("");
    try {
      await login(username().trim(), password());
      navigate("/", { replace: true });
    } catch (err) {
      setError(errText(err));
    } finally {
      setBusy(false);
    }
  };

  const cfg = authConfig();
  const isProxy = cfg?.mode === "proxy";
  const isOIDC = cfg?.mode === "oidc" && cfg.providers.length > 0;

  return (
    <div class="auth-wrap">
      <form class="auth-card" onSubmit={submit}>
        <div class="auth-brand">
          <svg
            width="44"
            height="44"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <rect x="2" y="3" width="20" height="14" rx="2" />
            <path d="M8 21h8M12 17v4" />
            <path d="M7 8h2m2 0h2m2 0h2M7 11h10" />
          </svg>
          <div class="auth-title">Dockge</div>
          <div class="auth-sub">{t("login.title")}</div>
        </div>
        <Show when={error()}>
          <div class="form-error">{error()}</div>
        </Show>

        <Show when={isProxy}>
          <button
            class="btn btn-primary"
            style={{ width: "100%", "justify-content": "center", "margin-bottom": "12px" }}
            onClick={() => {
              // 整页加载而非 SPA 跳转：让请求重新经过 Traefik → ProxyAuth 中间件链，
              // SPA 内跳转只会重复已失败的 boot()，形成登录页死循环。
              window.location.href = "/";
            }}
          >
            <LogIn size={14} />
            {t("login.ssoLogin")}
          </button>
          <div style={{ "text-align": "center" }}>
            <span class="text-dim" style={{ "font-size": "var(--text-xs)" }}>
              {t("login.proxyHint")}
            </span>
          </div>
        </Show>

        <Show when={!isProxy && !isOIDC}>
          <div class="form-group">
            <label class="form-label" for="login-username">{t("form.username")}</label>
            <input
              id="login-username"
              class="form-input"
              autocomplete="username"
              value={username()}
              onInput={(e) => setUsername(e.currentTarget.value)}
            />
          </div>
          <div class="form-group">
            <label class="form-label" for="login-password">{t("form.password")}</label>
            <input
              id="login-password"
              class="form-input"
              type="password"
              autocomplete="current-password"
              value={password()}
              onInput={(e) => setPassword(e.currentTarget.value)}
            />
          </div>
          <button
            class="btn btn-primary"
            type="submit"
            disabled={busy() || !username() || !password()}
            style={{ width: "100%", "justify-content": "center" }}
          >
            <LogIn size={14} />
            {busy() ? t("login.loggingIn") : t("login.login")}
          </button>
        </Show>

        <Show when={isOIDC}>
          <div style={{ display: "flex", "flex-direction": "column", gap: "8px", "margin-bottom": "12px" }}>
            <For each={cfg!.providers}>
              {(p) => (
                <button
                  class="btn btn-secondary"
                  type="button"
                  onClick={() => {
                    window.location.href = `/v1/oidc/${encodeURIComponent(p.id)}/auth`;
                  }}
                >
                  <LogIn size={14} />
                  {p.info.label}
                </button>
              )}
            </For>
          </div>
          <div style={{ "text-align": "center", "margin-bottom": "8px" }}>
            <span class="text-dim" style={{ "font-size": "var(--text-xs)" }}>{t("login.orPassword")}</span>
          </div>
          <div style={{ display: "flex", gap: "8px" }}>
            <input
              class="form-input"
              placeholder={t("form.usernamePlaceholder")}
              autocomplete="username"
              value={username()}
              onInput={(e) => setUsername(e.currentTarget.value)}
              style={{ flex: 1 }}
            />
            <input
              class="form-input"
              placeholder={t("form.passwordPlaceholder")}
              type="password"
              autocomplete="current-password"
              value={password()}
              onInput={(e) => setPassword(e.currentTarget.value)}
              style={{ flex: 1 }}
            />
            <button
              class="btn btn-primary"
              type="submit"
              disabled={busy() || !username() || !password()}
            >
              <LogIn size={14} />
            </button>
          </div>
        </Show>
      </form>
    </div>
  );
}
