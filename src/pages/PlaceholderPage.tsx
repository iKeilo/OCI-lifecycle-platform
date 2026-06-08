import { Construction } from "lucide-react";
import { PageHeader } from "../components/PageHeader";

type PlaceholderPageProps = {
  title: string;
  subtitle: string;
};

export function PlaceholderPage({ title, subtitle }: PlaceholderPageProps) {
  return (
    <div className="page-stack">
      <PageHeader eyebrow="预留模块" title={title} description={subtitle} />
      <section className="glass-panel empty-state">
        <Construction size={36} />
        <h2>框架占位已就绪</h2>
        <p>该路由已接入导航，后续可以直接补充业务实现和真实数据。</p>
      </section>
    </div>
  );
}
