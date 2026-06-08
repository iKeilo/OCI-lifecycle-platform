import { Boxes, Plus } from "lucide-react";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";

export function ResourcePoolsPage() {
  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="集群运营"
        title="资源池"
        description="资源池需要对接 Instance Configuration、Instance Pool 或内部资源分组后才会展示真实数据。当前不会显示演示池。"
        actions={
          <button className="primary-button" disabled title="资源池 API 尚未接入">
            <Plus size={18} />
            新建资源池
          </button>
        }
      />

      <section className="glass-panel section-card">
        <div className="section-title-row">
          <div>
            <h2>资源池库存</h2>
            <p>等待后端资源池 repository 与 OCI Instance Pool API 接入。</p>
          </div>
          <Boxes size={24} />
        </div>
        <AsyncState
          isLoading={false}
          error=""
          empty
          emptyText="暂无真实资源池。这里不再展示 a1-free-pool、edge-api-pool 等演示数据。"
        />
      </section>
    </div>
  );
}
