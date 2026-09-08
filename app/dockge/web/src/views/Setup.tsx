// 首次安装页：创建管理员账号后自动登录。
import { createSignal, Show } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { Rocket } from "lucide-solid";
import { setup } from "../store/index";
import { errText } from "../api/format";

export function Setup() {
  const navigate = useNavigate();
  const [username, setUsername] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [confirmPwd, setConfirmPwd] = createSignal("");
  const [error, setError] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  const strength = () => {
    const p = password();
    if (p.length >= 12 && /[^a-zA-Z0-9]/.test(p)) return { label: "强", ok: true };
    if (p.length >= 8) return { label: "中", ok: true };
    if (p.length >= 6) return { label: "弱", ok: false };
    return null;
  };

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (busy()) return;
    if (password() !== confirmPwd()) {
      setError("两次输入的密码不一致");
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
          <div class="auth-title">初始化 Dockge</div>
          <div class="auth-sub">创建首个管理员账号</div>
        </div>
        <Show when={error()}>
          <div class="form-error">{error()}</div>
        </Show>
        <div class="form-group">
          <label class="form-label" for="setup-username">
            用户名
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
            密码（至少 6 位）
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
            <p class="form-help">密码强度：{strength()!.label}</p>
          </Show>
        </div>
        <div class="form-group">
          <label class="form-label" for="setup-confirm">
            确认密码
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
          {busy() ? "创建中…" : "创建账号并进入"}
        </button>
      </form>
    </div>
  );
}
