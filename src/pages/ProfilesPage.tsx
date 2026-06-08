import { Eye, KeyRound, Plus, Power, PowerOff, ShieldCheck, Trash2, Wifi, X } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import { StatusPill } from "../components/StatusPill";
import {
  createProfile,
  deleteProfile,
  disableProfile,
  enableProfile,
  getOCIReadiness,
  listProfiles,
  testProfile
} from "../services/api";
import type { OCIReadOnlyValidationResult, OCIReadiness, Profile } from "../services/api";

export function ProfilesPage() {
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [readiness, setReadiness] = useState<OCIReadiness | null>(null);
  const [selectedProfileId, setSelectedProfileId] = useState("");
  const [validation, setValidation] = useState<OCIReadOnlyValidationResult | null>(null);
  const [testCompartmentId, setTestCompartmentId] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isTesting, setIsTesting] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const [actionError, setActionError] = useState("");
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  const selectedProfile = useMemo(
    () => profiles.find((profile) => profile.id === selectedProfileId) ?? profiles[0],
    [profiles, selectedProfileId]
  );

  async function load() {
    setIsLoading(true);
    setErrorMessage("");
    try {
      const [nextProfiles, nextReadiness] = await Promise.all([listProfiles(), getOCIReadiness()]);
      setProfiles(nextProfiles);
      setReadiness(nextReadiness);
      setSelectedProfileId((current) => current || nextProfiles[0]?.id || "");
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "加载账号与密钥失败");
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function handleTest(profile: Profile) {
    setIsTesting(true);
    setValidation(null);
    setActionError("");
    setActionMessage("");
    try {
      const result = await testProfile(profile.id, {
        region: profile.defaultRegion,
        compartmentId: testCompartmentId
      });
      setValidation(result);
      setActionMessage(`Profile ${profile.name} 只读验证通过`);
      await load();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "真实 OCI 只读验证失败");
      await load();
    } finally {
      setIsTesting(false);
    }
  }

  async function handleEnable(profile: Profile) {
    setActionError("");
    setActionMessage("");
    try {
      await enableProfile(profile.id);
      setActionMessage(`已启用 Profile ${profile.name}`);
      await load();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "启用 Profile 失败");
    }
  }

  async function handleDisable(profile: Profile) {
    setActionError("");
    setActionMessage("");
    try {
      await disableProfile(profile.id);
      setActionMessage(`已禁用 Profile ${profile.name}`);
      await load();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "禁用 Profile 失败");
    }
  }

  async function handleDelete(profile: Profile) {
    if (!window.confirm(`确认删除 Profile ${profile.name}？删除后后端不会再使用这组凭据。`)) return;
    setActionError("");
    setActionMessage("");
    try {
      await deleteProfile(profile.id);
      setActionMessage(`已删除 Profile ${profile.name}`);
      setSelectedProfileId("");
      setValidation(null);
      await load();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "删除 Profile 失败");
    }
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="身份与密钥"
        title="OCI Profile 管理"
        description="管理 OCI API signing key。私钥只进入后端，数据库模式下用 AES-GCM 加密存储，前端永远不会读取已保存私钥。"
        actions={
          <button className="primary-button" onClick={() => setIsCreateOpen(true)}>
            <Plus size={18} />
            添加 Profile
          </button>
        }
      />

      <section className="glass-panel section-card">
        <div className="section-title-row">
          <div>
            <h2>当前运行模式</h2>
            <p>{readiness?.message ?? "正在读取 OCI readiness"}</p>
          </div>
          <StatusPill status={readiness?.ready ? "Healthy" : "Limited"} />
        </div>
        {readiness?.missing?.length ? <div className="inline-error">缺少配置：{readiness.missing.join(", ")}</div> : null}
        <div className="modal-warning">
          <ShieldCheck size={18} />
          <span>环境变量凭据只作为兼容回退；真实控制面优先使用 Web 保存的 Profile。</span>
        </div>
      </section>

      {actionMessage ? <div className="inline-success">{actionMessage}</div> : null}
      {actionError ? <div className="inline-error">{actionError}</div> : null}

      <section className="split-layout">
        <div className="glass-panel section-card">
          <div className="section-title-row">
            <h2>Profile 列表</h2>
            <KeyRound size={22} />
          </div>
          <AsyncState
            isLoading={isLoading}
            error={errorMessage}
            empty={!isLoading && profiles.length === 0}
            emptyText="暂无 Profile。添加真实 OCI API signing key 后会显示在这里。"
          />
          {!isLoading && !errorMessage && profiles.length > 0 ? (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>名称</th>
                    <th>区域</th>
                    <th>指纹</th>
                    <th>状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {profiles.map((profile) => (
                    <tr className={selectedProfile?.id === profile.id ? "selected-row" : ""} key={profile.id}>
                      <td>
                        <div className="table-resource">
                          <KeyRound size={18} />
                          <strong>{profile.name}</strong>
                        </div>
                      </td>
                      <td>{profile.defaultRegion}</td>
                      <td>{profile.fingerprint}</td>
                      <td>
                        <StatusPill status={profile.status} />
                      </td>
                      <td>
                        <div className="table-actions">
                          <button className="icon-button bordered" aria-label="查看详情" onClick={() => setSelectedProfileId(profile.id)}>
                            <Eye size={16} />
                          </button>
                          <button className="icon-button bordered" aria-label="测试连接" disabled={isTesting} onClick={() => void handleTest(profile)}>
                            <Wifi size={16} />
                          </button>
                          {profile.status === "Disabled" ? (
                            <button className="icon-button bordered" aria-label="启用" onClick={() => void handleEnable(profile)}>
                              <Power size={16} />
                            </button>
                          ) : (
                            <button className="icon-button bordered" aria-label="禁用" onClick={() => void handleDisable(profile)}>
                              <PowerOff size={16} />
                            </button>
                          )}
                          <button className="icon-button bordered danger" aria-label="删除" onClick={() => void handleDelete(profile)}>
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
        </div>

        <div className="glass-panel section-card">
          <div className="section-title-row">
            <div>
              <h2>Profile 详情</h2>
              <p>{selectedProfile ? "非敏感字段和真实只读验证结果" : "选择一个 Profile 查看详情"}</p>
            </div>
            {selectedProfile ? <StatusPill status={selectedProfile.status} /> : null}
          </div>

          {selectedProfile ? (
            <>
              <div className="modal-summary-card">
                <div>
                  <span>Profile ID</span>
                  <strong>{selectedProfile.id}</strong>
                </div>
                <div>
                  <span>默认区域</span>
                  <strong>{selectedProfile.defaultRegion}</strong>
                </div>
                <div>
                  <span>最近检查</span>
                  <strong>{formatTime(selectedProfile.lastCheckedAt)}</strong>
                </div>
              </div>
              <dl className="resource-facts wide">
                <div>
                  <dt>Tenancy OCID</dt>
                  <dd>{selectedProfile.tenancyOcid}</dd>
                </div>
                <div>
                  <dt>User OCID</dt>
                  <dd>{selectedProfile.userOcid}</dd>
                </div>
                <div>
                  <dt>Fingerprint</dt>
                  <dd>{selectedProfile.fingerprint}</dd>
                </div>
              </dl>
              <div className="form-section">
                <div className="form-section-title">只读验证</div>
                <label>
                  Compartment OCID
                  <input
                    value={testCompartmentId}
                    onChange={(event) => setTestCompartmentId(event.target.value)}
                    placeholder="留空时使用 tenancy OCID"
                  />
                </label>
                <div className="button-row">
                  <button className="secondary-button" disabled={isTesting} onClick={() => void handleTest(selectedProfile)}>
                    <Wifi size={18} />
                    {isTesting ? "验证中..." : "测试连接"}
                  </button>
                </div>
              </div>
              {validation ? (
                <div className="modal-summary-card">
                  <div>
                    <span>验证结果</span>
                    <strong>{validation.verified ? "通过" : "失败"}</strong>
                  </div>
                  <div>
                    <span>区域订阅</span>
                    <strong>{validation.regions.length}</strong>
                  </div>
                  <div>
                    <span>实例返回</span>
                    <strong>{validation.instances.length}</strong>
                  </div>
                  <div>
                    <span>Region Request ID</span>
                    <strong>{validation.regionRequestId ?? "-"}</strong>
                  </div>
                  <div>
                    <span>Instances Request ID</span>
                    <strong>{validation.instancesRequestId ?? "-"}</strong>
                  </div>
                  <div>
                    <span>验证时间</span>
                    <strong>{formatTime(validation.validatedAt)}</strong>
                  </div>
                </div>
              ) : null}
            </>
          ) : (
            <AsyncState isLoading={false} error="" empty emptyText="暂无可查看的 Profile" />
          )}
        </div>
      </section>

      {isCreateOpen ? (
        <CreateProfileModal
          onClose={() => setIsCreateOpen(false)}
          onCreated={async () => {
            setIsCreateOpen(false);
            await load();
          }}
        />
      ) : null}
    </div>
  );
}

function CreateProfileModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => Promise<void> }) {
  const [form, setForm] = useState({
    configText: "",
    name: "",
    tenancyOcid: "",
    userOcid: "",
    fingerprint: "",
    defaultRegion: "ap-chuncheon-1",
    privateKeyFile: "",
    privateKey: ""
  });
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  const [parseMessage, setParseMessage] = useState("");
  const [pemMessage, setPemMessage] = useState("");

  function updateField<K extends keyof typeof form>(key: K, value: (typeof form)[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  function applyPastedConfig() {
    const parsed = parseOCIConfig(form.configText);
    setForm((current) => ({
      ...current,
      name: parsed.profileName || current.name,
      tenancyOcid: parsed.tenancyOcid || current.tenancyOcid,
      userOcid: parsed.userOcid || current.userOcid,
      fingerprint: parsed.fingerprint || current.fingerprint,
      defaultRegion: parsed.region || current.defaultRegion,
      privateKeyFile: parsed.keyFile || current.privateKeyFile
    }));
    const missing = missingParsedFields(parsed);
    setParseMessage(missing.length ? `已解析，仍缺少：${missing.join("、")}` : "已解析 OCI config，请继续添加 PEM 私钥。");
  }

  async function handlePEMFile(file?: File) {
    if (!file) return;
    setErrorMessage("");
    try {
      const text = await file.text();
      setForm((current) => ({
        ...current,
        privateKey: text,
        privateKeyFile: current.privateKeyFile || file.name
      }));
      setPemMessage(`已读取 PEM 文件：${file.name}`);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "读取 PEM 文件失败");
    }
  }

  async function handleCreate() {
    setIsSubmitting(true);
    setErrorMessage("");
    try {
      const parsed = parseOCIConfig(form.configText);
      const payload = {
        name: form.name || parsed.profileName,
        tenancyOcid: form.tenancyOcid || parsed.tenancyOcid,
        userOcid: form.userOcid || parsed.userOcid,
        fingerprint: form.fingerprint || parsed.fingerprint,
        defaultRegion: form.defaultRegion || parsed.region,
        privateKeyFile: form.privateKeyFile || parsed.keyFile,
        privateKey: form.privateKey
      };
      await createProfile(payload);
      await onCreated();
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "添加 Profile 失败");
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
              <KeyRound size={24} />
            </div>
            <div>
              <h2>添加 OCI Profile</h2>
              <p>直接粘贴 OCI config，再选择 PEM 文件或粘贴私钥内容。</p>
            </div>
          </div>
          <button className="icon-button bordered" aria-label="关闭添加 Profile" onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className="form-section">
          <div className="form-section-title">1. 粘贴 OCI config</div>
          <label>
            配置内容
            <textarea
              value={form.configText}
              onChange={(event) => {
                updateField("configText", event.target.value);
                setParseMessage("");
              }}
              placeholder={"[DEFAULT]\nuser=ocid1.user...\nfingerprint=xx:xx:xx\ntenancy=ocid1.tenancy...\nregion=ap-chuncheon-1\nkey_file=E:\\\\path\\\\to\\\\key.pem"}
            />
          </label>
          <div className="button-row">
            <button className="secondary-button" type="button" onClick={applyPastedConfig}>
              解析并填充
            </button>
          </div>
          {parseMessage ? <div className="inline-success">{parseMessage}</div> : null}
        </div>

        <div className="form-section">
          <div className="form-section-title">2. 确认解析结果</div>
          <div className="form-grid">
            <label>
              Profile 名称
              <input value={form.name} onChange={(event) => updateField("name", event.target.value)} placeholder="DEFAULT" />
            </label>
            <label>
              默认区域
              <input value={form.defaultRegion} onChange={(event) => updateField("defaultRegion", event.target.value)} placeholder="ap-chuncheon-1" />
            </label>
            <label>
              Tenancy OCID
              <input value={form.tenancyOcid} onChange={(event) => updateField("tenancyOcid", event.target.value)} placeholder="ocid1.tenancy..." />
            </label>
            <label>
              User OCID
              <input value={form.userOcid} onChange={(event) => updateField("userOcid", event.target.value)} placeholder="ocid1.user..." />
            </label>
            <label>
              Fingerprint
              <input value={form.fingerprint} onChange={(event) => updateField("fingerprint", event.target.value)} placeholder="xx:xx:xx" />
            </label>
            <label>
              私钥文件路径
              <input value={form.privateKeyFile} onChange={(event) => updateField("privateKeyFile", event.target.value)} placeholder="E:\\path\\to\\oci.pem" />
            </label>
          </div>
        </div>

        <div className="form-section">
          <div className="form-section-title">3. 添加 PEM 私钥</div>
          <label>
            PEM 文件
            <input
              type="file"
              accept=".pem,.key,.txt"
              onChange={(event) => {
                void handlePEMFile(event.target.files?.[0]);
                event.target.value = "";
              }}
            />
          </label>
          {pemMessage ? <div className="inline-success">{pemMessage}</div> : null}
          <label>
            PEM 私钥内容
            <textarea
              value={form.privateKey}
              onChange={(event) => updateField("privateKey", event.target.value)}
              placeholder="-----BEGIN PRIVATE KEY-----"
            />
          </label>
          <div className="modal-warning">
            <strong>密钥存储要求</strong>
            <span>提交 PEM 内容时，后端必须配置 DATABASE_URL 和 32 字节 PROFILE_KEY_ENCRYPTION_KEY。示例占位 key_file 不会被当作有效密钥。</span>
          </div>
        </div>

        {errorMessage ? <div className="inline-error">{errorMessage}</div> : null}

        <div className="button-row">
          <button className="secondary-button" onClick={onClose}>
            取消
          </button>
          <button className="primary-button" disabled={isSubmitting} onClick={handleCreate}>
            {isSubmitting ? "保存中..." : "保存 Profile"}
          </button>
        </div>
      </div>
    </div>
  );
}

type ParsedOCIConfig = {
  profileName: string;
  tenancyOcid: string;
  userOcid: string;
  fingerprint: string;
  region: string;
  keyFile: string;
};

function parseOCIConfig(value: string): ParsedOCIConfig {
  const parsed: ParsedOCIConfig = {
    profileName: "",
    tenancyOcid: "",
    userOcid: "",
    fingerprint: "",
    region: "",
    keyFile: ""
  };

  for (const rawLine of value.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#") || line.startsWith(";")) continue;

    const section = line.match(/^\[([^\]]+)]$/);
    if (section) {
      parsed.profileName = section[1].trim();
      continue;
    }

    const separatorIndex = line.indexOf("=");
    if (separatorIndex < 0) continue;

    const key = line.slice(0, separatorIndex).trim().toLowerCase();
    const raw = line.slice(separatorIndex + 1).replace(/\s+#.*$/, "").trim();
    switch (key) {
      case "user":
        parsed.userOcid = raw;
        break;
      case "fingerprint":
        parsed.fingerprint = raw;
        break;
      case "tenancy":
        parsed.tenancyOcid = raw;
        break;
      case "region":
        parsed.region = raw;
        break;
      case "key_file":
        parsed.keyFile = isPlaceholderKeyFile(raw) ? "" : raw;
        break;
    }
  }

  if (!parsed.profileName) {
    parsed.profileName = "DEFAULT";
  }
  return parsed;
}

function missingParsedFields(parsed: ParsedOCIConfig) {
  const missing: string[] = [];
  if (!parsed.userOcid) missing.push("user");
  if (!parsed.fingerprint) missing.push("fingerprint");
  if (!parsed.tenancyOcid) missing.push("tenancy");
  if (!parsed.region) missing.push("region");
  return missing;
}

function isPlaceholderKeyFile(value: string) {
  const normalized = value.toLowerCase();
  return !value || normalized.includes("todo") || normalized.includes("<path") || normalized.includes("private keyfile>");
}

function formatTime(value: string) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(date);
}
