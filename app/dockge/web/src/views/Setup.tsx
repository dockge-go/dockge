// 首次安装页：创建管理员账号后自动登录。
import { createSignal, Show } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { Rocket } from "lucide-solid";
import { setup } from "../store/index";
import { errText } from "../api/format";
import { t } from "../i18n";

export function Setup() {
  const navigate = useNavigate();
  const [username, setUsername] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [confirmPwd, setConfirmPwd] = createSignal("");
  const [error, setError] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  const strength = () => {
    const p = password();
    if (p.length >= 12 && /[^a-zA-Z0-9]/.test(p)) return { label: t("setup.strengthStrong"), ok: true };
    if (p.length >= 8) return { label: t("setup.strengthMedium"), ok: true };
    if (p.length >= 6) return { label: t("setup.strengthWeak"), ok: false };
    return null;
  };

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (busy()) return;
    if (password() !== confirmPwd()) {
      setError(t("setup.passwordMismatch"));
      return;
    }
    setBusy(true);
    setError("");
    try {
      await setup(username().trim(), password());
      navigate("/", { replace: true });
    } catch (err) {
      setError(errText(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div class="auth-wrap">
      <form class="auth-card" onSubmit={submit}>
        <div class="auth-brand">
          <Rocket size={40} />
          <div class="auth-title">{t("setup.initTitle")}</div>
          <div class="auth-sub">{t("setup.createAdmin")}</div>
        </div>
        <Show when={error()}>
          <div class="form-error">{error()}</div>
        </Show>
        <div class="form-group">
          <label class="form-label" for="setup-username">
            {t("form.username")}
          </label>
          <input
            id="setup-username"
            class="form-input"
            autocomplete="username"
            value={username()}
            onInput={(e) => setUsername(e.currentTarget.value)}
          />
        </div>
        <div class="form-group">
          <label class="form-label" for="setup-password">
            {t("setup.passwordHelp")}
          </label>
          <input
            id="setup-password"
            class="form-input"
            type="password"
            autocomplete="new-password"
            value={password()}
            onInput={(e) => setPassword(e.currentTarget.value)}
          />
          <Show when={strength()}>
            <p class="form-help">{t("setup.strength", { label: strength()!.label })}</p>
          </Show>
        </div>
        <div class="form-group">
          <label class="form-label" for="setup-confirm">
            {t("setup.confirmPassword")}
          </label>
          <input
            id="setup-confirm"
            class="form-input"
            type="password"
            autocomplete="new-password"
            value={confirmPwd()}
            onInput={(e) => setConfirmPwd(e.currentTarget.value)}
          />
        </div>
        <button
          class="btn btn-primary"
          type="submit"
          disabled={busy() || !username() || password().length < 6 || password() !== confirmPwd()}
          style={{ width: "100%", "justify-content": "center" }}
        >
          {busy() ? t("setup.creating") : t("setup.createAndEnter")}
        </button>
      </form>
    </div>
  );
}
