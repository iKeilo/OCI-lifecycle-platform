import { Check, CloudCog, HardDrive, Network, RefreshCw, Server, ShieldCheck, Tags } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import { createInstanceTask, getLaunchOptions, getLaunchOptionsForContext } from "../services/api";
import type { InstanceTemplate, Job, LaunchOption, LaunchOptions, ShapeOption } from "../services/api";

const steps = [
  { title: "上下文", icon: CloudCog, description: "Profile、区域、Compartment" },
  { title: "规格", icon: Server, description: "镜像、Shape、OCPU、内存" },
  { title: "网络", icon: Network, description: "VCN、子网、公网 IP、SSH" },
  { title: "存储", icon: HardDrive, description: "启动盘大小" },
  { title: "标签", icon: Tags, description: "审计和归属" },
  { title: "提交", icon: Check, description: "任务化执行" }
];

export function CreateInstancePage() {
  const [options, setOptions] = useState<LaunchOptions | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshingOptions, setIsRefreshingOptions] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [profileId, setProfileId] = useState("");
  const [region, setRegion] = useState("");
  const [compartmentId, setCompartmentId] = useState("");
  const [availabilityAd, setAvailabilityAd] = useState("");
  const [name, setName] = useState("");
  const [templateId, setTemplateId] = useState("");
  const [imageId, setImageId] = useState("");
  const [shape, setShape] = useState("VM.Standard.E3.Flex");
  const [ocpus, setOcpus] = useState(1);
  const [memoryGb, setMemoryGb] = useState(1);
  const [bootVolumeGb, setBootVolumeGb] = useState(50);
  const [assignPublicIp, setAssignPublicIp] = useState(false);
  const [enableIpv6, setEnableIpv6] = useState(false);
  const [vcnId, setVcnId] = useState("");
  const [subnetId, setSubnetId] = useState("");
  const [reservedPublicIp, setReservedPublicIp] = useState("");
  const [sshKey, setSshKey] = useState("");
  const [cloudInit, setCloudInit] = useState("");
  const [ownerTag, setOwnerTag] = useState("");
  const [purposeTag, setPurposeTag] = useState("");
  const [retryMode, setRetryMode] = useState<"success_stop" | "count" | "none">("none");
  const [retryMaxAttempts, setRetryMaxAttempts] = useState(3);
  const [retryDelayMinSeconds, setRetryDelayMinSeconds] = useState(3);
  const [retryDelayMaxSeconds, setRetryDelayMaxSeconds] = useState(12);
  const [requireApproval, setRequireApproval] = useState(false);
  const [snapshotBefore, setSnapshotBefore] = useState(true);
  const [generateRootPassword, setGenerateRootPassword] = useState(true);
  const [notifyRootPassword, setNotifyRootPassword] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [resultMessage, setResultMessage] = useState("");
  const [errorMessage, setErrorMessage] = useState("");

  const selectedProfile = useMemo(() => options?.profiles.find((profile) => profile.id === profileId), [options, profileId]);
  const selectedShape = useMemo(() => options?.shapes.find((item) => item.name === shape), [options, shape]);
  const selectedCompartment = useMemo(() => optionLabel(options?.compartments, compartmentId), [options, compartmentId]);
  const isRootTenancy = useMemo(() => {
    const label = selectedCompartment.toLowerCase();
    return label.includes("root tenancy") || label.includes("root");
  }, [selectedCompartment]);

  useEffect(() => {
    async function load() {
      setIsLoading(true);
      setLoadError("");
      try {
        const launchOptions = await getLaunchOptions();
        applyLoadedOptions(launchOptions);
      } catch (error) {
        setLoadError(error instanceof Error ? error.message : "加载创建选项失败");
      } finally {
        setIsLoading(false);
      }
    }

    void load();
  }, []);

  function applyLoadedOptions(launchOptions: LaunchOptions) {
    setOptions(launchOptions);
    const firstProfile = launchOptions.profiles[0];
    if (!profileId && firstProfile) {
      setProfileId(firstProfile.id);
      setRegion(firstProfile.defaultRegion);
    }
    if (!compartmentId && launchOptions.compartments[0]) {
      setCompartmentId(launchOptions.compartments[0].id);
    }
    if (!availabilityAd && launchOptions.availabilityAds[0]) {
      setAvailabilityAd(launchOptions.availabilityAds[0].id);
    }
    if (!shape && launchOptions.shapes[0]) {
      setShape(launchOptions.shapes[0].name);
    }
    if (!imageId && launchOptions.images[0]) {
      setImageId(launchOptions.images[0].id);
    }
    if (!vcnId && launchOptions.vcns[0]) {
      setVcnId(launchOptions.vcns[0].id);
    }
    if (!subnetId && launchOptions.subnets[0]) {
      setSubnetId(launchOptions.subnets[0].id);
      setAssignPublicIp(Boolean(launchOptions.subnets[0].public));
    }
  }

  function applyTemplateValues(template: InstanceTemplate) {
    setTemplateId(template.id);
    setProfileId(template.profileId);
    setRegion(template.region);
    setCompartmentId(template.compartment);
    setImageId(template.imageId);
    setShape(template.shape);
    setOcpus(template.ocpus);
    setMemoryGb(template.memoryGb);
    setBootVolumeGb(template.bootVolumeGb);
    setVcnId(template.vcnId);
    setSubnetId(template.subnetId);
    setAssignPublicIp(template.assignPublicIp);
    setOwnerTag(template.tags.owner ?? "");
    setPurposeTag(template.tags.purpose ?? "");
  }

  function applyTemplate(nextTemplateId: string) {
    setTemplateId(nextTemplateId);
    const template = options?.templates.find((item) => item.id === nextTemplateId);
    if (template) {
      applyTemplateValues(template);
    }
  }

  async function refreshRealOptions() {
    setIsRefreshingOptions(true);
    setLoadError("");
    try {
      const launchOptions = await getLaunchOptionsForContext({
        profileId,
        region,
        compartmentId,
        availabilityDomain: availabilityAd,
        vcnId,
        shape
      });
      applyLoadedOptions(launchOptions);
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "刷新真实 OCI 选项失败");
    } finally {
      setIsRefreshingOptions(false);
    }
  }

  function applyShape(nextShape: string) {
    setShape(nextShape);
    const shapeOption = options?.shapes.find((item) => item.name === nextShape);
    if (!shapeOption) return;
    if (shapeOption.minOcpus > 0 && ocpus < shapeOption.minOcpus) setOcpus(shapeOption.minOcpus);
    if (shapeOption.minMemoryGb > 0 && memoryGb < shapeOption.minMemoryGb) setMemoryGb(shapeOption.minMemoryGb);
  }

  function retryMaxRetries() {
    if (retryMode === "none") return 0;
    if (retryMode === "count") return Math.max(0, retryMaxAttempts);
    return 9999;
  }

  function normalizeRetryDelay(value: number) {
    return Number.isFinite(value) ? Math.max(0, Math.floor(value)) : 0;
  }

  function retryDelayBounds() {
    const min = normalizeRetryDelay(retryDelayMinSeconds);
    const max = normalizeRetryDelay(retryDelayMaxSeconds);
    return {
      min: Math.min(min, max),
      max: Math.max(min, max)
    };
  }

  async function handleSubmit() {
    setIsSubmitting(true);
    setResultMessage("");
    setErrorMessage("");
    try {
      const retryDelay = retryDelayBounds();
      const result = await createInstanceTask({
        name,
        profileId,
        region,
        compartment: selectedCompartment || compartmentId,
        compartmentId,
        availabilityAd,
        templateId,
        imageId,
        shape,
        ocpus,
        memoryGb,
        bootVolumeGb,
        assignPublicIp,
        enableIpv6,
        reservedPublicIp,
        vcnId,
        subnetId,
        sshKey,
        cloudInit,
        tags: compactTags({
          owner: ownerTag,
          purpose: purposeTag,
          managedBy: "oci-lifecycle-platform"
        }),
        maxRetries: retryMaxRetries(),
        retryMode,
        retryMaxAttempts: retryMode === "count" ? Math.max(0, retryMaxAttempts) : retryMaxRetries(),
        retryDelayMinSeconds: retryMode === "none" ? 0 : retryDelay.min,
        retryDelayMaxSeconds: retryMode === "none" ? 0 : retryDelay.max,
        requireApproval,
        snapshotBefore,
        generateRootPassword: isRootTenancy && generateRootPassword,
        notifyRootPassword
      });
      const job = "job" in result ? result.job : (result as Job);
      setResultMessage(`已提交创建任务 ${job.id}。请到任务中心查看 OCI Request ID、Work Request 和执行结果。`);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "创建实例任务失败");
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="创建"
        title="创建 OCI 实例"
        description="从真实 OCI Profile 和 Launch Options 选择参数，提交后由后端任务系统执行 LaunchInstance。"
      />

      <section className="wizard-layout">
        <div className="glass-panel wizard-steps">
          {steps.map((step, index) => {
            const Icon = step.icon;
            return (
              <div className={`wizard-step ${index === 0 ? "active" : ""}`} key={step.title}>
                <div className="wizard-step-index">
                  <Icon size={18} />
                </div>
                <div>
                  <strong>{step.title}</strong>
                  <p>{step.description}</p>
                </div>
              </div>
            );
          })}
        </div>

        <div className="glass-panel section-card wizard-panel">
          <AsyncState isLoading={isLoading} error={loadError} empty={!isLoading && !loadError && !options} />
          {!isLoading && options ? (
            <>
              <div className="section-title-row">
                <div>
                  <h2>真实创建参数</h2>
                  <p>选择上下文后刷新真实 OCI 选项；没有密钥或权限时会显示后端返回的真实错误。</p>
                </div>
                <button className="secondary-button" disabled={isRefreshingOptions} onClick={() => void refreshRealOptions()}>
                  <RefreshCw size={18} />
                  {isRefreshingOptions ? "刷新中..." : "刷新真实选项"}
                </button>
              </div>

              {options.errorMessage ? (
                <div className="inline-error">
                  {options.errorCode ? `${options.errorCode}: ` : ""}
                  {options.errorMessage}
                </div>
              ) : null}
              {options.verified ? (
                <div className="inline-success">已从 OCI 同步 {options.requestIds?.length ?? 0} 个请求结果，时间 {formatTime(options.lastSyncedAt)}</div>
              ) : null}

              <div className="form-section">
                <div className="form-section-title">
                  <CloudCog size={18} />
                  <span>上下文</span>
                </div>
                <div className="form-grid">
                  <label>
                    Profile
                    <select
                      value={profileId}
                      onChange={(event) => {
                        const nextProfile = options.profiles.find((profile) => profile.id === event.target.value);
                        setProfileId(event.target.value);
                        if (nextProfile) setRegion(nextProfile.defaultRegion);
                      }}
                    >
                      <option value="">请选择 Profile</option>
                      {options.profiles.map((profile) => (
                        <option value={profile.id} key={profile.id}>
                          {profile.name} / {profile.defaultRegion}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Region
                    {options.regions.length > 0 ? (
                      <select value={region} onChange={(event) => setRegion(event.target.value)}>
                        <option value="">请选择 Region</option>
                        {options.regions.map((item) => (
                          <option value={item.id} key={item.id}>
                            {item.label}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input value={region} onChange={(event) => setRegion(event.target.value)} placeholder="ap-chuncheon-1" />
                    )}
                  </label>
                  <label>
                    Compartment
                    {options.compartments.length > 0 ? (
                      <select value={compartmentId} onChange={(event) => setCompartmentId(event.target.value)}>
                        <option value="">留空使用 tenancy</option>
                        {options.compartments.map((item) => (
                          <option value={item.id} key={item.id}>
                            {item.label}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input value={compartmentId} onChange={(event) => setCompartmentId(event.target.value)} placeholder="留空使用 tenancy OCID" />
                    )}
                  </label>
                  <label>
                    Availability Domain
                    {options.availabilityAds.length > 0 ? (
                      <select value={availabilityAd} onChange={(event) => setAvailabilityAd(event.target.value)}>
                        <option value="">自动选择</option>
                        {options.availabilityAds.map((item) => (
                          <option value={item.id} key={item.id}>
                            {item.label}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input value={availabilityAd} onChange={(event) => setAvailabilityAd(event.target.value)} placeholder="可留空自动发现" />
                    )}
                  </label>
                </div>
                {selectedProfile ? <p className="muted-line">当前 Profile：{selectedProfile.name}，状态 {selectedProfile.status}</p> : null}
              </div>

              <div className="form-section">
                <div className="form-section-title">
                  <Server size={18} />
                  <span>镜像与规格</span>
                </div>
                <div className="form-grid">
                  <label>
                    实例名称
                    <input value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：oci-worker-01" />
                  </label>
                  <label>
                    模板
                    <select value={templateId} onChange={(event) => applyTemplate(event.target.value)}>
                      <option value="">不使用模板</option>
                      {options.templates.map((template) => (
                        <option value={template.id} key={template.id}>
                          {template.name} / {template.version}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Image
                    {options.images.length > 0 ? (
                      <select value={imageId} onChange={(event) => setImageId(event.target.value)}>
                        <option value="">自动选择兼容镜像</option>
                        {options.images.map((item) => (
                          <option value={item.id} key={item.id}>
                            {item.label}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input value={imageId} onChange={(event) => setImageId(event.target.value)} placeholder="可留空自动发现兼容镜像" />
                    )}
                  </label>
                  <label>
                    Shape
                    {options.shapes.length > 0 ? (
                      <select value={shape} onChange={(event) => applyShape(event.target.value)}>
                        <option value="">请选择 Shape</option>
                        {options.shapes.map((item) => (
                          <option value={item.name} key={item.name}>
                            {item.name}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input value={shape} onChange={(event) => setShape(event.target.value)} placeholder="VM.Standard.E3.Flex" />
                    )}
                  </label>
                  <label>
                    OCPU
                    <input type="number" min={1} value={ocpus} onChange={(event) => setOcpus(Number(event.target.value))} />
                  </label>
                  <label>
                    内存 GB
                    <input type="number" min={1} value={memoryGb} onChange={(event) => setMemoryGb(Number(event.target.value))} />
                  </label>
                  <label>
                    启动盘 GB
                    <input type="number" min={50} value={bootVolumeGb} onChange={(event) => setBootVolumeGb(Number(event.target.value))} />
                  </label>
                </div>
                {selectedShape ? <ShapeHint shape={selectedShape} /> : null}
              </div>

              <div className="form-section">
                <div className="form-section-title">
                  <Network size={18} />
                  <span>网络与访问</span>
                </div>
                <div className="form-grid">
                  <label>
                    VCN
                    {options.vcns.length > 0 ? (
                      <select value={vcnId} onChange={(event) => setVcnId(event.target.value)}>
                        <option value="">自动选择</option>
                        {options.vcns.map((item) => (
                          <option value={item.id} key={item.id}>
                            {item.label}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input value={vcnId} onChange={(event) => setVcnId(event.target.value)} placeholder="可留空自动选择" />
                    )}
                  </label>
                  <label>
                    Subnet
                    {options.subnets.length > 0 ? (
                      <select
                        value={subnetId}
                        onChange={(event) => {
                          const subnet = options.subnets.find((item) => item.id === event.target.value);
                          setSubnetId(event.target.value);
                          if (subnet) setAssignPublicIp(Boolean(subnet.public));
                        }}
                      >
                        <option value="">自动选择</option>
                        {options.subnets.map((item) => (
                          <option value={item.id} key={item.id}>
                            {item.label} / {item.public ? "public" : "private"}{item.ipv6Enabled ? " / IPv6" : ""}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input value={subnetId} onChange={(event) => setSubnetId(event.target.value)} placeholder="可留空自动选择第一个可用子网" />
                    )}
                  </label>
                  <label>
                    保留公网 IP
                    {options.reservedIps.length > 0 ? (
                      <select value={reservedPublicIp} onChange={(event) => setReservedPublicIp(event.target.value)}>
                        <option value="">不绑定保留公网 IP</option>
                        {options.reservedIps.map((item) => (
                          <option value={item.id} key={item.id}>
                            {item.label}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input value={reservedPublicIp} onChange={(event) => setReservedPublicIp(event.target.value)} placeholder="暂无可用保留公网 IP" />
                    )}
                  </label>
                  <label>
                    SSH 公钥
                    <input value={sshKey} onChange={(event) => setSshKey(event.target.value)} placeholder="ssh-ed25519 AAAA..." />
                  </label>
                  <label>
                    cloud-init
                    <input value={cloudInit} onChange={(event) => setCloudInit(event.target.value)} placeholder="#cloud-config 或启动脚本摘要" />
                  </label>
                </div>
              </div>

              <div className="switch-panel">
                <div className="switch-row">
                  <div>
                    <strong>分配公网 IP</strong>
                    <p>开启后请求 OCI 为主 VNIC 分配公网 IP；私有子网可能会拒绝公网 IP。</p>
                  </div>
                  <button className={`toggle-switch ${assignPublicIp ? "on" : ""}`} onClick={() => setAssignPublicIp((value) => !value)} />
                </div>
                <div className="switch-row">
                  <div>
                    <strong>启用 IPv6</strong>
                    <p>仅当所选子网已经启用 IPv6 时生效；否则 OCI 会返回真实错误。</p>
                  </div>
                  <button className={`toggle-switch ${enableIpv6 ? "on" : ""}`} onClick={() => setEnableIpv6((value) => !value)} />
                </div>
                <div className="switch-row">
                  <div>
                    <strong>需要审批</strong>
                    <p>当前阶段写入保护标记；后续会接入正式审批流。</p>
                  </div>
                  <button className={`toggle-switch ${requireApproval ? "on" : ""}`} onClick={() => setRequireApproval((value) => !value)} />
                </div>
                <div className="switch-row">
                  <div>
                    <strong>记录操作快照</strong>
                    <p>把创建参数写入任务输入和审计日志，便于失败排查。</p>
                  </div>
                  <button className={`toggle-switch ${snapshotBefore ? "on" : ""}`} onClick={() => setSnapshotBefore((value) => !value)} />
                </div>
                <div className={`switch-row ${isRootTenancy ? "warning-row" : ""}`}>
                  <div>
                    <strong>Root tenancy 随机 root 密码</strong>
                    <p>
                      {isRootTenancy
                        ? "当前选择 Root tenancy，提交后后端会生成随机 root 密码并通过 cloud-init 设置。"
                        : "仅在选择 Root tenancy 时生效；普通 compartment 不会自动生成 root 密码。"}
                    </p>
                  </div>
                  <button
                    className={`toggle-switch ${isRootTenancy && generateRootPassword ? "on" : ""}`}
                    disabled={!isRootTenancy}
                    onClick={() => setGenerateRootPassword((value) => !value)}
                  />
                </div>
                <div className="switch-row">
                  <div>
                    <strong>推送 root 密码通知</strong>
                    <p>生成密码后写入站内敏感通知；邮件服务启用时会同步推送邮件。</p>
                  </div>
                  <button className={`toggle-switch ${notifyRootPassword ? "on" : ""}`} onClick={() => setNotifyRootPassword((value) => !value)} />
                </div>
              </div>

              <div className="form-section">
                <div className="form-section-title">
                  <Tags size={18} />
                  <span>标签与重试</span>
                </div>
                <div className="form-grid">
                  <label>
                    owner 标签
                    <input value={ownerTag} onChange={(event) => setOwnerTag(event.target.value)} />
                  </label>
                  <label>
                    purpose 标签
                    <input value={purposeTag} onChange={(event) => setPurposeTag(event.target.value)} />
                  </label>
                </div>
                <div className="retry-policy-layout">
                  <div className="choice-grid retry-choice-grid">
                    {[
                      { value: "success_stop", title: "成功则停止模式", description: "按延迟范围持续重试，任务成功后立即停止。" },
                      { value: "count", title: "次数重试模式", description: "失败后最多重试指定次数，适合容量重试。" },
                      { value: "none", title: "无重试模式", description: "失败即停止，不创建自动重试计划。" }
                    ].map((item) => (
                      <button
                        className={`choice-card ${retryMode === item.value ? "active" : ""}`}
                        key={item.value}
                        onClick={() => setRetryMode(item.value as "success_stop" | "count" | "none")}
                        type="button"
                      >
                        <strong>{item.title}</strong>
                        <span>{item.description}</span>
                      </button>
                    ))}
                  </div>
                  {retryMode !== "none" ? (
                    <div className="retry-policy-panel">
                      {retryMode === "count" ? (
                        <label>
                          重试次数
                          <input
                            type="number"
                            min={1}
                            max={999}
                            value={retryMaxAttempts}
                            onChange={(event) => setRetryMaxAttempts(Number(event.target.value))}
                          />
                        </label>
                      ) : null}
                      <div>
                        <span className="field-label">延迟范围（秒）</span>
                        <div className="inline-number-range">
                          <input
                            aria-label="最小延迟秒数"
                            type="number"
                            min={0}
                            value={retryDelayMinSeconds}
                            onChange={(event) => setRetryDelayMinSeconds(Number(event.target.value))}
                          />
                          <span>到</span>
                          <input
                            aria-label="最大延迟秒数"
                            type="number"
                            min={0}
                            value={retryDelayMaxSeconds}
                            onChange={(event) => setRetryDelayMaxSeconds(Number(event.target.value))}
                          />
                        </div>
                      </div>
                    </div>
                  ) : null}
                </div>
              </div>

              <div className="preflight-card">
                <strong>
                  <ShieldCheck size={16} /> 提交说明
                </strong>
                <p>OCI 模式会创建真实 LaunchInstance 任务。没有密钥或权限时，任务会失败并保留真实错误码。</p>
              </div>

              {resultMessage ? (
                <div className="inline-success">
                  {resultMessage} <Link className="link-button" to="/jobs">查看任务中心</Link>
                </div>
              ) : null}
              {errorMessage ? <div className="inline-error">{errorMessage}</div> : null}

              <div className="button-row">
                <button className="primary-button" disabled={isSubmitting || !profileId || !name || !shape} onClick={handleSubmit}>
                  {isSubmitting ? "提交中..." : "创建实例任务"}
                </button>
              </div>
            </>
          ) : null}
        </div>
      </section>
    </div>
  );
}

function ShapeHint({ shape }: { shape: ShapeOption }) {
  return (
    <div className="modal-warning">
      <Server size={18} />
      <span>
        {shape.name}：OCPU {rangeLabel(shape.minOcpus, shape.maxOcpus)}，内存 {rangeLabel(shape.minMemoryGb, shape.maxMemoryGb)} GB，处理器 {shape.arch}
      </span>
    </div>
  );
}

function rangeLabel(min: number, max: number) {
  if (!min && !max) return "未知";
  if (min === max || !max) return String(min || max);
  return `${min}-${max}`;
}

function optionLabel(options: LaunchOption[] | undefined, id: string) {
  if (!id || !options) return "";
  return options.find((item) => item.id === id)?.label ?? "";
}

function compactTags(tags: Record<string, string>) {
  return Object.fromEntries(Object.entries(tags).filter(([, value]) => value.trim() !== ""));
}

function formatTime(value?: string) {
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
