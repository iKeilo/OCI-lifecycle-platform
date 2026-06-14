import { AlertCircle, CheckCircle2, Cpu, Database, KeyRound, RefreshCw, Server } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { getSelectedOCIContext, onOCIContextChange } from "../app/ociContext";
import { AsyncState } from "../components/AsyncState";
import { MetricCard } from "../components/MetricCard";
import { PageHeader } from "../components/PageHeader";
import { StatusPill } from "../components/StatusPill";
import { getOCIReadiness, listInstances, listJobs, listProfiles } from "../services/api";
import type { Instance, Job, OCIReadiness, Profile } from "../services/api";

export function DashboardPage() {
  const [instances, setInstances] = useState<Instance[]>([]);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [readiness, setReadiness] = useState<OCIReadiness | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [errorMessage, setErrorMessage] = useState("");

  async function load() {
    setIsLoading(true);
    setErrorMessage("");
    try {
      const context = getSelectedOCIContext();
      const [nextInstances, nextJobs, nextProfiles, nextReadiness] = await Promise.all([
        listInstances({ profileId: context.profileId, region: context.region }),
        listJobs(),
        listProfiles(),
        getOCIReadiness(context)
      ]);
      setInstances(nextInstances);
      setJobs(nextJobs);
      setProfiles(nextProfiles);
      setReadiness(nextReadiness);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "加载控制台数据失败");
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    void load();
    return onOCIContextChange(() => void load());
  }, []);

  const runningCount = useMemo(() => instances.filter((item) => item.status === "Running").length, [instances]);
  const stoppedCount = useMemo(() => instances.filter((item) => item.status === "Stopped").length, [instances]);
  const failedJobs = useMemo(() => jobs.filter((job) => job.status === "FAILED").length, [jobs]);
  const activeJobs = useMemo(
    () => jobs.filter((job) => ["PENDING", "RETRYING", "RUNNING", "WAITING_OCI", "VERIFYING"].includes(job.status)).length,
    [jobs]
  );
  const recentJobs = jobs.slice(0, 6);

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="真实控制台"
        title="OCI 机器生命周期控制台"
        description="这里仅展示后端 API 返回的真实控制面数据。没有配置 Profile、数据库或 OCI 凭据时，页面会保持空状态。"
        actions={
          <button className="secondary-button" onClick={() => void load()}>
            <RefreshCw size={16} />
            刷新
          </button>
        }
      />

      <AsyncState isLoading={isLoading} error={errorMessage} />

      {!isLoading && !errorMessage ? (
        <>
          <section className="hero-status glass-panel">
            <div className="hero-status-main">
              <div className="hero-check">
                {readiness?.ready ? <CheckCircle2 size={32} /> : <AlertCircle size={32} />}
              </div>
              <div>
                <h2>{readiness?.ready ? "OCI API 已就绪" : "等待真实 OCI 配置"}</h2>
                <p>{readiness?.message ?? "正在读取后端 readiness"}</p>
              </div>
            </div>
            <div className="hero-stat">
              <span>Profile</span>
              <strong>{profiles.length}</strong>
            </div>
            <div className="hero-stat">
              <span>实例</span>
              <strong>{instances.length}</strong>
            </div>
            <div className="hero-stat">
              <span>活动任务</span>
              <strong>{activeJobs}</strong>
            </div>
          </section>

          <div className="metric-grid">
            <MetricCard title="运行实例" value={String(runningCount)} detail="来自 /api/instances" icon={Server} progress={instances.length ? Math.round((runningCount / instances.length) * 100) : 0} accent="blue" />
            <MetricCard title="已停止实例" value={String(stoppedCount)} detail="来自真实库存或数据库" icon={Cpu} progress={instances.length ? Math.round((stoppedCount / instances.length) * 100) : 0} accent="yellow" />
            <MetricCard title="失败任务" value={String(failedJobs)} detail="来自任务中心" icon={Database} progress={jobs.length ? Math.round((failedJobs / jobs.length) * 100) : 0} accent="red" />
          </div>

          <div className="dashboard-grid">
            <section className="glass-panel section-card">
              <div className="section-title-row">
                <h2>最近任务</h2>
                <StatusPill status={activeJobs ? "RUNNING" : "Healthy"} />
              </div>
              <AsyncState isLoading={false} error="" empty={recentJobs.length === 0} emptyText="暂无任务。创建实例、生命周期操作或自动化规则后会出现在这里。" />
              {recentJobs.length > 0 ? (
                <div className="activity-list">
                  {recentJobs.map((job) => (
                    <div className="activity-item" key={job.id}>
                      <div className="activity-icon">
                        {job.status === "FAILED" ? <AlertCircle size={18} /> : <CheckCircle2 size={18} />}
                      </div>
                      <div>
                        <strong>{job.type}</strong>
                        <p>{job.resourceId || job.resourceType}</p>
                        <span>{formatTime(job.createdAt)}</span>
                      </div>
                      <StatusPill status={job.status} />
                    </div>
                  ))}
                </div>
              ) : null}
            </section>

            <section className="glass-panel section-card">
              <div className="section-title-row">
                <h2>Profile 状态</h2>
                <KeyRound size={22} />
              </div>
              <AsyncState isLoading={false} error="" empty={profiles.length === 0} emptyText="暂无 Profile。请进入“账号与密钥”添加 OCI API signing key 配置。" />
              {profiles.length > 0 ? (
                <dl className="resource-facts">
                  {profiles.map((profile) => (
                    <div key={profile.id}>
                      <dt>{profile.name}</dt>
                      <dd>{profile.defaultRegion} · {profile.status}</dd>
                    </div>
                  ))}
                </dl>
              ) : null}
            </section>
          </div>
        </>
      ) : null}
    </div>
  );
}

function formatTime(value: string) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value));
}
