// 侧栏：品牌 + 分组导航（实时徽标）+ 引擎状态（原型 <aside class="sidebar">）。
import { For } from "solid-js";
import { A } from "@solidjs/router";
import {
  Container,
  Database,
  HardDrive,
  Image,
  Info,
  Layers,
  LayoutGrid,
  Network,
  Settings,
} from "lucide-solid";
import { snapshot, sseOn } from "../store/index";
import { t } from "../i18n/index";
import type { JSX } from "solid-js";

interface NavEntry {
  to: string;
  label: () => string;
  icon: JSX.Element;
  end?: boolean;
  badge?: () => string | undefined;
}

export function Sidebar(props: { open: boolean; onNavigate: () => void }) {
  const groups: Array<{ label: () => string; items: NavEntry[] }> = [
    {
      label: () => t("nav.group.overview"),
      items: [{ to: "/", label: () => t("nav.dashboard"), icon: <LayoutGrid size={18} />, end: true }],
    },
    {
      label: () => t("nav.group.resources"),
      items: [
        {
          to: "/containers",
          label: () => t("nav.containers"),
          icon: <Container size={18} />,
          // 三个计数徽标均由容器状态流（SSE 长连接）随帧推送的 counts 实时驱动：
          // docker events 监听 container+image 事件，采集失败帧省略 counts、保持旧值。
          badge: () => {
            const snap = snapshot();
            if (!snap) return undefined;
            return `${snap.docker.containersRunning}/${snap.docker.containersTotal}`;
          },
        },
        {
          to: "/stacks",
          label: () => t("nav.stacks"),
          icon: <Layers size={18} />,
          badge: () => snapshot()?.docker.stacksTotal?.toString(),
        },
        {
          to: "/images",
          label: () => t("nav.images"),
          icon: <Image size={18} />,
          badge: () => snapshot()?.docker.imagesTotal?.toString(),
        },
        { to: "/volumes", label: () => t("nav.volumes"), icon: <HardDrive size={18} /> },
        { to: "/networks", label: () => t("nav.networks"), icon: <Network size={18} /> },
      ],
    },
    {
      label: () => t("nav.group.system"),
      items: [
        { to: "/sysinfo", label: () => t("nav.sysinfo"), icon: <Info size={18} /> },
        { to: "/sysdf", label: () => t("nav.sysdf"), icon: <Database size={18} /> },
        { to: "/settings", label: () => t("nav.settings"), icon: <Settings size={18} /> },
      ],
    },
  ];

  return (
    <aside class={`sidebar ${props.open ? "open" : ""}`} aria-label={t("aria.mainNav")}>
      <div class="sidebar-brand">
        <svg
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.75"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <rect x="2" y="3" width="20" height="14" rx="2" />
          <path d="M8 21h8M12 17v4" />
          <path d="M7 8h2m2 0h2m2 0h2M7 11h10" />
        </svg>
        <span class="sidebar-brand-name">Dockge</span>
      </div>
      <nav class="sidebar-nav">
        <For each={groups}>
          {(group) => (
            <div class="nav-group">
              <div class="nav-group-label">{group.label()}</div>
              <For each={group.items}>
                {(item) => (
                  <A
                    href={item.to}
                    end={item.end}
                    activeClass="active"
                    inactiveClass=""
                    class="nav-item"
                    onClick={props.onNavigate}
                  >
                    <span class="nav-item-icon">{item.icon}</span>
                    {item.label()}
                    <ShowBadge when={item.badge}>{item.badge?.()}</ShowBadge>
                  </A>
                )}
              </For>
            </div>
          )}
        </For>
      </nav>
      <div class="sidebar-footer">
        <div class="engine-badge">
          <span class={`engine-dot ${sseOn() ? "" : "disconnected"}`} />
          <span>Docker {snapshot()?.docker.version || "…"}</span>
        </div>
      </div>
    </aside>
  );
}

function ShowBadge(props: { when?: () => string | undefined; children: string | undefined }) {
  const v = props.when?.();
  return v ? <span class="nav-item-badge">{v}</span> : null;
}
