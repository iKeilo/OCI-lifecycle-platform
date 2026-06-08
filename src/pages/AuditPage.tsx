import { ScrollText } from "lucide-react";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";

export function AuditPage() {
  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="合规"
        title="审计日志"
        description="审计写入已经在后端落库；列表 API 接入前，本页不会展示手工编造的审计记录。"
      />

      <section className="glass-panel section-card">
        <div className="section-title-row">
          <div>
            <h2>系统审计</h2>
            <p>下一步会接入 `GET /api/audit-logs`，展示真实操作者、资源、OCI Request ID 和执行结果。</p>
          </div>
          <ScrollText size={22} />
        </div>
        <AsyncState isLoading={false} error="" empty emptyText="暂无可读取的真实审计日志。" />
      </section>
    </div>
  );
}
