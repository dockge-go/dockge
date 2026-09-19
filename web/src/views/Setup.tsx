// Setup 页（上游复刻）：首次初始化，创建唯一 admin；含语言下拉。
// 密码不匹配经 toast 提示（上游行为），不做前端强度/长度门槛。
import { createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";

import { errText } from "../api/format";
import { t, setLocale, useLocale, type LocaleKey } from "../i18n";
import { setup, toast } from "../store/index";
import { Toaster } from "../components/Toaster";
import { DeploymentCheck } from "../components/DeploymentCheck";

const LANG_OPTIONS: Array<{ value: LocaleKey; label: string }> = [
  { value: "zh-CN", label: "简体中文" },
  { value: "en-US", label: "English" },
];

export function Setup() {
  const navigate = useNavigate();
  const [username, setUsername] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [repeat, setRepeat] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  const submit = async (event: SubmitEvent) => {
    event.preventDefault();
    if (busy()) return;
    if (password() !== repeat()) {
      toast(t("setup.passwordsNoMatch"), "error");
      return;
    }
    setBusy(true);
    try {
      await setup(username().trim(), password());
      navigate("/", { replace: true });
    } catch (e) {
      toast(errText(e), "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div class="auth-center">
      <form class="card auth-card" onSubmit={submit}>
        <img class="auth-logo" src="/icon.svg" alt="Dockge" />
        <div class="auth-title">{t("setup.welcome")}</div>
        <p class="auth-sub">{t("setup.title")}</p>
        <DeploymentCheck />
        <div class="auth-lang">
          <select value={useLocale()} onChange={(e) => setLocale(e.currentTarget.value as LocaleKey)} aria-label={t("common.language")}>
            {LANG_OPTIONS.map((opt) => (
              <option value={opt.value}>{opt.label}</option>
            ))}
          </select>
        </div>
        <div class="form-floating">
          <input id="setup-username" value={username()} placeholder=" " autocomplete="username" onInput={(e) => setUsername(e.currentTarget.value)} />
          <label for="setup-username">{t("setup.username")}</label>
        </div>
        <div class="form-floating">
          <input id="setup-password" type="password" value={password()} placeholder=" " autocomplete="new-password" onInput={(e) => setPassword(e.currentTarget.value)} />
          <label for="setup-password">{t("setup.password")}</label>
        </div>
        <div class="form-floating">
          <input id="setup-repeat" type="password" value={repeat()} placeholder=" " autocomplete="new-password" onInput={(e) => setRepeat(e.currentTarget.value)} />
          <label for="setup-repeat">{t("setup.repeat")}</label>
        </div>
        <button class="btn btn-primary" type="submit" disabled={busy() || !username().trim() || !password() || !repeat()} style={{ width: "100%", "justify-content": "center" }}>
          {t("setup.submit")}
        </button>
      </form>
      <Toaster />
    </div>
  );
}
