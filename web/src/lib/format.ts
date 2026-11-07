/* 展示层格式化：字节、相对时间、端口、状态文案。
 */
import { t } from '../i18n';
import type { Container } from './data';

export function formatBytes(b: number): string {
  if (!Number.isFinite(b) || b < 0) return t('data.unknown');
  if (!b) return '0 B';
  const k = 1024, units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.max(0, Math.min(units.length - 1, Math.floor(Math.log(b) / Math.log(k))));
  return parseFloat((b / Math.pow(k, i)).toFixed(1)) + ' ' + units[i];
}

export function timeAgo(ms: number): string {
  if (!ms) return '—';
  const s = Math.floor((Date.now() - ms) / 1000);
  if (s < 60) return t('common.secAgo', { n: s });
  if (s < 3600) return t('common.minAgo', { n: Math.floor(s / 60) });
  if (s < 86400) return t('common.hourAgo', { n: Math.floor(s / 3600) });
  return t('common.dayAgo', { n: Math.floor(s / 86400) });
}

/* 容器显示名：去掉前导斜杠，取最后一段 */
export function ctrName(c: { Names: string[] }): string {
  return String((c.Names && c.Names[0]) || '').replace(/^\//, '').split('/').pop() || '';
}

/* 状态 → 徽标样式键（列表/详情/仪表盘共用配色规则） */
export function statusKind(c: Container): 'running' | 'paused' | 'exited' {
  if (c.State === 'running') return 'running';
  if (c.State === 'paused') return 'paused';
  return 'exited';
}

export function statusLabel(c: Container): string {
  const map: Record<string, string> = {
    running: t('status.running'), exited: t('status.exited'),
    paused: t('status.paused'), dead: t('status.dead')
  };
  return map[c.State] || c.State;
}

/* 状态时长文案：运行中显示已运行时长；已退出显示退出码与距退出的时长 */
export function statusText(c: Container): string {
  if (c.State === 'running') return t('status.upHours', { n: c.upHours ?? '—' });
  if (c.State === 'exited') {
    const code = c.exitCode ?? '—';
    if (c.exitHoursAgo != null) return t('status.exitedHours', { code, n: c.exitHoursAgo });
    return t('status.exitedDetail', { code, m: c.exitMinAgo ?? '—' });
  }
  return statusLabel(c);
}

export function portsText(c: Container): string {
  return (c.Ports || [])
    .map(p => {
      const base = p.PublicPort
        ? (p.IP === '0.0.0.0' || !p.IP ? '' : (p.IP.includes(':') ? '[' + p.IP + ']' : p.IP) + ':') + p.PublicPort + ':' + p.PrivatePort
        : String(p.PrivatePort || '');
      return base ? base + ((p.Proto || p.Type) === 'udp' ? '/udp' : '') : '';
    })
    .filter(Boolean)
    .join(', ');
}
