import { Globe2, Network, RefreshCw, Router, ShieldCheck } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { PageHeader } from "../components/PageHeader";
import { StatusPill } from "../components/StatusPill";
import {
  getLaunchOptions,
  getNetworkInventory,
  type LaunchOption,
  type NetworkInventory
} from "../services/api";

const emptyInventory: NetworkInventory = {
  verified: false,
  executionMode: "local",
  publicIps: [],
  privateIps: [],
  ipv6s: [],
  vcns: [],
  subnets: []
};

export function NetworkPage() {
  const [inventory, setInventory] = useState<NetworkInventory>(emptyInventory);
  const [profiles, setProfiles] = useState<LaunchOption[]>([]);
  const [compartments, setCompartments] = useState<LaunchOption[]>([]);
  const [vcns, setVCNs] = useState<LaunchOption[]>([]);
  const [filters, setFilters] = useState({ profileId: "", compartmentId: "", vcnId: "" });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  async function load() {
    setLoading(true);
    setError("");
    try {
      const [options, nextInventory] = await Promise.all([
        getLaunchOptions(),
        getNetworkInventory(filters)
      ]);
      setProfiles(options.profiles.map((profile) => ({ id: profile.id, label: profile.name, region: profile.defaultRegion })));
      setCompartments(options.compartments);
      setVCNs(options.vcns);
      setInventory(nextInventory);
    } catch (err) {
      setError(err instanceof Error ? err.message : "网络清单加载失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  const reserved = useMemo(() => inventory.publicIps.filter((item) => item.lifetime === "RESERVED"), [inventory.publicIps]);
  const assignedReserved = reserved.filter((item) => item.assignedEntityId);
  const ipv6EnabledSubnets = inventory.subnets.filter((item) => item.ipv6CidrBlocks?.length > 0);

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="OCI 网络"
        title="网络管理"
        description="集中查看预留公网 IP、私网 IP、IPv6、VCN 与 Subnet。所有数据来自 OCI SDK，未配置密钥时不会显示假数据。"
        actions={
          <button className="primary-button" type="button" onClick={() => void load()} disabled={loading}>
            <RefreshCw size={18} className={loading ? "spin" : ""} />
            刷新网络清单
          </button>
        }
      />

      <section className="glass-panel toolbar network-toolbar">
        <div className="toolbar-actions network-filters">
          <label>
            Profile
            <select value={filters.profileId} onChange={(event) => setFilters((current) => ({ ...current, profileId: event.target.value }))}>
              <option value="">默认 Profile</option>
              {profiles.map((profile) => (
                <option value={profile.id} key={profile.id}>{profile.label}</option>
              ))}
            </select>
          </label>
          <label>
            Compartment
            <select value={filters.compartmentId} onChange={(event) => setFilters((current) => ({ ...current, compartmentId: event.target.value }))}>
              <option value="">默认 Compartment</option>
              {compartments.map((item) => (
                <option value={item.id} key={item.id}>{item.label}</option>
              ))}
            </select>
          </label>
          <label>
            VCN
            <select value={filters.vcnId} onChange={(event) => setFilters((current) => ({ ...current, vcnId: event.target.value }))}>
              <option value="">全部 VCN</option>
              {vcns.map((item) => (
                <option value={item.id} key={item.id}>{item.label}</option>
              ))}
            </select>
          </label>
          <button className="secondary-button" type="button" onClick={() => void load()} disabled={loading}>
            应用筛选
          </button>
        </div>
      </section>

      {error ? <div className="inline-error">{error}</div> : null}
      {!loading && inventory.errorMessage ? (
        <div className="inline-error">
          {inventory.errorCode}: {inventory.errorMessage}
        </div>
      ) : null}

      <div className="network-metric-grid">
        <NetworkMetric icon={<Globe2 size={22} />} title="预留公网 IP" value={reserved.length} hint={`${assignedReserved.length} 个已绑定`} />
        <NetworkMetric icon={<Network size={22} />} title="VCN / Subnet" value={`${inventory.vcns.length}/${inventory.subnets.length}`} hint={`${ipv6EnabledSubnets.length} 个 Subnet 已启用 IPv6`} />
        <NetworkMetric icon={<Router size={22} />} title="私网 IP / IPv6" value={`${inventory.privateIps.length}/${inventory.ipv6s.length}`} hint={inventory.verified ? "OCI 清单已验证" : "等待 OCI 验证"} />
      </div>

      {loading ? <div className="async-state">正在从 OCI 读取网络资源...</div> : null}

      <section className="glass-panel section-card">
        <div className="section-title-row">
          <div>
            <h2>预留公网 IP</h2>
            <p>Reserved Public IP 可用于创建实例前预留，也可后续绑定到 Private IP。</p>
          </div>
          <ShieldCheck size={22} />
        </div>
        <NetworkTable
          compactLastColumn
          columns={["名称", "IP", "生命周期", "状态", "绑定资源", "OCID"]}
          rows={reserved.map((item) => [
            item.displayName,
            item.ipAddress || "-",
            item.lifetime || "-",
            item.lifecycleState || "-",
            item.assignedEntityId || "未绑定",
            item.id
          ])}
        />
      </section>

      <section className="glass-panel section-card">
        <div className="section-title-row">
          <div>
            <h2>VCN 与 Subnet</h2>
            <p>用于判断 IPv6 CIDR、Public Subnet 和后续实例创建网络选择。</p>
          </div>
          <Network size={22} />
        </div>
        <NetworkTable
          compactLastColumn
          columns={["类型", "名称", "CIDR", "IPv6 CIDR", "公网", "OCID"]}
          rows={[
            ...inventory.vcns.map((item) => ["VCN", item.displayName, item.cidrBlock || "-", item.ipv6CidrBlocks?.join(", ") || "-", "-", item.id]),
            ...inventory.subnets.map((item) => ["Subnet", item.displayName, item.cidrBlock || "-", item.ipv6CidrBlocks?.join(", ") || "-", item.public ? "允许" : "禁止", item.id])
          ]}
        />
      </section>

      <section className="glass-panel section-card">
        <div className="section-title-row">
          <div>
            <h2>Private IP 与 IPv6</h2>
            <p>实例级 IP 绑定仍在实例管理中操作，这里用于全局审视地址占用。</p>
          </div>
          <Globe2 size={22} />
        </div>
        <NetworkTable
          compactLastColumn
          columns={["类型", "地址", "状态", "VNIC", "Subnet", "OCID"]}
          rows={[
            ...inventory.privateIps.map((item) => ["Private IPv4", item.ipAddress, item.lifecycleState || "-", item.vnicId || "-", item.subnetId || "-", item.id]),
            ...inventory.ipv6s.map((item) => ["IPv6", item.ipAddress, item.lifecycleState || "-", item.vnicId || "-", item.subnetId || "-", item.id])
          ]}
        />
      </section>
    </div>
  );
}

function NetworkMetric({ icon, title, value, hint }: { icon: ReactNode; title: string; value: string | number; hint: string }) {
  return (
    <section className="glass-panel metric-card">
      <div className="metric-topline">
        <div className="metric-icon">{icon}</div>
        <StatusPill status="Active" />
      </div>
      <div className="metric-title">{title}</div>
      <div className="metric-value">{value}</div>
      <p className="muted-line">{hint}</p>
    </section>
  );
}

function NetworkTable({ columns, rows, compactLastColumn = false }: { columns: string[]; rows: string[][]; compactLastColumn?: boolean }) {
  if (rows.length === 0) {
    return <div className="async-state">暂无资源。若 OCI 已配置，请检查 Compartment、区域和 IAM 权限。</div>;
  }
  return (
    <div className="table-wrap network-table-wrap">
      <table className="network-table">
        <thead>
          <tr>
            {columns.map((column) => <th key={column}>{column}</th>)}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr key={`${row[0]}-${row[row.length - 1]}-${index}`}>
              {row.map((cell, cellIndex) => {
                const isLast = cellIndex === row.length - 1;
                return (
                  <td
                    key={`${cell}-${cellIndex}`}
                    className={`${isLast ? "linkish" : ""} ${compactLastColumn && isLast ? "ocid-cell" : ""}`}
                    title={isLast ? cell : undefined}
                  >
                    {cell}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
