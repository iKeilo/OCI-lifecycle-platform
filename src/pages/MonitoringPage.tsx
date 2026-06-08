import { Activity, BellRing, Cpu, Network } from "lucide-react";
import { MetricCard } from "../components/MetricCard";
import { PageHeader } from "../components/PageHeader";

export function MonitoringPage() {
  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="可观测性"
        title="监控告警"
        description="展示 OCI 指标和告警，用于健康检查、容量决策和自动化触发。"
      />
      <div className="metric-grid">
        <MetricCard title="CPU" value="43%" detail="Normal" icon={Cpu} progress={43} accent="blue" />
        <MetricCard title="网络" value="1.2 Gbps" detail="Healthy" icon={Network} progress={62} accent="green" />
        <MetricCard title="告警" value="0" detail="Quiet" icon={BellRing} progress={0} accent="green" />
      </div>
      <section className="glass-panel section-card">
        <div className="section-title-row">
          <h2>自动化信号就绪度</h2>
          <Activity size={22} />
        </div>
        <div className="guardrail-grid">
          <div><span>指标规则</span><strong>5</strong></div>
          <div><span>健康采集器</span><strong>98%</strong></div>
          <div><span>告警通道</span><strong>3</strong></div>
          <div><span>最近采集</span><strong>30 秒</strong></div>
        </div>
      </section>
    </div>
  );
}
