// 入口：路由装配（上游复刻路由表）。Setup 独立于 Gate；其余经认证门。
import { render } from "solid-js/web";
import { Route, Router } from "@solidjs/router";
import "./styles/app.css";
import { Gate } from "./layout/Layout";
import { Setup } from "./views/Setup";
import { DashboardHome } from "./views/DashboardHome";
import { Compose } from "./views/Compose";
import { TerminalPage } from "./views/TerminalPage";
import { SettingsPage } from "./views/SettingsPage";

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
