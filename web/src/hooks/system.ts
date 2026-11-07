import { createSignal, onMount, onCleanup } from 'solid-js';
import { app, refreshData } from '../store';

// 页面共享全局 SSE 指标；本地时钟只用于识别过期数据，不请求服务器。
export function useSystemMetrics() {
  const [now,setNow]=createSignal(Date.now());
  onMount(()=>{ const clock=setInterval(()=>setNow(Date.now()),1000); onCleanup(()=>clearInterval(clock)); });
  const stale=()=>app.systemReceivedAt>0 && now()-app.systemReceivedAt>7000;
  return { system:()=>stale() ? null : app.system, stale, refresh:()=>void refreshData() };
}
