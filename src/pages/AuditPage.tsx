import { RefreshCw, ScrollText } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import { AuditLog, AuditLogFilter, listAuditLogs } from "../services/api";

const statusOptions = [
  { value: "", label: "全部结果" },
  { value: "success", label: "成功" },
  { value: "failed", label: "失败" }
] as const;

export function AuditPage() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [filter, setFilter] = useState<AuditLogFilter>({ limit: 100, status: "" });
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");

  const failedCount = useMemo(() => logs.filter((log) => log.errorCode || log.errorMessage).length, [logs]);

  async function loadLogs({ silent = false } = {}) {
    if (silent) {
      setIsRefreshing(true);
    } else {
      setIsLoading(true);
    }
    setErrorMessage("");
    try {
      const nextLogs = await listAuditLogs(filter);
      setLogs(nextLogs);
      setSelectedLog((current) => {
        if (!current) return null;
        return nextLogs.find((log) => log.id === current.id) ?? current;
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

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="合规"
        title="审计日志"
        description="展示真实后端审计记录，包括操作者、资源、OCI Request ID、Work Request ID、任务结果和错误信息。"
        actions={
          <button className="secondary-button" onClick={() => void loadLogs({ silent: true })} disabled={isRefreshing}>
            <RefreshCw size={16} className={isRefreshing ? "spin" : undefined} />
            刷新
          </button>
        }
      />

      <section className="glass-panel section-card">
        <div className="section-title-row">
          <div>
            <h2>审计筛选</h2>
            <p>当前显示 {logs.length} 条记录，失败/异常 {failedCount} 条。</p>
          </div>
          <ScrollText size={22} />
        </div>
        <div className="form-grid">
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
            <input value={filter.resourceType ?? ""} onChange={(event) => updateFilter("resourceType", event.target.value)} placeholder="instance" />
          </label>
          <label>
            Profile
            <input value={filter.profileId ?? ""} onChange={(event) => updateFilter("profileId", event.target.value)} placeholder="profile-default" />
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
              onChange={(event) => updateFilter("limit", Number(event.target.value))}
            />
          </label>
        </div>
        <div className="toolbar-row">
          <button className="primary-button" onClick={() => void loadLogs({ silent: true })}>应用筛选</button>
          <button className="secondary-button" onClick={() => setFilter({ limit: 100, status: "" })}>重置</button>
        </div>
      </section>

      <section className="glass-panel section-card">
        <AsyncState isLoading={isLoading} error={errorMessage} empty={!isLoading && logs.length === 0} emptyText="暂无审计日志。" />
        {!isLoading && !errorMessage && logs.length > 0 ? (
          <div className="table-wrap">
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
                    <td>{log.actor || "-"}</td>
                    <td><strong>{log.action}</strong></td>
                    <td>{log.resourceType || "-"} / {log.resourceId || "-"}</td>
                    <td>{log.profileId || "-"}<br />{log.region || "-"}</td>
                    <td>{log.ociRequestId || "-"}</td>
                    <td>{log.errorCode || log.errorMessage ? <span className="danger-text">失败</span> : <span className="success-text">成功</span>}</td>
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
        <section className="glass-panel section-card">
          <div className="section-title-row">
            <div>
              <h2>审计详情 #{selectedLog.id}</h2>
              <p>{selectedLog.action} / {selectedLog.resourceId || "无资源 ID"}</p>
            </div>
            <button className="secondary-button compact-button" onClick={() => setSelectedLog(null)}>关闭</button>
          </div>
          <div className="detail-grid">
            <div><span>Work Request</span><strong>{selectedLog.ociWorkRequestId || "-"}</strong></div>
            <div><span>Compartment</span><strong>{selectedLog.compartmentId || "-"}</strong></div>
            <div><span>错误码</span><strong>{selectedLog.errorCode || "-"}</strong></div>
            <div><span>错误信息</span><strong>{selectedLog.errorMessage || "-"}</strong></div>
          </div>
          <pre className="code-preview">{JSON.stringify({
            requestPayload: selectedLog.requestPayload ?? {},
            resultPayload: selectedLog.resultPayload ?? {}
          }, null, 2)}</pre>
        </section>
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
