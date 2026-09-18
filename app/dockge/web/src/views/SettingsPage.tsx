// 设置页（上游复刻五子页）：General / Appearance / Security / Global Env / About。
// 单用户模型：Security = 改密 + Disable Auth；无用户管理页（一比一决策）。
import { Show, Switch, Match, createSignal, onMount } from "solid-js";
import { A, useParams } from "@solidjs/router";

import { api } from "../api/api";
import { errText } from "../api/format";
import { t, setLocale, useLocale, type LocaleKey, type MsgKey } from "../i18n";
import { setThemePref, useThemePref } from "../lib/theme";
import { snapshot, toast } from "../store/index";

const TABS = ["general", "appearance", "security", "globalEnv", "about"] as const;
type Tab = (typeof TABS)[number];

const LANG_OPTIONS: Array<{ value: LocaleKey; label: string }> = [
  { value: "zh-CN", label: "简体中文" },
  { value: "en-US", label: "English" },
];

export function SettingsPage() {
  const params = useParams();
  const tab = (): Tab => (TABS as readonly string[]).includes(params.tab ?? "") ? (params.tab as Tab) : "general";

  return (
    <div>
      <h1 style={{ "margin-bottom": "16px" }}>{t("nav.settings")}</h1>
      <div class="settings-grid">
        <nav class="settings-nav">
          {TABS.map((item) => (
            <A href={`/settings/${item}`} classList={{ active: tab() === item }}>
              {t(`settings.${item}` as MsgKey)}
            </A>
          ))}
        </nav>
        <div class="card">
          <Switch>
            <Match when={tab() === "general"}><GeneralTab /></Match>
            <Match when={tab() === "appearance"}><AppearanceTab /></Match>
            <Match when={tab() === "security"}><SecurityTab /></Match>
            <Match when={tab() === "globalEnv"}><GlobalEnvTab /></Match>
            <Match when={tab() === "about"}><AboutTab /></Match>
          </Switch>
        </div>
      </div>
    </div>
  );
}

function GeneralTab() {
  const [dockgeVersion, setDockgeVersion] = createSignal("");
  onMount(() => {
    api.versionCheck().then((r) => setDockgeVersion(r.currentVersion)).catch(() => {});
  });
  return (
    <div>
      <h2 class="card-title">{t("settings.general")}</h2>
      <div class="settings-row">
        <span class="settings-label">Dockge</span>
        <span class="mono">{dockgeVersion() || "dev"}</span>
      </div>
      <div class="settings-row">
        <span class="settings-label">Docker</span>
        <span class="mono">{snapshot()?.docker.version ?? "-"}</span>
      </div>
      <div class="settings-row">
        <span class="settings-label">Host</span>
        <span class="mono">{snapshot()?.docker.os ?? "-"} / {snapshot()?.docker.arch ?? "-"}</span>
      </div>
    </div>
  );
}

function AppearanceTab() {
  return (
    <div>
      <h2 class="card-title">{t("settings.appearance")}</h2>
      <div class="settings-row">
        <div>
          <div class="settings-label">{t("settings.theme")}</div>
        </div>
        <select class="form-select" style={{ width: "180px" }} value={useThemePref()} onChange={(e) => setThemePref(e.currentTarget.value as "auto" | "light" | "dark")}>
          <option value="auto">{t("theme.auto")}</option>
          <option value="light">{t("theme.light")}</option>
          <option value="dark">{t("theme.dark")}</option>
        </select>
      </div>
      <div class="settings-row">
        <div>
          <div class="settings-label">{t("settings.language")}</div>
        </div>
        <select class="form-select" style={{ width: "180px" }} value={useLocale()} onChange={(e) => setLocale(e.currentTarget.value as LocaleKey)}>
          {LANG_OPTIONS.map((opt) => (
            <option value={opt.value}>{opt.label}</option>
          ))}
        </select>
      </div>
    </div>
  );
}

function SecurityTab() {
  const [current, setCurrent] = createSignal("");
  const [next, setNext] = createSignal("");
  const [repeat, setRepeat] = createSignal("");
  const [disableEnabled, setDisableEnabled] = createSignal(false);
  const [disablePassword, setDisablePassword] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  onMount(() => {
    api.getDisableAuth().then((r) => setDisableEnabled(r.enabled)).catch(() => {});
  });

  const changePassword = async () => {
    if (busy() || next() !== repeat() || next().length < 6) return;
    setBusy(true);
    try {
      await api.changePassword(current(), next());
      toast(t("settings.saved"), "success");
      setCurrent(""); setNext(""); setRepeat("");
    } catch (error) {
      toast(errText(error), "error");
    } finally {
      setBusy(false);
    }
  };

  const toggleAuth = async (enable: boolean) => {
    if (busy()) return;
    setBusy(true);
    try {
      await api.toggleDisableAuth(!enable, disablePassword());
      setDisableEnabled(!enable);
      setDisablePassword("");
      toast(t("settings.saved"), "success");
    } catch (error) {
      toast(errText(error), "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <h2 class="card-title">{t("settings.security")}</h2>
      <div class="settings-row">
        <div>
          <div class="settings-label">{t("settings.changePassword")}</div>
        </div>
      </div>
      <div style={{ "max-width": "360px" }}>
        <div class="form-floating">
          <input id="sec-current" type="password" placeholder=" " value={current()} onInput={(e) => setCurrent(e.currentTarget.value)} />
          <label for="sec-current">{t("settings.currentPassword")}</label>
        </div>
        <div class="form-floating">
          <input id="sec-new" type="password" placeholder=" " value={next()} onInput={(e) => setNext(e.currentTarget.value)} />
          <label for="sec-new">{t("settings.newPassword")}</label>
        </div>
        <div class="form-floating">
          <input id="sec-repeat" type="password" placeholder=" " value={repeat()} onInput={(e) => setRepeat(e.currentTarget.value)} />
          <label for="sec-repeat">{t("settings.repeatPassword")}</label>
        </div>
        <button class="btn btn-primary" disabled={busy() || !current() || next().length < 6 || next() !== repeat()} onClick={() => void changePassword()}>
          {t("common.save")}
        </button>
      </div>
      <div class="settings-row" style={{ "margin-top": "20px" }}>
        <div>
          <div class="settings-label">{disableEnabled() ? t("settings.enableAuth") : t("settings.disableAuth")}</div>
          <div class="settings-desc">{t("settings.disableAuthDesc")}</div>
        </div>
      </div>
      <div style={{ display: "flex", gap: "8px", "max-width": "360px" }}>
        <div class="form-floating" style={{ flex: 1 }}>
          <input id="sec-disable" type="password" placeholder=" " value={disablePassword()} onInput={(e) => setDisablePassword(e.currentTarget.value)} />
          <label for="sec-disable">{t("settings.currentPassword")}</label>
        </div>
        <Show
          when={disableEnabled()}
          fallback={<button class="btn btn-danger" disabled={busy() || !disablePassword()} onClick={() => void toggleAuth(false)}>{t("settings.disableAuth")}</button>}
        >
          <button class="btn btn-secondary" disabled={busy() || !disablePassword()} onClick={() => void toggleAuth(true)}>{t("settings.enableAuth")}</button>
        </Show>
      </div>
    </div>
  );
}

function GlobalEnvTab() {
  const [content, setContent] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  onMount(() => {
    api.globalEnv().then((r) => setContent(r.globalENV)).catch(() => {});
  });

  const save = async () => {
    if (busy()) return;
    setBusy(true);
    try {
      await api.setGlobalEnv(content());
      toast(t("settings.saved"), "success");
    } catch (error) {
      toast(errText(error), "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <h2 class="card-title">{t("settings.globalEnv")}</h2>
      <p class="settings-desc">{t("settings.globalEnvDesc")}</p>
      <textarea class="form-input mono" rows="10" value={content()} onInput={(e) => setContent(e.currentTarget.value)} placeholder={"KEY=value"} />
      <div style={{ "margin-top": "10px" }}>
        <button class="btn btn-primary" disabled={busy()} onClick={() => void save()}>{t("common.save")}</button>
      </div>
    </div>
  );
}

function AboutTab() {
  const [version, setVersion] = createSignal("");
  onMount(() => {
    api.versionCheck().then((r) => setVersion(r.currentVersion)).catch(() => {});
  });
  return (
    <div>
      <h2 class="card-title">{t("settings.about")}</h2>
      <div class="settings-row">
        <span class="settings-label">{t("settings.version")}</span>
        <span class="mono">{version() || "dev"}</span>
      </div>
      <div class="settings-row">
        <span class="settings-label">{t("settings.upstream")}</span>
        <a href="https://github.com/louislam/dockge" target="_blank" rel="noreferrer">louislam/dockge</a>
      </div>
    </div>
  );
}
