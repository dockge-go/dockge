// 设置页：账号、全局环境变量、宿主终端开关与版本信息。
import { Show, createResource, createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { ExternalLink, KeyRound, LogOut, RefreshCw, Save } from "lucide-solid";
import { api } from "../api/api";
import { logout, toast, user } from "../store/index";
import { confirmDialog } from "../components/Confirm";
import { errText } from "../api/format";
import { t } from "../i18n";

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
      toast(t("toast.passwordMismatch"), "error");
      return;
    }
    setPwdBusy(true);
    try {
      await api.changePassword(oldPwd(), newPwd());
      toast(t("toast.passwordChanged"), "success");
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
      toast(t("toast.envSaved"), "success");
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
          ? t("set.versionUpdate", { current: r.currentVersion, latest: r.latestVersion })
          : t("set.versionCurrent", { current: r.currentVersion }),
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
            {t("set.account")}
          </h3>
          <p class="text-dim" style={{ "margin-top": 0 }}>
            {t("set.currentUser", { username: user()?.username ?? "—" })}
          </p>
          <form onSubmit={changePassword}>
            <div class="form-row">
              <div class="form-group">
                <label class="form-label">{t("set.currentPassword")}</label>
                <input class="form-input" type="password" autocomplete="current-password" value={oldPwd()} onInput={(e) => setOldPwd(e.currentTarget.value)} />
              </div>
              <div class="form-group">
                <label class="form-label">{t("set.newPassword")}</label>
                <input class="form-input" type="password" autocomplete="new-password" value={newPwd()} onInput={(e) => setNewPwd(e.currentTarget.value)} />
              </div>
            </div>
            <div class="form-group">
              <label class="form-label">{t("set.confirmNewPassword")}</label>
              <input class="form-input" type="password" autocomplete="new-password" value={confirmPwd()} onInput={(e) => setConfirmPwd(e.currentTarget.value)} />
            </div>
            <button class="btn btn-primary" type="submit" disabled={pwdBusy() || !oldPwd() || newPwd().length < 6 || newPwd() !== confirmPwd()}>
              {t("set.changePassword")}
            </button>
          </form>
          <div style={{ "margin-top": "20px", "border-top": "1px solid var(--border-soft)", "padding-top": "16px" }}>
            <button
              class="btn btn-secondary"
              onClick={async () => {
                if (!(await confirmDialog(t("set.confirmLogout"), t("set.confirmLogoutDesc"), false))) return;
                logout();
                navigate("/login", { replace: true });
              }}
            >
              <LogOut size={14} /> {t("set.logout")}
            </button>
          </div>
        </div>

        {/* 全局环境变量 */}
        <div class="setting-card">
          <h3>{t("set.globalEnv")}</h3>
          <p class="text-dim" style={{ "margin-top": 0 }} innerHTML={t("set.globalEnvDesc")} />
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
            <Save size={14} /> {t("set.saveEnv")}
          </button>
        </div>

        {/* 关于 */}
        <div class="setting-card" style={{ background: "var(--surface)", border: "1px solid var(--border-soft)" }}>
          <h3>{t("set.about")}</h3>
          <div class="text-dim" style={{ "line-height": 1.8 }}>
            <div><strong>Dockge</strong> — Container Management</div>
            <div>{t("set.aboutDesc")}</div>
            <div style={{ color: "var(--muted)", "margin-top": "8px" }}>{t("set.aboutInspiration")}</div>
          </div>
          <div style={{ display: "flex", "align-items": "center", gap: "12px", "margin-top": "16px", "flex-wrap": "wrap" }}>
            <button class="btn btn-secondary" onClick={() => void checkVersion()}>
              <RefreshCw size={14} /> {t("set.checkUpdate")}
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
