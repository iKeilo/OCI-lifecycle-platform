import {
  ArrowUpDown,
  Download,
  FileUp,
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
  Settings,
  ShieldAlert,
  Square,
  Trash2,
  X,
  Zap
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { Link } from "react-router-dom";
import { getSelectedOCIContext, onOCIContextChange } from "../app/ociContext";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import { StatusPill } from "../components/StatusPill";
import { createFirewallTask, createIPTask, createInstanceAction, createInstanceReinstallTask, getFirewallRules, getLaunchOptionsForContext, listInstances } from "../services/api";
import type { FirewallRule, FirewallRulesInventory, FirewallTaskPayload, Instance, InstanceActionPayload, LaunchOptions } from "../services/api";

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
  const [firewallInstance, setFirewallInstance] = useState<Instance | null>(null);
  const [systemSettingsInstance, setSystemSettingsInstance] = useState<Instance | null>(null);
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
        preserveBootVolume: false,
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
                  <button className="secondary-button instance-firewall-button" onClick={() => setFirewallInstance(instance)} disabled={isTerminalStatus(instance.status)}>
                    <ShieldAlert size={16} />
                    防火墙
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
                  <button className="secondary-button" onClick={() => setSystemSettingsInstance(instance)} disabled={isTerminalStatus(instance.status)}>
                    <Settings size={16} />
                    系统设置
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

      {firewallInstance ? (
        <FirewallManagementModal
          instance={firewallInstance}
          onClose={() => setFirewallInstance(null)}
          onCreated={(jobId) => {
            setActionMessage(`已创建防火墙任务 ${jobId}，可在任务中心查看执行状态。`);
            setFirewallInstance(null);
            void reloadInstances();
          }}
        />
      ) : null}

      {systemSettingsInstance ? (
        <InstanceSystemSettingsModal
          instance={systemSettingsInstance}
          onClose={() => setSystemSettingsInstance(null)}
          onCreated={(jobId) => {
            setActionMessage(`已创建系统设置任务 ${jobId}，可在任务中心查看执行状态。`);
            setSystemSettingsInstance(null);
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
  const [deleteBootVolume, setDeleteBootVolume] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const label = labelOverride || actionLabel(action);
  const isDanger = action === "TERMINATE";

  async function handleConfirm() {
    if (isSubmitting) return;
    setIsSubmitting(true);
    const succeeded = await onConfirm(!deleteBootVolume);
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
              <strong>删除启动盘</strong>
              <p>默认开启，终止实例时同时删除启动盘，避免产生孤儿磁盘费用。关闭后会保留启动盘用于人工恢复或取数。</p>
            </div>
            <button
              aria-label="终止实例时删除启动盘"
              className={`toggle-switch ${deleteBootVolume ? "on" : ""}`}
              onClick={() => setDeleteBootVolume((value) => !value)}
            />
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

function InstanceSystemSettingsModal({
  instance,
  onClose,
  onCreated
}: {
  instance: Instance;
  onClose: () => void;
  onCreated: (jobId: string) => void;
}) {
  const context = getSelectedOCIContext();
  const profileId = instance.profileId || context.profileId;
  const region = instance.region || context.region;
  const compartmentId = instance.compartmentId || "";
  const currentBootVolumeGb = Math.max(50, Number(instance.bootVolumeGb) || 50);
  const currentBootVolumeVpusPerGb = normalizeBootVolumeVpus(Number(instance.bootVolumeVpusPerGb) || 10);
  const [options, setOptions] = useState<LaunchOptions | null>(null);
  const [loadingOptions, setLoadingOptions] = useState(false);
  const [optionsError, setOptionsError] = useState("");
  const [imageId, setImageId] = useState("");
  const [bootVolumeSizeGb, setBootVolumeSizeGb] = useState(currentBootVolumeGb);
  const [bootVolumeVpusPerGb, setBootVolumeVpusPerGb] = useState(currentBootVolumeVpusPerGb);
  const [preserveOldBootVolume, setPreserveOldBootVolume] = useState(true);
  const [confirmationName, setConfirmationName] = useState("");
  const [note, setNote] = useState("");
  const [submitError, setSubmitError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    let cancelled = false;
    async function loadOptions() {
      setLoadingOptions(true);
      setOptionsError("");
      try {
        const value = await getLaunchOptionsForContext({
          profileId,
          region,
          compartmentId,
          shape: instance.shape
        });
        if (cancelled) return;
        setOptions(value);
        const images = reinstallImagesForShape(value, instance.shape);
        if (images.length > 0) {
          setImageId((current) => current || images[0].id);
        }
      } catch (error) {
        if (!cancelled) setOptionsError(error instanceof Error ? error.message : "加载镜像选项失败");
      } finally {
        if (!cancelled) setLoadingOptions(false);
      }
    }
    void loadOptions();
    return () => {
      cancelled = true;
    };
  }, [compartmentId, instance.shape, profileId, region]);

  const images = options ? reinstallImagesForShape(options, instance.shape) : [];
  const selectedImage = images.find((item) => item.id === imageId);
  const bootVolumeTooSmall = bootVolumeSizeGb < currentBootVolumeGb;
  const canSubmitReinstall = imageId && confirmationName.trim() === instance.name.trim() && !bootVolumeTooSmall && !isSubmitting;

  async function submitReinstall() {
    if (!canSubmitReinstall) return;
    setIsSubmitting(true);
    setSubmitError("");
    try {
      const job = await createInstanceReinstallTask(instance.id, {
        profileId,
        region,
        compartmentId,
        imageId,
        imageName: selectedImage?.label ?? "",
        bootVolumeSizeGb,
        bootVolumeVpusPerGb,
        preserveOldBootVolume,
        createBootVolumeBackup: false,
        confirmationName,
        note
      });
      onCreated(job.id);
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : "创建重装系统任务失败");
      setIsSubmitting(false);
    }
  }

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true">
      <div className="action-modal glass-modal system-settings-modal">
        <div className="modal-header-row">
          <div className="modal-title-block">
            <div className="modal-icon compact">
              <Settings size={24} />
            </div>
            <div>
              <h2>系统设置</h2>
              <p>{instance.name} 的系统重装。高风险操作会进入任务中心、审计日志和通知中心。</p>
            </div>
          </div>
          <button className="icon-button bordered" aria-label="关闭系统设置" onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className="modal-summary-card">
          <div><span>实例</span><strong>{instance.name}</strong></div>
          <div><span>Shape</span><strong>{instance.shape}</strong></div>
          <div><span>区域</span><strong>{region}</strong></div>
          <div><span>启动盘</span><strong>{currentBootVolumeGb} GB / {currentBootVolumeVpusPerGb} VPUs</strong></div>
        </div>

        <div className="form-section">
          <div className="form-section-title">
            <RefreshCw size={18} />
            <span>目标镜像</span>
          </div>
          {loadingOptions ? <div className="async-state compact">正在加载当前 Shape 兼容镜像...</div> : null}
          {optionsError ? <div className="inline-error">{optionsError}</div> : null}
          <label>
            Image
            <select value={imageId} onChange={(event) => setImageId(event.target.value)} disabled={loadingOptions || images.length === 0}>
              {images.length === 0 ? <option value="">没有可用镜像选项</option> : null}
              {images.map((image) => (
                <option value={image.id} key={image.id}>{image.label}</option>
              ))}
            </select>
          </label>
          <p className="muted-line">镜像选项来自 Launch Options 的 Shape/Image 缓存，并按当前 Shape 过滤。</p>
        </div>

        <div className="form-section">
          <div className="form-section-title">
            <HardDrive size={18} />
            <span>启动盘</span>
          </div>
          <div className="form-grid">
            <label>
              目标启动盘 GB
              <input
                type="number"
                min={currentBootVolumeGb}
                value={bootVolumeSizeGb}
                onChange={(event) => setBootVolumeSizeGb(Number(event.target.value))}
              />
            </label>
            <label>
              目标硬盘性能
              <select value={bootVolumeVpusPerGb} onChange={(event) => setBootVolumeVpusPerGb(Number(event.target.value))}>
                {BOOT_VOLUME_VPU_OPTIONS.map((value) => (
                  <option value={value} key={value}>{value} VPUs/GB{value === 10 ? " / Balanced" : ""}</option>
                ))}
              </select>
            </label>
          </div>
          {bootVolumeTooSmall ? <div className="inline-error">目标启动盘不能小于当前大小 {currentBootVolumeGb} GB。OCI 启动盘不能降盘。</div> : null}
        </div>

        <div className="switch-panel">
          <div className="switch-row">
            <div>
              <strong>保留旧启动盘</strong>
              <p>开启后 OCI 在成功替换启动盘后保留旧启动盘，便于人工回滚和取数。关闭属于高风险操作。</p>
            </div>
            <button className={`toggle-switch ${preserveOldBootVolume ? "on" : ""}`} onClick={() => setPreserveOldBootVolume((value) => !value)} />
          </div>
        </div>

        <div className="form-section">
          <div className="form-grid">
            <label>
              输入实例名确认
              <input value={confirmationName} onChange={(event) => setConfirmationName(event.target.value)} placeholder={instance.name} />
            </label>
            <label>
              任务备注
              <input value={note} onChange={(event) => setNote(event.target.value)} placeholder="例如：重装为新的 Oracle Linux 镜像" />
            </label>
          </div>
        </div>

        <div className="modal-warning">
          <ShieldAlert size={18} />
          <span>重装系统会替换启动盘中的操作系统。请确认重要数据已备份；提交后由后端调用真实 OCI UpdateInstance，并向管理员推送任务创建、成功或失败通知。</span>
        </div>
        {submitError ? <div className="inline-error">{submitError}</div> : null}
        <div className="button-row">
          <button className="secondary-button" disabled={isSubmitting} onClick={onClose}>取消</button>
          <button className="primary-button danger" disabled={!canSubmitReinstall} onClick={() => void submitReinstall()}>
            {isSubmitting ? "提交中..." : "创建重装系统任务"}
          </button>
        </div>
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

const firewallQuickPorts = [
  { label: "SSH", protocol: "tcp" as const, portMin: 22, portMax: 22 },
  { label: "HTTP", protocol: "tcp" as const, portMin: 80, portMax: 80 },
  { label: "HTTPS", protocol: "tcp" as const, portMin: 443, portMax: 443 },
  { label: "RDP", protocol: "tcp" as const, portMin: 3389, portMax: 3389 }
];

function shortID(value: string) {
  if (!value) return "-";
  if (value.length <= 18) return value;
  return `${value.slice(0, 10)}...${value.slice(-6)}`;
}

function parseFirewallPorts(value: string): Array<{ portMin: number; portMax: number }> {
  const ranges: Array<{ portMin: number; portMax: number }> = [];
  const parts = value.split(",").map((part) => part.trim()).filter(Boolean);
  for (const part of parts) {
    const [rawMin, rawMax] = part.split("-").map((item) => item.trim());
    const portMin = Number(rawMin);
    const portMax = rawMax ? Number(rawMax) : portMin;
    if (!Number.isInteger(portMin) || !Number.isInteger(portMax) || portMin <= 0 || portMin > 65535 || portMax < portMin || portMax > 65535) {
      throw new Error("端口格式不正确，请输入 80、80,88 或 90-99 这类格式。");
    }
    ranges.push({ portMin, portMax });
  }
  if (ranges.length === 0) {
    throw new Error("请输入端口。");
  }
  return ranges;
}

type PendingFirewallChange = {
  id: string;
  label: string;
  payload: FirewallTaskPayload;
};

function FirewallManagementModal({
  instance,
  onClose,
  onCreated
}: {
  instance: Instance;
  onClose: () => void;
  onCreated: (jobId: string) => void;
}) {
  const context = getSelectedOCIContext();
  const [inventory, setInventory] = useState<FirewallRulesInventory | null>(null);
  const [isLoadingRules, setIsLoadingRules] = useState(true);
  const [rulesError, setRulesError] = useState("");
  const [searchTerm, setSearchTerm] = useState("");
  const [selectedRuleIds, setSelectedRuleIds] = useState<string[]>([]);
  const [showRuleEditor, setShowRuleEditor] = useState(false);
  const [firewallEnabled, setFirewallEnabled] = useState(true);
  const [pingBlocked, setPingBlocked] = useState(false);
  const [snapshotBefore, setSnapshotBefore] = useState(true);
  const [protocol, setProtocol] = useState<FirewallTaskPayload["protocol"] | "tcp_udp">("tcp");
  const [portExpression, setPortExpression] = useState("22");
  const [sourceMode, setSourceMode] = useState("all");
  const [customSourceCidr, setCustomSourceCidr] = useState("");
  const [rulePolicy, setRulePolicy] = useState<FirewallTaskPayload["action"]>("open");
  const [direction, setDirection] = useState("ingress");
  const [targetScope, setTargetScope] = useState<FirewallTaskPayload["targetScope"]>("auto");
  const [note, setNote] = useState("SSH 远程服务");
  const [pendingChanges, setPendingChanges] = useState<PendingFirewallChange[]>([]);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");

  const sourceCidr = sourceMode === "custom" ? customSourceCidr.trim() : "0.0.0.0/0";
  const rules = inventory?.rules ?? [];
  const broadRules = rules.filter((rule) => rule.isBroadRule);
  const filteredRules = rules.filter((rule) => {
    const target = `${rule.protocol} ${rule.portLabel} ${rule.source} ${rule.remark ?? ""} ${rule.containerType}`.toLowerCase();
    return target.includes(searchTerm.trim().toLowerCase());
  });
  const selectedRules = rules.filter((rule) => selectedRuleIds.includes(rule.id));

  const loadRules = useCallback(async () => {
    setIsLoadingRules(true);
    setRulesError("");
    try {
      const next = await getFirewallRules(instance.id, {
        profileId: context.profileId,
        region: context.region,
        vnicId: "primary"
      });
      setInventory(next);
      setFirewallEnabled(next.verified);
    } catch (error) {
      setRulesError(error instanceof Error ? error.message : "读取防火墙规则失败");
    } finally {
      setIsLoadingRules(false);
    }
  }, [context.profileId, context.region, instance.id]);

  useEffect(() => {
    void loadRules();
  }, [loadRules]);

  function applyQuickPort(option: (typeof firewallQuickPorts)[number]) {
    setProtocol(option.protocol);
    setPortExpression(option.portMin === option.portMax ? String(option.portMin) : `${option.portMin}-${option.portMax}`);
    setNote(`${option.label} 服务`);
    setShowRuleEditor(true);
  }

  function queueFirewallChange(label: string, payload: FirewallTaskPayload) {
    const id = `${Date.now()}-${Math.random().toString(16).slice(2)}`;
    setPendingChanges((current) => [...current, { id, label, payload }]);
  }

  function removePendingChange(id: string) {
    setPendingChanges((current) => current.filter((change) => change.id !== id));
  }

  async function applyPendingChanges() {
    if (isSubmitting || pendingChanges.length === 0) return;
    setIsSubmitting(true);
    setErrorMessage("");
    try {
      const jobIDs: string[] = [];
      for (const change of pendingChanges) {
        const job = await createFirewallTask(instance.id, change.payload);
        jobIDs.push(job.id);
      }
      setPendingChanges([]);
      onCreated(jobIDs.join(", "));
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "应用防火墙任务失败");
      setIsSubmitting(false);
    }
  }

  function submitPortRuleModal() {
    if (direction !== "ingress") {
      setErrorMessage("当前版本只支持 OCI 入站规则。出站规则需要单独设计，避免误断开实例访问。");
      return;
    }
    let ranges: Array<{ portMin: number; portMax: number }>;
    try {
      ranges = parseFirewallPorts(portExpression);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "端口格式不正确。");
      return;
    }
    const protocols: FirewallTaskPayload["protocol"][] = protocol === "tcp_udp" ? ["tcp", "udp"] : [protocol];
    for (const currentProtocol of protocols) {
      for (const range of ranges) {
        const portLabel = range.portMin === range.portMax ? String(range.portMin) : `${range.portMin}-${range.portMax}`;
        queueFirewallChange(`${rulePolicy === "open" ? "放行" : "不放行"} ${currentProtocol.toUpperCase()}/${portLabel}`, {
          action: rulePolicy,
          protocol: currentProtocol,
          portMin: range.portMin,
          portMax: range.portMax,
          sourceCidr,
          targetScope,
          vnicId: "primary",
          snapshotBefore,
          note: protocols.length > 1 ? `${note || "端口规则"} / ${currentProtocol.toUpperCase()}` : note
        });
      }
    }
    setShowRuleEditor(false);
    setErrorMessage("");
  }
  function editRule(rule: FirewallRule) {
    if (rule.protocol !== "tcp" && rule.protocol !== "udp") return;
    setProtocol(rule.protocol);
    setPortExpression(rule.portMax && rule.portMax !== rule.portMin ? `${rule.portMin || 1}-${rule.portMax}` : String(rule.portMin || 1));
    if (rule.source === "0.0.0.0/0" || rule.source === "-" || rule.source === "") {
      setSourceMode("all");
      setCustomSourceCidr("");
    } else {
      setSourceMode("custom");
      setCustomSourceCidr(rule.source);
    }
    setTargetScope(rule.containerType === "nsg" ? "nsg" : "security_list");
    setNote(rule.remark || "");
    setRulePolicy("open");
    setShowRuleEditor(true);
  }

  function deleteRule(rule: FirewallRule) {
    if (rule.isBroadRule) {
      if (!window.confirm("强制删除宽规则风险很高，可能立即中断 SSH、面板或业务访问。确认加入待应用列表？")) return;
      queueFirewallChange(`强制删除宽规则 ${rule.protocol}/${rule.source}`, {
        action: "delete_broad",
        protocol: "tcp",
        portMin: 1,
        portMax: 1,
        sourceCidr: rule.source === "-" ? "0.0.0.0/0" : rule.source,
        targetScope: rule.containerType === "nsg" ? "nsg" : "security_list",
        vnicId: "primary",
        containerId: rule.containerId,
        containerType: rule.containerType,
        ruleId: rule.id,
        snapshotBefore,
        note: `强制删除宽规则：${rule.containerType}/${rule.source}`
      });
      setErrorMessage("");
      return;
    }
    if (!rule.editable || (rule.protocol !== "tcp" && rule.protocol !== "udp")) {
      setErrorMessage("该规则不是精确 TCP/UDP 端口规则，当前不会自动修改非端口规则。");
      return;
    }
    queueFirewallChange(`?? ${rule.protocol}/${rule.portLabel}`, {
      action: "close",
      protocol: rule.protocol,
      portMin: rule.portMin || 0,
      portMax: rule.portMax || rule.portMin || 0,
      sourceCidr: rule.source,
      targetScope: rule.containerType === "nsg" ? "nsg" : "security_list",
      vnicId: "primary",
      snapshotBefore,
      note: `删除防火墙规则：${rule.protocol}/${rule.portLabel}`
    });
    setErrorMessage("");
  }

  function bulkDeleteSelected() {
    const candidates = selectedRules.filter((rule) => rule.isBroadRule || (rule.editable && (rule.protocol === "tcp" || rule.protocol === "udp")));
    if (candidates.length === 0) {
      setErrorMessage("请选择可以删除的精确端口规则或宽规则。");
      return;
    }
    candidates.forEach((rule) => deleteRule(rule));
  }
  function toggleRuleSelection(ruleId: string, checked: boolean) {
    setSelectedRuleIds((current) => checked ? Array.from(new Set([...current, ruleId])) : current.filter((id) => id !== ruleId));
  }

  function toggleAllVisible(checked: boolean) {
    setSelectedRuleIds(checked ? filteredRules.map((rule) => rule.id) : []);
  }

  function exportRules() {
    const blob = new Blob([JSON.stringify(inventory ?? { rules: [] }, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `${instance.name || instance.id}-firewall-rules.json`;
    anchor.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true">
      <div className="firewall-console glass-modal">
        <div className="modal-header-row firewall-header">
          <div className="modal-title-block">
            <div className="modal-icon compact">
              <ShieldAlert size={24} />
            </div>
            <div>
              <h2>防火墙设置</h2>
              <p>{instance.name} / {instance.primaryIp || instance.privateIp || "未分配 IP"} / {instance.region}</p>
            </div>
          </div>
          <button className="icon-button bordered" aria-label="关闭防火墙设置" onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className="firewall-switchbar">
          <div className="firewall-switch-item">
            <span>防火墙开关</span>
            <button className={`toggle-switch ${firewallEnabled ? "on" : ""}`} type="button" onClick={() => setFirewallEnabled((value) => !value)} />
          </div>
          <div className="firewall-switch-item">
            <span>禁 ping</span>
            <button className={`toggle-switch ${pingBlocked ? "on" : ""}`} type="button" onClick={() => setPingBlocked((value) => !value)} />
          </div>
          <div className="firewall-logline">
            <span>规则来源：</span>
            <strong>{inventory?.nsgIds?.length ? "NSG" : inventory?.securityListIds?.length ? "Security List" : "未加载"}</strong>
            <span>{inventory?.loadedAt ? `最后读取 ${new Date(inventory.loadedAt).toLocaleString()}` : ""}</span>
          </div>
          <button className="secondary-button compact" type="button" disabled>清空日志</button>
          <button className="secondary-button compact" type="button" disabled>清理缓存</button>
        </div>

        <div className="firewall-rule-summary">
          <strong>端口规则：{rules.length}</strong>
        </div>

        {broadRules.length > 0 ? (
          <div className="modal-warning firewall-warning">
            <ShieldAlert size={18} />
            <span>检测到 {broadRules.length} 条全部放行或宽规则。关闭单个端口不会真正关闭访问，需要先人工收窄宽规则。</span>
          </div>
        ) : null}
        {!inventory?.verified && inventory?.errorMessage ? <div className="inline-error">{inventory.errorMessage}</div> : null}
        {rulesError ? <div className="inline-error">{rulesError}</div> : null}
        {errorMessage ? <div className="inline-error">{errorMessage}</div> : null}

        <div className="firewall-toolbar">
          <div className="button-row compact">
            <button className="primary-button" type="button" onClick={() => setShowRuleEditor((value) => !value)}>添加端口规则</button>
            <button className="secondary-button" type="button" disabled><FileUp size={15} /> 导入规则</button>
            <button className="secondary-button" type="button" onClick={exportRules}><Download size={15} /> 导出规则</button>
            <button className="secondary-button" type="button" disabled>端口防扫描</button>
            <button className="secondary-button" type="button" onClick={() => void loadRules()} disabled={isLoadingRules}>
              <RefreshCw size={15} /> {isLoadingRules ? "刷新中" : "刷新"}
            </button>
          </div>
          <label className="search-input firewall-search">
            <Search size={16} />
            <input value={searchTerm} onChange={(event) => setSearchTerm(event.target.value)} placeholder="请输入端口/来源/备注" />
          </label>
        </div>

        {showRuleEditor ? createPortal((
          <div className="firewall-rule-dialog-backdrop">
            <div className="firewall-rule-dialog" role="dialog" aria-modal="true">
              <div className="firewall-rule-dialog-header">
                <h3>添加端口规则</h3>
                <button className="icon-button" type="button" aria-label="关闭添加端口规则" onClick={() => setShowRuleEditor(false)}>
                  <X size={18} />
                </button>
              </div>
              <div className="firewall-rule-form">
                <label>
                  <span>协议</span>
                  <select value={protocol} onChange={(event) => setProtocol(event.target.value as FirewallTaskPayload["protocol"] | "tcp_udp")}>
                    <option value="tcp">TCP</option>
                    <option value="udp">UDP</option>
                    <option value="tcp_udp">TCP/UDP</option>
                  </select>
                </label>
                <label>
                  <span><i>*</i> 端口</span>
                  <input value={portExpression} onChange={(event) => setPortExpression(event.target.value)} placeholder="请输入端口" />
                </label>
                <label>
                  <span>来源</span>
                  <select value={sourceMode} onChange={(event) => setSourceMode(event.target.value)}>
                    <option value="all">所有 IP</option>
                    <option value="custom">指定 CIDR</option>
                  </select>
                </label>
                {sourceMode === "custom" ? (
                  <label>
                    <span>CIDR</span>
                    <input value={customSourceCidr} onChange={(event) => setCustomSourceCidr(event.target.value)} placeholder="例如 203.0.113.10/32 或 ::/0" />
                  </label>
                ) : null}
                <label>
                  <span>策略</span>
                  <select value={rulePolicy} onChange={(event) => setRulePolicy(event.target.value as FirewallTaskPayload["action"])}>
                    <option value="open">放行</option>
                    <option value="close">不放行</option>
                  </select>
                </label>
                <label>
                  <span>方向</span>
                  <select value={direction} onChange={(event) => setDirection(event.target.value)}>
                    <option value="ingress">入站(默认)</option>
                  </select>
                </label>
                <label>
                  <span>备注</span>
                  <input value={note} onChange={(event) => setNote(event.target.value)} placeholder="请填写备注，可为空" />
                </label>
              </div>
              <ul className="firewall-rule-help">
                <li>支持添加多个端口，如：80,88</li>
                <li>支持添加多个端口范围，如：80,88,90-99,110-120</li>
                <li>“不放行”会删除完全匹配的 OCI 放行规则；宽规则需要先人工收窄。</li>
                <li>该功能修改 OCI NSG / Security List，不会修改实例系统内防火墙。</li>
              </ul>
              <div className="firewall-rule-dialog-actions">
                <button className="secondary-button" type="button" disabled={isSubmitting} onClick={() => setShowRuleEditor(false)}>取消</button>
                <button className="primary-button" type="button" disabled={isSubmitting} onClick={() => void submitPortRuleModal()}>
                  {isSubmitting ? "提交中..." : "确定"}
                </button>
              </div>
            </div>
          </div>
        ), document.body) : null}

        <div className="table-wrap firewall-table-wrap">
          <table className="network-table firewall-table">
            <thead>
              <tr>
                <th><input type="checkbox" checked={filteredRules.length > 0 && filteredRules.every((rule) => selectedRuleIds.includes(rule.id))} onChange={(event) => toggleAllVisible(event.target.checked)} /></th>
                <th>协议</th>
                <th>端口</th>
                <th>状态</th>
                <th>策略</th>
                <th>方向</th>
                <th>来源</th>
                <th>备注</th>
                <th>时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {isLoadingRules ? (
                <tr><td colSpan={10} className="empty-table-cell">正在读取 OCI 防火墙规则...</td></tr>
              ) : filteredRules.length === 0 ? (
                <tr><td colSpan={10} className="empty-table-cell">暂无可展示规则</td></tr>
              ) : filteredRules.map((rule) => (
                <tr key={rule.id} className={rule.isBroadRule ? "broad-rule-row" : ""}>
                  <td><input type="checkbox" checked={selectedRuleIds.includes(rule.id)} onChange={(event) => toggleRuleSelection(rule.id, event.target.checked)} /></td>
                  <td>{rule.protocol}</td>
                  <td>{rule.portLabel}</td>
                  <td>{rule.status}</td>
                  <td className="success-text">{rule.policy}</td>
                  <td>{rule.direction}</td>
                  <td>{rule.source === "0.0.0.0/0" || rule.source === "::/0" ? "所有 IP" : rule.source}</td>
                  <td>{rule.remark || `${rule.containerType === "nsg" ? "NSG" : "Security List"} / ${shortID(rule.containerId)}`}</td>
                  <td>{rule.time || "--"}</td>
                  <td>
                    <div className="inline-actions">
                      <button type="button" onClick={() => editRule(rule)} disabled={!rule.editable}>修改</button>
                      <button type="button" onClick={() => deleteRule(rule)} disabled={isSubmitting}>删除</button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {pendingChanges.length > 0 ? (
          <div className="firewall-pending-panel">
            <strong>待应用变更</strong>
            <div>
              {pendingChanges.map((change) => (
                <span key={change.id}>
                  {change.label}
                  <button type="button" onClick={() => removePendingChange(change.id)}>移除</button>
                </span>
              ))}
            </div>
          </div>
        ) : null}
        <div className="firewall-footer">
          <label>
            <input type="checkbox" checked={selectedRuleIds.length > 0} onChange={(event) => event.target.checked ? toggleAllVisible(true) : setSelectedRuleIds([])} />
          </label>
          <select value="" onChange={() => undefined}>
            <option value="">请选择批量操作</option>
            <option value="delete">删除所选规则</option>
          </select>
          <button className="secondary-button" type="button" disabled={pendingChanges.length === 0 || isSubmitting} onClick={() => setPendingChanges([])}>清空待应用</button>
          <button className="primary-button soft" type="button" disabled={selectedRuleIds.length === 0 || isSubmitting} onClick={() => bulkDeleteSelected()}>批量操作</button>
          <button className="primary-button" type="button" disabled={pendingChanges.length === 0 || isSubmitting} onClick={() => void applyPendingChanges()}>应用{pendingChanges.length ? ` (${pendingChanges.length})` : ""}</button>
          <span>共 {filteredRules.length} 条</span>
        </div>
      </div>
    </div>
  );
}

function LegacyFirewallManagementModal({
  instance,
  onClose,
  onCreated
}: {
  instance: Instance;
  onClose: () => void;
  onCreated: (jobId: string) => void;
}) {
  const [action, setAction] = useState<FirewallTaskPayload["action"]>("open");
  const [protocol, setProtocol] = useState<FirewallTaskPayload["protocol"]>("tcp");
  const [portMin, setPortMin] = useState(22);
  const [portMax, setPortMax] = useState(22);
  const [sourceCidr, setSourceCidr] = useState("0.0.0.0/0");
  const [targetScope, setTargetScope] = useState<FirewallTaskPayload["targetScope"]>("auto");
  const [vnicId, setVnicId] = useState("primary");
  const [snapshotBefore, setSnapshotBefore] = useState(true);
  const [note, setNote] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");

  const normalizedPortMin = Number(portMin) || 0;
  const normalizedPortMax = Number(portMax) || normalizedPortMin;
  const invalidPortRange = normalizedPortMin <= 0 || normalizedPortMin > 65535 || normalizedPortMax < normalizedPortMin || normalizedPortMax > 65535;

  function applyQuickPort(option: (typeof firewallQuickPorts)[number]) {
    setProtocol(option.protocol);
    setPortMin(option.portMin);
    setPortMax(option.portMax);
  }

  async function handleCreateTask() {
    if (isSubmitting || invalidPortRange) return;
    setIsSubmitting(true);
    setErrorMessage("");
    try {
      const job = await createFirewallTask(instance.id, {
        action,
        protocol,
        portMin: normalizedPortMin,
        portMax: normalizedPortMax,
        sourceCidr,
        targetScope,
        vnicId,
        snapshotBefore,
        note
      });
      onCreated(job.id);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "创建防火墙任务失败");
      setIsSubmitting(false);
    }
  }

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true">
      <div className="action-modal glass-modal">
        <div className="modal-header-row">
          <div className="modal-title-block">
            <div className="modal-icon compact">
              <ShieldAlert size={24} />
            </div>
            <div>
              <h2>防火墙管理</h2>
              <p>{instance.name} 的 OCI 入站端口开放与关闭任务。</p>
            </div>
          </div>
          <button className="icon-button bordered" aria-label="关闭防火墙管理" onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className="modal-summary-card">
          <div><span>公网 IP</span><strong>{instance.primaryIp || "-"}</strong></div>
          <div><span>私网 IP</span><strong>{instance.privateIp || "-"}</strong></div>
          <div><span>区域</span><strong>{instance.region}</strong></div>
          <div><span>Shape</span><strong>{instance.shape}</strong></div>
        </div>

        <div className="form-section">
          <div className="form-section-title">
            <ShieldAlert size={18} />
            <span>规则动作</span>
          </div>
          <div className="choice-grid retry-choice-grid">
            <button className={`choice-card ${action === "open" ? "active" : ""}`} type="button" onClick={() => setAction("open")}>
              <strong>开放端口</strong>
              <span>如果规则不存在，则追加一条 OCI 入站规则。</span>
            </button>
            <button className={`choice-card ${action === "close" ? "active" : ""}`} type="button" onClick={() => setAction("close")}>
              <strong>关闭端口</strong>
              <span>删除协议、端口、来源 CIDR 完全匹配的 OCI 入站规则。</span>
            </button>
          </div>
        </div>

        <div className="form-section">
          <div className="button-row compact">
            {firewallQuickPorts.map((option) => (
              <button className="secondary-button" key={option.label} type="button" onClick={() => applyQuickPort(option)}>
                {option.label} {option.portMin}
              </button>
            ))}
          </div>
          <div className="form-grid">
            <label>
              协议
              <select value={protocol} onChange={(event) => setProtocol(event.target.value as FirewallTaskPayload["protocol"])}>
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
              </select>
            </label>
            <label>
              起始端口
              <input type="number" min={1} max={65535} value={portMin} onChange={(event) => setPortMin(Number(event.target.value))} />
            </label>
            <label>
              结束端口
              <input type="number" min={1} max={65535} value={portMax} onChange={(event) => setPortMax(Number(event.target.value))} />
            </label>
            <label>
              来源 CIDR
              <input value={sourceCidr} onChange={(event) => setSourceCidr(event.target.value)} placeholder="0.0.0.0/0 或 ::/0" />
            </label>
            <label>
              作用范围
              <select value={targetScope} onChange={(event) => setTargetScope(event.target.value as FirewallTaskPayload["targetScope"])}>
                <option value="auto">自动：优先 NSG，否则 Security List</option>
                <option value="nsg">仅 Network Security Group</option>
                <option value="security_list">仅子网 Security List</option>
              </select>
            </label>
            <label>
              VNIC
              <select value={vnicId} onChange={(event) => setVnicId(event.target.value)}>
                <option value="primary">primary-vnic / {instance.privateIp || "-"}</option>
              </select>
            </label>
            <label className="full-row">
              任务备注
              <input value={note} onChange={(event) => setNote(event.target.value)} placeholder="例如：临时开放 SSH 排障，完成后关闭" />
            </label>
          </div>
        </div>

        <div className="switch-row">
          <div>
            <strong>操作前记录快照</strong>
            <p>记录目标 VNIC、NSG 和 Security List 信息，用于审计和回滚判断。</p>
          </div>
          <button className={`toggle-switch ${snapshotBefore ? "on" : ""}`} onClick={() => setSnapshotBefore((value) => !value)} />
        </div>

        <div className="modal-warning">
          <ShieldAlert size={18} />
          <span>该功能修改 OCI 网络安全规则，不会修改实例系统内的 iptables、firewalld 或 Windows 防火墙。</span>
        </div>
        {invalidPortRange ? <div className="inline-error">端口范围必须在 1-65535 之间，且结束端口不能小于起始端口。</div> : null}
        {errorMessage ? <div className="inline-error">{errorMessage}</div> : null}

        <div className="button-row">
          <button className="secondary-button" disabled={isSubmitting} onClick={onClose}>取消</button>
          <button className="primary-button" disabled={isSubmitting || invalidPortRange} onClick={() => void handleCreateTask()}>
            {isSubmitting ? "创建中..." : action === "open" ? "创建开放端口任务" : "创建关闭端口任务"}
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

function reinstallImagesForShape(options: LaunchOptions, shapeName: string) {
  const byShape = options.shapeImages?.[shapeName] ?? [];
  if (byShape.length > 0) return byShape;
  return options.images ?? [];
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
