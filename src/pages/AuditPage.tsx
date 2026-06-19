import { Download, FileJson, RefreshCw, ScrollText, ShieldCheck, TriangleAlert } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import type { AuditLog, AuditLogFilter } from "../services/api";
import { listAuditLogs } from "../services/api";

const statusOptions = [
  { value: "", label: "全部结果" },
  { value: "success", label: "成功" },
  { value: "failed", label: "失败" }
] as const;

const defaultFilter: AuditLogFilter = { limit: 100, status: "" };

export function AuditPage() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [filter, setFilter] = useState<AuditLogFilter>(defaultFilter);
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");

  const summary = useMemo(() => {
    const failed = logs.filter(isFailedAudit).length;
    const success = logs.length - failed;
    const actors = new Set(logs.map((log) => safeText(log.actor)).filter(Boolean)).size;
    return { success, failed, actors };
  }, [logs]);

  async function loadLogs({ silent = false } = {}) {
    if (silent) {
      setIsRefreshing(true);
    } else {
      setIsLoading(true);
    }
    setErrorMessage("");
    try {
      const nextLogs = await listAuditLogs(cleanFilter(filter));
      setLogs(Array.isArray(nextLogs) ? nextLogs : []);
      setSelectedLog((current) => {
        if (!current) return null;
        return nextLogs.find((log) => log.id === current.id) ?? null;
      });
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "加载审计日志失败");
    } finally {
      setIsLoading(false);
      setIsRefreshing(false);
    }
  }

  useEffect(() => {
    void loadLogs();
  }, []);

  function updateFilter(key: keyof AuditLogFilter, value: string | number) {
    setFilter((current) => ({ ...current, [key]: value }));
  }

  function resetFilter() {
    setFilter(defaultFilter);
    setSelectedLog(null);
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="平台治理"
        title="审计日志"
        description="集中查看真实后端审计记录，追踪操作者、资源、OCI Request ID、Work Request、请求参数、执行结果和错误原因。"
        actions={
          <div className="table-action-row">
            <button className="secondary-button" onClick={() => exportAuditLogs(logs, "json")} disabled={logs.length === 0}>
              <FileJson size={16} />
              导出 JSON
            </button>
            <button className="secondary-button" onClick={() => exportAuditLogs(logs, "csv")} disabled={logs.length === 0}>
              <Download size={16} />
              导出 CSV
            </button>
            <button className="secondary-button" onClick={() => void loadLogs({ silent: true })} disabled={isRefreshing}>
              <RefreshCw size={16} className={isRefreshing ? "spin" : undefined} />
              刷新
            </button>
          </div>
        }
      />

      <section className="audit-summary-grid">
        <div className="metric-card">
          <div className="metric-topline">
            <span className="metric-title">当前记录</span>
            <ScrollText size={18} />
          </div>
          <div className="metric-value">{logs.length}</div>
        </div>
        <div className="metric-card green">
          <div className="metric-topline">
            <span className="metric-title">成功操作</span>
            <ShieldCheck size={18} />
          </div>
          <div className="metric-value">{summary.success}</div>
        </div>
        <div className="metric-card yellow">
          <div className="metric-topline">
            <span className="metric-title">失败/异常</span>
            <TriangleAlert size={18} />
          </div>
          <div className="metric-value">{summary.failed}</div>
        </div>
        <div className="metric-card blue">
          <div className="metric-topline">
            <span className="metric-title">操作者</span>
            <ShieldCheck size={18} />
          </div>
          <div className="metric-value">{summary.actors}</div>
        </div>
      </section>

      <section className="glass-panel section-card">
        <div className="section-title-row">
          <div>
            <h2>筛选条件</h2>
            <p>筛选在后端执行，适合排查某个 Profile、区域、实例、Request ID 或失败操作。</p>
          </div>
          <ScrollText size={22} />
        </div>
        <div className="form-grid audit-filter-grid">
          <label>
            操作者
            <input value={filter.actor ?? ""} onChange={(event) => updateFilter("actor", event.target.value)} placeholder="admin" />
          </label>
          <label>
            动作
            <input value={filter.action ?? ""} onChange={(event) => updateFilter("action", event.target.value)} placeholder="job.SUCCESS" />
          </label>
          <label>
            资源类型
            <input value={filter.resourceType ?? ""} onChange={(event) => updateFilter("resourceType", event.target.value)} placeholder="instance / job / profile" />
          </label>
          <label>
            资源 ID
            <input value={filter.resourceId ?? ""} onChange={(event) => updateFilter("resourceId", event.target.value)} placeholder="ocid1.instance..." />
          </label>
          <label>
            Profile
            <input value={filter.profileId ?? ""} onChange={(event) => updateFilter("profileId", event.target.value)} placeholder="profile-default" />
          </label>
          <label>
            区域
            <input value={filter.region ?? ""} onChange={(event) => updateFilter("region", event.target.value)} placeholder="ap-chuncheon-1" />
          </label>
          <label>
            Compartment
            <input value={filter.compartmentId ?? ""} onChange={(event) => updateFilter("compartmentId", event.target.value)} placeholder="ocid1.compartment..." />
          </label>
          <label>
            OCI Request ID
            <input value={filter.ociRequestId ?? ""} onChange={(event) => updateFilter("ociRequestId", event.target.value)} placeholder="request id" />
          </label>
          <label>
            Work Request
            <input value={filter.ociWorkRequestId ?? ""} onChange={(event) => updateFilter("ociWorkRequestId", event.target.value)} placeholder="work request id" />
          </label>
          <label>
            结果
            <select value={filter.status ?? ""} onChange={(event) => updateFilter("status", event.target.value)}>
              {statusOptions.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </select>
          </label>
          <label>
            条数
            <input
              type="number"
              min={10}
              max={500}
              value={filter.limit ?? 100}
              onChange={(event) => updateFilter("limit", clampLimit(Number(event.target.value)))}
            />
          </label>
        </div>
        <div className="toolbar-row">
          <button className="primary-button" onClick={() => void loadLogs({ silent: true })}>应用筛选</button>
          <button className="secondary-button" onClick={resetFilter}>重置</button>
        </div>
      </section>

      <section className="glass-panel section-card">
        <AsyncState isLoading={isLoading} error={errorMessage} empty={!isLoading && logs.length === 0} emptyText="暂无审计日志。" />
        {!isLoading && !errorMessage && logs.length > 0 ? (
          <div className="table-wrap audit-table-wrap">
            <table>
              <thead>
                <tr>
                  <th>时间</th>
                  <th>操作者</th>
                  <th>动作</th>
                  <th>资源</th>
                  <th>Profile / 区域</th>
                  <th>OCI Request ID</th>
                  <th>结果</th>
                  <th>详情</th>
                </tr>
              </thead>
              <tbody>
                {logs.map((log) => (
                  <tr key={log.id}>
                    <td>{formatTime(log.createdAt)}</td>
                    <td>{safeText(log.actor, "-")}</td>
                    <td><strong>{safeText(log.action, "-")}</strong></td>
                    <td>
                      {safeText(log.resourceType, "-")}
                      <br />
                      <span className="muted-cell">{safeText(log.resourceId, "-")}</span>
                    </td>
                    <td>
                      {safeText(log.profileId, "-")}
                      <br />
                      <span className="muted-cell">{safeText(log.region, "-")}</span>
                    </td>
                    <td className="mono-cell">{safeText(log.ociRequestId, "-")}</td>
                    <td>{isFailedAudit(log) ? <span className="danger-text">失败</span> : <span className="success-text">成功</span>}</td>
                    <td>
                      <button className="secondary-button compact-button" onClick={() => setSelectedLog(log)}>查看</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>

      {selectedLog ? (
        <section className="glass-panel section-card audit-detail-panel">
          <div className="section-title-row">
            <div>
              <h2>审计详情 #{selectedLog.id}</h2>
              <p>{safeText(selectedLog.action, "未知动作")} / {safeText(selectedLog.resourceId, "无资源 ID")}</p>
            </div>
            <button className="secondary-button compact-button" onClick={() => setSelectedLog(null)}>关闭</button>
          </div>
          <div className="detail-grid">
            <div><span>操作者</span><strong>{safeText(selectedLog.actor, "-")}</strong></div>
            <div><span>资源类型</span><strong>{safeText(selectedLog.resourceType, "-")}</strong></div>
            <div><span>区域</span><strong>{safeText(selectedLog.region, "-")}</strong></div>
            <div><span>Compartment</span><strong>{safeText(selectedLog.compartmentId, "-")}</strong></div>
            <div><span>OCI Request</span><strong>{safeText(selectedLog.ociRequestId, "-")}</strong></div>
            <div><span>Work Request</span><strong>{safeText(selectedLog.ociWorkRequestId, "-")}</strong></div>
            <div><span>错误码</span><strong>{safeText(selectedLog.errorCode, "-")}</strong></div>
            <div><span>错误信息</span><strong>{safeText(selectedLog.errorMessage, "-")}</strong></div>
          </div>
          <pre className="code-preview">{safeJSONStringify({
            requestPayload: selectedLog.requestPayload ?? {},
            resultPayload: selectedLog.resultPayload ?? {}
          })}</pre>
        </section>
      ) : null}
    </div>
  );
}

function cleanFilter(filter: AuditLogFilter): AuditLogFilter {
  return Object.fromEntries(
    Object.entries(filter).filter(([, value]) => value !== "" && value !== undefined && value !== null)
  ) as AuditLogFilter;
}

function clampLimit(value: number) {
  if (!Number.isFinite(value)) return 100;
  return Math.max(10, Math.min(500, Math.trunc(value)));
}

function isFailedAudit(log: AuditLog) {
  return Boolean(safeText(log.errorCode) || safeText(log.errorMessage));
}

function safeText(value: unknown, fallback = "") {
  if (value === null || value === undefined) return fallback;
  return String(value).trim() || fallback;
}

function safeJSONStringify(value: unknown) {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return "{\n  \"error\": \"无法序列化审计详情\"\n}";
  }
}

function formatTime(value: string) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  }).format(date);
}

function exportAuditLogs(logs: AuditLog[], format: "json" | "csv") {
  if (logs.length === 0) return;
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
  if (format === "json") {
    downloadText(`audit-logs-${timestamp}.json`, JSON.stringify(logs, null, 2), "application/json;charset=utf-8");
    return;
  }
  const headers = ["id", "createdAt", "actor", "action", "resourceType", "resourceId", "profileId", "region", "compartmentId", "ociRequestId", "ociWorkRequestId", "status", "errorCode", "errorMessage"];
  const rows = logs.map((log) => [
    log.id,
    log.createdAt,
    log.actor,
    log.action,
    log.resourceType,
    log.resourceId,
    log.profileId,
    log.region,
    log.compartmentId,
    log.ociRequestId,
    log.ociWorkRequestId,
    isFailedAudit(log) ? "failed" : "success",
    log.errorCode ?? "",
    log.errorMessage ?? ""
  ]);
  const csv = [headers, ...rows].map((row) => row.map(csvCell).join(",")).join("\n");
  downloadText(`audit-logs-${timestamp}.csv`, csv, "text/csv;charset=utf-8");
}

function csvCell(value: unknown) {
  const text = value === null || value === undefined ? "" : String(value);
  return `"${text.replace(/"/g, '""')}"`;
}

function downloadText(filename: string, content: string, type: string) {
  const blob = new Blob([content], { type });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}
