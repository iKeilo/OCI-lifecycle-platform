import { Bell, ChevronDown, Languages, LogOut, Moon, RefreshCw, Search, Settings2 } from "lucide-react";
import { useEffect, useMemo, useState, type PropsWithChildren } from "react";
import { Link, NavLink } from "react-router-dom";
import { navGroups, productMark as ProductMark } from "../app/navigation";
import { getOCIReadiness, listNotifications, listProfiles } from "../services/api";
import type { OCIReadiness, Profile } from "../services/api";

type AppShellProps = PropsWithChildren<{
  onLogout?: () => void | Promise<void>;
}>;

export function AppShell({ children, onLogout }: AppShellProps) {
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [readiness, setReadiness] = useState<OCIReadiness | null>(null);
  const [unreadNotifications, setUnreadNotifications] = useState(0);

  useEffect(() => {
    let cancelled = false;
    async function loadContext() {
      try {
        const [nextProfiles, nextReadiness, notifications] = await Promise.all([listProfiles(), getOCIReadiness(), listNotifications(true)]);
        if (!cancelled) {
          setProfiles(nextProfiles);
          setReadiness(nextReadiness);
          setUnreadNotifications(notifications.unreadCount);
        }
      } catch {
        if (!cancelled) {
          setProfiles([]);
          setReadiness(null);
        }
      }
    }
    void loadContext();
    return () => {
      cancelled = true;
    };
  }, []);

  const quickStats = useMemo(() => {
    const profile = profiles[0];
    return [
      { label: "Profile", value: profile?.name ?? "未配置", tone: profile ? "neutral" : "warning" },
      { label: "区域", value: profile?.defaultRegion ?? "未配置", tone: profile ? "neutral" : "warning" },
      { label: "模式", value: readiness?.executionMode ?? "local", tone: "neutral" },
      { label: "OCI", value: readiness?.ready ? "就绪" : "未就绪", tone: readiness?.ready ? "success" : "warning" }
    ];
  }, [profiles, readiness]);

  return (
    <div className="app-shell">
      <aside className="sidebar glass-panel">
        <div className="brand-block">
          <div className="brand-mark">
            <ProductMark size={28} />
          </div>
          <div>
            <div className="brand-title">Oracle Cloud</div>
            <div className="brand-subtitle">机器生命周期平台</div>
          </div>
        </div>

        <nav className="sidebar-nav" aria-label="主导航">
          {navGroups.map((group) => (
            <div className="nav-group" key={group.label}>
              <div className="nav-group-label">{group.label}</div>
              {group.items.map((item) => {
                const Icon = item.icon;
                return (
                  <NavLink
                    className={({ isActive }) => `nav-item ${isActive ? "active" : ""}`}
                    key={item.path}
                    to={item.path}
                  >
                    <Icon size={19} strokeWidth={2} />
                    <span>{item.label}</span>
                  </NavLink>
                );
              })}
            </div>
          ))}
        </nav>
      </aside>

      <div className="workspace">
        <header className="topbar glass-panel">
          <div className="search-box">
            <Search size={19} />
            <input aria-label="搜索" placeholder="搜索资源、服务和文档" />
            <kbd>Ctrl K</kbd>
          </div>

          <div className="context-strip">
            {quickStats.map((stat) => (
              <button className={`context-chip ${stat.tone}`} key={stat.label}>
                <span>{stat.label}</span>
                <strong>{stat.value}</strong>
                <ChevronDown size={14} />
              </button>
            ))}
          </div>

          <div className="topbar-actions">
            <button className="language-button" aria-label="当前语言：简体中文">
              <Languages size={18} />
              <span>简体中文</span>
            </button>
            <button className="icon-button" aria-label="主题">
              <Moon size={20} />
            </button>
            <button className="icon-button" aria-label="刷新" onClick={() => window.location.reload()}>
              <RefreshCw size={20} />
            </button>
            <button className="icon-button" aria-label="设置">
              <Settings2 size={20} />
            </button>
            <Link className="icon-button notification-button" aria-label="通知" to="/notifications">
              <Bell size={20} />
              {unreadNotifications > 0 ? <span>{unreadNotifications > 99 ? "99+" : unreadNotifications}</span> : null}
            </Link>
            <button className="icon-button" aria-label="退出" onClick={() => void onLogout?.()}>
              <LogOut size={20} />
            </button>
            <div className="avatar" aria-label="管理员账号">A</div>
          </div>
        </header>

        <main className="content">{children}</main>
      </div>
    </div>
  );
}
