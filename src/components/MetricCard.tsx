import type { LucideIcon } from "lucide-react";

type MetricCardProps = {
  title: string;
  value: string;
  detail: string;
  accent?: "red" | "blue" | "green" | "yellow";
  icon: LucideIcon;
  progress?: number;
};

const detailLabels: Record<string, string> = {
  Healthy: "正常",
  Warning: "警告",
  Normal: "正常",
  Quiet: "无告警"
};

export function MetricCard({ title, value, detail, icon: Icon, accent = "red", progress = 0 }: MetricCardProps) {
  return (
    <section className={`metric-card glass-panel ${accent}`}>
      <div className="metric-topline">
        <div className="metric-icon">
          <Icon size={22} />
        </div>
        <span className={`status-pill ${detail.toLowerCase().replace(/\s+/g, "-")}`}>
          {detailLabels[detail] ?? detail}
        </span>
      </div>
      <div className="metric-title">{title}</div>
      <div className="metric-value">{value}</div>
      <div className="progress-track" aria-label={`${title} utilization`}>
        <span style={{ width: `${progress}%` }} />
      </div>
    </section>
  );
}
