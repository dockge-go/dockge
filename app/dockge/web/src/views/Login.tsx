// 登录页：用户名密码 → JWT 入库 → 跳转仪表盘；未安装时引导至 /setup。
import { createSignal, onMount, Show } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { LogIn } from "lucide-solid";
import { api, getToken } from "../api/api";
import { login } from "../store/index";
import { errText } from "../api/format";

export function Login() {
  const navigate = useNavigate();
  const [username, setUsername] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [error, setError] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  onMount(async () => {
    if (getToken()) {
      navigate("/", { replace: true });
      return;
    }
    try {
      const r = await api.needSetup();
      if (r.needSetup) navigate("/setup", { replace: true });
    } catch {
      // 探测失败不阻塞登录表单
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
          <div class="auth-sub">登录到容器管理控制台</div>
        </div>
        <Show when={error()}>
          <div class="form-error">{error()}</div>
        </Show>
        <div class="form-group">
          <label class="form-label" for="login-username">
            用户名
          </label>
          <input
            id="login-username"
            class="form-input"
            autocomplete="username"
            value={username()}
            onInput={(e) => setUsername(e.currentTarget.value)}
          />
        </div>
        <div class="form-group">
          <label class="form-label" for="login-password">
            密码
          </label>
          <input
            id="login-password"
            class="form-input"
            type="password"
            autocomplete="current-password"
            value={password()}
            onInput={(e) => setPassword(e.currentTarget.value)}
          />
        </div>
        <button class="btn btn-primary" type="submit" disabled={busy() || !username() || !password()} style={{ width: "100%", "justify-content": "center" }}>
          <LogIn size={14} />
          {busy() ? "登录中…" : "登录"}
        </button>
      </form>
    </div>
  );
}
