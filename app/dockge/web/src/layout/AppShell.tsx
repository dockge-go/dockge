// 应用外壳：认证守卫 + 布局（侧栏 / 顶栏 / 内容区）+ 移动端抽屉。
import { Show, createEffect, createSignal, onCleanup, onMount, type JSX } from "solid-js";
import { useLocation, useNavigate } from "@solidjs/router";
import { authed, boot, refresh, startContainerStatusStream } from "../store/index";
import { t } from "../i18n/index";
import { Sidebar } from "../components/Sidebar";
import { Topbar } from "../components/Topbar";
import { Toaster } from "../components/Toaster";
import { ConfirmHost } from "../components/Confirm";
import { SpinnerBlock } from "../components/widgets";

export function AppShell(props: { children?: JSX.Element }) {
  const navigate = useNavigate();
  const location = useLocation();
  const [ready, setReady] = createSignal(false);
  const [sidebarOpen, setSidebarOpen] = createSignal(false);

  onMount(async () => {
    if (!authed()) {
      const ok = await boot();
      if (!ok) {
        navigate("/login", { replace: true });
        return;
      }
    } else {
      await refresh(false);
      startContainerStatusStream();
    }
    setReady(true);
  });

  onCleanup(() => {
    // SSE 由 store 管理；这里只复位抽屉
    setSidebarOpen(false);
  });

  // 会话中 token 失效（401）时回到登录页
  createEffect(() => {
    if (ready() && !authed()) navigate("/login", { replace: true });
  });

  // 切换路由时收起移动端侧栏
  createEffect(() => {
    void location.pathname;
    setSidebarOpen(false);
  });

  return (
    <Show when={ready()} fallback={<SpinnerBlock label={t("common.connecting")} />}>
      <div class="app-shell">
        <Sidebar open={sidebarOpen()} onNavigate={() => setSidebarOpen(false)} />
        <div class="main-content">
          <Topbar onHamburger={() => setSidebarOpen((v) => !v)} />
          <div class="content-area">
            {props.children}
          </div>
        </div>
      </div>
      <div
        class={`sidebar-backdrop ${sidebarOpen() ? "open" : ""}`}
        onClick={() => setSidebarOpen(false)}
      />
      <Toaster />
      <ConfirmHost />
    </Show>
  );
}
