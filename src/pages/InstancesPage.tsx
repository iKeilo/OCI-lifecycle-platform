import {
  ArrowUpDown,
  Globe2,
  Grid2X2,
  HardDrive,
  List,
  MoreVertical,
  Plus,
  Power,
  PowerOff,
  RefreshCw,
  Search,
  Server,
  ShieldAlert,
  Square,
  Trash2,
  X,
  Zap
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { getSelectedOCIContext, onOCIContextChange } from "../app/ociContext";
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
  { value: "Terminating", label: "正在终止" },
  { value: "Terminated", label: "已终止" }
];

const ipModes = ["保留当前公网 IP", "分配临时公网 IP", "绑定保留公网 IP", "释放公网 IP"];
const HOURS_PER_DAY = 24;
const HOURS_PER_MONTH = 730;
const ALWAYS_FREE_E2_MICRO_COUNT = 2;
const ALWAYS_FREE_A1_OCPUS = 4;
const ALWAYS_FREE_A1_MEMORY_GB = 24;
const ALWAYS_FREE_BOOT_VOLUME_GB = 200;
const ALWAYS_FREE_BOOT_VOLUME_VPUS_PER_GB = 10;
const STANDARD_FLEX_PRICE = { ocpuHour: 0.0255, memoryGbHour: 0.0015 };
const A1_FLEX_PRICE = { ocpuHour: 0.01, memoryGbHour: 0.0015 };
const BOOT_VOLUME_GB_MONTH = 0.0255;
const BOOT_VOLUME_VPU_GB_MONTH = 0.0017;
const BOOT_VOLUME_VPU_OPTIONS = [10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120];

function isTerminalStatus(status: Instance["status"]) {
  return status === "Terminating" || status === "Terminated";
}

export function InstancesPage() {
  const [statusFilter, setStatusFilter] = useState<"All" | Instance["status"]>("All");
  const [hideTerminated, setHideTerminated] = useState(false);
  const [instances, setInstances] = useState<Instance[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [errorMessage, setErrorMessage] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const [actionError, setActionError] = useState("");
  const [selectedIpInstance, setSelectedIpInstance] = useState<Instance | null>(null);
  const [pendingAction, setPendingAction] = useState<{
    instance: Instance;
    action: InstanceActionPayload["action"];
    label?: string;
    overrides?: Partial<InstanceActionPayload>;
  } | null>(null);
  const [resizeInstance, setResizeInstance] = useState<Instance | null>(null);

  const reloadInstances = useCallback(async () => {
    setIsLoading(true);
    setErrorMessage("");
    try {
      const context = getSelectedOCIContext();
      setInstances(await listInstances({ profileId: context.profileId, region: context.region }));
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "加载实例失败");
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void reloadInstances();
    return onOCIContextChange(() => void reloadInstances());
  }, [reloadInstances]);

  const filteredInstances = useMemo(() => {
    const visibleInstances = hideTerminated ? instances.filter((instance) => instance.status !== "Terminated") : instances;
    if (statusFilter === "All") return visibleInstances;
    return visibleInstances.filter((instance) => instance.status === statusFilter);
  }, [hideTerminated, instances, statusFilter]);

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
        targetBootVolumeGb: 0,
        targetBootVolumeVpusPerGb: 0,
        expandBootVolume: false,
        snapshotBefore: true,
        note: "",
        ...overrides
      });
      setActionMessage(`已创建任务 ${job.id}，可在任务中心查看执行状态。`);
      void reloadInstances();
      return true;
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "创建实例操作任务失败");
      return false;
    }
  }

  async function submitOneClickIPv6(instance: Instance) {
    setActionMessage("");
    setActionError("");
    try {
      const job = await createIPTask(instance.id, {
        mode: "enable-ipv6",
        reservedPublicIp: "",
        dnsLabel: "",
        vnicId: "primary",
        note: "one-click-ipv6",
        enableIpv6: true,
        autoConfigureIpv6: true,
        ipv6Strategy: "additive",
        networkChangeMode: "additive",
        routeTableMode: "merge_existing",
        securityMode: "append",
        allowIrreversibleVcnIpv6: true,
        allowPublicIpv4Change: false,
        openSshIpv6: true,
        openHttpIpv6: false,
        openHttpsIpv6: false,
        snapshotBefore: true
      });
      setActionMessage(`已创建一键 IPv6 任务 ${job.id}，可在任务中心查看执行状态。`);
      void reloadInstances();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "创建一键 IPv6 任务失败");
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
        <div className="instance-filter-controls">
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
          <label className="plain-switch-control">
            <span>隐藏已终止机器</span>
            <button
              type="button"
              className={`toggle-switch ${hideTerminated ? "on" : ""}`}
              aria-label="隐藏已终止机器"
              aria-pressed={hideTerminated}
              onClick={() => setHideTerminated((value) => !value)}
            />
          </label>
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
                <div><dt>IPv6</dt><dd className={instanceIPv6Addresses(instance).length > 0 ? "linkish" : ""}>{formatIPv6(instance)}</dd></div>
                <div><dt>配置</dt><dd>{instance.ocpus} OCPU / {instance.memoryGb} GB</dd></div>
              </dl>

              <div className="instance-card-footer">
                <StatusPill status={instance.status} />
                <div className="instance-actions">
                  <button className="secondary-button" onClick={() => void submitOneClickIPv6(instance)} disabled={isTerminalStatus(instance.status)}>
                    <Globe2 size={16} />
                    一键 IPv6
                  </button>
                  <button className="secondary-button" onClick={() => setSelectedIpInstance(instance)} disabled={isTerminalStatus(instance.status)}>
                    <Globe2 size={16} />
                    IP 管理
                  </button>
                  {instance.status === "Stopped" ? (
                    <button className="secondary-button" onClick={() => setPendingAction({ instance, action: "START" })}>
                      <Zap size={16} />
                      启动
                    </button>
                  ) : (
                    <button className="secondary-button" onClick={() => setPendingAction({ instance, action: "STOP", label: "停止", overrides: { graceful: false } })} disabled={instance.status !== "Running"}>
                      <Square size={16} />
                      停止
                    </button>
                  )}
                  <button className="secondary-button" onClick={() => setPendingAction({ instance, action: "STOP", label: "关机", overrides: { graceful: true } })} disabled={instance.status !== "Running"}>
                    <PowerOff size={16} />
                    关机
                  </button>
                  <button className="secondary-button" onClick={() => setPendingAction({ instance, action: "REBOOT" })} disabled={instance.status !== "Running"}>
                    <Power size={16} />
                    重启
                  </button>
                  <button className="secondary-button" onClick={() => setResizeInstance(instance)} disabled={isTerminalStatus(instance.status)}>
                    <ArrowUpDown size={16} />
                    升降级
                  </button>
                  <button className="secondary-button" onClick={() => setPendingAction({ instance, action: "TERMINATE" })} disabled={isTerminalStatus(instance.status)}>
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
          onCreated={(jobId) => {
            setActionMessage(`已创建 IP 管理任务 ${jobId}，可在任务中心查看执行状态。`);
            setSelectedIpInstance(null);
            void reloadInstances();
          }}
        />
      ) : null}

      {pendingAction ? (
        <ConfirmActionModal
          instance={pendingAction.instance}
          action={pendingAction.action}
          label={pendingAction.label}
          onClose={() => setPendingAction(null)}
          onConfirm={async (preserveBootVolume) => {
            const succeeded = await submitAction(pendingAction.instance, pendingAction.action, {
              ...pendingAction.overrides,
              preserveBootVolume
            });
            if (succeeded) {
              setPendingAction(null);
            }
            return succeeded;
          }}
        />
      ) : null}

      {resizeInstance ? (
        <ResizeModal
          instance={resizeInstance}
          instances={instances}
          onClose={() => setResizeInstance(null)}
          onSubmit={async (payload) => {
            const succeeded = await submitAction(resizeInstance, "RESIZE", payload);
            if (succeeded) {
              setResizeInstance(null);
            }
            return succeeded;
          }}
        />
      ) : null}
    </div>
  );
}

function ConfirmActionModal({
  instance,
  action,
  label: labelOverride,
  onClose,
  onConfirm
}: {
  instance: Instance;
  action: InstanceActionPayload["action"];
  label?: string;
  onClose: () => void;
  onConfirm: (preserveBootVolume: boolean) => Promise<boolean>;
}) {
  const [preserveBootVolume, setPreserveBootVolume] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const label = labelOverride || actionLabel(action);
  const isDanger = action === "TERMINATE";

  async function handleConfirm() {
    if (isSubmitting) return;
    setIsSubmitting(true);
    const succeeded = await onConfirm(preserveBootVolume);
    if (!succeeded) {
      setIsSubmitting(false);
    }
  }

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
        <button className="primary-button full" disabled={isSubmitting} onClick={() => void handleConfirm()}>
          {isSubmitting ? "提交中..." : `确认${label}`}
        </button>
        <button className="secondary-button full" disabled={isSubmitting} onClick={onClose}>取消</button>
      </div>
    </div>
  );
}

function ResizeModal({
  instance,
  instances,
  onClose,
  onSubmit
}: {
  instance: Instance;
  instances: Instance[];
  onClose: () => void;
  onSubmit: (payload: Partial<InstanceActionPayload>) => Promise<boolean>;
}) {
  const currentBootVolumeGb = Math.max(50, Number(instance.bootVolumeGb) || 0);
  const currentBootVolumeVpusPerGb = normalizeBootVolumeVpus(Number(instance.bootVolumeVpusPerGb) || 10);
  const [targetShape, setTargetShape] = useState(instance.shape);
  const [targetOcpus, setTargetOcpus] = useState(Math.max(1, Number(instance.ocpus) || 1));
  const [targetMemoryGb, setTargetMemoryGb] = useState(Math.max(1, Number(instance.memoryGb) || 1));
  const [targetBootVolumeGb, setTargetBootVolumeGb] = useState(currentBootVolumeGb);
  const [targetBootVolumeVpusPerGb, setTargetBootVolumeVpusPerGb] = useState(currentBootVolumeVpusPerGb);
  const [snapshotBefore, setSnapshotBefore] = useState(true);
  const [validationError, setValidationError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const targetIsFlexible = isFlexibleShapeName(targetShape);
  const bootVolumeTooSmall = targetBootVolumeGb < currentBootVolumeGb;
  const expandBootVolume = targetBootVolumeGb > currentBootVolumeGb;
  const budgetChange = estimateResizeBudgetChange(instance, instances, {
    targetShape,
    targetOcpus: targetIsFlexible ? targetOcpus : Math.max(1, Number(instance.ocpus) || 1),
    targetMemoryGb: targetIsFlexible ? targetMemoryGb : Math.max(1, Number(instance.memoryGb) || 1),
    targetBootVolumeGb,
    targetBootVolumeVpusPerGb
  });

  async function submitResize() {
    if (isSubmitting) return;
    if (bootVolumeTooSmall) {
      setValidationError(`目标启动盘不能小于当前大小 ${currentBootVolumeGb} GB。OCI 启动盘只能扩容，不能降盘。`);
      return;
    }
    if (targetIsFlexible && (targetOcpus <= 0 || targetMemoryGb <= 0)) {
      setValidationError("Flex Shape 的目标 OCPU 和内存必须大于 0。");
      return;
    }
    setValidationError("");
    setIsSubmitting(true);
    const succeeded = await onSubmit({
      targetShape,
      targetOcpus: targetIsFlexible ? targetOcpus : Math.max(1, Number(instance.ocpus) || 1),
      targetMemoryGb: targetIsFlexible ? targetMemoryGb : Math.max(1, Number(instance.memoryGb) || 1),
      targetBootVolumeGb,
      targetBootVolumeVpusPerGb,
      expandBootVolume,
      snapshotBefore
    });
    if (!succeeded) {
      setIsSubmitting(false);
    }
  }

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
              <input
                value={targetShape}
                onChange={(event) => {
                  setTargetShape(event.target.value);
                  setValidationError("");
                }}
              />
            </label>
            <label>
              目标 OCPU
              <input
                disabled={!targetIsFlexible}
                type="number"
                min={1}
                value={targetOcpus}
                onChange={(event) => setTargetOcpus(Number(event.target.value))}
              />
            </label>
            <label>
              目标内存 GB
              <input
                disabled={!targetIsFlexible}
                type="number"
                min={1}
                value={targetMemoryGb}
                onChange={(event) => setTargetMemoryGb(Number(event.target.value))}
              />
            </label>
            <label>
              目标启动盘 GB
              <input
                type="number"
                min={currentBootVolumeGb}
                value={targetBootVolumeGb}
                onChange={(event) => {
                  setTargetBootVolumeGb(Number(event.target.value));
                  setValidationError("");
                }}
              />
            </label>
            <label>
              目标硬盘性能
              <select value={targetBootVolumeVpusPerGb} onChange={(event) => setTargetBootVolumeVpusPerGb(Number(event.target.value))}>
                {BOOT_VOLUME_VPU_OPTIONS.map((value) => (
                  <option value={value} key={value}>
                    {value} VPUs/GB{value === 10 ? " / Balanced" : ""}
                  </option>
                ))}
              </select>
            </label>
          </div>
          {!targetIsFlexible ? (
            <p className="muted-line">当前目标 Shape 为固定规格，不支持编辑 OCPU 和内存，提交时不会向 OCI 发送 ShapeConfig。</p>
          ) : null}
        </div>
        <div className="switch-row">
          <div>
            <strong>硬盘扩容与性能</strong>
            <p>当前启动盘 {currentBootVolumeGb} GB / {currentBootVolumeVpusPerGb} VPUs/GB。容量只能往上调；性能可在 10-120 VPUs/GB 范围内调整。</p>
          </div>
          <div className={`status-chip ${expandBootVolume ? "success" : bootVolumeTooSmall ? "failed" : ""}`}>
            <HardDrive size={15} />
            {bootVolumeTooSmall ? "不能降盘" : expandBootVolume ? `扩容到 ${targetBootVolumeGb} GB` : "容量不变"} / {targetBootVolumeVpusPerGb} VPUs
          </div>
        </div>
        <ResizeBudgetCard budget={budgetChange} />
        {bootVolumeTooSmall || validationError ? (
          <div className="inline-error">
            {validationError || `目标启动盘不能小于当前大小 ${currentBootVolumeGb} GB。OCI 启动盘只能扩容，不能降盘。`}
          </div>
        ) : null}
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
          <button className="secondary-button" disabled={isSubmitting} onClick={onClose}>取消</button>
          <button
            className="primary-button"
            disabled={bootVolumeTooSmall || isSubmitting}
            onClick={() => void submitResize()}
          >
            {isSubmitting ? "提交中..." : "创建升降级任务"}
          </button>
        </div>
      </div>
    </div>
  );
}

type ResizeBudgetEstimate = {
  currentHourly: number;
  targetHourly: number;
  deltaHourly: number;
  deltaDaily: number;
  deltaMonthly: number;
  statusLabel: string;
  blockers: string[];
};

function ResizeBudgetCard({ budget }: { budget: ResizeBudgetEstimate }) {
  return (
    <div className="resize-budget-card">
      <div>
        <span>当前每小时</span>
        <strong>{formatMoney(budget.currentHourly, "hour")}</strong>
      </div>
      <div>
        <span>目标每小时</span>
        <strong>{formatMoney(budget.targetHourly, "hour")}</strong>
      </div>
      <div className={budget.deltaHourly > 0 ? "increase" : budget.deltaHourly < 0 ? "decrease" : ""}>
        <span>预算变化</span>
        <strong>{formatDeltaMoney(budget.deltaHourly, "hour")} / 小时</strong>
        <small>{formatDeltaMoney(budget.deltaDaily, "day")} / 天 · {formatDeltaMoney(budget.deltaMonthly, "month")} / 月</small>
      </div>
      <div>
        <span>免费额度判断</span>
        <strong>{budget.statusLabel}</strong>
        {budget.blockers.length > 0 ? <small>{budget.blockers.join("；")}</small> : <small>提升后的计算、容量和性能仍在免费边界内</small>}
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
  onCreated: (jobId: string) => void;
}) {
  const existingIPv6Addresses = instanceIPv6Addresses(instance);
  const hasIPv6 = existingIPv6Addresses.length > 0;
  const [mode, setMode] = useState(ipModes[0]);
  const [reservedPublicIp, setReservedPublicIp] = useState("");
  const [dnsLabel, setDNSLabel] = useState(instance.name);
  const [vnicId, setVnicId] = useState("primary");
  const [note, setNote] = useState("");
  const [enableIPv6, setEnableIPv6] = useState(hasIPv6);
  const [ipv6Strategy, setIpv6Strategy] = useState<"assign_only" | "additive" | "clone_route_table" | "replace_public_path">("additive");
  const [routeTableMode, setRouteTableMode] = useState<"merge_existing" | "clone">("merge_existing");
  const [securityMode, setSecurityMode] = useState<"append" | "none">("append");
  const [allowIrreversibleVcnIpv6, setAllowIrreversibleVcnIpv6] = useState(true);
  const [allowPublicIpv4Change, setAllowPublicIpv4Change] = useState(false);
  const [openSshIpv6, setOpenSshIpv6] = useState(true);
  const [openHttpIpv6, setOpenHttpIpv6] = useState(false);
  const [openHttpsIpv6, setOpenHttpsIpv6] = useState(false);
  const [snapshotBefore, setSnapshotBefore] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  const disablingIPv6 = hasIPv6 && !enableIPv6;

  async function handleCreateTask() {
    if (isSubmitting) return;
    setIsSubmitting(true);
    setErrorMessage("");
    try {
      const job = await createIPTask(instance.id, {
        mode,
        reservedPublicIp,
        dnsLabel,
        vnicId,
        note: note || (disablingIPv6 ? "disable-ipv6" : ""),
        enableIpv6: enableIPv6,
        disableIpv6: disablingIPv6,
        autoConfigureIpv6: enableIPv6 && ipv6Strategy !== "assign_only",
        ipv6Strategy,
        networkChangeMode: ipv6Strategy,
        routeTableMode: ipv6Strategy === "clone_route_table" ? "clone" : routeTableMode,
        securityMode,
        allowIrreversibleVcnIpv6,
        allowPublicIpv4Change,
        openSshIpv6,
        openHttpIpv6,
        openHttpsIpv6,
        snapshotBefore
      });
      onCreated(job.id);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "创建 IP 管理任务失败");
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
              <p>{instance.name} 的公网 IP、私网 IP、IPv6 与保留 IP 操作。</p>
            </div>
          </div>
          <button className="icon-button bordered" aria-label="关闭 IP 管理" onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className="modal-summary-card">
          <div><span>公网 IP</span><strong>{instance.primaryIp}</strong></div>
          <div><span>私网 IP</span><strong>{instance.privateIp}</strong></div>
          <div><span>IPv6</span><strong>{existingIPv6Addresses.length > 0 ? existingIPv6Addresses.join(", ") : "未分配"}</strong></div>
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
              <p>{hasIPv6 ? "当前实例已有 IPv6，关闭后会创建任务删除该 VNIC 上的 IPv6 地址。" : "如果当前子网支持 IPv6，可创建任务为实例分配 IPv6 地址。"}</p>
            </div>
            <button className={`toggle-switch ${enableIPv6 ? "on" : ""}`} onClick={() => setEnableIPv6((value) => !value)} />
          </div>
          {disablingIPv6 ? (
            <div className="modal-warning">
              <ShieldAlert size={18} />
              <span>提交后将调用 OCI DeleteIpv6 关闭当前实例 IPv6。正在使用 IPv6 的 SSH、HTTP、DNS 或业务连接会中断。</span>
            </div>
          ) : null}
          {enableIPv6 ? (
            <div className="form-section compact">
              <div className="form-section-title">
                <Globe2 size={18} />
                <span>IPv6 网络编排方式</span>
              </div>
              <div className="choice-grid retry-choice-grid">
                <button
                  className={`choice-card ${ipv6Strategy === "assign_only" ? "active" : ""}`}
                  onClick={() => setIpv6Strategy("assign_only")}
                  type="button"
                >
                  <strong>只添加 IPv6</strong>
                  <span>使用当前 VNIC 和子网，若 VCN/子网未启用 IPv6，OCI 会返回真实错误。</span>
                </button>
                <button
                  className={`choice-card ${ipv6Strategy === "additive" ? "active" : ""}`}
                  onClick={() => setIpv6Strategy("additive")}
                  type="button"
                >
                  <strong>原地双栈增配</strong>
                  <span>自动给 VCN/Subnet 添加 IPv6 CIDR，复用或创建 IGW，并合并追加 ::/0 路由。</span>
                </button>
                <button
                  className={`choice-card ${ipv6Strategy === "clone_route_table" ? "active" : ""}`}
                  onClick={() => setIpv6Strategy("clone_route_table")}
                  type="button"
                >
                  <strong>克隆路由表</strong>
                  <span>复制当前路由表后追加 IPv6 路由，再把子网切到新路由表，适合降低共享路由表风险。</span>
                </button>
                <button
                  className={`choice-card ${ipv6Strategy === "replace_public_path" ? "active" : ""}`}
                  onClick={() => setIpv6Strategy("replace_public_path")}
                  type="button"
                >
                  <strong>危险公网路径替换</strong>
                  <span>保留给后续高级场景。该模式可能导致当前临时 IPv4 公网 IP 变化，必须显式确认。</span>
                </button>
              </div>
              {ipv6Strategy !== "assign_only" ? (
                <div className="form-grid">
                  <label>
                    路由表处理
                    <select
                      value={ipv6Strategy === "clone_route_table" ? "clone" : routeTableMode}
                      onChange={(event) => setRouteTableMode(event.target.value as "merge_existing" | "clone")}
                      disabled={ipv6Strategy === "clone_route_table"}
                    >
                      <option value="merge_existing">合并追加到当前路由表</option>
                      <option value="clone">克隆路由表后切换子网</option>
                    </select>
                  </label>
                  <label>
                    安全规则处理
                    <select value={securityMode} onChange={(event) => setSecurityMode(event.target.value as "append" | "none")}>
                      <option value="append">追加 IPv6 最小安全规则</option>
                      <option value="none">不修改安全规则</option>
                    </select>
                  </label>
                </div>
              ) : null}
              {ipv6Strategy !== "assign_only" && securityMode === "append" ? (
                <div className="switch-panel nested">
                  <div className="switch-row">
                    <div>
                      <strong>允许 IPv6 SSH</strong>
                      <p>追加 TCP/22 入站规则，方便 IPv6 连通性验证。</p>
                    </div>
                    <button className={`toggle-switch ${openSshIpv6 ? "on" : ""}`} onClick={() => setOpenSshIpv6((value) => !value)} />
                  </div>
                  <div className="switch-row">
                    <div>
                      <strong>允许 IPv6 HTTP / HTTPS</strong>
                      <p>按需追加 TCP/80 和 TCP/443 入站规则。</p>
                    </div>
                    <div className="inline-toggle-group">
                      <button className={`toggle-switch ${openHttpIpv6 ? "on" : ""}`} aria-label="允许 IPv6 HTTP" onClick={() => setOpenHttpIpv6((value) => !value)} />
                      <button className={`toggle-switch ${openHttpsIpv6 ? "on" : ""}`} aria-label="允许 IPv6 HTTPS" onClick={() => setOpenHttpsIpv6((value) => !value)} />
                    </div>
                  </div>
                </div>
              ) : null}
              {ipv6Strategy !== "assign_only" ? (
                <div className="switch-panel nested">
                  <div className="switch-row">
                    <div>
                      <strong>确认给 VCN 添加 IPv6 CIDR</strong>
                      <p>OCI 的 Oracle GUA IPv6 CIDR 属于高风险网络变更，本任务按不可逆处理。</p>
                    </div>
                    <button className={`toggle-switch ${allowIrreversibleVcnIpv6 ? "on" : ""}`} onClick={() => setAllowIrreversibleVcnIpv6((value) => !value)} />
                  </div>
                  {ipv6Strategy === "replace_public_path" ? (
                    <div className="switch-row danger-row">
                      <div>
                        <strong>允许当前 IPv4 公网 IP 变化</strong>
                        <p>危险公网路径替换可能拿到新的临时 IPv4。需要稳定 IPv4 时请先使用预留 IP。</p>
                      </div>
                      <button className={`toggle-switch ${allowPublicIpv4Change ? "on" : ""}`} onClick={() => setAllowPublicIpv4Change((value) => !value)} />
                    </div>
                  ) : null}
                </div>
              ) : null}
            </div>
          ) : null}
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
        {errorMessage ? <div className="inline-error">{errorMessage}</div> : null}

        <div className="button-row">
          <button className="secondary-button" disabled={isSubmitting} onClick={onClose}>取消</button>
          <button className="primary-button" disabled={isSubmitting} onClick={handleCreateTask}>
            {isSubmitting ? "创建中..." : disablingIPv6 ? "创建关闭 IPv6 任务" : "创建 IP 任务"}
          </button>
        </div>
      </div>
    </div>
  );
}

function instanceIPv6Addresses(instance: Instance) {
  const values = [...(instance.ipv6Addresses ?? []), instance.primaryIpv6].map((value) => String(value || "").trim()).filter(Boolean);
  return Array.from(new Set(values));
}

function formatIPv6(instance: Instance) {
  const addresses = instanceIPv6Addresses(instance);
  if (addresses.length === 0) return "未分配";
  if (addresses.length === 1) return addresses[0];
  return `${addresses[0]} +${addresses.length - 1}`;
}

function isFlexibleShapeName(shape: string) {
  return String(shape || "").trim().toLowerCase().endsWith(".flex");
}

function estimateResizeBudgetChange(
  instance: Instance,
  instances: Instance[],
  target: {
    targetShape: string;
    targetOcpus: number;
    targetMemoryGb: number;
    targetBootVolumeGb: number;
    targetBootVolumeVpusPerGb: number;
  }
): ResizeBudgetEstimate {
  const currentHourly = estimateAccountHourly(instances).hourly;
  const targetInstance = {
    ...instance,
    shape: target.targetShape,
    ocpus: Number(target.targetOcpus) || 0,
    memoryGb: Number(target.targetMemoryGb) || 0,
    bootVolumeGb: Math.max(50, Number(target.targetBootVolumeGb) || 0),
    bootVolumeVpusPerGb: normalizeBootVolumeVpus(Number(target.targetBootVolumeVpusPerGb) || 10)
  };
  const targetInstances = instances.map((item) => (item.id === instance.id ? targetInstance : item));
  const targetEstimate = estimateAccountHourly(targetInstances);
  const targetHourly = targetEstimate.hourly;
  const deltaHourly = targetHourly - currentHourly;
  return {
    currentHourly,
    targetHourly,
    deltaHourly,
    deltaDaily: deltaHourly * HOURS_PER_DAY,
    deltaMonthly: deltaHourly * HOURS_PER_MONTH,
    statusLabel: targetEstimate.blockers.length === 0 ? "免费额度内" : "超出免费额度",
    blockers: targetEstimate.blockers
  };
}

function estimateAccountHourly(instances: Instance[]) {
  const active = instances.filter((instance) => !String(instance.status).toLowerCase().includes("terminat"));
  const e2Micro = active.filter((instance) => instance.shape === "VM.Standard.E2.1.Micro");
  const a1Instances = active.filter((instance) => instance.shape === "VM.Standard.A1.Flex");
  const standardInstances = active.filter((instance) => instance.shape !== "VM.Standard.E2.1.Micro" && instance.shape !== "VM.Standard.A1.Flex");
  const a1Ocpus = a1Instances.reduce((sum, instance) => sum + (Number(instance.ocpus) || 0), 0);
  const a1MemoryGb = a1Instances.reduce((sum, instance) => sum + (Number(instance.memoryGb) || 0), 0);
  const bootVolumeGb = active.reduce((sum, instance) => sum + Math.max(50, Number(instance.bootVolumeGb) || 0), 0);
  const billableBootGb = Math.max(0, bootVolumeGb - ALWAYS_FREE_BOOT_VOLUME_GB);
  const billableBaseVpusGb = active.reduce((sum, instance) => {
    const gb = Math.max(50, Number(instance.bootVolumeGb) || 0);
    const vpus = normalizeBootVolumeVpus(Number(instance.bootVolumeVpusPerGb) || 10);
    if (bootVolumeGb <= ALWAYS_FREE_BOOT_VOLUME_GB) return sum;
    const ratio = bootVolumeGb > 0 ? billableBootGb / bootVolumeGb : 0;
    return sum + gb * ratio * Math.min(vpus, ALWAYS_FREE_BOOT_VOLUME_VPUS_PER_GB);
  }, 0);
  const billableUpliftVpusGb = active.reduce((sum, instance) => {
    const gb = Math.max(50, Number(instance.bootVolumeGb) || 0);
    const vpus = normalizeBootVolumeVpus(Number(instance.bootVolumeVpusPerGb) || 10);
    return sum + gb * Math.max(0, vpus - ALWAYS_FREE_BOOT_VOLUME_VPUS_PER_GB);
  }, 0);
  const storageHourly =
    (billableBootGb * BOOT_VOLUME_GB_MONTH + (billableBaseVpusGb + billableUpliftVpusGb) * BOOT_VOLUME_VPU_GB_MONTH) / HOURS_PER_MONTH;
  const e2BillableCount = Math.max(0, e2Micro.length - ALWAYS_FREE_E2_MICRO_COUNT);
  const e2Hourly = e2BillableCount * (STANDARD_FLEX_PRICE.ocpuHour + STANDARD_FLEX_PRICE.memoryGbHour);
  const a1Hourly =
    Math.max(0, a1Ocpus - ALWAYS_FREE_A1_OCPUS) * A1_FLEX_PRICE.ocpuHour +
    Math.max(0, a1MemoryGb - ALWAYS_FREE_A1_MEMORY_GB) * A1_FLEX_PRICE.memoryGbHour;
  const standardHourly = standardInstances.reduce(
    (sum, instance) => sum + (Number(instance.ocpus) || 0) * STANDARD_FLEX_PRICE.ocpuHour + (Number(instance.memoryGb) || 0) * STANDARD_FLEX_PRICE.memoryGbHour,
    0
  );
  const blockers = [
    ...(e2BillableCount > 0 ? [`E2.1.Micro 超出 ${e2BillableCount} 台`] : []),
    ...(a1Ocpus > ALWAYS_FREE_A1_OCPUS ? [`A1 OCPU ${a1Ocpus}/${ALWAYS_FREE_A1_OCPUS}`] : []),
    ...(a1MemoryGb > ALWAYS_FREE_A1_MEMORY_GB ? [`A1 内存 ${a1MemoryGb}/${ALWAYS_FREE_A1_MEMORY_GB} GB`] : []),
    ...(billableBootGb > 0 ? [`启动盘容量超出 ${billableBootGb} GB`] : []),
    ...(billableUpliftVpusGb > 0 ? [`硬盘性能超过 ${ALWAYS_FREE_BOOT_VOLUME_VPUS_PER_GB} VPUs/GB`] : []),
    ...(standardInstances.length > 0 ? [`${standardInstances.length} 台非 Always Free Shape`] : [])
  ];
  return {
    hourly: e2Hourly + a1Hourly + standardHourly + storageHourly,
    blockers
  };
}

function normalizeBootVolumeVpus(value: number) {
  if (!Number.isFinite(value) || value <= 0) return 10;
  return Math.min(120, Math.max(10, Math.round(value)));
}

function formatMoney(value: number, unit: "hour" | "day" | "month") {
  const digits = unit === "hour" ? 4 : 2;
  return `$${value.toFixed(digits)}`;
}

function formatDeltaMoney(value: number, unit: "hour" | "day" | "month") {
  const prefix = value > 0 ? "+" : value < 0 ? "-" : "";
  return `${prefix}${formatMoney(Math.abs(value), unit)}`;
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
