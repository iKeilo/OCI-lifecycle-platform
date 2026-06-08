import { Shield, UserRoundCheck, Users } from "lucide-react";
import { PageHeader } from "../components/PageHeader";

const roles = [
  ["超级管理员", "拥有所有权限，可管理密钥配置、用户权限和系统设置"],
  ["运维管理员", "可管理实例、模板、资源池、自动化和任务"],
  ["普通操作员", "可执行 SSH、启动、停止、重启，并查看任务状态"],
  ["审计员", "只能只读访问审计日志和任务记录"]
];

export function UsersPage() {
  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="访问控制"
        title="用户与权限"
        description="通过内部 RBAC 区分密钥管理、生命周期操作、审批和审计访问权限。"
      />
      <div className="card-grid two">
        {roles.map(([role, description]) => (
          <section className="glass-panel section-card" key={role}>
            <div className="section-title-row">
              <h2>{role}</h2>
              {role === "超级管理员" ? <Shield size={24} /> : role === "普通操作员" ? <UserRoundCheck size={24} /> : <Users size={24} />}
            </div>
            <p>{description}</p>
          </section>
        ))}
      </div>
    </div>
  );
}
