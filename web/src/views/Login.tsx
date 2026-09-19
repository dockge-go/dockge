// 登录页（上游复刻）：居中卡片、floating labels、Remember me、错误 alert。
import { Show, createSignal } from "solid-js";

import { errText } from "../api/format";
import { t } from "../i18n";
import { login } from "../store/index";

export function Login() {
  const [username, setUsername] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [remember, setRemember] = createSignal(true);
  const [error, setError] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  const submit = async (event: SubmitEvent) => {
    event.preventDefault();
    if (busy()) return;
    setBusy(true);
    setError("");
    try {
      await login(username().trim(), password(), remember());
    } catch (e) {
      setError(errText(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div class="auth-center">
      <form class="card auth-card" onSubmit={submit}>
        <img class="auth-logo" src="/icon.svg" alt="Dockge" />
        <div class="auth-title">Dockge</div>
        <p class="auth-sub">{t("login.title")}</p>
        <Show when={error()}>
          <div class="auth-error" role="alert">{error()}</div>
        </Show>
        <div class="form-floating">
          <input id="login-username" value={username()} placeholder=" " autocomplete="username" onInput={(e) => setUsername(e.currentTarget.value)} />
          <label for="login-username">{t("login.username")}</label>
        </div>
        <div class="form-floating">
          <input id="login-password" type="password" value={password()} placeholder=" " autocomplete="current-password" onInput={(e) => setPassword(e.currentTarget.value)} />
          <label for="login-password">{t("login.password")}</label>
        </div>
        <div class="auth-row">
          <label>
            <input type="checkbox" checked={remember()} onChange={(e) => setRemember(e.currentTarget.checked)} />
            {t("login.remember")}
          </label>
        </div>
        <button class="btn btn-primary" type="submit" disabled={busy() || !username().trim() || !password()} style={{ width: "100%", "justify-content": "center" }}>
          {t("login.submit")}
        </button>
      </form>
    </div>
  );
}
