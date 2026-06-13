import { CalendarClock, Pause, Play, Plus, ShieldCheck, TimerReset, Workflow, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import { StatusPill } from "../components/StatusPill";
import { createAutomationTask, listAutomations } from "../services/api";
import type { AutomationRule } from "../services/api";

export function AutomationsPage() {
  const [isTaskModalOpen, setIsTaskModalOpen] = useState(false);
  const [automations, setAutomations] = useState<AutomationRule[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [errorMessage, setErrorMessage] = useState("");

  const reloadAutomations = useCallback(async () => {
    setIsLoading(true);
    setErrorMessage("");
    try {
      setAutomations(await listAutomations());
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "加载自动化规则失败");
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void reloadAutomations();
  }, [reloadAutomations]);

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="策略引擎"
        title="自动化规则"
        description="创建带护栏的定时操作、指标扩缩容、容量重试和到期回收任务。"
        actions={
          <button className="primary-button" onClick={() => setIsTaskModalOpen(true)}>
            <Plus size={18} />
            添加任务
          </button>
        }
      />

      <AsyncState
        isLoading={isLoading}
        error={errorMessage}
        empty={!isLoading && automations.length === 0}
        emptyText="暂无自动化规则"
      />

      {!isLoading && !errorMessage ? (
        <div className="card-grid two">
          {automations.map((rule) => (
          <section className="glass-panel section-card" key={rule.name}>
            <div className="section-title-row">
              <div>
                <h2>{rule.name}</h2>
                <p>{rule.type}</p>
              </div>
              <Workflow size={24} />
            </div>
            <dl className="resource-facts">
              <div><dt>目标</dt><dd>{rule.targetPool}</dd></div>
              <div><dt>触发条件</dt><dd>{rule.triggerInterval}</dd></div>
              <div><dt>执行动作</dt><dd>{rule.action}</dd></div>
              <div><dt>安全护栏</dt><dd>最多 {rule.maxInstances} 台，每日 {rule.maxDailyRuns} 次</dd></div>
              <div><dt>状态</dt><dd><StatusPill status={rule.enabled ? "Enabled" : "Paused"} /></dd></div>
            </dl>
            <div className="button-row">
              <button className="secondary-button">
                {rule.enabled ? <Pause size={16} /> : <Play size={16} />}
                {rule.enabled ? "暂停" : "启用"}
              </button>
              <button className="primary-button">立即执行</button>
            </div>
          </section>
          ))}
        </div>
      ) : null}

      {isTaskModalOpen ? (
        <AutomationTaskModal
          onClose={() => setIsTaskModalOpen(false)}
          onCreated={reloadAutomations}
        />
      ) : null}
    </div>
  );
}

function AutomationTaskModal({
  onClose,
  onCreated
}: {
  onClose: () => void;
  onCreated: () => Promise<void>;
}) {
  const [form, setForm] = useState({
    name: "",
    type: "容量重试",
    targetPool: "",
    action: "创建 1 台实例",
    triggerInterval: "每 5 分钟",
    cooldown: "30 分钟",
    maxRetries: 3,
    failurePolicy: "达到上限后暂停并通知",
    maxInstances: 4,
    maxDailyRuns: 24,
    regionScope: "仅当前区域",
    notifyChannel: "邮件 + Webhook",
    enableImmediately: true,
    approvalRequired: true
  });
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [resultMessage, setResultMessage] = useState("");
  const [errorMessage, setErrorMessage] = useState("");

  function updateField<K extends keyof typeof form>(key: K, value: (typeof form)[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  async function handleCreateTask() {
    setIsSubmitting(true);
    setResultMessage("");
    setErrorMessage("");
    try {
      const result = await createAutomationTask(form);
      setResultMessage(`已添加自动化任务：${result.rule.name}，关联任务 ${result.job.id}`);
      void onCreated();
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "添加自动化任务失败");
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
              <Workflow size={24} />
            </div>
            <div>
              <h2>添加自动化任务</h2>
              <p>配置触发条件、执行动作和安全护栏，提交后任务会进入自动化调度器。</p>
            </div>
          </div>
          <button className="icon-button bordered" aria-label="关闭添加任务" onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className="form-section">
          <div className="form-section-title">
            <CalendarClock size={18} />
            <span>基础配置</span>
          </div>
          <div className="form-grid">
            <label>
              任务名称
              <input
                placeholder="例如：A1 容量自动创建"
                value={form.name}
                onChange={(event) => updateField("name", event.target.value)}
              />
            </label>
            <label>
              任务类型
              <select value={form.type} onChange={(event) => updateField("type", event.target.value)}>
                <option value="容量重试">容量重试</option>
                <option value="定时创建实例">定时创建实例</option>
                <option value="定时停止实例">定时停止实例</option>
                <option value="指标触发扩容">指标触发扩容</option>
                <option value="到期回收">到期回收</option>
              </select>
            </label>
            <label>
              目标范围
              <input
                value={form.targetPool}
                onChange={(event) => updateField("targetPool", event.target.value)}
                placeholder="例如：按标签 owner=ops、实例 OCID 或 Compartment"
              />
            </label>
            <label>
              执行动作
              <select value={form.action} onChange={(event) => updateField("action", event.target.value)}>
                <option value="创建 1 台实例">创建 1 台实例</option>
                <option value="停止实例">停止实例</option>
                <option value="重启实例">重启实例</option>
                <option value="仅发送通知">仅发送通知</option>
              </select>
            </label>
          </div>
        </div>

        <div className="form-section">
          <div className="form-section-title">
            <TimerReset size={18} />
            <span>触发与失败处理</span>
          </div>
          <div className="form-grid">
            <label>
              触发周期
              <select value={form.triggerInterval} onChange={(event) => updateField("triggerInterval", event.target.value)}>
                <option value="每 5 分钟">每 5 分钟</option>
                <option value="每 15 分钟">每 15 分钟</option>
                <option value="每 1 小时">每 1 小时</option>
                <option value="每天固定时间">每天固定时间</option>
              </select>
            </label>
            <label>
              冷却时间
              <input value={form.cooldown} onChange={(event) => updateField("cooldown", event.target.value)} />
            </label>
            <label>
              失败重试上限
              <input
                type="number"
                min="0"
                value={form.maxRetries}
                onChange={(event) => updateField("maxRetries", Number(event.target.value))}
              />
            </label>
            <label>
              失败策略
              <select value={form.failurePolicy} onChange={(event) => updateField("failurePolicy", event.target.value)}>
                <option value="达到上限后暂停并通知">达到上限后暂停并通知</option>
                <option value="仅通知管理员">仅通知管理员</option>
                <option value="标记为需要人工处理">标记为需要人工处理</option>
              </select>
            </label>
          </div>
        </div>

        <div className="form-section">
          <div className="form-section-title">
            <ShieldCheck size={18} />
            <span>安全护栏</span>
          </div>
          <div className="form-grid">
            <label>
              最大实例数
              <input
                type="number"
                min="1"
                value={form.maxInstances}
                onChange={(event) => updateField("maxInstances", Number(event.target.value))}
              />
            </label>
            <label>
              最大每日执行次数
              <input
                type="number"
                min="1"
                value={form.maxDailyRuns}
                onChange={(event) => updateField("maxDailyRuns", Number(event.target.value))}
              />
            </label>
            <label>
              区域白名单
              <select value={form.regionScope} onChange={(event) => updateField("regionScope", event.target.value)}>
                <option value="仅当前区域">仅当前区域</option>
                <option value="新加坡 / 首尔 / 东京">新加坡 / 首尔 / 东京</option>
                <option value="自定义区域列表">自定义区域列表</option>
              </select>
            </label>
            <label>
              通知渠道
              <select value={form.notifyChannel} onChange={(event) => updateField("notifyChannel", event.target.value)}>
                <option value="邮件 + Webhook">邮件 + Webhook</option>
                <option value="仅邮件">仅邮件</option>
                <option value="仅 Webhook">仅 Webhook</option>
              </select>
            </label>
          </div>
        </div>

        <div className="switch-panel">
          <div className="switch-row">
            <div>
              <strong>创建后立即启用</strong>
              <p>启用后，调度器会按照触发条件自动创建任务。</p>
            </div>
            <button
              className={`toggle-switch ${form.enableImmediately ? "on" : ""}`}
              aria-label="创建后立即启用"
              onClick={() => updateField("enableImmediately", !form.enableImmediately)}
            />
          </div>
          <div className="switch-row">
            <div>
              <strong>高风险动作需要审批</strong>
              <p>删除、释放 IP、批量缩容等动作会先进入审批队列。</p>
            </div>
            <button
              className={`toggle-switch ${form.approvalRequired ? "on" : ""}`}
              aria-label="高风险动作需要审批"
              onClick={() => updateField("approvalRequired", !form.approvalRequired)}
            />
          </div>
        </div>
        {resultMessage ? <div className="inline-success">{resultMessage}</div> : null}
        {errorMessage ? <div className="inline-error">{errorMessage}</div> : null}

        <div className="button-row">
          <button className="secondary-button" onClick={onClose}>
            取消
          </button>
          <button className="primary-button" disabled={isSubmitting} onClick={handleCreateTask}>
            {isSubmitting ? "添加中..." : "添加任务"}
          </button>
        </div>
      </div>
    </div>
  );
}
