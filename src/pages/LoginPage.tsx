import { LockKeyhole, ShieldCheck } from "lucide-react";
import { useState, type FormEvent } from "react";
import { login } from "../services/api";

type LoginPageProps = {
  onAuthenticated: () => void;
};

export function LoginPage({ onAuthenticated }: LoginPageProps) {
  const [password, setPassword] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSubmitting(true);
    setErrorMessage("");
    try {
      await login(password);
      onAuthenticated();
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "登录失败");
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <main className="auth-screen">
      <section className="auth-card glass-panel">
        <div className="auth-mark">
          <ShieldCheck size={28} />
        </div>
        <div>
          <p className="eyebrow">Panel Login</p>
          <h1>登录 OCI 机器生命周期平台</h1>
          <p className="muted">请输入面板登录密码。OCI API 密钥只保存在后端，不会进入浏览器。</p>
        </div>

        <form className="auth-form" onSubmit={handleSubmit}>
          <label>
            面板密码
            <div className="auth-input">
              <LockKeyhole size={18} />
              <input
                autoComplete="current-password"
                autoFocus
                minLength={8}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="输入安装时设置的密码"
                required
                type="password"
                value={password}
              />
            </div>
          </label>
          {errorMessage ? <div className="inline-error">{errorMessage}</div> : null}
          <button className="primary-button full" disabled={isSubmitting} type="submit">
            {isSubmitting ? "登录中..." : "登录控制台"}
          </button>
        </form>
      </section>
    </main>
  );
}
