// 设置页：账号安全（改密/2FA/登出）、用户管理（admin）、全局环境变量与版本信息。
import { createResource, createSignal, Show } from "solid-js";
import { RefreshCw, Save } from "lucide-solid";
import { api } from "../api/api";
import { toast, user } from "../store/index";
import { SecurityCard } from "../components/SecurityCard";
import { UsersCard } from "../components/UsersCard";
import { errText } from "../api/format";
import { t } from "../i18n";

export function Settings() {
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
      <h2 class="section-title" style={{ "margin-bottom": "24px" }}>{t("view.settings")}</h2>
      <div class="settings-col">
        <SecurityCard />

        <Show when={user()?.role === "admin"}>
          <UsersCard />
        </Show>

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
          </div>
        </div>
      </div>
    </div>
  );
}
