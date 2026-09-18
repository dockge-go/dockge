// 用户管理卡（仅 admin）：账号列表（角色/状态/来源/2FA）+ 创建 Sheet +
// 行内操作（角色切换、启停、删除）。停用即时生效——对方的下一次请求即 401。
// 领域守卫（不能操作本人、至少保留一名活跃 admin）由后端裁决，错误原文 toast。
import { createResource, createSignal, For } from "solid-js";
import { Plus, ShieldCheck, UserPlus } from "lucide-solid";
import { api, type UserRow } from "../api/api";
import { confirmDialog } from "./Confirm";
import { Sheet } from "./Sheet";
import { toast } from "../store/index";
import { errText } from "../api/format";
import { t } from "../i18n";

export function UsersCard() {
  const [users, { refetch }] = createResource(async () => (await api.users()).list);
  const [createOpen, setCreateOpen] = createSignal(false);
  const [newName, setNewName] = createSignal("");
  const [newPwd, setNewPwd] = createSignal("");
  const [newRole, setNewRole] = createSignal("member");
  const [busy, setBusy] = createSignal(false);

  const act = async (fn: () => Promise<unknown>, successKey?: Parameters<typeof t>[0]) => {
    try {
      await fn();
      if (successKey) toast(t(successKey), "success");
      void refetch();
    } catch (err) {
      toast(errText(err), "error");
    }
  };

  const create = async () => {
    if (busy() || !newName().trim() || newPwd().length < 6) return;
    setBusy(true);
    try {
      await api.createUser(newName().trim(), newPwd(), newRole());
      toast(t("users.created", { name: newName().trim() }), "success");
      setCreateOpen(false);
      setNewName("");
      setNewPwd("");
      setNewRole("member");
      void refetch();
    } catch (err) {
      toast(errText(err), "error");
    } finally {
      setBusy(false);
    }
  };

  const sourceLabel = (s: string) =>
    s === "proxy" ? t("users.sourceProxy") : s === "oidc" ? t("users.sourceOIDC") : t("users.sourceLocal");

  return (
    <div class="setting-card">
      <h3 style={{ display: "flex", "align-items": "center", gap: "6px" }}>
        <ShieldCheck size={15} style={{ "vertical-align": "-2px" }} />
        {t("users.title")}
      </h3>
      <p class="text-dim" style={{ "font-size": "var(--text-xs)", "margin": "4px 0 12px" }}>{t("users.desc")}</p>
      <table class="data-table">
        <thead>
          <tr>
            <th>{t("common.name")}</th>
            <th>{t("users.role")}</th>
            <th>{t("common.status")}</th>
            <th>{t("users.source")}</th>
            <th style={{ width: "130px", "text-align": "right" }}>{t("users.actions")}</th>
          </tr>
        </thead>
        <tbody>
          <For each={users() ?? []}>
            {(u: UserRow) => (
              <tr>
                <td>
                  <div class="cell-main">{u.username}</div>
                  <div class="cell-sub">#{u.id} · {sourceLabel(u.source)}</div>
                </td>
                <td>
                  <select
                    class="form-input"
                    style={{ padding: "4px 8px", "font-size": "var(--text-xs)", width: "auto" }}
                    value={u.role}
                    disabled={busy()}
                    aria-label={t("users.role")}
                    onChange={(e) => void act(() => api.setUserRole(u.id, e.currentTarget.value), "users.roleChanged")}
                  >
                    <option value="admin">admin</option>
                    <option value="member">member</option>
                  </select>
                </td>
                <td>
                  <span class={`status-badge ${u.active ? "status-running" : "status-exited"}`}>
                    {u.active ? t("users.active") : t("users.disabled")}
                  </span>
                </td>
                <td class="cell-dim">{sourceLabel(u.source)}</td>
                <td class="row-actions-cell">
                  <div class="cell-actions" style={{ "justify-content": "flex-end" }}>
                    <button
                      class="btn-clear"
                      style={{ "font-size": "var(--text-xs)" }}
                      onClick={() => void act(() => api.setUserActive(u.id, !u.active), u.active ? "users.disabled" : "users.enabled")}
                    >
                      {u.active ? t("users.disable") : t("users.enable")}
                    </button>
                    <button
                      class="btn-clear"
                      style={{ "font-size": "var(--text-xs)", color: "var(--danger)" }}
                      onClick={async () => {
                        if (!(await confirmDialog(t("users.confirmDelete", { name: u.username }), t("users.confirmDeleteDesc", { name: u.username })))) return;
                        await act(() => api.removeUser(u.id), "users.deleted");
                      }}
                    >
                      {t("common.delete")}
                    </button>
                  </div>
                </td>
              </tr>
            )}
          </For>
        </tbody>
      </table>
      <div style={{ "margin-top": "14px", display: "flex", "justify-content": "flex-end" }}>
        <button class="btn btn-primary" onClick={() => setCreateOpen(true)}>
          <Plus size={14} /> {t("users.create")}
        </button>
      </div>

      <Sheet
        open={createOpen()}
        onOpenChange={setCreateOpen}
        title={t("users.create")}
        footer={
          <>
            <button class="btn btn-secondary" onClick={() => setCreateOpen(false)}>{t("common.cancel")}</button>
            <button class="btn btn-primary" disabled={busy() || !newName().trim() || newPwd().length < 6} onClick={() => void create()}>
              <UserPlus size={14} /> {busy() ? t("users.creating") : t("users.create")}
            </button>
          </>
        }
      >
        <div class="form-group">
          <label class="form-label">{t("form.username")}</label>
          <input class="form-input" value={newName()} placeholder="ops" onInput={(e) => setNewName(e.currentTarget.value)} />
        </div>
        <div class="form-group">
          <label class="form-label">{t("form.password")}</label>
          <input class="form-input" type="password" autocomplete="new-password" value={newPwd()} onInput={(e) => setNewPwd(e.currentTarget.value)} />
          <p class="form-help">{t("users.passwordHelp")}</p>
        </div>
        <div class="form-group">
          <label class="form-label">{t("users.role")}</label>
          <select class="form-input" value={newRole()} onChange={(e) => setNewRole(e.currentTarget.value)} aria-label={t("users.role")}>
            <option value="member">member</option>
            <option value="admin">admin</option>
          </select>
          <p class="form-help">{t("users.roleHelp")}</p>
        </div>
      </Sheet>
    </div>
  );
}
