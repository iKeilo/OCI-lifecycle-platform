import {
  ArrowUpDown,
  Globe2,
  Grid2X2,
  List,
  MoreVertical,
  Plus,
  Power,
  RefreshCw,
  Search,
  Server,
  ShieldAlert,
  Square,
  Terminal,
  Trash2,
  X,
  Zap
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import { StatusPill } from "../components/StatusPill";
import { createIPTask, createInstanceAction, listInstances } from "../services/api";
import type { Instance, InstanceActionPayload } from "../services/api";

const statusFilters: Array<{ value: "All" | Instance["status"]; label: string }> = [
  { value: "All", label: "全部实例" },
  { value: "Running", label: "运行中" },
  { value: "Stopped", label: "已停止" },
  { value: "Provisioning", label: "创建中" },
  { value: "Terminated", label: "已终止" }
];

const ipModes = ["保留当前公网 IP", "分配临时公网 IP", "绑定保留公网 IP", "释放公网 IP"];

export function InstancesPage() {
  const [statusFilter, setStatusFilter] = useState<"All" | Instance["status"]>("All");
  const [instances, setInstances] = useState<Instance[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [errorMessage, setErrorMessage] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const [actionError, setActionError] = useState("");
  const [selectedIpInstance, setSelectedIpInstance] = useState<Instance | null>(null);
  const [pendingAction, setPendingAction] = useState<{ instance: Instance; action: InstanceActionPayload["action"] } | null>(null);
  const [resizeInstance, setResizeInstance] = useState<Instance | null>(null);

  const reloadInstances = useCallback(async () => {
    setIsLoading(true);
    setErrorMessage("");
    try {
      setInstances(await listInstances());
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "加载实例失败");
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void reloadInstances();
  }, [reloadInstances]);

  const filteredInstances = useMemo(() => {
    if (statusFilter === "All") return instances;
    return instances.filter((instance) => instance.status === statusFilter);
  }, [instances, statusFilter]);

  async function submitAction(instance: Instance, action: InstanceActionPayload["action"], overrides: Partial<InstanceActionPayload> = {}) {
    setActionMessage("");
    setActionError("");
    try {
      const job = await createInstanceAction(instance.id, {
        action,
        graceful: true,
        preserveBootVolume: true,
        targetShape: "",
        targetOcpus: 0,
        targetMemoryGb: 0,
        snapshotBefore: true,
        note: "",
        ...overrides
      });
      setActionMessage(`已创建任务 ${job.id}，可在任务中心查看执行状态。`);
      await reloadInstances();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "创建实例操作任务失败");
    }
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="计算资源"
        title="实例管理"
        description="在当前配置、区域和隔间下管理 OCI Compute 实例；所有生命周期操作都会进入任务中心执行。"
        actions={
          <Link className="primary-button" to="/create">
            <Plus size={19} />
            创建实例
          </Link>
        }
      />

      {actionMessage ? <div className="inline-success">{actionMessage} <Link className="link-button" to="/jobs">查看任务中心</Link></div> : null}
      {actionError ? <div className="inline-error">{actionError}</div> : null}

      <section className="toolbar glass-panel">
        <div className="segmented-control">
          {statusFilters.map((status) => (
            <button
              className={statusFilter === status.value ? "active" : ""}
              key={status.value}
              onClick={() => setStatusFilter(status.value)}
            >
              {status.label}
            </button>
          ))}
        </div>
        <div className="toolbar-actions">
          <div className="inline-search">
            <Search size={18} />
            <input placeholder="按名称、IP、OCID 或 Shape 筛选..." />
          </div>
          <button className="icon-button bordered" aria-label="刷新实例" onClick={reloadInstances}>
            <RefreshCw size={18} />
          </button>
          <button className="icon-button bordered" aria-label="表格视图">
            <List size={18} />
          </button>
          <button className="icon-button bordered active" aria-label="卡片视图">
            <Grid2X2 size={18} />
          </button>
        </div>
      </section>

      <AsyncState
        isLoading={isLoading}
        error={errorMessage}
        empty={!isLoading && !errorMessage && filteredInstances.length === 0}
        emptyText="当前筛选条件下没有实例"
      />

      {!isLoading && !errorMessage ? (
        <div className="instance-grid">
          {filteredInstances.map((instance) => (
            <article className="instance-card glass-panel" key={instance.id}>
              <div className="instance-card-header">
                <div className="resource-icon">
                  <Server size={22} />
                </div>
                <div>
                  <h2>{instance.name}</h2>
                  <p>{instance.created}</p>
                </div>
                <button className="icon-button small" aria-label="更多操作">
                  <MoreVertical size={18} />
                </button>
              </div>

              <dl className="resource-facts">
                <div><dt>规格</dt><dd>{instance.shape}</dd></div>
                <div><dt>区域</dt><dd>{instance.region}</dd></div>
                <div><dt>公网 IP</dt><dd className="linkish">{instance.primaryIp}</dd></div>
                <div><dt>私网 IP</dt><dd>{instance.privateIp}</dd></div>
                <div><dt>配置</dt><dd>{instance.ocpus} OCPU / {instance.memoryGb} GB</dd></div>
              </dl>

              <div className="instance-card-footer">
                <StatusPill status={instance.status} />
                <div className="instance-actions">
                  <button className="secondary-button">
                    <Terminal size={16} />
                    SSH
                  </button>
                  <button className="secondary-button" onClick={() => setSelectedIpInstance(instance)} disabled={instance.status === "Terminated"}>
                    <Globe2 size={16} />
                    IP 管理
                  </button>
                  {instance.status === "Stopped" ? (
                    <button className="secondary-button" onClick={() => setPendingAction({ instance, action: "START" })}>
                      <Zap size={16} />
                      启动
                    </button>
                  ) : (
                    <button className="secondary-button" onClick={() => setPendingAction({ instance, action: "STOP" })} disabled={instance.status !== "Running"}>
                      <Square size={16} />
                      停止
                    </button>
                  )}
                  <button className="secondary-button" onClick={() => setPendingAction({ instance, action: "REBOOT" })} disabled={instance.status !== "Running"}>
                    <Power size={16} />
                    重启
                  </button>
                  <button className="secondary-button" onClick={() => setResizeInstance(instance)} disabled={instance.status === "Terminated"}>
                    <ArrowUpDown size={16} />
                    升降级
                  </button>
                  <button className="secondary-button" onClick={() => setPendingAction({ instance, action: "TERMINATE" })} disabled={instance.status === "Terminated"}>
                    <Trash2 size={16} />
                    终止
                  </button>
                </div>
              </div>
            </article>
          ))}
        </div>
      ) : null}

      {selectedIpInstance ? (
        <IpManagementModal
          instance={selectedIpInstance}
          onClose={() => setSelectedIpInstance(null)}
          onCreated={reloadInstances}
        />
      ) : null}

      {pendingAction ? (
        <ConfirmActionModal
          instance={pendingAction.instance}
          action={pendingAction.action}
          onClose={() => setPendingAction(null)}
          onConfirm={async (preserveBootVolume) => {
            await submitAction(pendingAction.instance, pendingAction.action, { preserveBootVolume });
            setPendingAction(null);
          }}
        />
      ) : null}

      {resizeInstance ? (
        <ResizeModal
          instance={resizeInstance}
          onClose={() => setResizeInstance(null)}
          onSubmit={async (payload) => {
            await submitAction(resizeInstance, "RESIZE", payload);
            setResizeInstance(null);
          }}
        />
      ) : null}
    </div>
  );
}

function ConfirmActionModal({
  instance,
  action,
  onClose,
  onConfirm
}: {
  instance: Instance;
  action: InstanceActionPayload["action"];
  onClose: () => void;
  onConfirm: (preserveBootVolume: boolean) => Promise<void>;
}) {
  const [preserveBootVolume, setPreserveBootVolume] = useState(true);
  const label = actionLabel(action);
  const isDanger = action === "TERMINATE";

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true">
      <div className="confirm-modal glass-modal">
        <div className={`modal-icon ${isDanger ? "danger" : ""}`}>
          {isDanger ? <Trash2 size={34} /> : <Power size={34} />}
        </div>
        <h2>{label}实例？</h2>
        <p>你将对 <strong>{instance.name}</strong> 创建一条可审计任务。系统会在任务中心追踪状态、Request ID 和执行结果。</p>
        {isDanger ? (
          <div className="switch-row">
            <div>
              <strong>保留启动盘</strong>
              <p>关闭后代表终止实例并删除启动盘，属于高风险操作。</p>
            </div>
            <button className={`toggle-switch ${preserveBootVolume ? "on" : ""}`} onClick={() => setPreserveBootVolume((value) => !value)} />
          </div>
        ) : null}
        <button className="primary-button full" onClick={() => void onConfirm(preserveBootVolume)}>
          确认{label}
        </button>
        <button className="secondary-button full" onClick={onClose}>取消</button>
      </div>
    </div>
  );
}

function ResizeModal({
  instance,
  onClose,
  onSubmit
}: {
  instance: Instance;
  onClose: () => void;
  onSubmit: (payload: Partial<InstanceActionPayload>) => Promise<void>;
}) {
  const [targetShape, setTargetShape] = useState(instance.shape);
  const [targetOcpus, setTargetOcpus] = useState(instance.ocpus);
  const [targetMemoryGb, setTargetMemoryGb] = useState(instance.memoryGb);
  const [snapshotBefore, setSnapshotBefore] = useState(true);

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true">
      <div className="action-modal glass-modal">
        <div className="modal-header-row">
          <div className="modal-title-block">
            <div className="modal-icon compact">
              <ArrowUpDown size={24} />
            </div>
            <div>
              <h2>升降级实例</h2>
              <p>{instance.name} 当前为 {instance.shape} / {instance.ocpus} OCPU / {instance.memoryGb} GB。</p>
            </div>
          </div>
          <button className="icon-button bordered" aria-label="关闭升降级" onClick={onClose}>
            <X size={18} />
          </button>
        </div>
        <div className="form-section">
          <div className="form-grid">
            <label>
              目标 Shape
              <input value={targetShape} onChange={(event) => setTargetShape(event.target.value)} />
            </label>
            <label>
              目标 OCPU
              <input type="number" min={1} value={targetOcpus} onChange={(event) => setTargetOcpus(Number(event.target.value))} />
            </label>
            <label>
              目标内存 GB
              <input type="number" min={1} value={targetMemoryGb} onChange={(event) => setTargetMemoryGb(Number(event.target.value))} />
            </label>
          </div>
        </div>
        <div className="switch-row">
          <div>
            <strong>变更前记录快照</strong>
            <p>记录原 Shape、OCPU、内存和任务输入，便于失败后回滚判断。</p>
          </div>
          <button className={`toggle-switch ${snapshotBefore ? "on" : ""}`} onClick={() => setSnapshotBefore((value) => !value)} />
        </div>
        <div className="modal-warning">
          <ShieldAlert size={18} />
          <span>OCI 实例升降级可能触发重启。提交后会创建任务，由后端执行并验证状态。</span>
        </div>
        <div className="button-row">
          <button className="secondary-button" onClick={onClose}>取消</button>
          <button
            className="primary-button"
            onClick={() => void onSubmit({ targetShape, targetOcpus, targetMemoryGb, snapshotBefore })}
          >
            创建升降级任务
          </button>
        </div>
      </div>
    </div>
  );
}

function IpManagementModal({
  instance,
  onClose,
  onCreated
}: {
  instance: Instance;
  onClose: () => void;
  onCreated: () => Promise<void>;
}) {
  const [mode, setMode] = useState(ipModes[0]);
  const [reservedPublicIp, setReservedPublicIp] = useState("");
  const [dnsLabel, setDNSLabel] = useState(instance.name);
  const [vnicId, setVnicId] = useState("primary");
  const [note, setNote] = useState("");
  const [enableIPv6, setEnableIPv6] = useState(false);
  const [snapshotBefore, setSnapshotBefore] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [resultMessage, setResultMessage] = useState("");
  const [errorMessage, setErrorMessage] = useState("");

  async function handleCreateTask() {
    setIsSubmitting(true);
    setResultMessage("");
    setErrorMessage("");
    try {
      const job = await createIPTask(instance.id, {
        mode,
        reservedPublicIp,
        dnsLabel,
        vnicId,
        note,
        enableIpv6: enableIPv6,
        snapshotBefore
      });
      setResultMessage(`已创建 IP 管理任务 ${job.id}`);
      void onCreated();
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "创建 IP 管理任务失败");
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true">
      <div className="action-modal glass-modal">
        <div className="modal-header-row">
          <div className="modal-title-block">
            <div className="modal-icon compact">
              <Globe2 size={24} />
            </div>
            <div>
              <h2>IP 管理</h2>
              <p>{instance.name} 的公网 IP、私网 IP 与保留 IP 操作。</p>
            </div>
          </div>
          <button className="icon-button bordered" aria-label="关闭 IP 管理" onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className="modal-summary-card">
          <div><span>公网 IP</span><strong>{instance.primaryIp}</strong></div>
          <div><span>私网 IP</span><strong>{instance.privateIp}</strong></div>
          <div><span>区域</span><strong>{instance.region}</strong></div>
        </div>

        <div className="form-section">
          <div className="form-section-title">
            <Globe2 size={18} />
            <span>公网 IP 策略</span>
          </div>
          <div className="choice-grid">
            {ipModes.map((ipMode) => (
              <button className={`choice-card ${ipMode === mode ? "active" : ""}`} key={ipMode} onClick={() => setMode(ipMode)}>
                <strong>{ipMode}</strong>
                <span>提交后会创建任务，后端统一执行并记录审计。</span>
              </button>
            ))}
          </div>
        </div>

        <div className="form-section">
          <div className="form-grid">
            <label>
              保留公网 IP
              <select value={reservedPublicIp} onChange={(event) => setReservedPublicIp(event.target.value)}>
                <option value="">暂无真实保留公网 IP 目录</option>
              </select>
            </label>
            <label>
              DNS 标签
              <input value={dnsLabel} onChange={(event) => setDNSLabel(event.target.value)} />
            </label>
            <label>
              主 VNIC
              <select value={vnicId} onChange={(event) => setVnicId(event.target.value)}>
                <option value="primary">primary-vnic / {instance.privateIp}</option>
              </select>
            </label>
            <label>
              任务备注
              <input value={note} onChange={(event) => setNote(event.target.value)} placeholder="例如：迁移公网 IP 到新实例" />
            </label>
          </div>
        </div>

        <div className="switch-panel">
          <div className="switch-row">
            <div>
              <strong>启用 IPv6</strong>
              <p>如果当前子网支持 IPv6，可创建任务为实例分配 IPv6 地址。</p>
            </div>
            <button className={`toggle-switch ${enableIPv6 ? "on" : ""}`} onClick={() => setEnableIPv6((value) => !value)} />
          </div>
          <div className="switch-row">
            <div>
              <strong>操作前创建快照记录</strong>
              <p>记录当前 IP、VNIC 和安全组信息，用于审计与回滚判断。</p>
            </div>
            <button className={`toggle-switch ${snapshotBefore ? "on" : ""}`} onClick={() => setSnapshotBefore((value) => !value)} />
          </div>
        </div>

        <div className="modal-warning">
          <ShieldAlert size={18} />
          <span>释放公网 IP 或更换保留 IP 会影响 SSH、业务访问和 DNS 解析。</span>
        </div>
        {resultMessage ? <div className="inline-success">{resultMessage}</div> : null}
        {errorMessage ? <div className="inline-error">{errorMessage}</div> : null}

        <div className="button-row">
          <button className="secondary-button" onClick={onClose}>取消</button>
          <button className="primary-button" disabled={isSubmitting} onClick={handleCreateTask}>
            {isSubmitting ? "创建中..." : "创建 IP 任务"}
          </button>
        </div>
      </div>
    </div>
  );
}

function actionLabel(action: InstanceActionPayload["action"]) {
  switch (action) {
    case "START":
      return "启动";
    case "STOP":
      return "停止";
    case "REBOOT":
      return "重启";
    case "TERMINATE":
      return "终止";
    case "RESIZE":
      return "升降级";
    default:
      return "操作";
  }
}
