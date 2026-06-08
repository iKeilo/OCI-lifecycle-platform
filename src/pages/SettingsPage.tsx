import { Settings } from "lucide-react";
import { PageHeader } from "../components/PageHeader";

const settings = [
  ["安全", "会话超时、密钥加密、操作确认、审计保留"],
  ["任务", "默认超时、Work Request 轮询间隔、重试次数、失败通知"],
  ["自动化", "全局开关、默认冷却时间、每日执行上限、重试策略"],
  ["通知", "邮件、Webhook 路由、任务失败通道、自动化告警"]
];

export function SettingsPage() {
  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="平台"
        title="系统设置"
        description="配置平台默认策略，让 OCI 操作保持可预测、有边界、可追踪。"
      />
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
