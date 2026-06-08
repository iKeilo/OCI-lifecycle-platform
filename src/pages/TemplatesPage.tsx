import { Copy, Layers, Plus, RefreshCw } from "lucide-react";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import { StatusPill } from "../components/StatusPill";
import { listTemplates } from "../services/api";
import type { InstanceTemplate } from "../services/api";

export function TemplatesPage() {
  const [templates, setTemplates] = useState<InstanceTemplate[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [errorMessage, setErrorMessage] = useState("");

  async function loadTemplates() {
    setIsLoading(true);
    setErrorMessage("");
    try {
      setTemplates(await listTemplates());
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "加载模板失败");
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    void loadTemplates();
  }, []);

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="可复用创建定义"
        title="模板管理"
        description="模板由后端资源目录提供，用于手动创建、自动化创建和资源池扩容。前端不再维护演示模板。"
        actions={
          <>
            <button className="secondary-button" onClick={() => void loadTemplates()}>
              <RefreshCw size={18} />
              刷新
            </button>
            <button className="primary-button">
              <Plus size={18} />
              新建模板
            </button>
          </>
        }
      />

      <AsyncState isLoading={isLoading} error={errorMessage} empty={!isLoading && templates.length === 0} emptyText="暂无可用模板" />

      {!isLoading && !errorMessage ? (
        <div className="card-grid two">
          {templates.map((template) => (
            <section className="glass-panel section-card" key={template.id}>
              <div className="section-title-row">
                <div>
                  <h2>{template.name}</h2>
                  <p>{template.version} · {template.region} · {template.compartment}</p>
                </div>
                <Layers size={24} />
              </div>
              <dl className="resource-facts">
                <div><dt>镜像</dt><dd>{template.imageName}</dd></div>
                <div><dt>Shape</dt><dd>{template.shape}</dd></div>
                <div><dt>资源</dt><dd>{template.ocpus} OCPU / {template.memoryGb} GB / {template.bootVolumeGb} GB</dd></div>
                <div><dt>网络</dt><dd>{template.vcnId} / {template.subnetId}</dd></div>
                <div><dt>状态</dt><dd><StatusPill status={template.status} /></dd></div>
              </dl>
              <div className="button-row">
                <button className="secondary-button">
                  <Copy size={16} />
                  复制
                </button>
                <Link className="primary-button" to="/create">创建实例</Link>
              </div>
            </section>
          ))}
        </div>
      ) : null}
    </div>
  );
}
