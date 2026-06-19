import { Save, ShieldCheck, ShieldAlert } from "lucide-react";
import { useEffect, useState } from "react";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import { getSecurityGuardrails, updateSecurityGuardrails, type SecurityGuardrails } from "../services/api";

const defaults: SecurityGuardrails = {
  enabled: true,
  allowedRegions: [],
  deniedRegions: [],
  maxOcpusPerInstance: 4,
  maxMemoryGbPerInstance: 24,
  maxBootVolumeGb: 200,
  maxRetryAttempts: 20,
  maxPublicIpBatchCount: 10,
  requireApprovalForTerminate: true,
  blockBootVolumeDeletion: true,
  blockPublicIpv6RouteChanges: false,
  blockRootPasswordWithoutEmail: true,
  requireTemplateForLaunch: false
};

export function GuardrailsPage() {
  const [settings, setSettings] = useState<SecurityGuardrails>(defaults);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const next = await getSecurityGuardrails();
        if (!cancelled) setSettings({ ...defaults, ...next });
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "加载安全护栏失败");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  function patch(next: Partial<SecurityGuardrails>) {
    setSettings((current) => ({ ...current, ...next }));
  }

  async function save() {
    setSaving(true);
    setMessage("");
    setError("");
    try {
      const next = await updateSecurityGuardrails(settings);
      setSettings({ ...defaults, ...next });
      setMessage("安全护栏已保存");
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存安全护栏失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="平台治理"
        title="安全护栏"
        description="在 API 层限制区域、容量、重试、删机删盘、批量公网 IP 和 IPv6 网络改造，防止绕过 UI 的高风险操作。"
        actions={
          <button className="primary-button" type="button" disabled={saving} onClick={() => void save()}>
            <Save size={16} />
            {saving ? "保存中..." : "保存护栏"}
          </button>
        }
      />

      {message ? <div className="inline-success">{message}</div> : null}
      {error ? <div className="inline-error">{error}</div> : null}
      <AsyncState isLoading={loading} error="" />

      {!loading ? (
        <>
          <section className="glass-panel section-card">
            <div className="section-title-row">
              <div>
                <h2>全局开关与区域边界</h2>
                <p>允许区域为空表示不限制；禁止区域优先级更高。</p>
              </div>
              <ShieldCheck size={22} />
            </div>
            <div className="form-grid">
              <label className="switch-row">
                <input type="checkbox" checked={settings.enabled} onChange={(event) => patch({ enabled: event.target.checked })} />
                <span>启用安全护栏</span>
              </label>
              <label>
                允许区域
                <input value={settings.allowedRegions.join(", ")} onChange={(event) => patch({ allowedRegions: splitList(event.target.value) })} placeholder="ap-chuncheon-1, us-ashburn-1" />
              </label>
              <label>
                禁止区域
                <input value={settings.deniedRegions.join(", ")} onChange={(event) => patch({ deniedRegions: splitList(event.target.value) })} placeholder="例如 eu-frankfurt-1" />
              </label>
            </div>
          </section>

          <section className="glass-panel section-card">
            <div className="section-title-row">
              <div>
                <h2>容量与重试上限</h2>
                <p>这些限制会在创建实例、升降级、自动化任务和批量公网 IP 任务创建前执行。</p>
              </div>
              <ShieldAlert size={22} />
            </div>
            <div className="form-grid">
              <label>
                单实例最大 OCPU
                <input type="number" min={1} value={settings.maxOcpusPerInstance} onChange={(event) => patch({ maxOcpusPerInstance: Number(event.target.value) })} />
              </label>
              <label>
                单实例最大内存 GB
                <input type="number" min={1} value={settings.maxMemoryGbPerInstance} onChange={(event) => patch({ maxMemoryGbPerInstance: Number(event.target.value) })} />
              </label>
              <label>
                最大启动盘 GB
                <input type="number" min={50} value={settings.maxBootVolumeGb} onChange={(event) => patch({ maxBootVolumeGb: Number(event.target.value) })} />
              </label>
              <label>
                最大重试次数
                <input type="number" min={0} value={settings.maxRetryAttempts} onChange={(event) => patch({ maxRetryAttempts: Number(event.target.value) })} />
              </label>
              <label>
                批量公网 IP 最大数量
                <input type="number" min={1} value={settings.maxPublicIpBatchCount} onChange={(event) => patch({ maxPublicIpBatchCount: Number(event.target.value) })} />
              </label>
            </div>
          </section>

          <section className="glass-panel section-card">
            <div className="section-title-row">
              <div>
                <h2>危险操作策略</h2>
                <p>这些开关会阻止删机删盘、root 密码无通知、IPv6 自动改路由等高风险行为。</p>
              </div>
              <ShieldAlert size={22} />
            </div>
            <div className="choice-grid">
              <ToggleCard title="终止实例必须确认" checked={settings.requireApprovalForTerminate} onChange={(value) => patch({ requireApprovalForTerminate: value })} description="终止实例需要勾选快照/确认参数，减少误删。" />
              <ToggleCard title="禁止删除启动盘" checked={settings.blockBootVolumeDeletion} onChange={(value) => patch({ blockBootVolumeDeletion: value })} description="终止实例时必须保留启动盘。" />
              <ToggleCard title="禁止 IPv6 改公网路径" checked={settings.blockPublicIpv6RouteChanges} onChange={(value) => patch({ blockPublicIpv6RouteChanges: value })} description="阻止自动修改 VCN/Subnet/Route/Security List/NSG。" />
              <ToggleCard title="root 密码必须通知" checked={settings.blockRootPasswordWithoutEmail} onChange={(value) => patch({ blockRootPasswordWithoutEmail: value })} description="生成 root 密码时必须启用通知，避免凭据丢失。" />
              <ToggleCard title="创建实例必须使用模板" checked={settings.requireTemplateForLaunch} onChange={(value) => patch({ requireTemplateForLaunch: value })} description="强制使用已审核模板作为创建实例输入。" />
            </div>
          </section>
        </>
      ) : null}
    </div>
  );
}

function ToggleCard({ title, description, checked, onChange }: { title: string; description: string; checked: boolean; onChange: (value: boolean) => void }) {
  return (
    <button className={`choice-card ${checked ? "active" : ""}`} type="button" onClick={() => onChange(!checked)}>
      <ShieldCheck size={22} />
      <strong>{title}</strong>
      <span>{description}</span>
    </button>
  );
}

function splitList(value: string) {
  return value.split(",").map((item) => item.trim()).filter(Boolean);
}
