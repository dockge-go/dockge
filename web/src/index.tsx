import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import App from './App';
import Dashboard from './views/Dashboard';
import Containers from './views/Containers';
import ContainerDetail from './views/ContainerDetail';
import Stacks from './views/Stacks';
import Images from './views/Images';
import Volumes from './views/Volumes';
import Networks from './views/Networks';
import SysInfo from './views/SysInfo';
import SysDf from './views/SysDf';
import Settings from './views/Settings';
import './styles/app.css';

render(() => (
  <Router root={App}>
    <Route path="/" component={Dashboard} />
    <Route path="/containers" component={Containers} />
    <Route path="/containers/:id" component={ContainerDetail} />
    <Route path="/stacks" component={Stacks} />
    <Route path="/images" component={Images} />
    <Route path="/volumes" component={Volumes} />
    <Route path="/networks" component={Networks} />
    <Route path="/sysinfo" component={SysInfo} />
    <Route path="/sysdf" component={SysDf} />
    <Route path="/settings" component={Settings} />
    <Route path="*" component={Dashboard} />
  </Router>
), document.getElementById('root')!);
