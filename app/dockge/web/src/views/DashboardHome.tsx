// 首页右栏（上游复刻）：统计三格 + Docker Run 转换器。
// 转换结果经 store.draftYaml 交接给 /compose 新建编辑器。
import { createMemo, createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";

import { api } from "../api/api";
import { errText } from "../api/format";
import { t } from "../i18n";
import { setDraftYaml, snapshot, toast } from "../store/index";

export function DashboardHome() {
  const navigate = useNavigate();
  const [runCommand, setRunCommand] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  const stacks = createMemo(() => snapshot()?.stacks ?? []);
  const active = () => stacks().filter((s) => s.status === 3).length;
  const exited = () => stacks().filter((s) => s.status === 4 || s.status === 2).length;
  const inactive = () => stacks().length - active() - exited();

  const convert = async () => {
    if (!runCommand().trim() || busy()) return;
    setBusy(true);
    try {
      const result = await api.composerize(runCommand().trim());
      setDraftYaml(result.composeTemplate);
      toast(t("home.convertSuccess"), "success");
      navigate("/compose");
    } catch (error) {
      toast(errText(error), "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <h1 style={{ "margin-bottom": "16px" }}>{t("nav.home")}</h1>
      <div class="card">
        <div class="stats-row">
          <div class="stat-cell stat-active">
            <div class="num">{active()}</div>
            <div class="label">{t("home.active")}</div>
          </div>
          <div class="stat-cell stat-exited">
            <div class="num">{exited()}</div>
            <div class="label">{t("home.exited")}</div>
          </div>
          <div class="stat-cell stat-inactive">
            <div class="num">{inactive()}</div>
            <div class="label">{t("home.inactive")}</div>
          </div>
        </div>
      </div>
      <div class="card">
        <h2 class="card-title">{t("home.dockerRunTitle")}</h2>
        <p class="settings-desc">{t("home.dockerRunDesc")}</p>
        <textarea
          class="form-input mono"
          rows="4"
          value={runCommand()}
          placeholder={t("home.dockerRunPlaceholder")}
          onInput={(e) => setRunCommand(e.currentTarget.value)}
        />
        <div style={{ "margin-top": "10px" }}>
          <button class="btn btn-primary" disabled={!runCommand().trim() || busy()} onClick={() => void convert()}>
            {t("home.convert")}
          </button>
        </div>
      </div>
    </div>
  );
}
