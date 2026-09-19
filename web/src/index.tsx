// 入口：路由装配（上游复刻路由表）。Setup 独立于 Gate；其余经认证门。
// 重页面（编辑器 CodeMirror、终端 xterm）按需加载：首屏只载入外壳与首页。
import { lazy } from "solid-js";
import { render } from "solid-js/web";
import { Route, Router } from "@solidjs/router";
import "./styles/app.css";
import { Gate } from "./layout/Layout";
import { Setup } from "./views/Setup";
import { DashboardHome } from "./views/DashboardHome";

const Compose = lazy(() => import("./views/Compose").then((m) => ({ default: m.Compose })));
const TerminalPage = lazy(() => import("./views/TerminalPage").then((m) => ({ default: m.TerminalPage })));
const SettingsPage = lazy(() => import("./views/SettingsPage").then((m) => ({ default: m.SettingsPage })));

const root = document.getElementById("root");

render(
  () => (
    <Router>
      <Route path="/setup" component={Setup} />
      <Route path="/" component={Gate}>
        <Route path="/" component={DashboardHome} />
        <Route path="/compose" component={Compose} />
        <Route path="/compose/:name" component={Compose} />
        <Route path="/terminal/:stack/:service/:type" component={TerminalPage} />
        <Route path="/settings/:tab" component={SettingsPage} />
        <Route path="*" component={DashboardHome} />
      </Route>
    </Router>
  ),
  root!,
);
