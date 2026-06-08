import { Eye, RefreshCw, RotateCcw, XCircle } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import { StatusPill } from "../components/StatusPill";
import { cancelJob, getJob, listJobs, retryJob } from "../services/api";
import type { Job } from "../services/api";

const activeStatuses = new Set(["PENDING", "RETRYING", "RUNNING", "WAITING_OCI", "VERIFYING"]);
const retryableStatuses = new Set(["FAILED", "CANCELLED", "ROLLBACK_REQUIRED", "MANUAL_REQUIRED"]);

export function JobsPage() {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [selectedJob, setSelectedJob] = useState<Job | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const [actionError, setActionError] = useState("");

  const hasActiveJobs = useMemo(() => jobs.some((job) => activeStatuses.has(job.status)), [jobs]);

  async function loadJobs({ silent = false } = {}) {
    if (silent) {
      setIsRefreshing(true);
    } else {
      setIsLoading(true);
    }
    setErrorMessage("");
    try {
      const nextJobs = await listJobs();
      setJobs(nextJobs);
      setSelectedJob((current) => {
        if (!current) return null;
        return nextJobs.find((job) => job.id === current.id) ?? current;
      });
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "加载任务失败");
    } finally {
      setIsLoading(false);
      setIsRefreshing(false);
    }
  }

  useEffect(() => {
    void loadJobs();
  }, []);

  useEffect(() => {
    if (!hasActiveJobs) return;
    const timer = window.setInterval(() => {
      void loadJobs({ silent: true });
    }, 1800);
    return () => window.clearInterval(timer);
  }, [hasActiveJobs]);

  async function openJob(jobId: string) {
    setActionError("");
    setActionMessage("");
    try {
      setSelectedJob(await getJob(jobId));
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "读取任务详情失败");
    }
  }

  async function handleCancel(jobId: string) {
    setActionError("");
    setActionMessage("");
    try {
      const job = await cancelJob(jobId);
      setActionMessage(`${job.id} 已取消`);
      await loadJobs({ silent: true });
      setSelectedJob(job);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "取消任务失败");
    }
  }

  async function handleRetry(jobId: string) {
    setActionError("");
    setActionMessage("");
    try {
      const job = await retryJob(jobId);
      setActionMessage(`已创建重试任务 ${job.id}`);
      await loadJobs({ silent: true });
      setSelectedJob(job);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "重试任务失败");
    }
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="异步控制平面"
        title="任务中心"
        description="所有 OCI 变更操作都会作为任务追踪，并关联 Request ID、Work Request ID、参数、结果、重试和审计记录。"
        actions={
          <button className="secondary-button" onClick={() => void loadJobs({ silent: true })} disabled={isRefreshing}>
            <RefreshCw size={16} className={isRefreshing ? "spin" : undefined} />
            刷新
          </button>
        }
      />

      {actionMessage ? <div className="inline-success">{actionMessage}</div> : null}
      {actionError ? <div className="inline-error">{actionError}</div> : null}

      <section className="glass-panel section-card">
        <AsyncState isLoading={isLoading} error={errorMessage} empty={!isLoading && jobs.length === 0} />
        {!isLoading && !errorMessage && jobs.length > 0 ? (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>任务</th>
                  <th>类型</th>
                  <th>资源</th>
                  <th>状态</th>
                  <th>OCI Request ID</th>
                  <th>Work Request</th>
                  <th>创建时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {jobs.map((job) => (
                  <tr key={job.id}>
                    <td><strong>{job.id}</strong></td>
                    <td>{job.type}</td>
                    <td>{job.resourceId}</td>
                    <td><StatusPill status={job.status} /></td>
                    <td>{job.ociRequestId || "-"}</td>
                    <td>{job.ociWorkRequestId || "-"}</td>
                    <td>{formatTime(job.createdAt)}</td>
                    <td>
                      <div className="table-action-row">
                        <button className="secondary-button compact-button" onClick={() => void openJob(job.id)}>
                          <Eye size={14} />
                          查看
                        </button>
                        {activeStatuses.has(job.status) ? (
                          <button className="secondary-button compact-button" onClick={() => void handleCancel(job.id)}>
                            <XCircle size={14} />
                            取消
                          </button>
                        ) : null}
                        {retryableStatuses.has(job.status) && job.retryCount < job.maxRetries ? (
                          <button className="secondary-button compact-button" onClick={() => void handleRetry(job.id)}>
                            <RotateCcw size={14} />
                            重试
                          </button>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>

      {selectedJob ? (
        <JobDetailModal
          job={selectedJob}
          onClose={() => setSelectedJob(null)}
          onCancel={handleCancel}
          onRetry={handleRetry}
        />
      ) : null}
    </div>
  );
}

type JobDetailModalProps = {
  job: Job;
  onClose: () => void;
  onCancel: (jobId: string) => Promise<void>;
  onRetry: (jobId: string) => Promise<void>;
};

function JobDetailModal({ job, onClose, onCancel, onRetry }: JobDetailModalProps) {
  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true">
      <div className="action-modal glass-modal">
        <div className="modal-header-row">
          <div className="modal-title-block">
            <div className="modal-icon compact">
              <RefreshCw size={24} />
            </div>
            <div>
              <h2>{job.id}</h2>
              <p>{job.type} · {job.resourceType} · {job.region}</p>
            </div>
          </div>
          <button className="secondary-button compact-button" onClick={onClose}>关闭</button>
        </div>

        <div className="modal-summary-card">
          <div>
            <span>状态</span>
            <strong><StatusPill status={job.status} /></strong>
          </div>
          <div>
            <span>重试</span>
            <strong>{job.retryCount}/{job.maxRetries}</strong>
          </div>
          <div>
            <span>操作者</span>
            <strong>{job.createdBy || "-"}</strong>
          </div>
        </div>

        <div className="form-section">
          <div className="form-section-title">执行链路</div>
          <dl className="resource-facts">
            <div><dt>资源 ID</dt><dd>{job.resourceId || "-"}</dd></div>
            <div><dt>OCI Request ID</dt><dd>{job.ociRequestId || "-"}</dd></div>
            <div><dt>Work Request ID</dt><dd>{job.ociWorkRequestId || "-"}</dd></div>
            <div><dt>创建时间</dt><dd>{formatDateTime(job.createdAt)}</dd></div>
            <div><dt>开始时间</dt><dd>{formatDateTime(job.startedAt)}</dd></div>
            <div><dt>完成时间</dt><dd>{formatDateTime(job.finishedAt)}</dd></div>
          </dl>
        </div>

        {job.errorCode || job.errorMessage ? (
          <div className="modal-warning">
            <strong>{job.errorCode || "任务错误"}</strong>
            <span>{job.errorMessage}</span>
          </div>
        ) : null}

        <div className="form-grid">
          <JsonBlock title="输入参数" value={job.input} />
          <JsonBlock title="执行结果" value={job.result} />
        </div>

        <div className="button-row">
          {activeStatuses.has(job.status) ? (
            <button className="secondary-button" onClick={() => void onCancel(job.id)}>
              <XCircle size={16} />
              取消任务
            </button>
          ) : null}
          {retryableStatuses.has(job.status) && job.retryCount < job.maxRetries ? (
            <button className="primary-button" onClick={() => void onRetry(job.id)}>
              <RotateCcw size={16} />
              创建重试任务
            </button>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function JsonBlock({ title, value }: { title: string; value?: Record<string, unknown> }) {
  return (
    <div className="form-section">
      <div className="form-section-title">{title}</div>
      <pre className="json-block">{JSON.stringify(value ?? {}, null, 2)}</pre>
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

function formatDateTime(value?: string) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  }).format(new Date(value));
}
