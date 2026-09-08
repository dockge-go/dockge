// 设置页：账号、全局环境变量、宿主终端开关与版本信息。
import { Show, createResource, createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { ExternalLink, KeyRound, LogOut, RefreshCw, Save } from "lucide-solid";
import { api } from "../api/api";
import { logout, toast, user } from "../store/index";
import { confirmDialog } from "../components/Confirm";
import { errText } from "../api/format";

export function Settings() {
  const navigate = useNavigate();

  // ---- 改密 ----
  const [oldPwd, setOldPwd] = createSignal("");
  const [newPwd, setNewPwd] = createSignal("");
  const [confirmPwd, setConfirmPwd] = createSignal("");
  const [pwdBusy, setPwdBusy] = createSignal(false);

  const changePassword = async (e: SubmitEvent) => {
    e.preventDefault();
    if (newPwd() !== confirmPwd()) {
      toast("两次输入的新密码不一致", "error");
      return;
    }
    setPwdBusy(true);
    try {
      await api.changePassword(oldPwd(), newPwd());
      toast("密码已修改", "success");
      setOldPwd("");
      setNewPwd("");
      setConfirmPwd("");
    } catch (err) {
      toast(errText(err), "error");
    } finally {
      setPwdBusy(false);
    }
  };

  // ---- 全局环境变量 ----
  const [envText, setEnvText] = createSignal("");
  const [envDirty, setEnvDirty] = createSignal(false);
  const envResource = createResource(async () => {
    try {
      const r = await api.globalEnv();
      setEnvText(r.globalENV);
      setEnvDirty(false);
    } catch (e) {
      toast(errText(e), "error");
    }
  });
  void envResource[0];

  const saveEnv = async () => {
    try {
      await api.setGlobalEnv(envText());
      setEnvDirty(false);
      toast("全局环境变量已保存", "success");
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  // ---- 版本检查 ----
  const [versionInfo, setVersionInfo] = createSignal<string>("");

  const checkVersion = async () => {
    try {
      const r = await api.versionCheck();
      setVersionInfo(
        r.hasUpdate
          ? `当前 ${r.currentVersion} · 最新 ${r.latestVersion}（有更新可用）`
          : `当前 ${r.currentVersion} · 已是最新`,
      );
    } catch (e) {
      toast(errText(e), "error");
    }
  };

  return (
    <div class="view-section">
      <h2 class="section-title" style={{ "margin-bottom": "24px" }}>Settings</h2>
      <div class="settings-col">
        {/* 账号 */}
        <div class="setting-card">
          <h3>
            <KeyRound size={15} style={{ "vertical-align": "-2px", "margin-right": "6px" }} />
            账号
          </h3>
          <p class="text-dim" style={{ "margin-top": 0 }}>
            当前用户：<strong>{user()?.username ?? "—"}</strong>
          </p>
          <form onSubmit={changePassword}>
            <div class="form-row">
              <div class="form-group">
                <label class="form-label">当前密码</label>
                <input class="form-input" type="password" autocomplete="current-password" value={oldPwd()} onInput={(e) => setOldPwd(e.currentTarget.value)} />
              </div>
              <div class="form-group">
                <label class="form-label">新密码（至少 6 位）</label>
                <input class="form-input" type="password" autocomplete="new-password" value={newPwd()} onInput={(e) => setNewPwd(e.currentTarget.value)} />
              </div>
            </div>
            <div class="form-group">
              <label class="form-label">确认新密码</label>
              <input class="form-input" type="password" autocomplete="new-password" value={confirmPwd()} onInput={(e) => setConfirmPwd(e.currentTarget.value)} />
            </div>
            <button class="btn btn-primary" type="submit" disabled={pwdBusy() || !oldPwd() || newPwd().length < 6 || newPwd() !== confirmPwd()}>
              修改密码
            </button>
          </form>
          <div style={{ "margin-top": "20px", "border-top": "1px solid var(--border-soft)", "padding-top": "16px" }}>
            <button
              class="btn btn-secondary"
              onClick={async () => {
                if (!(await confirmDialog("退出登录", "确定退出当前会话？", false))) return;
                logout();
                navigate("/login", { replace: true });
              }}
            >
              <LogOut size={14} /> 退出登录
            </button>
          </div>
        </div>

        {/* 全局环境变量 */}
        <div class="setting-card">
          <h3>全局环境变量（global.env）</h3>
          <p class="text-dim" style={{ "margin-top": 0 }}>
            部署栈时自动注入到 compose 环境，可用 <span class="inline-code">${"{}"}</span> 语法在 YAML 中引用。
          </p>
          <textarea
            class="form-textarea"
            spellcheck={false}
            value={envText()}
            onInput={(e) => {
              setEnvText(e.currentTarget.value);
              setEnvDirty(true);
            }}
          />
          <button class="btn btn-primary" style={{ "margin-top": "12px" }} disabled={!envDirty()} onClick={() => void saveEnv()}>
            <Save size={14} /> 保存
          </button>
        </div>

        {/* 关于 */}
        <div class="setting-card" style={{ background: "var(--surface)", border: "1px solid var(--border-soft)" }}>
          <h3>关于</h3>
          <div class="text-dim" style={{ "line-height": 1.8 }}>
            <div><strong>Dockge</strong> — Container Management</div>
            <div>Go 后端 · SolidJS 前端 · Apple 设计语言</div>
            <div style={{ color: "var(--muted)", "margin-top": "8px" }}>灵感来自 Podman Desktop 与 Dockge</div>
          </div>
          <div style={{ display: "flex", "align-items": "center", gap: "12px", "margin-top": "16px", "flex-wrap": "wrap" }}>
            <button class="btn btn-secondary" onClick={() => void checkVersion()}>
              <RefreshCw size={14} /> 检查更新
            </button>
            <Show when={versionInfo()}>
              <span class="text-dim">{versionInfo()}</span>
            </Show>
            <a class="btn btn-ghost" href="https://github.com/louislam/dockge" target="_blank" rel="noreferrer">
              <ExternalLink size={14} /> GitHub
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}
