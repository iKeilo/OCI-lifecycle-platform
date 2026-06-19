import { KeyRound, Plus, Save, Shield, Trash2, UserRoundCheck, Users } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import {
  getAccessControlSettings,
  setAccessUserPassword,
  updateAccessControlSettings,
  type AccessControlSettings,
  type AccessUser
} from "../services/api";

const emptySettings: AccessControlSettings = { enabled: true, roles: [], users: [] };

export function UsersPage() {
  const [settings, setSettings] = useState<AccessControlSettings>(emptySettings);
  const [selectedUserId, setSelectedUserId] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const selectedUser = useMemo(() => settings.users.find((user) => user.id === selectedUserId) ?? settings.users[0], [settings.users, selectedUserId]);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const next = await getAccessControlSettings();
        if (!cancelled) {
          setSettings(normalizeAccessSettings(next));
          setSelectedUserId(next.users[0]?.id ?? "");
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "加载用户与权限失败");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  function updateUser(userId: string, patch: Partial<AccessUser>) {
    setSettings((current) => ({
      ...current,
      users: current.users.map((user) => user.id === userId ? { ...user, ...patch } : user)
    }));
  }

  function addUser() {
    const id = uniqueUserId(settings.users);
    setSettings((current) => ({
      ...current,
      users: [...current.users, {
        id,
        displayName: "New Operator",
        email: "",
        roleId: "operator",
        status: "active",
        allowedProfiles: [],
        allowedRegions: [],
        allowedCompartments: [],
        passwordSet: false
      }]
    }));
    setSelectedUserId(id);
  }

  function removeUser(userId: string) {
    if (userId === "admin") {
      setError("内置 admin 用户不能删除");
      return;
    }
    setSettings((current) => ({ ...current, users: current.users.filter((user) => user.id !== userId) }));
    if (selectedUserId === userId) setSelectedUserId("admin");
  }

  async function saveSettings() {
    setSaving(true);
    setMessage("");
    setError("");
    try {
      const next = await updateAccessControlSettings(settings);
      setSettings(normalizeAccessSettings(next));
      setMessage("用户与权限已保存");
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存用户与权限失败");
    } finally {
      setSaving(false);
    }
  }

  async function savePassword() {
    if (!selectedUser) return;
    setSaving(true);
    setMessage("");
    setError("");
    try {
      const next = await setAccessUserPassword(selectedUser.id, password);
      setPassword("");
      setSettings(normalizeAccessSettings(next));
      setMessage(`${selectedUser.displayName || selectedUser.id} 的登录密码已更新`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "更新用户密码失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="访问控制"
        title="用户与权限"
        description="通过平台 RBAC 控制密钥、实例生命周期、网络、自动化、审计和系统设置访问。用户范围限制会在 API 层校验，避免绕过 UI 提权。"
        actions={
          <div className="table-action-row">
            <button className="secondary-button" type="button" onClick={addUser}>
              <Plus size={16} />
              添加用户
            </button>
            <button className="primary-button" type="button" disabled={saving} onClick={() => void saveSettings()}>
              <Save size={16} />
              保存权限
            </button>
          </div>
        }
      />

      {message ? <div className="inline-success">{message}</div> : null}
      {error ? <div className="inline-error">{error}</div> : null}
      <AsyncState isLoading={loading} error="" />

      {!loading ? (
        <>
          <div className="card-grid two">
            {settings.roles.map((role) => (
              <section className="glass-panel section-card" key={role.id}>
                <div className="section-title-row">
                  <h2>{role.name}</h2>
                  {role.id === "super_admin" ? <Shield size={24} /> : role.id === "operator" ? <UserRoundCheck size={24} /> : <Users size={24} />}
                </div>
                <p>{role.description}</p>
                <div className="guardrail-list">
                  {role.permissions.map((permission) => <li key={permission}>{permission}</li>)}
                </div>
              </section>
            ))}
          </div>

          <section className="glass-panel section-card">
            <div className="section-title-row">
              <div>
                <h2>用户列表</h2>
                <p>禁用用户会立即阻止新请求。范围为空表示不限制；填写后必须命中对应 Profile、区域或 Compartment。</p>
              </div>
              <Users size={22} />
            </div>
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>用户</th>
                    <th>角色</th>
                    <th>状态</th>
                    <th>密码</th>
                    <th>范围</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {settings.users.map((user) => (
                    <tr key={user.id}>
                      <td>
                        <strong>{user.displayName}</strong>
                        <br />
                        <span className="muted-cell">{user.id} · {user.email || "未设置邮箱"}</span>
                      </td>
                      <td>{roleName(settings, user.roleId)}</td>
                      <td>{user.status === "active" ? "启用" : "禁用"}</td>
                      <td>{user.passwordSet ? "已设置" : user.id === "admin" ? "使用面板密码" : "未设置"}</td>
                      <td className="muted-cell">
                        Profile: {displayScope(user.allowedProfiles)}
                        <br />
                        Region: {displayScope(user.allowedRegions)}
                        <br />
                        Compartment: {displayScope(user.allowedCompartments)}
                      </td>
                      <td>
                        <div className="table-action-row">
                          <button className="secondary-button compact-button" type="button" onClick={() => setSelectedUserId(user.id)}>编辑</button>
                          {user.id !== "admin" ? (
                            <button className="secondary-button compact-button danger" type="button" onClick={() => removeUser(user.id)}>
                              <Trash2 size={14} />
                              删除
                            </button>
                          ) : null}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>

          {selectedUser ? (
            <section className="glass-panel section-card">
              <div className="section-title-row">
                <div>
                  <h2>编辑用户：{selectedUser.id}</h2>
                  <p>用户 ID 会被后端规范化，禁止路径穿越字符；密码仅保存 bcrypt hash。</p>
                </div>
                <KeyRound size={22} />
              </div>
              <div className="form-grid">
                <label>
                  用户 ID
                  <input value={selectedUser.id} disabled={selectedUser.passwordSet || selectedUser.id === "admin"} onChange={(event) => updateUser(selectedUser.id, { id: event.target.value })} />
                </label>
                <label>
                  显示名称
                  <input value={selectedUser.displayName} onChange={(event) => updateUser(selectedUser.id, { displayName: event.target.value })} />
                </label>
                <label>
                  邮箱
                  <input value={selectedUser.email} onChange={(event) => updateUser(selectedUser.id, { email: event.target.value })} />
                </label>
                <label>
                  角色
                  <select value={selectedUser.roleId} onChange={(event) => updateUser(selectedUser.id, { roleId: event.target.value })}>
                    {settings.roles.map((role) => <option key={role.id} value={role.id}>{role.name}</option>)}
                  </select>
                </label>
                <label>
                  状态
                  <select value={selectedUser.status} onChange={(event) => updateUser(selectedUser.id, { status: event.target.value })}>
                    <option value="active">启用</option>
                    <option value="disabled">禁用</option>
                  </select>
                </label>
                <label>
                  允许 Profile
                  <input value={(selectedUser.allowedProfiles ?? []).join(", ")} onChange={(event) => updateUser(selectedUser.id, { allowedProfiles: splitList(event.target.value) })} placeholder="留空为不限制" />
                </label>
                <label>
                  允许区域
                  <input value={(selectedUser.allowedRegions ?? []).join(", ")} onChange={(event) => updateUser(selectedUser.id, { allowedRegions: splitList(event.target.value) })} placeholder="ap-chuncheon-1, us-ashburn-1" />
                </label>
                <label>
                  允许 Compartment
                  <input value={(selectedUser.allowedCompartments ?? []).join(", ")} onChange={(event) => updateUser(selectedUser.id, { allowedCompartments: splitList(event.target.value) })} placeholder="ocid1.compartment..." />
                </label>
              </div>
              <div className="toolbar-row">
                <input type="password" value={password} minLength={8} onChange={(event) => setPassword(event.target.value)} placeholder="设置或重置该用户密码" />
                <button className="secondary-button" type="button" disabled={saving || password.length < 8} onClick={() => void savePassword()}>
                  <KeyRound size={16} />
                  设置密码
                </button>
              </div>
            </section>
          ) : null}
        </>
      ) : null}
    </div>
  );
}

function normalizeAccessSettings(settings: AccessControlSettings): AccessControlSettings {
  return { enabled: true, roles: settings.roles ?? [], users: settings.users ?? [] };
}

function splitList(value: string) {
  return value.split(",").map((item) => item.trim()).filter(Boolean);
}

function displayScope(values?: string[]) {
  return values && values.length > 0 ? values.join(", ") : "全部";
}

function roleName(settings: AccessControlSettings, roleId: string) {
  return settings.roles.find((role) => role.id === roleId)?.name ?? roleId;
}

function uniqueUserId(users: AccessUser[]) {
  let index = users.length + 1;
  let id = `operator-${index}`;
  const existing = new Set(users.map((user) => user.id));
  while (existing.has(id)) {
    index += 1;
    id = `operator-${index}`;
  }
  return id;
}
