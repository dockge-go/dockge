import { For, Show } from 'solid-js';
import { app, t, lang } from '../store';
import type { HostSystem } from '../lib/system';

export default function SystemStatus(props:{metrics:{system:()=>HostSystem|null;stale:()=>boolean}}) {
  const host=props.metrics.system;
  const time=()=>{const stamp=host()?.sampledAt;return stamp ? new Date(stamp).toLocaleTimeString(lang()==='zh' ? 'zh-CN' : 'en-US',{hour12:false}) : '';};
  return <div class="host-status">
    <Show when={host()?.source} fallback={<p class="form-help">{app.systemError ? t('sysinfo.unavailable') : props.metrics.stale() ? t('sysinfo.stale') : host() ? t('sysinfo.unavailable') : t('sysinfo.connecting')}</p>}>
      <p class="form-help"><span class="host-status-dot" />{t('sysinfo.live', {time:time()})} · {t(host()?.source==='host-mount' ? 'sysinfo.hostMount' : 'sysinfo.localSource')}</p>
    </Show>
    <Show when={app.systemError}><div class="host-error" role="status">{app.systemError}</div></Show>
    <For each={Object.entries(host()?.errors || {})}>{([key,message])=><div class="host-error" role="status">{key}: {message}</div>}</For>
  </div>;
}
