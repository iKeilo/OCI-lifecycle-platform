import { useEffect, useState } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "../components/AppShell";
import { AccountPage } from "../pages/AccountPage";
import { AuditPage } from "../pages/AuditPage";
import { AutomationsPage } from "../pages/AutomationsPage";
import { CreateInstancePage } from "../pages/CreateInstancePage";
import { DashboardPage } from "../pages/DashboardPage";
import { EmailPage } from "../pages/EmailPage";
import { InstancesPage } from "../pages/InstancesPage";
import { JobsPage } from "../pages/JobsPage";
import { LoginPage } from "../pages/LoginPage";
import { MonitoringPage } from "../pages/MonitoringPage";
import { NetworkPage } from "../pages/NetworkPage";
import { NotificationsPage } from "../pages/NotificationsPage";
import { PlaceholderPage } from "../pages/PlaceholderPage";
import { ProfilesPage } from "../pages/ProfilesPage";
import { ResourcePoolsPage } from "../pages/ResourcePoolsPage";
import { SettingsPage } from "../pages/SettingsPage";
import { TemplatesPage } from "../pages/TemplatesPage";
import { UsersPage } from "../pages/UsersPage";
import { getAuthStatus, logout, type AuthStatus } from "../services/api";

export function App() {
  const [authStatus, setAuthStatus] = useState<AuthStatus | null>(null);
  const [authError, setAuthError] = useState("");

  useEffect(() => {
    let cancelled = false;
    async function loadAuthStatus() {
      try {
        const status = await getAuthStatus();
        if (!cancelled) {
          setAuthStatus(status);
          setAuthError("");
        }
      } catch (error) {
        if (!cancelled) {
          setAuthStatus({ authEnabled: true, authenticated: false });
          setAuthError(error instanceof Error ? error.message : "无法连接后端认证服务");
        }
      }
    }
    void loadAuthStatus();
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleLogout() {
    try {
      const status = await logout();
      setAuthStatus(status.authEnabled ? status : { authEnabled: false, authenticated: true });
    } catch {
      setAuthStatus({ authEnabled: true, authenticated: false });
    }
  }

  if (!authStatus) {
    return (
      <main className="auth-screen">
        <section className="auth-card glass-panel">
          <p className="eyebrow">Panel Login</p>
          <h1>正在检查登录状态</h1>
          <p className="muted">正在连接后端 API...</p>
        </section>
      </main>
    );
  }

  if (!authStatus.authenticated) {
    return <LoginPage onAuthenticated={() => setAuthStatus({ authEnabled: true, authenticated: true })} />;
  }

  return (
    <AppShell onLogout={handleLogout}>
      {authError ? <div className="inline-error">{authError}</div> : null}
      <Routes>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/account" element={<AccountPage />} />
        <Route path="/profiles" element={<ProfilesPage />} />
        <Route path="/instances" element={<InstancesPage />} />
        <Route path="/create" element={<CreateInstancePage />} />
        <Route path="/templates" element={<TemplatesPage />} />
        <Route path="/pools" element={<ResourcePoolsPage />} />
        <Route path="/network" element={<NetworkPage />} />
        <Route path="/automations" element={<AutomationsPage />} />
        <Route path="/jobs" element={<JobsPage />} />
        <Route path="/monitoring" element={<MonitoringPage />} />
        <Route path="/audit" element={<AuditPage />} />
        <Route path="/users" element={<UsersPage />} />
        <Route path="/email" element={<EmailPage />} />
        <Route path="/guardrails" element={<PlaceholderPage title="安全护栏" subtitle="自动化、预算、区域和危险操作的全局限制将在这里配置。" />} />
        <Route path="/notifications" element={<NotificationsPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </AppShell>
  );
}
