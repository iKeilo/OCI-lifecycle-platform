import { Link2, Mail, Send, Settings } from "lucide-react";
import { useEffect, useState } from "react";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import {
  getEmailSettings,
  getWebhookSettings,
  testEmail,
  testWebhook,
  updateEmailSettings,
  updateWebhookSettings
} from "../services/api";
import type { EmailSettings, WebhookSettings } from "../services/api";

const emptySettings: EmailSettings = {
  enabled: false,
  host: "",
  port: 587,
  username: "",
  password: "",
  passwordSet: false,
  from: "",
  to: [],
  useTls: false,
  startTls: true
};

const emptyWebhookSettings: WebhookSettings = {
  enabled: false,
  url: "",
  secret: "",
  secretSet: false,
  headers: {}
};

export function EmailPage() {
  const [settings, setSettings] = useState<EmailSettings>(emptySettings);
  const [webhookSettings, setWebhookSettings] = useState<WebhookSettings>(emptyWebhookSettings);
  const [toText, setToText] = useState("");
  const [testTo, setTestTo] = useState("");
  const [headersText, setHeadersText] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [isSavingWebhook, setIsSavingWebhook] = useState(false);
  const [isTesting, setIsTesting] = useState(false);
  const [isTestingWebhook, setIsTestingWebhook] = useState(false);
  const [message, setMessage] = useState("");
  const [errorMessage, setErrorMessage] = useState("");

  useEffect(() => {
    async function load() {
      setIsLoading(true);
      setErrorMessage("");
      try {
        const [emailValue, webhookValue] = await Promise.all([getEmailSettings(), getWebhookSettings()]);
        setSettings({ ...emptySettings, ...emailValue, password: "" });
        setToText((emailValue.to ?? []).join(", "));
        setTestTo((emailValue.to ?? [])[0] ?? "");
        setWebhookSettings({ ...emptyWebhookSettings, ...webhookValue, secret: "" });
        setHeadersText(headersToText(webhookValue.headers ?? {}));
      } catch (error) {
        setErrorMessage(error instanceof Error ? error.message : "加载通知设置失败");
      } finally {
        setIsLoading(false);
      }
    }
    void load();
  }, []);

  async function saveEmail() {
    setIsSaving(true);
    setMessage("");
    setErrorMessage("");
    try {
      const saved = await updateEmailSettings({
        ...settings,
        to: toText.split(",").map((item) => item.trim()).filter(Boolean)
      });
      setSettings({ ...saved, password: "" });
      setToText((saved.to ?? []).join(", "));
      setMessage("邮件设置已保存");
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "保存邮件设置失败");
    } finally {
      setIsSaving(false);
    }
  }

  async function saveWebhook() {
    setIsSavingWebhook(true);
    setMessage("");
    setErrorMessage("");
    try {
      const saved = await updateWebhookSettings({
        ...webhookSettings,
        headers: parseHeaders(headersText)
      });
      setWebhookSettings({ ...saved, secret: "" });
      setHeadersText(headersToText(saved.headers ?? {}));
      setMessage("Webhook 设置已保存");
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "保存 Webhook 设置失败");
    } finally {
      setIsSavingWebhook(false);
    }
  }

  async function sendTest() {
    setIsTesting(true);
    setMessage("");
    setErrorMessage("");
    try {
      const result = await testEmail(testTo);
      if (!result.verified) {
        setErrorMessage(result.message);
      } else {
        setMessage(result.message || "测试邮件已发送");
      }
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "测试邮件发送失败");
    } finally {
      setIsTesting(false);
    }
  }

  async function sendWebhookTest() {
    setIsTestingWebhook(true);
    setMessage("");
    setErrorMessage("");
    try {
      const result = await testWebhook();
      if (!result.verified) {
        setErrorMessage(result.message);
      } else {
        setMessage(result.message || "测试 Webhook 已发送");
      }
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "测试 Webhook 发送失败");
    } finally {
      setIsTestingWebhook(false);
    }
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="通知"
        title="通知通道"
        description="配置 SMTP 和 Webhook 后，站内通知可以同步推送到邮箱或外部系统。敏感通知在 Webhook 中只发送脱敏提示。"
      />

      <AsyncState isLoading={isLoading} error="" empty={false} />
      {message ? <div className="inline-success">{message}</div> : null}
      {errorMessage ? <div className="inline-error">{errorMessage}</div> : null}

      {!isLoading ? (
        <>
          <section className="glass-panel section-card">
            <div className="form-section-title">
              <Settings size={18} />
              <span>SMTP 配置</span>
            </div>
            <div className="form-grid">
              <label>
                SMTP Host
                <input value={settings.host} onChange={(event) => setSettings({ ...settings, host: event.target.value })} placeholder="smtp.example.com" />
              </label>
              <label>
                SMTP Port
                <input type="number" value={settings.port} onChange={(event) => setSettings({ ...settings, port: Number(event.target.value) })} />
              </label>
              <label>
                Username
                <input value={settings.username} onChange={(event) => setSettings({ ...settings, username: event.target.value })} />
              </label>
              <label>
                Password
                <input
                  type="password"
                  value={settings.password ?? ""}
                  onChange={(event) => setSettings({ ...settings, password: event.target.value })}
                  placeholder={settings.passwordSet ? "已设置，留空则保留" : "SMTP 密码或授权码"}
                />
              </label>
              <label>
                From
                <input value={settings.from} onChange={(event) => setSettings({ ...settings, from: event.target.value })} placeholder="OCI Panel <panel@example.com>" />
              </label>
              <label>
                To
                <input value={toText} onChange={(event) => setToText(event.target.value)} placeholder="admin@example.com, ops@example.com" />
              </label>
            </div>

            <div className="switch-panel compact-switches">
              <div className="switch-row">
                <div>
                  <strong>启用邮件推送</strong>
                  <p>开启后，站内通知会尝试同步发往默认收件人。</p>
                </div>
                <button className={`toggle-switch ${settings.enabled ? "on" : ""}`} onClick={() => setSettings({ ...settings, enabled: !settings.enabled })} />
              </div>
              <div className="switch-row">
                <div>
                  <strong>STARTTLS</strong>
                  <p>适用于 587 端口，大多数 SMTP 服务使用此模式。</p>
                </div>
                <button className={`toggle-switch ${settings.startTls ? "on" : ""}`} onClick={() => setSettings({ ...settings, startTls: !settings.startTls })} />
              </div>
              <div className="switch-row">
                <div>
                  <strong>TLS 直连</strong>
                  <p>适用于 465 端口；开启后不会再走明文连接。</p>
                </div>
                <button className={`toggle-switch ${settings.useTls ? "on" : ""}`} onClick={() => setSettings({ ...settings, useTls: !settings.useTls })} />
              </div>
            </div>

            <div className="form-actions">
              <button className="primary-button" onClick={() => void saveEmail()} disabled={isSaving}>
                <Mail size={18} />
                {isSaving ? "保存中..." : "保存邮件设置"}
              </button>
            </div>

            <div className="email-test-row">
              <label>
                测试收件人
                <input value={testTo} onChange={(event) => setTestTo(event.target.value)} placeholder="留空使用默认收件人" />
              </label>
              <button className="secondary-button" onClick={() => void sendTest()} disabled={isTesting}>
                <Send size={18} />
                {isTesting ? "发送中..." : "发送测试邮件"}
              </button>
            </div>
          </section>

          <section className="glass-panel section-card">
            <div className="form-section-title">
              <Link2 size={18} />
              <span>Webhook 配置</span>
            </div>
            <div className="form-grid">
              <label>
                Webhook URL
                <input value={webhookSettings.url} onChange={(event) => setWebhookSettings({ ...webhookSettings, url: event.target.value })} placeholder="https://example.com/webhook" />
              </label>
              <label>
                Secret
                <input
                  type="password"
                  value={webhookSettings.secret ?? ""}
                  onChange={(event) => setWebhookSettings({ ...webhookSettings, secret: event.target.value })}
                  placeholder={webhookSettings.secretSet ? "已设置，留空则保留" : "可选 HMAC secret"}
                />
              </label>
            </div>
            <label className="wide-label">
              Headers
              <textarea value={headersText} onChange={(event) => setHeadersText(event.target.value)} placeholder="X-Token: abc&#10;X-Env: prod" />
            </label>
            <div className="switch-panel compact-switches">
              <div className="switch-row">
                <div>
                  <strong>启用 Webhook 推送</strong>
                  <p>开启后，站内通知会向 Webhook URL 发送 JSON。敏感内容会脱敏。</p>
                </div>
                <button className={`toggle-switch ${webhookSettings.enabled ? "on" : ""}`} onClick={() => setWebhookSettings({ ...webhookSettings, enabled: !webhookSettings.enabled })} />
              </div>
            </div>
            <div className="form-actions">
              <button className="primary-button" onClick={() => void saveWebhook()} disabled={isSavingWebhook}>
                <Link2 size={18} />
                {isSavingWebhook ? "保存中..." : "保存 Webhook 设置"}
              </button>
              <button className="secondary-button" onClick={() => void sendWebhookTest()} disabled={isTestingWebhook}>
                <Send size={18} />
                {isTestingWebhook ? "发送中..." : "发送测试 Webhook"}
              </button>
            </div>
          </section>
        </>
      ) : null}
    </div>
  );
}

function parseHeaders(value: string) {
  const headers: Record<string, string> = {};
  value.split("\n").forEach((line) => {
    const index = line.indexOf(":");
    if (index <= 0) return;
    const key = line.slice(0, index).trim();
    const headerValue = line.slice(index + 1).trim();
    if (key) headers[key] = headerValue;
  });
  return headers;
}

function headersToText(headers: Record<string, string>) {
  return Object.entries(headers).map(([key, value]) => `${key}: ${value}`).join("\n");
}
