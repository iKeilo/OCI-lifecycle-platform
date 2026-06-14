import { Globe2, Network, RefreshCw, Router, ShieldCheck } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { getSelectedOCIContext, onOCIContextChange } from "../app/ociContext";
import { PageHeader } from "../components/PageHeader";
import { StatusPill } from "../components/StatusPill";
import {
  createPublicIPBatchTask,
  getLaunchOptions,
  getNetworkInventory,
  type LaunchOption,
  type NetworkInventory,
  type PublicIPResource
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
  const [filters, setFilters] = useState(() => ({ ...getSelectedOCIContext(), compartmentId: "", vcnId: "" }));
  const [batchCount, setBatchCount] = useState(1);
  const [batchPrefix, setBatchPrefix] = useState("reserved-public-ip");
  const [selectedPublicIpIds, setSelectedPublicIpIds] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [isSubmittingBatch, setIsSubmittingBatch] = useState(false);
  const [error, setError] = useState("");
  const [taskMessage, setTaskMessage] = useState("");

  async function load(nextFilters = filters) {
    setLoading(true);
    setError("");
    try {
      const [options, nextInventory] = await Promise.all([
        getLaunchOptions(),
        getNetworkInventory(nextFilters)
      ]);
      setProfiles(options.profiles.map((profile) => ({ id: profile.id, label: profile.name, region: profile.defaultRegion })));
      setCompartments(options.compartments);
      setVCNs(options.vcns);
      setInventory(nextInventory);
      setSelectedPublicIpIds((current) => {
        const nextDeletableIds = new Set(nextInventory.publicIps.filter(canDeleteReservedIP).map((item) => item.id));
        return new Set([...current].filter((id) => nextDeletableIds.has(id)));
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "网络清单加载失败");
    } finally {
      setLoading(false);
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

  const reserved = useMemo(() => inventory.publicIps.filter((item) => item.lifetime === "RESERVED"), [inventory.publicIps]);
  const assignedReserved = reserved.filter((item) => item.assignedEntityId);
  const deletableReserved = reserved.filter(canDeleteReservedIP);
  const ipv6EnabledSubnets = inventory.subnets.filter((item) => item.ipv6CidrBlocks?.length > 0);

  function togglePublicIP(id: string) {
    setSelectedPublicIpIds((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function selectAllDeletablePublicIPs() {
    setSelectedPublicIpIds(new Set(deletableReserved.map((item) => item.id)));
  }

  async function submitPublicIPBatch(action: "create" | "delete") {
    const deletableIds = new Set(deletableReserved.map((item) => item.id));
    const publicIpIds = Array.from(selectedPublicIpIds).filter((id) => deletableIds.has(id));
    if (action === "delete" && publicIpIds.length === 0) {
      setError("请选择未绑定的预留公网 IP 后再批量删除。");
      return;
    }
    setIsSubmittingBatch(true);
    setTaskMessage("");
    setError("");
    try {
      const job = await createPublicIPBatchTask({
        action,
        profileId: filters.profileId,
        region: filters.region,
        compartmentId: filters.compartmentId,
        count: action === "create" ? batchCount : undefined,
        displayPrefix: action === "create" ? batchPrefix : undefined,
        publicIpIds: action === "delete" ? publicIpIds : undefined,
        note: action === "create" ? "批量申请预留公网 IP" : "批量删除未绑定预留公网 IP"
      });
      setTaskMessage(`已提交${action === "create" ? "批量申请" : "批量删除"}任务 ${job.id}，请到任务中心查看执行结果。`);
      if (action === "delete") setSelectedPublicIpIds(new Set());
    } catch (err) {
      setError(err instanceof Error ? err.message : "提交批量公网 IP 任务失败");
    } finally {
      setIsSubmittingBatch(false);
    }
  }

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
            <select
              value={filters.profileId}
              onChange={(event) => {
                const profile = profiles.find((item) => item.id === event.target.value);
                setFilters((current) => ({ ...current, profileId: event.target.value, region: profile?.region || current.region }));
              }}
            >
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
      {taskMessage ? <div className="inline-success">{taskMessage}</div> : null}
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
        <div className="public-ip-batch-panel">
          <div className="public-ip-batch-create">
            <label>
              申请数量
              <input type="number" min={1} max={50} value={batchCount} onChange={(event) => setBatchCount(clampBatchCount(Number(event.target.value)))} />
            </label>
            <label>
              名称前缀
              <input value={batchPrefix} onChange={(event) => setBatchPrefix(event.target.value)} placeholder="reserved-public-ip" />
            </label>
            <button className="primary-button" type="button" disabled={isSubmittingBatch || batchCount <= 0} onClick={() => void submitPublicIPBatch("create")}>
              批量申请
            </button>
          </div>
          <div className="public-ip-batch-actions">
            <span>已选择 {selectedPublicIpIds.size} 个，未绑定可删除 {deletableReserved.length} 个</span>
            <button className="secondary-button" type="button" disabled={deletableReserved.length === 0} onClick={selectAllDeletablePublicIPs}>
              选择未绑定
            </button>
            <button className="secondary-button danger" type="button" disabled={isSubmittingBatch || selectedPublicIpIds.size === 0} onClick={() => void submitPublicIPBatch("delete")}>
              批量删除
            </button>
          </div>
        </div>
        <ReservedPublicIPTable
          rows={reserved}
          selectedIds={selectedPublicIpIds}
          onToggle={togglePublicIP}
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

function ReservedPublicIPTable({
  rows,
  selectedIds,
  onToggle
}: {
  rows: PublicIPResource[];
  selectedIds: Set<string>;
  onToggle: (id: string) => void;
}) {
  if (rows.length === 0) {
    return <div className="async-state">暂无预留公网 IP。可以使用上方“批量申请”创建 Reserved Public IP。</div>;
  }
  return (
    <div className="table-wrap network-table-wrap">
      <table className="network-table reserved-ip-table">
        <thead>
          <tr>
            <th>选择</th>
            <th>名称</th>
            <th>IP</th>
            <th>状态</th>
            <th>绑定资源</th>
            <th>OCID</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((item) => {
            const canDelete = canDeleteReservedIP(item);
            return (
              <tr className={!canDelete ? "disabled-row" : ""} key={item.id}>
                <td>
                  <input
                    aria-label={`选择 ${item.displayName || item.ipAddress || item.id}`}
                    checked={selectedIds.has(item.id)}
                    disabled={!canDelete}
                    type="checkbox"
                    onChange={() => onToggle(item.id)}
                  />
                </td>
                <td>{item.displayName || "-"}</td>
                <td>{item.ipAddress || "-"}</td>
                <td>{item.lifecycleState || "-"}</td>
                <td>{item.assignedEntityId || "未绑定"}</td>
                <td className="linkish ocid-cell" title={item.id}>{item.id}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function canDeleteReservedIP(item: PublicIPResource) {
  return !item.assignedEntityId && item.lifetime === "RESERVED" && String(item.lifecycleState || "").toUpperCase() !== "ASSIGNED";
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

function clampBatchCount(value: number) {
  if (!Number.isFinite(value)) return 1;
  return Math.min(50, Math.max(1, Math.floor(value)));
}
