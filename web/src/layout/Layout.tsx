// 全局布局（上游复刻）：顶栏（logo + Home + 头像菜单）+ 首页双栏
// （左：+ Compose 与栈列表侧栏；右：路由出口）。
// Gate 承担 boot/认证/Setup 引导；未登录整体替换为 Login。
import { Show, Suspense, createEffect, createMemo, createSignal, onCleanup, onMount, type JSX } from "solid-js";
import { A, useNavigate, type RouteSectionProps } from "@solidjs/router";
import { ChevronDown } from "lucide-solid";

import { api } from "../api/api";
import { errText } from "../api/format";
import { StackList } from "../components/StackList";
import { ConfirmHost } from "../components/Confirm";
import { Toaster } from "../components/Toaster";
import { t } from "../i18n";
import { Login } from "../views/Login";
import { authed, boot, logout, refresh, toast, user } from "../store/index";

/** 认证门：启动会话校验；未初始化跳 Setup，未登录渲染 Login。 */
export function Gate(props: RouteSectionProps) {
  const navigate = useNavigate();
  const [booted, setBooted] = createSignal(false);
  onMount(() => {
    void boot().then(async (ok) => {
      if (!ok) {
        try {
          const need = await api.needSetup();
          if (need.needSetup) {
            navigate("/setup", { replace: true });
            return;
          }
        } catch {
          // 后端不可达时照常进入（Login 页会呈现错误）
        }
      }
      setBooted(true);
    });
  });
  return (
    <Show when={booted()} fallback={<div class="empty-hint">{t("common.loading")}</div>}>
      <Show when={authed()} fallback={<Login />}>
        <Layout>{props.children}</Layout>
      </Show>
    </Show>
  );
}

function Layout(props: { children?: JSX.Element }) {
  const navigate = useNavigate();
  const [menuOpen, setMenuOpen] = createSignal(false);

  // 栈状态轮询：上游为每 10s 推送（cron */10s），本实现以同频拉取对齐，
  // 避免外部变更（CLI/其他标签页）后侧栏停留在陈旧状态；页面不可见时暂停。
  onMount(() => {
    const timer = setInterval(() => {
      if (!document.hidden) void refresh(false);
    }, 10_000);
    onCleanup(() => clearInterval(timer));
  });

  const initial = createMemo(() => (user()?.username ?? "?").charAt(0).toUpperCase());

  // Escape 关闭头像下拉（对齐上游 Bootstrap dropdown 行为）
  createEffect(() => {
    if (!menuOpen()) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMenuOpen(false);
    };
    document.addEventListener("keydown", onKey);
    onCleanup(() => document.removeEventListener("keydown", onKey));
  });

  const scanStacks = async () => {
    setMenuOpen(false);
    try {
      await refresh(false);
      toast(t("toast.scanDone"), "success");
    } catch (error) {
      toast(errText(error), "error");
    }
  };

  return (
      <div class="app-main">
        <header class="topbar">
          <A class="topbar-brand" href="/">
            <img src="/icon.svg" alt="Dockge" />
            <span>Dockge</span>
          </A>
          <span class="topbar-spacer" />
          <nav class="topbar-pills">
            <A href="/" end class="topbar-pill" activeClass="active">
              {t("nav.home")}
            </A>
          </nav>
          <div class="avatar-menu">
            <button class="avatar" aria-label={user()?.username} onClick={() => setMenuOpen((v) => !v)}>
              {initial()}
              <ChevronDown size={13} />
            </button>
            <Show when={menuOpen()}>
              <div class="avatar-dropdown">
                <div class="dd-user">{t("nav.signedInAs", { name: user()?.username ?? "" })}</div>
                <button onClick={() => void scanStacks()}>{t("nav.scanStacks")}</button>
                <button onClick={() => { setMenuOpen(false); navigate("/settings/general"); }}>{t("nav.settings")}</button>
                <button onClick={() => { logout(); setMenuOpen(false); }}>{t("nav.logout")}</button>
              </div>
            </Show>
          </div>
        </header>
        <div class="dashboard-grid">
          <aside class="stacklist-col">
            <A class="btn btn-primary" href="/compose" style={{ "justify-content": "center" }}>
              + {t("compose.newTitle")}
            </A>
            <StackList />
          </aside>
          <main class="content-col">
            <Suspense fallback={<div class="empty-hint">{t("common.loading")}</div>}>{props.children}</Suspense>
          </main>
        </div>
        <ConfirmHost />
        <Toaster />
      </div>
  );
}
