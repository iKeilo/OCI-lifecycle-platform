import { Bell, ChevronDown, LogOut, Menu, Moon, RefreshCw, Search, Settings2, Sun, X } from "lucide-react";
import { useEffect, useMemo, useState, type PropsWithChildren } from "react";
import { Link, NavLink, useLocation, useNavigate } from "react-router-dom";
import { getSelectedOCIContext, setSelectedOCIContext } from "../app/ociContext";
import { navGroups, productMark as ProductMark } from "../app/navigation";
import {
  getAccountSettings,
  getAppearanceSettings,
  getOCIReadiness,
  listNotifications,
  listProfiles,
  updateAppearanceSettings
} from "../services/api";
import type { AccountSettings, AppearanceSettings, OCIReadiness, Profile } from "../services/api";

type AppShellProps = PropsWithChildren<{
  onLogout?: () => void | Promise<void>;
}>;

const GITHUB_PROJECT_URL = "https://github.com/iKeilo/OCI-lifecycle-platform";
const GITHUB_LATEST_RELEASE_API = "https://api.github.com/repos/iKeilo/OCI-lifecycle-platform/releases/latest";
const APP_VERSION = normalizeVersion(__APP_VERSION__ || "1.0.20");

type VersionCheckState = {
  latestVersion: string;
  releaseUrl: string;
  hasUpdate: boolean;
  checked: boolean;
  checking: boolean;
  error: string;
};

export function AppShell({ children, onLogout }: AppShellProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [readiness, setReadiness] = useState<OCIReadiness | null>(null);
  const [unreadNotifications, setUnreadNotifications] = useState(0);
  const [account, setAccount] = useState<AccountSettings | null>(null);
  const [appearance, setAppearance] = useState<AppearanceSettings>({
    theme: "light",
    backgroundMode: "aurora",
    backgroundImage: "",
    language: "zh-CN"
  });
  const [selectedProfileId, setSelectedProfileId] = useState(() => getSelectedOCIContext().profileId);
  const [selectedRegion, setSelectedRegion] = useState("");
  const [profileMenuOpen, setProfileMenuOpen] = useState(false);
  const [contextMessage, setContextMessage] = useState("");
  const [loggingOut, setLoggingOut] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [versionModalOpen, setVersionModalOpen] = useState(false);
  const [versionCheck, setVersionCheck] = useState<VersionCheckState>({
    latestVersion: "",
    releaseUrl: GITHUB_PROJECT_URL,
    hasUpdate: false,
    checked: false,
    checking: false,
    error: ""
  });

  useEffect(() => {
    let cancelled = false;
    async function loadContext() {
      try {
        const savedContext = getSelectedOCIContext();
        const [nextProfiles, nextReadiness, notifications, nextAccount, nextAppearance] = await Promise.all([
          listProfiles(),
          getOCIReadiness(savedContext),
          listNotifications(true),
          getAccountSettings(),
          getAppearanceSettings()
        ]);
        const savedProfile = nextProfiles.find((profile) => profile.id === savedContext.profileId);
        const firstProfile = nextProfiles[0];
        const activeProfile = savedProfile ?? firstProfile;
        const activeRegion = savedContext.region || activeProfile?.defaultRegion || "";
        if (!cancelled) {
          setProfiles(nextProfiles);
          setReadiness(nextReadiness);
          setUnreadNotifications(notifications.unreadCount);
          setAccount(nextAccount);
          setAppearance(nextAppearance);
          setSelectedProfileId(activeProfile?.id || "");
          setSelectedRegion(activeRegion);
          if (activeProfile) {
            setSelectedOCIContext({ profileId: activeProfile.id, region: activeRegion });
          }
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

  useEffect(() => {
    void checkLatestVersion(false);
  }, []);

  useEffect(() => {
    document.documentElement.dataset.theme = appearance.theme;
    document.documentElement.dataset.background = appearance.backgroundMode;
    document.documentElement.lang = appearance.language || "zh-CN";
    if (appearance.backgroundMode === "image" && appearance.backgroundImage) {
      document.documentElement.style.setProperty("--custom-background-image", `url("${appearance.backgroundImage}")`);
    } else {
      document.documentElement.style.removeProperty("--custom-background-image");
    }
  }, [appearance]);

  async function toggleTheme() {
    const next: AppearanceSettings = {
      ...appearance,
      theme: appearance.theme === "dark" ? "light" : "dark"
    };
    setAppearance(next);
    try {
      setAppearance(await updateAppearanceSettings(next));
    } catch {
      // Keep the immediate local switch; the settings page can show persistence errors on save.
    }
  }

  async function handleLogoutClick() {
    setLoggingOut(true);
    try {
      await onLogout?.();
      navigate("/", { replace: true });
    } finally {
      setLoggingOut(false);
    }
  }

  async function selectProfile(profile: Profile) {
    const nextRegion = profile.defaultRegion || selectedRegion;
    setSelectedProfileId(profile.id);
    setSelectedRegion(nextRegion);
    setProfileMenuOpen(false);
    setContextMessage("");
    const nextContext = setSelectedOCIContext({ profileId: profile.id, region: nextRegion });
    try {
      setReadiness(await getOCIReadiness(nextContext));
      setContextMessage(`已切换到 ${profile.name}`);
    } catch (error) {
      setContextMessage(error instanceof Error ? error.message : "切换 Profile 后刷新 OCI 状态失败");
    }
  }

  async function updateRegion(region: string) {
    setSelectedRegion(region);
    const nextContext = setSelectedOCIContext({ profileId: selectedProfileId, region });
    try {
      setReadiness(await getOCIReadiness(nextContext));
    } catch {
      // Page-level refreshes still receive the new context; keep the UI responsive.
    }
  }

  async function checkLatestVersion(showErrors: boolean) {
    setVersionCheck((current) => ({ ...current, checking: true, error: showErrors ? "" : current.error }));
    try {
      const response = await fetch(GITHUB_LATEST_RELEASE_API, { headers: { Accept: "application/vnd.github+json" } });
      if (!response.ok) throw new Error(`GitHub Release 检查失败：HTTP ${response.status}`);
      const release = (await response.json()) as { tag_name?: string; html_url?: string; prerelease?: boolean };
      const latestVersion = normalizeVersion(release.tag_name || "");
      setVersionCheck({
        latestVersion,
        releaseUrl: release.html_url || GITHUB_PROJECT_URL,
        hasUpdate: latestVersion ? compareVersions(latestVersion, APP_VERSION) > 0 : false,
        checked: true,
        checking: false,
        error: ""
      });
    } catch (error) {
      setVersionCheck((current) => ({
        ...current,
        checked: true,
        checking: false,
        error: showErrors ? (error instanceof Error ? error.message : "版本检查失败") : ""
      }));
    }
  }

  function openVersionModal() {
    setVersionModalOpen(true);
    void checkLatestVersion(true);
  }

  const regionOptions = useMemo(() => {
    const values = profiles.map((profile) => profile.defaultRegion).filter(Boolean);
    return Array.from(new Set(values));
  }, [profiles]);
  const selectedProfile = useMemo(
    () => profiles.find((profile) => profile.id === selectedProfileId) ?? profiles[0],
    [profiles, selectedProfileId]
  );

  const quickStats = useMemo(() => {
    return [
      { label: "模式", value: readiness?.executionMode ?? "local", tone: "neutral" },
      { label: "OCI", value: readiness?.ready ? "就绪" : "未就绪", tone: readiness?.ready ? "success" : "warning" }
    ];
  }, [readiness]);
  const flatNavItems = useMemo(() => navGroups.flatMap((group) => group.items.map((item) => ({ ...item, group: group.label }))), []);
  const currentNavItem = useMemo(() => {
    const sorted = [...flatNavItems].sort((a, b) => b.path.length - a.path.length);
    return sorted.find((item) => (item.path === "/" ? location.pathname === "/" : location.pathname.startsWith(item.path)));
  }, [flatNavItems, location.pathname]);
  const mobilePrimaryNav = useMemo(() => {
    const primaryPaths = ["/", "/instances", "/create", "/network", "/jobs"];
    return primaryPaths.map((path) => flatNavItems.find((item) => item.path === path)).filter(Boolean);
  }, [flatNavItems]);

  return (
    <div className="app-shell">
      <header className="mobile-topbar glass-panel">
        <a className="mobile-brand-button" href={GITHUB_PROJECT_URL} target="_blank" rel="noreferrer" aria-label="打开 GitHub 项目">
          <span className="mobile-brand-mark"><ProductMark size={20} /></span>
          <span>
            <strong>Oracle Cloud</strong>
            <small>{currentNavItem?.label ?? "控制台"}</small>
          </span>
        </a>
        <div className="mobile-topbar-actions">
          <button className="icon-button" aria-label="主题" onClick={() => void toggleTheme()}>
            {appearance.theme === "dark" ? <Sun size={19} /> : <Moon size={19} />}
          </button>
          <Link className="icon-button notification-button" aria-label="通知" to="/notifications">
            <Bell size={19} />
            {unreadNotifications > 0 ? <span>{unreadNotifications > 99 ? "99+" : unreadNotifications}</span> : null}
          </Link>
          <button className="icon-button" aria-label="打开菜单" type="button" onClick={() => setMobileMenuOpen(true)}>
            <Menu size={20} />
          </button>
        </div>
      </header>

      {mobileMenuOpen ? (
        <div className="mobile-drawer-backdrop" role="presentation" onClick={() => setMobileMenuOpen(false)}>
          <aside className="mobile-nav-drawer glass-modal" role="dialog" aria-modal="true" aria-label="移动端导航" onClick={(event) => event.stopPropagation()}>
            <div className="mobile-drawer-header">
              <div>
                <strong>移动控制台</strong>
                <span>{selectedProfile?.name ?? "Profile 未配置"} / {readiness?.ready ? "OCI 就绪" : "OCI 未就绪"}</span>
              </div>
              <button className="icon-button bordered" aria-label="关闭菜单" type="button" onClick={() => setMobileMenuOpen(false)}>
                <X size={18} />
              </button>
            </div>
            <div className="mobile-context-card">
              <label>
                区域
                <select value={selectedRegion} onChange={(event) => void updateRegion(event.target.value)}>
                  {regionOptions.length === 0 ? <option value="">未配置</option> : null}
                  {regionOptions.map((region) => (
                    <option value={region} key={region}>{region}</option>
                  ))}
                </select>
              </label>
              <div>
                <span>模式</span>
                <strong>{readiness?.executionMode ?? "local"}</strong>
              </div>
            </div>
            <nav className="mobile-drawer-nav" aria-label="移动端完整导航">
              {navGroups.map((group) => (
                <div className="mobile-drawer-group" key={group.label}>
                  <div className="mobile-drawer-label">{group.label}</div>
                  {group.items.map((item) => {
                    const Icon = item.icon;
                    return (
                      <NavLink
                        className={({ isActive }) => `mobile-drawer-link ${isActive ? "active" : ""}`}
                        key={item.path}
                        to={item.path}
                        onClick={() => setMobileMenuOpen(false)}
                      >
                        <Icon size={19} />
                        <span>{item.label}</span>
                      </NavLink>
                    );
                  })}
                </div>
              ))}
            </nav>
            <div className="mobile-drawer-actions">
              <button className="secondary-button" type="button" onClick={() => { setMobileMenuOpen(false); navigate("/account"); }}>
                <Settings2 size={17} />
                账号设置
              </button>
              <button className="secondary-button danger" disabled={loggingOut} type="button" onClick={() => void handleLogoutClick()}>
                <LogOut size={17} className={loggingOut ? "spin" : ""} />
                退出登录
              </button>
            </div>
          </aside>
        </div>
      ) : null}

      <aside className="sidebar glass-panel">
        <a className="brand-block" href={GITHUB_PROJECT_URL} target="_blank" rel="noreferrer" aria-label="打开 GitHub 项目">
          <div className="brand-mark">
            <ProductMark size={28} />
          </div>
          <div>
            <div className="brand-title">Oracle Cloud</div>
            <div className="brand-subtitle">机器生命周期平台</div>
          </div>
        </a>

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
            <div className="profile-context-menu-wrap">
              <button
                className={`context-chip profile-context-button ${selectedProfile ? "neutral" : "warning"}`}
                type="button"
                onClick={() => setProfileMenuOpen((value) => !value)}
                aria-expanded={profileMenuOpen}
              >
                <span>Profile</span>
                <strong>{selectedProfile?.name ?? "未配置"}</strong>
                <ChevronDown size={14} />
              </button>
              {profileMenuOpen ? (
                <div className="profile-menu glass-modal">
                  <div className="profile-menu-header">
                    <strong>切换 OCI Profile</strong>
                    <button className="icon-button small" aria-label="关闭 Profile 菜单" type="button" onClick={() => setProfileMenuOpen(false)}>
                      <X size={15} />
                    </button>
                  </div>
                  {profiles.length === 0 ? (
                    <div className="profile-menu-empty">
                      <p>暂无 Profile。</p>
                      <button className="secondary-button compact-button" type="button" onClick={() => { setProfileMenuOpen(false); navigate("/profiles"); }}>
                        去添加
                      </button>
                    </div>
                  ) : (
                    <div className="profile-menu-list">
                      {profiles.map((profile) => (
                        <button
                          className={`profile-menu-item ${profile.id === selectedProfile?.id ? "active" : ""}`}
                          key={profile.id}
                          type="button"
                          onClick={() => void selectProfile(profile)}
                        >
                          <span>
                            <strong>{profile.name}</strong>
                            <small>{profile.defaultRegion}</small>
                          </span>
                          <em>{profile.status || "Unknown"}</em>
                        </button>
                      ))}
                    </div>
                  )}
                  {contextMessage ? <p className="profile-menu-message">{contextMessage}</p> : null}
                </div>
              ) : null}
            </div>
            <label className={`context-chip context-chip-select ${selectedRegion ? "neutral" : "warning"}`}>
              <span>区域</span>
              <select value={selectedRegion} onChange={(event) => void updateRegion(event.target.value)}>
                {regionOptions.length === 0 ? <option value="">未配置</option> : null}
                {regionOptions.map((region) => (
                  <option value={region} key={region}>
                    {region}
                  </option>
                ))}
              </select>
              <ChevronDown size={14} />
            </label>
            {quickStats.map((stat) => (
              <div className={`context-chip ${stat.tone}`} key={stat.label}>
                <span>{stat.label}</span>
                <strong>{stat.value}</strong>
              </div>
            ))}
          </div>

          <div className="topbar-actions">
            <button className="version-button" aria-label={`当前版本 ${APP_VERSION}`} type="button" onClick={openVersionModal}>
              <span>v{APP_VERSION}</span>
              {versionCheck.hasUpdate ? <i aria-label="发现新版本" /> : null}
            </button>
            <button className="icon-button" aria-label="主题" title="切换白天/黑夜背景" onClick={() => void toggleTheme()}>
              {appearance.theme === "dark" ? <Sun size={20} /> : <Moon size={20} />}
            </button>
            <button className="icon-button" aria-label="刷新" onClick={() => window.location.reload()}>
              <RefreshCw size={20} />
            </button>
            <button className="icon-button" aria-label="账号设置" title="账号设置" onClick={() => navigate("/account")}>
              <Settings2 size={20} />
            </button>
            <Link className="icon-button notification-button" aria-label="通知" to="/notifications">
              <Bell size={20} />
              {unreadNotifications > 0 ? <span>{unreadNotifications > 99 ? "99+" : unreadNotifications}</span> : null}
            </Link>
            <button className="icon-button" aria-label="退出" title="退出登录" disabled={loggingOut} onClick={() => void handleLogoutClick()}>
              <LogOut size={20} className={loggingOut ? "spin" : ""} />
            </button>
            <button className="avatar avatar-button" aria-label="账号设置" title="账号设置" onClick={() => navigate("/account")}>
              {account?.avatar ? <img src={account.avatar} alt="账号头像" /> : account?.avatarInitial || "A"}
            </button>
          </div>
        </header>

        <main className="content">{children}</main>
        <footer className="app-footer">
          <a href={GITHUB_PROJECT_URL} target="_blank" rel="noreferrer">
            Powered by OCI-lifecycle-platform
          </a>
        </footer>
      </div>

      {versionModalOpen ? (
        <VersionUpdateModal
          state={versionCheck}
          onClose={() => setVersionModalOpen(false)}
          onRefresh={() => void checkLatestVersion(true)}
        />
      ) : null}

      <nav className="mobile-bottom-nav glass-panel" aria-label="移动端主导航">
        {mobilePrimaryNav.map((item) => {
          if (!item) return null;
          const Icon = item.icon;
          return (
            <NavLink className={({ isActive }) => `mobile-bottom-item ${isActive ? "active" : ""}`} key={item.path} to={item.path}>
              <Icon size={19} />
              <span>{item.label}</span>
            </NavLink>
          );
        })}
      </nav>
    </div>
  );
}

function VersionUpdateModal({
  state,
  onClose,
  onRefresh
}: {
  state: VersionCheckState;
  onClose: () => void;
  onRefresh: () => void;
}) {
  const latest = state.latestVersion || APP_VERSION;
  return (
    <div className="modal-backdrop version-backdrop" role="dialog" aria-modal="true">
      <div className="version-modal glass-modal">
        <h2>版本更新</h2>
        <p>打开窗口后会重新检查当前版本和最新 Release。</p>

        <div className="version-columns">
          <div>
            <span>当前版本</span>
            <strong>v{APP_VERSION}</strong>
          </div>
          <div>
            <span>最新版本</span>
            <strong className={state.hasUpdate ? "version-new" : ""}>v{latest}</strong>
          </div>
        </div>

        <div className="version-channel">
          <div>
            <strong>更新通道</strong>
            <p>稳定版用于正式发布，开发版用于测试标签。</p>
          </div>
          <div className="segmented-control compact">
            <button className="active" type="button">稳定版</button>
            <button type="button">开发版</button>
          </div>
        </div>

        {state.checking ? <div className="version-message checking">正在检查 GitHub Release...</div> : null}
        {!state.checking && state.error ? <div className="version-message danger">{state.error}</div> : null}
        {!state.checking && !state.error && state.hasUpdate ? (
          <div className="version-message success">发现新版本 v{latest}，确认后将更新到该版本。</div>
        ) : null}
        {!state.checking && !state.error && state.checked && !state.hasUpdate ? (
          <div className="version-message">当前已是最新稳定版。</div>
        ) : null}

        <ul className="version-notes">
          <li>升级会替换 compose 并更新 FLUX_VERSION</li>
          <li>升级过程中面板会短暂不可用</li>
          <li>失败时会尝试自动回滚旧配置</li>
        </ul>

        <div className="version-actions">
          <button className="secondary-button" type="button" onClick={onClose}>取消</button>
          <button className="icon-button bordered" aria-label="重新检查" disabled={state.checking} type="button" onClick={onRefresh}>
            <RefreshCw size={18} className={state.checking ? "spin" : ""} />
          </button>
          <a className={`primary-button ${state.hasUpdate ? "" : "disabled"}`} href={state.hasUpdate ? state.releaseUrl : undefined} target="_blank" rel="noreferrer">
            {state.hasUpdate ? `更新到 v${latest}` : "暂无更新"}
          </a>
        </div>
      </div>
    </div>
  );
}

function normalizeVersion(value: string) {
  return String(value || "").trim().replace(/^v/i, "") || "0.0.0";
}

function compareVersions(left: string, right: string) {
  const leftParts = normalizeVersion(left).split(/[.-]/).map((part) => Number.parseInt(part, 10) || 0);
  const rightParts = normalizeVersion(right).split(/[.-]/).map((part) => Number.parseInt(part, 10) || 0);
  const length = Math.max(leftParts.length, rightParts.length);
  for (let index = 0; index < length; index += 1) {
    const diff = (leftParts[index] ?? 0) - (rightParts[index] ?? 0);
    if (diff !== 0) return diff;
  }
  return 0;
}
