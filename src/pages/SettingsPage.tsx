import { Image, Languages, Moon, Save, Settings, Sun } from "lucide-react";
import { useEffect, useState } from "react";
import { PageHeader } from "../components/PageHeader";
import {
  getAppearanceSettings,
  updateAppearanceSettings,
  type AppearanceSettings
} from "../services/api";

const defaultAppearance: AppearanceSettings = {
  theme: "light",
  backgroundMode: "aurora",
  backgroundImage: "",
  language: "zh-CN"
};

const settings = [
  ["安全", "会话超时、密钥加密、操作确认、审计保留"],
  ["任务", "默认超时、Work Request 轮询间隔、重试策略、失败通知"],
  ["自动化", "全局开关、默认冷却时间、每日执行上限、审批边界"],
  ["通知", "邮件、Webhook 路由、任务失败通道、自动化告警"]
];

export function SettingsPage() {
  const [appearance, setAppearance] = useState<AppearanceSettings>(defaultAppearance);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const next = await getAppearanceSettings();
        if (!cancelled) {
          setAppearance(next);
          applyAppearance(next);
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "外观设置加载失败");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  function updateLocal(next: AppearanceSettings) {
    setAppearance(next);
    applyAppearance(next);
  }

  async function handleBackgroundFile(file: File | null) {
    if (!file) return;
    if (file.size > 512 * 1024) {
      setError("背景图片请控制在 512KB 以内");
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      updateLocal({ ...appearance, backgroundMode: "image", backgroundImage: String(reader.result || ""), language: appearance.language || "zh-CN" });
    };
    reader.readAsDataURL(file);
  }

  async function save() {
    setSaving(true);
    setMessage("");
    setError("");
    try {
      const next = await updateAppearanceSettings(appearance);
      setAppearance(next);
      applyAppearance(next);
      setMessage("外观设置已保存");
    } catch (err) {
      setError(err instanceof Error ? err.message : "外观设置保存失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="平台"
        title="系统设置"
        description="配置平台默认策略，让 OCI 操作保持可预测、有边界、可追踪。"
      />

      {loading ? <div className="async-state">正在读取系统设置...</div> : null}
      {message ? <div className="inline-success">{message}</div> : null}
      {error ? <div className="inline-error">{error}</div> : null}

      <section className="glass-panel section-card">
        <div className="section-title-row">
          <div>
            <h2>外观与背景</h2>
            <p>顶部主题按钮会立即切换白天/黑夜；这里可以设置默认背景并持久化。</p>
          </div>
          <Image size={22} />
        </div>

        <div className="choice-grid">
          <button className={`choice-card ${appearance.theme === "light" ? "active" : ""}`} type="button" onClick={() => updateLocal({ ...appearance, theme: "light" })}>
            <Sun size={22} />
            <strong>白天模式</strong>
            <span>浅色玻璃背景，适合日常运维。</span>
          </button>
          <button className={`choice-card ${appearance.theme === "dark" ? "active" : ""}`} type="button" onClick={() => updateLocal({ ...appearance, theme: "dark" })}>
            <Moon size={22} />
            <strong>黑夜模式</strong>
            <span>降低亮度，并调整卡片、文字和背景层级。</span>
          </button>
          <button className={`choice-card ${appearance.backgroundMode === "aurora" ? "active" : ""}`} type="button" onClick={() => updateLocal({ ...appearance, backgroundMode: "aurora", backgroundImage: "" })}>
            <Settings size={22} />
            <strong>柔光背景</strong>
            <span>使用当前 Oracle 风格的柔和渐变。</span>
          </button>
          <button className={`choice-card ${appearance.backgroundMode === "plain" ? "active" : ""}`} type="button" onClick={() => updateLocal({ ...appearance, backgroundMode: "plain", backgroundImage: "" })}>
            <Settings size={22} />
            <strong>纯净背景</strong>
            <span>减少装饰光效，适合长时间监控。</span>
          </button>
        </div>

        <div className="form-section">
          <div className="form-section-title">
            <Image size={18} />
            自定义背景
          </div>
          <div className="toolbar-row">
            <label className="secondary-button compact-button">
              上传背景图
              <input type="file" accept="image/png,image/jpeg,image/webp" hidden onChange={(event) => void handleBackgroundFile(event.target.files?.[0] ?? null)} />
            </label>
            {appearance.backgroundImage ? (
              <button className="secondary-button compact-button" type="button" onClick={() => updateLocal({ ...appearance, backgroundMode: "aurora", backgroundImage: "" })}>
                清除背景图
              </button>
            ) : null}
          </div>
          <p className="muted-line">背景图会以 data URL 保存到平台设置中。为了页面响应速度，建议使用压缩后的 WebP/JPEG。</p>
        </div>

        <div className="button-row">
          <button className="primary-button" type="button" disabled={saving} onClick={() => void save()}>
            <Save size={18} />
            {saving ? "保存中..." : "保存外观设置"}
          </button>
        </div>
      </section>

      <section className="glass-panel section-card">
        <div className="section-title-row">
          <div>
            <h2>语言</h2>
            <p>设置平台界面默认语言。顶部栏不再放语言切换，避免和版本更新入口混在一起。</p>
          </div>
          <Languages size={22} />
        </div>

        <div className="choice-grid">
          <button className={`choice-card ${appearance.language === "zh-CN" ? "active" : ""}`} type="button" onClick={() => updateLocal({ ...appearance, language: "zh-CN" })}>
            <Languages size={22} />
            <strong>简体中文</strong>
            <span>默认语言，适合当前中文控制台。</span>
          </button>
          <button className={`choice-card ${appearance.language === "en-US" ? "active" : ""}`} type="button" onClick={() => updateLocal({ ...appearance, language: "en-US" })}>
            <Languages size={22} />
            <strong>English</strong>
            <span>保留语言位，后续接入完整英文文案。</span>
          </button>
        </div>

        <div className="button-row">
          <button className="primary-button" type="button" disabled={saving} onClick={() => void save()}>
            <Save size={18} />
            {saving ? "保存中..." : "保存语言设置"}
          </button>
        </div>
      </section>

      <div className="card-grid two">
        {settings.map(([title, description]) => (
          <section className="glass-panel section-card" key={title}>
            <div className="section-title-row">
              <h2>{title}</h2>
              <Settings size={22} />
            </div>
            <p>{description}</p>
          </section>
        ))}
      </div>
    </div>
  );
}

function applyAppearance(appearance: AppearanceSettings) {
  document.documentElement.dataset.theme = appearance.theme;
  document.documentElement.dataset.background = appearance.backgroundMode;
  document.documentElement.lang = appearance.language || "zh-CN";
  if (appearance.backgroundMode === "image" && appearance.backgroundImage) {
    document.documentElement.style.setProperty("--custom-background-image", `url("${appearance.backgroundImage}")`);
  } else {
    document.documentElement.style.removeProperty("--custom-background-image");
  }
}
