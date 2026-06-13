import { AlertTriangle, ArrowDownWideNarrow, Bell, DollarSign, FileWarning, Gauge, Power, Save, Server, ShieldCheck, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { PageHeader } from "../components/PageHeader";
import { getBudgetSettings, listInstances, updateBudgetSettings } from "../services/api";
import type { BudgetSettings, Instance } from "../services/api";

type BudgetPolicyState = {
  enabled: boolean;
  monthlyBudget: number;
  actualSpend: number;
  forecastSpend: number;
  thresholdPercent: number;
  scopeMode: BudgetSettings["scopeMode"];
  profileId: string;
  region: string;
  compartmentId: string;
  resourcePool: string;
  tagKey: string;
  tagValue: string;
  manualInstanceIds: string;
  actionMode: BudgetSettings["actionMode"];
  downgradePreset: string;
  deleteBootVolume: boolean;
  requireApproval: boolean;
};

const defaultPolicy: BudgetPolicyState = {
  enabled: false,
  monthlyBudget: 10,
  actualSpend: 0,
  forecastSpend: 0,
  thresholdPercent: 90,
  scopeMode: "tag",
  profileId: "DEFAULT",
  region: "ap-chuncheon-1",
  compartmentId: "Root tenancy",
  resourcePool: "",
  tagKey: "budget.autoAction",
  tagValue: "enabled",
  manualInstanceIds: "",
  actionMode: "downgrade",
  downgradePreset: "free-first",
  deleteBootVolume: false,
  requireApproval: true
};

const actionModes = [
  {
    id: "notify",
    title: "只通知",
    description: "超出阈值后只发站内通知、邮件和 Webhook，不改动实例。"
  },
  {
    id: "downgrade",
    title: "优先降配",
    description: "先尝试把候选机器降到预设规格或免费范围，失败后进入人工处理。"
  },
  {
    id: "delete",
    title: "超出预算即删机",
    description: "只处理明确纳入范围并允许自动删除的实例，默认保留启动卷。"
  }
] as const;

const flow = [
  ["采集", "OCI Budgets、Cost Usage、实例清单和本地价格估算"],
  ["判定", "实际花费、预测花费、免费额度、每日增量和阈值"],
  ["候选", "按 Profile、Compartment、标签、手工选择和保护标记筛选机器"],
  ["执行", "冻结创建、停止、降配、终止实例，全部进入任务中心"],
  ["审计", "记录操作者、策略、候选列表、OCI Request ID 和执行结果"]
];

const guardrails = [
  "预算熔断默认只通知；自动降配和自动删机必须显式启用。",
  "只处理带有 budget.autoAction=enabled 或被策略范围选中的机器。",
  "受保护实例、平台关键实例、最近创建冷却期内实例永远跳过。",
  "删除实例默认 preserveBootVolume=true；删除启动卷需要单独开关。",
  "每次执行都有最大实例数、最大 OCPU、最大月预算、最大每日执行次数。",
  "自动降配前必须做兼容性检查：镜像、架构、Shape、AD、容量、服务限制。",
  "删机或大规模降配建议强制审批；紧急熔断模式才允许自动执行。"
];

export function BudgetManagementPage() {
  const [instances, setInstances] = useState<Instance[]>([]);
  const [isLoadingInstances, setIsLoadingInstances] = useState(false);
  const [instanceError, setInstanceError] = useState("");
  const [manualSearch, setManualSearch] = useState("");
  const [policy, setPolicy] = useState<BudgetPolicyState>(defaultPolicy);
  const [isLoadingSettings, setIsLoadingSettings] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [saveMessage, setSaveMessage] = useState("");
  const [settingsError, setSettingsError] = useState("");

  const actualPercent = useMemo(() => percent(policy.actualSpend, policy.monthlyBudget), [policy.actualSpend, policy.monthlyBudget]);
  const forecastPercent = useMemo(() => percent(policy.forecastSpend, policy.monthlyBudget), [policy.forecastSpend, policy.monthlyBudget]);
  const triggered = Math.max(actualPercent, forecastPercent) >= policy.thresholdPercent;
  const scopeSummary = useMemo(() => summarizeScope(policy), [policy]);
  const manualCount = useMemo(
    () => policy.manualInstanceIds.split(/\r?\n|,/).map((item) => item.trim()).filter(Boolean).length,
    [policy.manualInstanceIds]
  );
  const selectedManualIds = useMemo(() => parseManualInstanceIds(policy.manualInstanceIds), [policy.manualInstanceIds]);
  const filteredInstances = useMemo(() => filterInstances(instances, manualSearch), [instances, manualSearch]);
  const selectableInstances = useMemo(() => filteredInstances.filter((instance) => canSelectInstance(instance)), [filteredInstances]);

  useEffect(() => {
    void loadBudgetSettings();
    void loadBudgetInstances();
  }, []);

  async function loadBudgetSettings() {
    setIsLoadingSettings(true);
    setSettingsError("");
    try {
      setPolicy(policyFromSettings(await getBudgetSettings()));
    } catch (error) {
      setSettingsError(error instanceof Error ? error.message : "加载预算配置失败");
    } finally {
      setIsLoadingSettings(false);
    }
  }

  async function loadBudgetInstances() {
    setIsLoadingInstances(true);
    setInstanceError("");
    try {
      setInstances(await listInstances());
    } catch (error) {
      setInstanceError(error instanceof Error ? error.message : "加载实例列表失败");
    } finally {
      setIsLoadingInstances(false);
    }
  }

  function update<K extends keyof typeof policy>(key: K, value: (typeof policy)[K]) {
    setPolicy((current) => ({ ...current, [key]: value }));
    setSaveMessage("");
  }

  async function savePolicy() {
    setIsSaving(true);
    setSettingsError("");
    setSaveMessage("");
    try {
      const saved = await updateBudgetSettings(settingsFromPolicy(policy));
      setPolicy(policyFromSettings(saved));
      setSaveMessage(`已保存预算配置${saved.enabled ? "，预算管理已开启" : "，预算管理已关闭"}。`);
    } catch (error) {
      setSettingsError(error instanceof Error ? error.message : "保存预算配置失败");
    } finally {
      setIsSaving(false);
    }
  }

  function setManualSelection(ids: Set<string>) {
    update("manualInstanceIds", Array.from(ids).sort().join("\n"));
  }

  function toggleManualInstance(instance: Instance) {
    const id = instanceIdentity(instance);
    if (!id || !canSelectInstance(instance)) return;
    const next = new Set(selectedManualIds);
    if (next.has(id)) {
      next.delete(id);
    } else {
      next.add(id);
    }
    setManualSelection(next);
  }

  function selectVisibleInstances() {
    const next = new Set(selectedManualIds);
    selectableInstances.forEach((instance) => {
      const id = instanceIdentity(instance);
      if (id) next.add(id);
    });
    setManualSelection(next);
  }

  function clearManualInstances() {
    update("manualInstanceIds", "");
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="平台治理"
        title="预算管理"
        description="管理 OCI 预算阈值、免费额度、自动降配和预算熔断。当前页面是策略入口，真实执行器接入后才会自动改动实例。"
        actions={
          <div className="budget-header-actions">
            <button
              className={`secondary-button budget-state-button ${policy.enabled ? "enabled" : ""}`}
              disabled={isLoadingSettings || isSaving}
              type="button"
              onClick={() => update("enabled", !policy.enabled)}
            >
              <Power size={18} />
              {policy.enabled ? "预算管理：开启" : "预算管理：关闭"}
            </button>
            <button className="primary-button" disabled={isLoadingSettings || isSaving} type="button" onClick={() => void savePolicy()}>
              <Save size={18} />
              {isSaving ? "保存中..." : "保存配置"}
            </button>
            <button className="secondary-button" disabled={isLoadingSettings || isSaving} type="button" onClick={() => void loadBudgetSettings()}>
              重新加载
            </button>
          </div>
        }
      />

      <section className="glass-panel budget-control-panel">
        <div>
          <span className={`status-pill ${policy.enabled ? "success" : "quiet"}`}>
            {policy.enabled ? "已开启" : "已关闭"}
          </span>
          <h2>预算管理总开关</h2>
          <p>保存后会持久化开关状态、管理范围和超预算动作。关闭时只保存策略，不生成预算评估任务。</p>
        </div>
        <div className="budget-control-actions">
          <button
            aria-label="切换预算管理开关"
            className={`toggle-switch ${policy.enabled ? "on" : ""}`}
            disabled={isLoadingSettings || isSaving}
            type="button"
            onClick={() => update("enabled", !policy.enabled)}
          />
          <button className="primary-button" disabled={isLoadingSettings || isSaving} type="button" onClick={() => void savePolicy()}>
            <Save size={18} />
            {isSaving ? "保存中..." : "保存配置"}
          </button>
        </div>
      </section>

      <div className="metric-grid">
        <section className="glass-panel metric-card">
          <div className="metric-topline">
            <div className="metric-icon"><DollarSign size={18} /></div>
            <span>月预算</span>
          </div>
          <div className="metric-value">${policy.monthlyBudget.toFixed(2)}</div>
          <p>策略预算上限，可映射 OCI Budget 或本地预算。</p>
        </section>
        <section className="glass-panel metric-card blue">
          <div className="metric-topline">
            <div className="metric-icon"><Gauge size={18} /></div>
            <span>实际用量</span>
          </div>
          <div className="metric-value">{actualPercent.toFixed(0)}%</div>
          <p>${policy.actualSpend.toFixed(2)} / ${policy.monthlyBudget.toFixed(2)}</p>
        </section>
        <section className="glass-panel metric-card yellow">
          <div className="metric-topline">
            <div className="metric-icon"><FileWarning size={18} /></div>
            <span>预测用量</span>
          </div>
          <div className="metric-value">{forecastPercent.toFixed(0)}%</div>
          <p>${policy.forecastSpend.toFixed(2)} forecast</p>
        </section>
        <section className={`glass-panel metric-card ${triggered ? "yellow" : "green"}`}>
          <div className="metric-topline">
            <div className="metric-icon"><AlertTriangle size={18} /></div>
            <span>触发状态</span>
          </div>
          <div className="metric-value">{triggered ? "触发" : "正常"}</div>
          <p>阈值 {policy.thresholdPercent}%</p>
        </section>
      </div>

      <section className="glass-panel section-card">
        <div className="section-title-row">
          <div>
            <h2>预算策略</h2>
            <p>这里配置预算熔断策略。保存后会持久化开关状态和范围配置；真实执行仍必须经过后端 dry-run、审批和任务中心。</p>
          </div>
          <ShieldCheck size={24} />
        </div>

        <div className="budget-save-bar">
          <div className="switch-row">
            <div>
              <strong>启用预算管理</strong>
              <p>关闭时只保存策略配置，不创建预算评估任务；开启后由后端评估器按配置生成候选和任务。</p>
            </div>
            <button className={`toggle-switch ${policy.enabled ? "on" : ""}`} disabled={isLoadingSettings || isSaving} onClick={() => update("enabled", !policy.enabled)} />
          </div>
          <div className="budget-save-actions">
            <button className="primary-button" disabled={isLoadingSettings || isSaving} onClick={() => void savePolicy()}>
              {isSaving ? "保存中..." : "保存配置"}
            </button>
            <button className="secondary-button" disabled={isLoadingSettings || isSaving} onClick={() => void loadBudgetSettings()}>
              重新加载
            </button>
          </div>
        </div>

        {settingsError ? <div className="inline-error">{settingsError}</div> : null}
        {saveMessage ? <div className="inline-success">{saveMessage}</div> : null}
        {isLoadingSettings ? <div className="inline-success">正在加载已保存的预算配置...</div> : null}

        <div className="form-grid">
          <label>
            月预算 USD
            <input type="number" min="1" value={policy.monthlyBudget} onChange={(event) => update("monthlyBudget", Number(event.target.value))} />
          </label>
          <label>
            触发阈值 %
            <input type="number" min="1" max="10000" value={policy.thresholdPercent} onChange={(event) => update("thresholdPercent", Number(event.target.value))} />
          </label>
          <label>
            当前实际花费 USD
            <input type="number" min="0" value={policy.actualSpend} onChange={(event) => update("actualSpend", Number(event.target.value))} />
          </label>
          <label>
            预测花费 USD
            <input type="number" min="0" value={policy.forecastSpend} onChange={(event) => update("forecastSpend", Number(event.target.value))} />
          </label>
          <label>
            降配预设
            <select value={policy.downgradePreset} onChange={(event) => update("downgradePreset", event.target.value)}>
              <option value="free-first">优先免费范围</option>
              <option value="min-flex">Flex 最小规格</option>
              <option value="custom">自定义 Shape/OCPU/内存</option>
              <option value="stop-only">无法降配则停止</option>
            </select>
          </label>
        </div>

        <div className="form-section">
          <div className="form-section-title">
            <Server size={18} />
            <span>机器范围选择</span>
          </div>
          <p className="muted-line">预算动作只会处理这里选中的范围，不会默认扫全账号。后端落地时还会再次校验保护标记和审批状态。</p>
          <div className="scope-mode-grid">
            {([
              ["tag", "按标签", "只处理匹配标签的机器，推荐用于自动动作。"],
              ["compartment", "按 Compartment", "限定 Profile / Region / Compartment。"],
              ["manual", "手工选择实例", "从实例表格中勾选本策略会影响的机器。"]
            ] as const).map(([id, title, description]) => (
              <button className={`choice-card ${policy.scopeMode === id ? "active" : ""}`} key={id} type="button" onClick={() => update("scopeMode", id)}>
                <strong>{title}</strong>
                <span>{description}</span>
              </button>
            ))}
          </div>

          <div className="scope-editor">
            <div className="scope-common-grid">
              <label>
                Profile
                <input value={policy.profileId} onChange={(event) => update("profileId", event.target.value)} placeholder="DEFAULT 或 profile ID" />
              </label>
              <label>
                Region
                <input value={policy.region} onChange={(event) => update("region", event.target.value)} placeholder="ap-chuncheon-1" />
              </label>
            </div>

            {policy.scopeMode === "tag" ? (
              <div className="scope-mode-panel">
                <div>
                  <strong>按标签筛选</strong>
                  <p>推荐用于自动化预算动作。后端 dry-run 会只匹配带有该标签的实例。</p>
                </div>
                <div className="form-grid">
                  <label>
                    Compartment
                    <input value={policy.compartmentId} onChange={(event) => update("compartmentId", event.target.value)} placeholder="Root tenancy 或 compartment OCID" />
                  </label>
                  <label>
                    标签 Key
                    <input value={policy.tagKey} onChange={(event) => update("tagKey", event.target.value)} placeholder="budget.autoAction" />
                  </label>
                  <label>
                    标签 Value
                    <input value={policy.tagValue} onChange={(event) => update("tagValue", event.target.value)} placeholder="enabled" />
                  </label>
                </div>
              </div>
            ) : null}

            {policy.scopeMode === "compartment" ? (
              <div className="scope-mode-panel">
                <div>
                  <strong>按 Compartment 筛选</strong>
                  <p>只处理指定 Profile / Region / Compartment 内的实例。危险动作仍建议保留审批。</p>
                </div>
                <div className="form-grid">
                  <label className="span-two">
                    Compartment
                    <input value={policy.compartmentId} onChange={(event) => update("compartmentId", event.target.value)} placeholder="Root tenancy 或 compartment OCID" />
                  </label>
                </div>
              </div>
            ) : null}

            {policy.scopeMode === "manual" ? (
              <div className="scope-mode-panel">
                <div>
                  <strong>按勾选实例精确控制</strong>
                  <p>只有下方表格中勾选的实例会进入影响范围。空选择意味着候选数量为 0。</p>
                </div>
              </div>
            ) : null}
          </div>

          {policy.scopeMode === "manual" ? (
            <ManualInstanceSelector
              error={instanceError}
              instances={filteredInstances}
              isLoading={isLoadingInstances}
              manualSearch={manualSearch}
              onClear={clearManualInstances}
              onRefresh={() => void loadBudgetInstances()}
              onSearch={setManualSearch}
              onSelectVisible={selectVisibleInstances}
              onToggle={toggleManualInstance}
              selectedIds={selectedManualIds}
              totalCount={instances.length}
            />
          ) : null}

          <div className="scope-summary">
            <strong>当前候选范围</strong>
            <p>{scopeSummary}</p>
            <span>{policy.scopeMode === "manual" ? `手工选择 ${manualCount} 台` : "实际候选数量需要后端 dry-run 后返回"}</span>
          </div>
        </div>

        <div className="budget-action-grid">
          {actionModes.map((mode) => {
            const active = policy.actionMode === mode.id;
            const Icon = mode.id === "notify" ? Bell : mode.id === "downgrade" ? ArrowDownWideNarrow : Trash2;
            return (
              <button className={`choice-card ${active ? "active" : ""}`} key={mode.id} type="button" onClick={() => update("actionMode", mode.id)}>
                <Icon size={22} />
                <strong>{mode.title}</strong>
                <span>{mode.description}</span>
              </button>
            );
          })}
        </div>

        <div className="switch-panel">
          <div className="switch-row">
            <div>
              <strong>危险动作需要审批</strong>
              <p>建议保持开启。删机、释放公网 IP、删除启动卷都应进入审批队列。</p>
            </div>
            <button className={`toggle-switch ${policy.requireApproval ? "on" : ""}`} onClick={() => update("requireApproval", !policy.requireApproval)} />
          </div>
          <div className="switch-row">
            <div>
              <strong>允许同时删除启动卷</strong>
              <p>默认关闭。开启后才允许在终止实例时删除 boot volume。</p>
            </div>
            <button className={`toggle-switch ${policy.deleteBootVolume ? "on" : ""}`} onClick={() => update("deleteBootVolume", !policy.deleteBootVolume)} />
          </div>
        </div>
      </section>

      <div className="card-grid two">
        <section className="glass-panel section-card">
          <div className="section-title-row">
            <div>
              <h2>执行流程</h2>
              <p>预算超限不会直接碰机器，先生成可审计的候选和任务。</p>
            </div>
            <Server size={24} />
          </div>
          <div className="governance-flow">
            {flow.map(([title, description], index) => (
              <div key={title}>
                <span>{index + 1}</span>
                <strong>{title}</strong>
                <p>{description}</p>
              </div>
            ))}
          </div>
        </section>

        <section className="glass-panel section-card">
          <div className="section-title-row">
            <div>
              <h2>安全护栏</h2>
              <p>这些规则应写入后端校验，前端只负责展示和配置。</p>
            </div>
            <AlertTriangle size={24} />
          </div>
          <ul className="guardrail-list">
            {guardrails.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </section>
      </div>
    </div>
  );
}

function percent(value: number, total: number) {
  if (!Number.isFinite(value) || !Number.isFinite(total) || total <= 0) return 0;
  return (value / total) * 100;
}

function ManualInstanceSelector({
  error,
  instances,
  isLoading,
  manualSearch,
  onClear,
  onRefresh,
  onSearch,
  onSelectVisible,
  onToggle,
  selectedIds,
  totalCount
}: {
  error: string;
  instances: Instance[];
  isLoading: boolean;
  manualSearch: string;
  onClear: () => void;
  onRefresh: () => void;
  onSearch: (value: string) => void;
  onSelectVisible: () => void;
  onToggle: (instance: Instance) => void;
  selectedIds: Set<string>;
  totalCount: number;
}) {
  return (
    <div className="manual-instance-panel">
      <div className="manual-instance-toolbar">
        <div>
          <strong>手工影响范围</strong>
          <p>从真实实例清单中勾选机器；预算降配或删机只会进入这些被选中的实例范围。</p>
        </div>
        <div className="manual-instance-actions">
          <input value={manualSearch} onChange={(event) => onSearch(event.target.value)} placeholder="搜索名称、IP、Shape、Region" />
          <button className="secondary-button" type="button" onClick={onRefresh} disabled={isLoading}>
            {isLoading ? "刷新中..." : "刷新实例"}
          </button>
          <button className="secondary-button" type="button" onClick={onSelectVisible} disabled={isLoading || instances.length === 0}>
            选择当前可操作
          </button>
          <button className="secondary-button danger" type="button" onClick={onClear} disabled={selectedIds.size === 0}>
            清空
          </button>
        </div>
      </div>

      <div className="manual-instance-meta">
        <span>实例总数 {totalCount}</span>
        <span>当前显示 {instances.length}</span>
        <span>已选择 {selectedIds.size}</span>
      </div>

      {error ? <div className="inline-error">{error}</div> : null}

      <div className="table-wrap manual-instance-table-wrap">
        <table className="manual-instance-table">
          <thead>
            <tr>
              <th>选择</th>
              <th>实例</th>
              <th>状态</th>
              <th>规格</th>
              <th>区域</th>
              <th>IP</th>
              <th>启动盘</th>
              <th>保护</th>
            </tr>
          </thead>
          <tbody>
            {instances.map((instance) => {
              const id = instanceIdentity(instance);
              const selectable = canSelectInstance(instance);
              return (
                <tr className={!selectable ? "disabled-row" : ""} key={id || instance.id}>
                  <td>
                    <input
                      aria-label={`选择 ${instance.name}`}
                      checked={id ? selectedIds.has(id) : false}
                      disabled={!selectable || !id}
                      type="checkbox"
                      onChange={() => onToggle(instance)}
                    />
                  </td>
                  <td>
                    <strong>{instance.name || instance.id}</strong>
                    <span>{instance.ociInstanceId || instance.id}</span>
                  </td>
                  <td><span className={`status-pill ${statusClass(instance.status)}`}>{instance.status}</span></td>
                  <td>{instance.shape} / {instance.ocpus}c {instance.memoryGb}G</td>
                  <td>{instance.region}</td>
                  <td>{instance.primaryIp || instance.privateIp || "-"}</td>
                  <td>{instance.bootVolumeGb || 0} GB</td>
                  <td>{instance.protected ? "受保护" : selectable ? "可操作" : "不可选"}</td>
                </tr>
              );
            })}
            {!isLoading && instances.length === 0 ? (
              <tr>
                <td colSpan={8}>没有可显示实例。请刷新实例管理同步结果，或检查当前 OCI Profile 权限。</td>
              </tr>
            ) : null}
            {isLoading ? (
              <tr>
                <td colSpan={8}>正在加载实例列表...</td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function parseManualInstanceIds(value: string) {
  return new Set(value.split(/\r?\n|,/).map((item) => item.trim()).filter(Boolean));
}

function instanceIdentity(instance: Instance) {
  return instance.ociInstanceId || instance.id;
}

function canSelectInstance(instance: Instance) {
  const status = String(instance.status).toLowerCase();
  return !instance.protected && !status.includes("terminat");
}

function filterInstances(instances: Instance[], query: string) {
  const normalized = query.trim().toLowerCase();
  if (!normalized) return instances;
  return instances.filter((instance) => {
    const haystack = [
      instance.id,
      instance.ociInstanceId,
      instance.name,
      instance.status,
      instance.shape,
      instance.region,
      instance.compartment,
      instance.primaryIp,
      instance.privateIp
    ].join(" ").toLowerCase();
    return haystack.includes(normalized);
  });
}

function statusClass(status: string) {
  const normalized = String(status).toLowerCase();
  if (normalized.includes("running")) return "running";
  if (normalized.includes("stopped")) return "stopped";
  if (normalized.includes("terminat")) return "terminating";
  if (normalized.includes("provision")) return "provisioning";
  return "warning";
}

function policyFromSettings(settings: BudgetSettings): BudgetPolicyState {
  return {
    enabled: Boolean(settings.enabled),
    monthlyBudget: settings.monthlyBudgetUsd || defaultPolicy.monthlyBudget,
    actualSpend: settings.actualSpendUsd || 0,
    forecastSpend: settings.forecastSpendUsd || 0,
    thresholdPercent: settings.thresholdPercent || defaultPolicy.thresholdPercent,
    scopeMode: normalizeScopeMode(settings.scopeMode),
    profileId: settings.profileId || defaultPolicy.profileId,
    region: settings.region || defaultPolicy.region,
    compartmentId: settings.compartmentId || defaultPolicy.compartmentId,
    resourcePool: settings.resourcePool || "",
    tagKey: settings.tagKey || defaultPolicy.tagKey,
    tagValue: settings.tagValue || defaultPolicy.tagValue,
    manualInstanceIds: (settings.manualInstanceIds ?? []).join("\n"),
    actionMode: settings.actionMode || defaultPolicy.actionMode,
    downgradePreset: settings.downgradePreset || defaultPolicy.downgradePreset,
    deleteBootVolume: Boolean(settings.deleteBootVolume),
    requireApproval: settings.requireApproval !== false
  };
}

function normalizeScopeMode(scopeMode: string): BudgetPolicyState["scopeMode"] {
  if (scopeMode === "compartment" || scopeMode === "manual") return scopeMode;
  return "tag";
}

function settingsFromPolicy(policy: BudgetPolicyState): BudgetSettings {
  return {
    enabled: policy.enabled,
    monthlyBudgetUsd: policy.monthlyBudget,
    actualSpendUsd: policy.actualSpend,
    forecastSpendUsd: policy.forecastSpend,
    thresholdPercent: policy.thresholdPercent,
    scopeMode: policy.scopeMode,
    profileId: policy.profileId,
    region: policy.region,
    compartmentId: policy.compartmentId,
    resourcePool: policy.resourcePool,
    tagKey: policy.tagKey,
    tagValue: policy.tagValue,
    manualInstanceIds: Array.from(parseManualInstanceIds(policy.manualInstanceIds)),
    actionMode: policy.actionMode,
    downgradePreset: policy.downgradePreset,
    deleteBootVolume: policy.deleteBootVolume,
    requireApproval: policy.requireApproval
  };
}

function summarizeScope(policy: {
  scopeMode: string;
  profileId: string;
  region: string;
  compartmentId: string;
  resourcePool: string;
  tagKey: string;
  tagValue: string;
  manualInstanceIds: string;
}) {
  const base = [`Profile=${policy.profileId || "*"}`, `Region=${policy.region || "*"}`];
  if (policy.scopeMode === "manual") {
    const ids = policy.manualInstanceIds.split(/\r?\n|,/).map((item) => item.trim()).filter(Boolean);
    return ids.length > 0 ? `仅手工选择的 ${ids.length} 台实例` : "手工实例范围为空，不会选中任何机器";
  }
  if (policy.scopeMode === "pool") {
    return [...base, `Pool=${policy.resourcePool || "未指定"}`].join(" / ");
  }
  if (policy.scopeMode === "compartment") {
    return [...base, `Compartment=${policy.compartmentId || "*"}`].join(" / ");
  }
  return [...base, `Compartment=${policy.compartmentId || "*"}`, `Tag ${policy.tagKey || "*"}=${policy.tagValue || "*"}`].join(" / ");
}
