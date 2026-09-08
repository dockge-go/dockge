// 入口：路由装配（登录/Setup 独立于 AppShell 守卫之外）。
import { render } from "solid-js/web";
import { Route, Router } from "@solidjs/router";
import "./styles/app.css";
import { AppShell } from "./layout/AppShell";
import { Login } from "./views/Login";
import { Setup } from "./views/Setup";
import { Dashboard } from "./views/Dashboard";
import { Containers } from "./views/Containers";
import { Stacks } from "./views/Stacks";
import { Images } from "./views/Images";
import { Volumes } from "./views/Volumes";
import { Networks } from "./views/Networks";
import { SysInfo } from "./views/SysInfo";
import { SysDf } from "./views/SysDf";
import { Settings } from "./views/Settings";

const root = document.getElementById("root");

render(
  () => (
    <Router>
      <Route path="/login" component={Login} />
      <Route path="/setup" component={Setup} />
      <Route path="/" component={AppShell}>
        <Route path="/" component={Dashboard} />
        <Route path="/containers" component={Containers} />
        <Route path="/stacks" component={Stacks} />
        <Route path="/images" component={Images} />
        <Route path="/volumes" component={Volumes} />
        <Route path="/networks" component={Networks} />
        <Route path="/sysinfo" component={SysInfo} />
        <Route path="/sysdf" component={SysDf} />
        <Route path="/settings" component={Settings} />
      </Route>
    </Router>
  ),
  root!,
);
