import { createSignal, onMount, onCleanup } from 'solid-js';
import { app, setApp, user } from '../store';
import { api } from '../lib/api';
import { AuthAPI } from '../lib/auth';
import { startSystemPolling, type HostSystem } from '../lib/system';

// 仪表盘与系统页共享轻量采样；路由卸载、隐藏或退出后不再请求。
export function useSystemMetrics() {
  const [now,setNow]=createSignal(Date.now());
  let refresh=()=>{};
  onMount(()=>{
    const clock=setInterval(()=>setNow(Date.now()),1000);
    const polling=startSystemPolling({
      request:(signal)=>api<HostSystem>('/system',{signal}),
      session:()=>user() ? AuthAPI.token() : null,
      visible:()=>document.visibilityState==='visible',
      success:system=>{setNow(Date.now());setApp({system,systemReceivedAt:Date.now(),systemError:''});},
      failure:error=>{setApp({system:null,systemReceivedAt:0,systemError:error.message});}
    });
    refresh=polling.refresh;
    document.addEventListener('visibilitychange',polling.visibilityChanged);
    onCleanup(()=>{clearInterval(clock);polling.stop();document.removeEventListener('visibilitychange',polling.visibilityChanged);});
  });
  const stale=()=>app.systemReceivedAt>0 && now()-app.systemReceivedAt>7000;
  return { system:()=>stale() ? null : app.system, stale, refresh:()=>refresh() };
}
