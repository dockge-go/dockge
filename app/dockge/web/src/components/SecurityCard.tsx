// 账号安全卡：修改密码与登出。
// （2FA/TOTP 已按 2026-09-12 决策整体移除——功能冗余，见 REQUIREMENTS Q7。）
import { createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { KeyRound, LogOut } from "lucide-solid";
import { api } from "../api/api";
import { confirmDialog } from "./Confirm";
import { logout, toast } from "../store/index";
import { errText } from "../api/format";
import { t } from "../i18n";

export function SecurityCard() {
  const navigate = useNavigate();

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

  return (
    <div class="setting-card">
      <h3>
        <KeyRound size={15} style={{ "vertical-align": "-2px", "margin-right": "6px" }} />
        {t("sec.title")}
      </h3>
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
  );
}
