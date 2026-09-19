// 首页（上游复刻）：统计三格 + Docker Run 转换器。
// 转换结果经 store.draftYaml 交接给 /compose 新建编辑器。
import { createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";

import { api } from "../api/api";
import { errText } from "../api/format";
import { t } from "../i18n";
import { setDraftYaml, snapshot, toast } from "../store/index";

export function DashboardHome() {
  const navigate = useNavigate();
  const [runCommand, setRunCommand] = createSignal("");
  const [busy, setBusy] = createSignal(false);

  const stacks = () => snapshot()?.stacks ?? [];
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
    <div class="home-page">
      <h1>{t("nav.home")}</h1>

      <div class="card stats-row">
        <div class="stat-cell stat-active">
          <h3 class="label">{t("home.active")}</h3>
          <span class="num">{active()}</span>
        </div>
        <div class="stat-cell stat-exited">
          <h3 class="label">{t("home.exited")}</h3>
          <span class="num">{exited()}</span>
        </div>
        <div class="stat-cell stat-inactive">
          <h3 class="label">{t("home.inactive")}</h3>
          <span class="num">{inactive()}</span>
        </div>
      </div>

      <h2>{t("home.dockerRunTitle")}</h2>
      <textarea
        class="form-input mono"
        rows="4"
        value={runCommand()}
        placeholder="docker run ..."
        aria-label={t("home.dockerRunTitle")}
        onInput={(e) => setRunCommand(e.currentTarget.value)}
      />
      <button class="btn btn-secondary" style={{ "margin-top": "12px" }} disabled={!runCommand().trim() || busy()} onClick={() => void convert()}>
        {t("home.convert")}
      </button>
    </div>
  );
}
