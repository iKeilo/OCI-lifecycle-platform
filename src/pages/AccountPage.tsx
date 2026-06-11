import { Camera, KeyRound, Save, UserRound } from "lucide-react";
import { useEffect, useState } from "react";
import { PageHeader } from "../components/PageHeader";
import {
  getAccountSettings,
  updateAccountPassword,
  updateAccountProfile,
  type AccountSettings
} from "../services/api";

const emptyAccount: AccountSettings = {
  displayName: "Administrator",
  email: "",
  avatar: "",
  avatarInitial: "A",
  passwordSet: false
};

export function AccountPage() {
  const [account, setAccount] = useState<AccountSettings>(emptyAccount);
  const [profileForm, setProfileForm] = useState({ displayName: "Administrator", email: "", avatar: "" });
  const [passwordForm, setPasswordForm] = useState({ currentPassword: "", newPassword: "", repeatPassword: "" });
  const [loading, setLoading] = useState(true);
  const [savingProfile, setSavingProfile] = useState(false);
  const [savingPassword, setSavingPassword] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const next = await getAccountSettings();
        if (!cancelled) {
          setAccount(next);
          setProfileForm({
            displayName: next.displayName || "Administrator",
            email: next.email || "",
            avatar: next.avatar || ""
          });
          setError("");
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "账号设置加载失败");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleAvatarFile(file: File | null) {
    if (!file) return;
    if (file.size > 256 * 1024) {
      setError("头像文件请控制在 256KB 以内");
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      setProfileForm((current) => ({ ...current, avatar: String(reader.result || "") }));
    };
    reader.readAsDataURL(file);
  }

  async function saveProfile() {
    setSavingProfile(true);
    setMessage("");
    setError("");
    try {
      const next = await updateAccountProfile(profileForm);
      setAccount(next);
      setProfileForm({
        displayName: next.displayName,
        email: next.email || "",
        avatar: next.avatar || ""
      });
      setMessage("账号资料已保存");
    } catch (err) {
      setError(err instanceof Error ? err.message : "账号资料保存失败");
    } finally {
      setSavingProfile(false);
    }
  }

  async function savePassword() {
    setMessage("");
    setError("");
    if (passwordForm.newPassword !== passwordForm.repeatPassword) {
      setError("两次输入的新密码不一致");
      return;
    }
    setSavingPassword(true);
    try {
      const next = await updateAccountPassword({
        currentPassword: passwordForm.currentPassword,
        newPassword: passwordForm.newPassword
      });
      setAccount(next);
      setPasswordForm({ currentPassword: "", newPassword: "", repeatPassword: "" });
      setMessage("密码已更新，当前会话已重新签发");
    } catch (err) {
      setError(err instanceof Error ? err.message : "密码修改失败");
    } finally {
      setSavingPassword(false);
    }
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="账号"
        title="账号设置"
        description="管理面板账号的头像、显示名称和登录密码。修改密码必须提交原密码。"
      />

      {loading ? <div className="async-state">正在读取账号设置...</div> : null}
      {message ? <div className="inline-success">{message}</div> : null}
      {error ? <div className="inline-error">{error}</div> : null}

      <div className="split-layout">
        <section className="glass-panel section-card">
          <div className="section-title-row">
            <div>
              <h2>个人资料</h2>
              <p>头像只保存在平台设置里，不会写入 OCI 账号。</p>
            </div>
            <UserRound size={22} />
          </div>

          <div className="account-profile-row">
            <div className="avatar large">
              {profileForm.avatar ? <img src={profileForm.avatar} alt="账号头像" /> : account.avatarInitial || "A"}
            </div>
            <label className="secondary-button compact-button">
              <Camera size={16} />
              上传头像
              <input type="file" accept="image/png,image/jpeg,image/webp" hidden onChange={(event) => void handleAvatarFile(event.target.files?.[0] ?? null)} />
            </label>
            {profileForm.avatar ? (
              <button className="secondary-button compact-button" type="button" onClick={() => setProfileForm((current) => ({ ...current, avatar: "" }))}>
                清除头像
              </button>
            ) : null}
          </div>

          <div className="form-grid">
            <label>
              显示名称
              <input value={profileForm.displayName} onChange={(event) => setProfileForm((current) => ({ ...current, displayName: event.target.value }))} />
            </label>
            <label>
              邮箱
              <input value={profileForm.email} placeholder="admin@example.com" onChange={(event) => setProfileForm((current) => ({ ...current, email: event.target.value }))} />
            </label>
          </div>

          <div className="button-row">
            <button className="primary-button" type="button" disabled={savingProfile} onClick={() => void saveProfile()}>
              <Save size={18} />
              {savingProfile ? "保存中..." : "保存资料"}
            </button>
          </div>
        </section>

        <section className="glass-panel section-card">
          <div className="section-title-row">
            <div>
              <h2>修改密码</h2>
              <p>需要验证当前密码，成功后立即更新面板登录密码。</p>
            </div>
            <KeyRound size={22} />
          </div>

          <div className="form-grid">
            <label>
              原密码
              <input type="password" value={passwordForm.currentPassword} onChange={(event) => setPasswordForm((current) => ({ ...current, currentPassword: event.target.value }))} />
            </label>
            <label>
              新密码
              <input type="password" value={passwordForm.newPassword} onChange={(event) => setPasswordForm((current) => ({ ...current, newPassword: event.target.value }))} />
            </label>
            <label>
              重复新密码
              <input type="password" value={passwordForm.repeatPassword} onChange={(event) => setPasswordForm((current) => ({ ...current, repeatPassword: event.target.value }))} />
            </label>
            <div className="form-note">
              <strong>密码要求</strong>
              <span>至少 8 个字符。密码 hash 会持久化保存，接口不会返回明文或 hash。</span>
            </div>
          </div>

          <div className="button-row">
            <button className="primary-button" type="button" disabled={savingPassword} onClick={() => void savePassword()}>
              <KeyRound size={18} />
              {savingPassword ? "修改中..." : "修改密码"}
            </button>
          </div>
        </section>
      </div>
    </div>
  );
}
