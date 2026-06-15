import { CheckCircle2, ClipboardList, Copy, Edit3, Plus, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { getSelectedOCIContext, onOCIContextChange } from "../app/ociContext";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import { StatusPill } from "../components/StatusPill";
import {
  createTemplate,
  deleteTemplate,
  listTemplates,
  validateTemplate,
  type InstanceTemplate,
  type TemplatePayload
} from "../services/api";

export function TemplatesPage() {
  const [templates, setTemplates] = useState<InstanceTemplate[]>([]);
  const [filters, setFilters] = useState(() => ({ ...getSelectedOCIContext(), q: "", status: "" }));
  const [isLoading, setIsLoading] = useState(true);
  const [errorMessage, setErrorMessage] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const [actionError, setActionError] = useState("");

  const stats = useMemo(() => {
    const active = templates.filter((item) => item.status === "ACTIVE").length;
    const invalid = templates.filter((item) => item.validationStatus === "INVALID").length;
    const ipv6 = templates.filter((item) => item.enableIpv6).length;
    return { total: templates.length, active, invalid, ipv6 };
  }, [templates]);
  const profileFilterOptions = useMemo(
    () => uniqueFilterOptions([filters.profileId, ...templates.map((item) => item.profileId)]),
    [filters.profileId, templates]
  );
  const regionFilterOptions = useMemo(
    () => uniqueFilterOptions([filters.region, ...templates.map((item) => item.region)]),
    [filters.region, templates]
  );

  async function load(nextFilters = filters) {
    setIsLoading(true);
    setErrorMessage("");
    try {
      const items = await listTemplates({
        profileId: nextFilters.profileId,
        region: nextFilters.region,
        status: nextFilters.status,
        q: nextFilters.q
      });
      setTemplates(items);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "加载模板失败");
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    void load();
    return onOCIContextChange((context) => {
      setFilters((current) => {
        const next = { ...current, profileId: context.profileId, region: context.region };
        void load(next);
        return next;
      });
    });
  }, []);

  async function handleDelete(template: InstanceTemplate) {
    if (!window.confirm(`确认删除模板 ${template.name}？历史任务不会被删除。`)) return;
    setActionError("");
    setActionMessage("");
    try {
      await deleteTemplate(template.id);
      setActionMessage(`已删除模板 ${template.name}`);
      await load();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "删除模板失败");
    }
  }

  async function handleDuplicate(template: InstanceTemplate) {
    setActionError("");
    setActionMessage("");
    try {
      const created = await createTemplate({
        ...templateToPayload(template),
        name: `${template.name} 副本`,
        status: "ACTIVE"
      });
      setActionMessage(`已复制模板 ${created.name}`);
      await load();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "复制模板失败");
    }
  }

  async function handleCheck(template: InstanceTemplate) {
    setActionError("");
    setActionMessage("");
    try {
      const result = await validateTemplate(template.id);
      setActionMessage(result.verified ? `模板 ${template.name} 字段完整` : result.errorMessage || "模板字段不完整");
      await load();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "检查模板字段失败");
    }
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="预输入"
        title="模板管理"
        description="新建模板会在模板管理内调用真实 Launch Options，使用上下文、镜像规格、网络访问和公网/IPv6 选项保存预输入模板，不跳转创建实例。"
        actions={
          <Link className="primary-button" to="/templates/new">
            <Plus size={18} />
            新建模板
          </Link>
        }
      />

      <div className="network-metric-grid">
        <TemplateMetric title="模板总数" value={stats.total} hint="用户创建的预输入模板" />
        <TemplateMetric title="启用模板" value={stats.active} hint="可在创建实例页选择" />
        <TemplateMetric title="字段缺失" value={stats.invalid} hint="本地字段检查未通过" />
        <TemplateMetric title="IPv6 模板" value={stats.ipv6} hint="默认开启 IPv6 预输入" />
      </div>

      <section className="glass-panel toolbar network-toolbar">
        <div className="toolbar-actions network-filters">
          <label>
            搜索
            <input
              value={filters.q}
              onChange={(event) => setFilters((current) => ({ ...current, q: event.target.value }))}
              placeholder="名称 / Shape / 镜像 / 区域"
            />
          </label>
          <label>
            Profile
            <select value={filters.profileId} onChange={(event) => setFilters((current) => ({ ...current, profileId: event.target.value }))}>
              <option value="">全部 Profile</option>
              {profileFilterOptions.map((value) => (
                <option value={value} key={value}>
                  {value}
                </option>
              ))}
            </select>
          </label>
          <label>
            Region
            <select value={filters.region} onChange={(event) => setFilters((current) => ({ ...current, region: event.target.value }))}>
              <option value="">全部 Region</option>
              {regionFilterOptions.map((value) => (
                <option value={value} key={value}>
                  {value}
                </option>
              ))}
            </select>
          </label>
          <label>
            状态
            <select value={filters.status} onChange={(event) => setFilters((current) => ({ ...current, status: event.target.value }))}>
              <option value="">全部</option>
              <option value="ACTIVE">ACTIVE</option>
              <option value="DISABLED">DISABLED</option>
              <option value="ARCHIVED">ARCHIVED</option>
              <option value="DRAFT">DRAFT</option>
            </select>
          </label>
          <button className="secondary-button" onClick={() => void load()} disabled={isLoading}>
            应用筛选
          </button>
        </div>
      </section>

      {actionMessage ? <div className="inline-success">{actionMessage}</div> : null}
      {actionError ? <div className="inline-error">{actionError}</div> : null}

      <section className="glass-panel section-card">
        <div className="section-title-row">
          <div>
            <h2>模板列表</h2>
            <p>没有内置演示模板。列表中的模板均来自用户保存的预输入配置。</p>
          </div>
          <ClipboardList size={22} />
        </div>
        <AsyncState isLoading={isLoading} error={errorMessage} empty={!isLoading && !errorMessage && templates.length === 0} emptyText="暂无模板。可以新建模板，或稍后在创建实例页保存当前配置为模板。" />
        {!isLoading && !errorMessage && templates.length > 0 ? (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>名称</th>
                  <th>Profile / Region</th>
                  <th>Shape</th>
                  <th>启动盘</th>
                  <th>网络</th>
                  <th>状态</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {templates.map((template) => (
                  <tr key={template.id}>
                    <td>
                      <div className="table-resource">
                        <ClipboardList size={18} />
                        <div>
                          <strong>{template.name}</strong>
                          <p className="muted-line">{template.description || template.version}</p>
                          {templateInstanceName(template) ? <p className="muted-line">实例：{templateInstanceName(template)}</p> : null}
                        </div>
                      </div>
                    </td>
                    <td>
                      <div>{template.profileId || "-"}</div>
                      <span className="muted-line">{template.region || "-"}</span>
                    </td>
                    <td>
                      <div>{template.shape}</div>
                      <span className="muted-line">{template.ocpus} OCPU / {template.memoryGb} GB</span>
                    </td>
                    <td>
                      <div>{template.bootVolumeGb} GB</div>
                      <span className="muted-line">{template.bootVolumeVpusPerGb || 10} VPUs/GB</span>
                    </td>
                    <td>
                      <div>{template.assignPublicIp ? "公网 IPv4" : "不分配公网 IPv4"}</div>
                      <span className="muted-line">{template.enableIpv6 ? "启用 IPv6" : "IPv6 关闭"}</span>
                    </td>
                    <td>
                      <div className="status-stack">
                        <StatusPill status={template.status} />
                        <span className="muted-line">{template.validationStatus || "UNVERIFIED"}</span>
                      </div>
                    </td>
                    <td>
                      <div className="table-actions">
                        <Link className="secondary-button compact" to={`/create?templateId=${encodeURIComponent(template.id)}`}>
                          使用
                        </Link>
                        <button className="icon-button bordered" aria-label="检查字段" onClick={() => void handleCheck(template)}>
                          <CheckCircle2 size={16} />
                        </button>
                        <Link
                          className="icon-button bordered"
                          aria-label="编辑模板"
                          to={`/templates/${encodeURIComponent(template.id)}/edit`}
                        >
                          <Edit3 size={16} />
                        </Link>
                        <button className="icon-button bordered" aria-label="复制模板" onClick={() => void handleDuplicate(template)}>
                          <Copy size={16} />
                        </button>
                        <button className="icon-button bordered danger" aria-label="删除模板" onClick={() => void handleDelete(template)}>
                          <Trash2 size={16} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>

    </div>
  );
}

function TemplateMetric({ title, value, hint }: { title: string; value: number; hint: string }) {
  return (
    <section className="glass-panel metric-card">
      <div className="metric-topline">
        <div className="metric-icon"><ClipboardList size={20} /></div>
      </div>
      <div className="metric-title">{title}</div>
      <div className="metric-value">{value}</div>
      <p className="muted-line">{hint}</p>
    </section>
  );
}

function uniqueFilterOptions(values: string[]) {
  return Array.from(new Set(values.map((value) => value.trim()).filter(Boolean))).sort((a, b) => a.localeCompare(b));
}

function templateInstanceName(template: InstanceTemplate) {
  const configText = template.configText?.trim();
  if (!configText || template.configFormat !== "json") return "";
  try {
    const parsed = JSON.parse(configText) as { instance?: { name?: unknown }; instanceName?: unknown };
    const value = parsed.instance?.name ?? parsed.instanceName;
    return typeof value === "string" ? value.trim() : "";
  } catch {
    return "";
  }
}

function templateToPayload(template: InstanceTemplate): TemplatePayload {
  return {
    name: template.name,
    description: template.description || "",
    version: template.version || "v1",
    profileId: template.profileId || "",
    region: template.region || "",
    compartment: template.compartment || "",
    compartmentId: template.compartmentId || "",
    availabilityAd: template.availabilityAd || "",
    imageId: template.imageId || "",
    imageName: template.imageName || "",
    shape: template.shape || "VM.Standard.E2.1.Micro",
    ocpus: template.ocpus || 1,
    memoryGb: template.memoryGb || 1,
    bootVolumeGb: template.bootVolumeGb || 50,
    bootVolumeVpusPerGb: template.bootVolumeVpusPerGb || 10,
    vcnId: template.vcnId || "",
    subnetId: template.subnetId || "",
    assignPublicIp: Boolean(template.assignPublicIp),
    enableIpv6: Boolean(template.enableIpv6),
    reservedPublicIp: template.reservedPublicIp || "",
    sshKey: template.sshKey || "",
    cloudInit: template.cloudInit || "",
    tags: template.tags || {},
    configFormat: template.configFormat || "json",
    configText: template.configText || "",
    status: template.status || "ACTIVE"
  };
}
