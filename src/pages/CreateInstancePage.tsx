import { ClipboardList, CloudCog, HardDrive, Network, RefreshCw, Save, Server, ShieldCheck, Tags } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { getSelectedOCIContext, onOCIContextChange } from "../app/ociContext";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import { createInstanceTask, createTemplate, getLaunchOptionsForContext, listInstances, listTemplates, updateTemplate } from "../services/api";
import type { BootVolumeUsage, Instance, InstanceTemplate, Job, LaunchOption, LaunchOptions, ShapeOption } from "../services/api";

const ALWAYS_FREE_SHAPES = new Set(["VM.Standard.E2.1.Micro", "VM.Standard.A1.Flex"]);
const ALWAYS_FREE_BOOT_VOLUME_GB = 200;
const ALWAYS_FREE_E2_MICRO_COUNT = 2;
const ALWAYS_FREE_A1_OCPUS = 4;
const ALWAYS_FREE_A1_MEMORY_GB = 24;
const ALWAYS_FREE_BOOT_VOLUME_VPUS_PER_GB = 10;
const HOURS_PER_DAY = 24;
const HOURS_PER_MONTH = 730;
const STANDARD_FLEX_PRICE = { ocpuHour: 0.0255, memoryGbHour: 0.0015 };
const A1_FLEX_PRICE = { ocpuHour: 0.01, memoryGbHour: 0.0015 };
const BOOT_VOLUME_GB_MONTH = 0.0255;
const BOOT_VOLUME_VPU_GB_MONTH = 0.0017;
const BOOT_VOLUME_VPU_OPTIONS = [10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120];
const TEMPLATE_BOOT_VOLUME_OPTIONS = [50, 60, 75, 100, 150, 200, 256, 512, 1024];

export function CreateInstancePage() {
  const [searchParams] = useSearchParams();
  const isTemplateMode = searchParams.get("mode") === "template";
  const editTemplateId = searchParams.get("editTemplateId") || "";
  const [options, setOptions] = useState<LaunchOptions | null>(null);
  const [templates, setTemplates] = useState<InstanceTemplate[]>([]);
  const [selectedTemplateId, setSelectedTemplateId] = useState(searchParams.get("templateId") || "");
  const [appliedTemplate, setAppliedTemplate] = useState<InstanceTemplate | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshingOptions, setIsRefreshingOptions] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [profileId, setProfileId] = useState("");
  const [region, setRegion] = useState("");
  const [compartmentId, setCompartmentId] = useState("");
  const [availabilityAd, setAvailabilityAd] = useState("");
  const [name, setName] = useState("");
  const [imageId, setImageId] = useState("");
  const [shape, setShape] = useState("VM.Standard.E3.Flex");
  const [ocpus, setOcpus] = useState(1);
  const [memoryGb, setMemoryGb] = useState(1);
  const [bootVolumeGb, setBootVolumeGb] = useState(50);
  const [bootVolumeVpusPerGb, setBootVolumeVpusPerGb] = useState(10);
  const [bootVolumeUsage, setBootVolumeUsage] = useState<BootVolumeUsage | null>(null);
  const [inventoryBootVolumeGb, setInventoryBootVolumeGb] = useState<number | null>(null);
  const [inventoryInstances, setInventoryInstances] = useState<Instance[]>([]);
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
  const [isSavingTemplate, setIsSavingTemplate] = useState(false);
  const optionsRefreshSeq = useRef(0);
  const shapeCatalogRef = useRef<ShapeOption[]>([]);

  const selectedProfile = useMemo(() => options?.profiles.find((profile) => profile.id === profileId), [options, profileId]);
  const selectedShape = useMemo(() => options?.shapes.find((item) => item.name === shape), [options, shape]);
  const isOcpuFrozen = isFixedShapeRange(selectedShape?.minOcpus, selectedShape?.maxOcpus);
  const isMemoryFrozen = isFixedShapeRange(selectedShape?.minMemoryGb, selectedShape?.maxMemoryGb);
  const selectedCompartment = useMemo(() => optionLabel(options?.compartments, compartmentId), [options, compartmentId]);
  const isRootTenancy = useMemo(() => {
    const label = selectedCompartment.toLowerCase();
    return label.includes("root tenancy") || label.includes("root");
  }, [selectedCompartment]);
  const budget = useMemo(
    () => estimateBudget(shape, ocpus, memoryGb, bootVolumeGb, bootVolumeVpusPerGb, bootVolumeUsage, inventoryBootVolumeGb, inventoryInstances, selectedShape),
    [shape, ocpus, memoryGb, bootVolumeGb, bootVolumeVpusPerGb, bootVolumeUsage, inventoryBootVolumeGb, inventoryInstances, selectedShape]
  );
  const bootVolumeUsageLabel = formatBootVolumeUsageLabel(bootVolumeUsage, inventoryBootVolumeGb);

  useEffect(() => {
    async function load() {
      setIsLoading(true);
      setLoadError("");
      try {
        const context = getSelectedOCIContext();
        const [launchOptions, instances, templateItems] = await Promise.all([
          getLaunchOptionsForContext({ profileId: context.profileId, region: context.region }),
          listInstances({ profileId: context.profileId, region: context.region }).catch(() => []),
          listTemplates({ profileId: context.profileId, region: context.region }).catch(() => [])
        ]);
        applyLoadedOptions(launchOptions);
        setTemplates(templateItems);
        if (context.profileId) setProfileId(context.profileId);
        if (context.region) setRegion(context.region);
        setInventoryInstances(instances);
        setInventoryBootVolumeGb(sumBootVolumes(instances));
        const requestedTemplateId = searchParams.get("templateId");
        if (requestedTemplateId) {
          const requestedTemplate = templateItems.find((item) => item.id === requestedTemplateId);
          if (requestedTemplate) {
            applyTemplate(requestedTemplate);
          }
        }
      } catch (error) {
        setLoadError(error instanceof Error ? error.message : "加载创建选项失败");
      } finally {
        setIsLoading(false);
      }
    }

    void load();
    return onOCIContextChange((context) => {
      setProfileId(context.profileId);
      setRegion(context.region);
      void refreshLaunchOptionsForContext({ profileId: context.profileId, region: context.region });
    });
  }, []);

  useEffect(() => {
    if (!selectedShape) return;
    setOcpus((current) => clampShapeValue(current, selectedShape.minOcpus, selectedShape.maxOcpus));
    setMemoryGb((current) => clampShapeValue(current, selectedShape.minMemoryGb, selectedShape.maxMemoryGb));
  }, [selectedShape?.name, selectedShape?.minOcpus, selectedShape?.maxOcpus, selectedShape?.minMemoryGb, selectedShape?.maxMemoryGb]);

  function applyLoadedOptions(launchOptions: LaunchOptions, preferredShape = shape) {
    const normalizedOptions = keepSelectedShapeOption(launchOptions, options, preferredShape, shapeCatalogRef.current);
    shapeCatalogRef.current = mergeShapeLists(shapeCatalogRef.current, normalizedOptions.shapes);
    setOptions(normalizedOptions);
    setBootVolumeUsage(normalizedOptions.bootVolumeUsage ?? null);
    const currentImageStillCompatible = imageId && normalizedOptions.images.some((item) => item.id === imageId);
    const firstProfile = normalizedOptions.profiles[0];
    if (!profileId && firstProfile) {
      setProfileId(firstProfile.id);
      setRegion(firstProfile.defaultRegion);
    }
    if (!compartmentId && normalizedOptions.compartments[0]) {
      setCompartmentId(normalizedOptions.compartments[0].id);
    }
    if (!availabilityAd && normalizedOptions.availabilityAds[0]) {
      setAvailabilityAd(normalizedOptions.availabilityAds[0].id);
    }
    if (!shape && normalizedOptions.shapes[0]) {
      setShape(normalizedOptions.shapes[0].name);
    }
    if ((!imageId || !currentImageStillCompatible) && normalizedOptions.images[0]) {
      setImageId(normalizedOptions.images[0].id);
    } else if (imageId && !currentImageStillCompatible && normalizedOptions.images.length === 0) {
      setImageId("");
    }
    if (!vcnId && normalizedOptions.vcns[0]) {
      setVcnId(normalizedOptions.vcns[0].id);
    }
    if (!subnetId && normalizedOptions.subnets[0]) {
      setSubnetId(normalizedOptions.subnets[0].id);
      setAssignPublicIp(Boolean(normalizedOptions.subnets[0].public));
    }
  }

  async function refreshLaunchOptionsForContext(overrides: Partial<{
    profileId: string;
    region: string;
    compartmentId: string;
    availabilityAd: string;
    vcnId: string;
    shape: string;
  }> = {}) {
    const sequence = optionsRefreshSeq.current + 1;
    optionsRefreshSeq.current = sequence;
    setIsRefreshingOptions(true);
    setLoadError("");
    try {
      const context = {
        profileId,
        region,
        compartmentId,
        availabilityAd,
        vcnId,
        shape,
        ...overrides
      };
      const [launchOptions, instances, templateItems] = await Promise.all([
        getLaunchOptionsForContext({
          profileId: context.profileId,
          region: context.region,
          compartmentId: context.compartmentId,
          availabilityDomain: context.availabilityAd,
          vcnId: context.vcnId,
          shape: context.shape
        }),
        listInstances({ profileId: context.profileId, region: context.region }).catch(() => []),
        listTemplates({ profileId: context.profileId, region: context.region }).catch(() => [])
      ]);
      if (sequence !== optionsRefreshSeq.current) return;
      applyLoadedOptions(launchOptions, context.shape);
      setInventoryInstances(instances);
      setInventoryBootVolumeGb(sumBootVolumes(instances));
      setTemplates(templateItems);
    } catch (error) {
      if (sequence !== optionsRefreshSeq.current) return;
      setLoadError(error instanceof Error ? error.message : "刷新选项失败");
    } finally {
      if (sequence === optionsRefreshSeq.current) {
        setIsRefreshingOptions(false);
      }
    }
  }

  async function refreshRealOptions() {
    await refreshLaunchOptionsForContext();
  }

  function applyShape(nextShape: string) {
    setShape(nextShape);
    const shapeOption = options?.shapes.find((item) => item.name === nextShape);
    if (shapeOption) {
      setOcpus((current) => clampShapeValue(current, shapeOption.minOcpus, shapeOption.maxOcpus));
      setMemoryGb((current) => clampShapeValue(current, shapeOption.minMemoryGb, shapeOption.maxMemoryGb));
    }
    void refreshLaunchOptionsForContext({ shape: nextShape });
  }

  function applyTemplate(template: InstanceTemplate) {
    setSelectedTemplateId(template.id);
    setAppliedTemplate(template);
    if (isTemplateMode) setName(template.name);
    if (!isTemplateMode) {
      const defaultInstanceName = templateInstanceName(template);
      if (defaultInstanceName) setName(defaultInstanceName);
    }
    if (template.profileId) setProfileId(template.profileId);
    if (template.region) setRegion(template.region);
    if (template.compartmentId) setCompartmentId(template.compartmentId);
    if (template.availabilityAd) setAvailabilityAd(template.availabilityAd);
    if (template.imageId) setImageId(template.imageId);
    if (template.shape) setShape(template.shape);
    if (template.ocpus > 0) setOcpus(template.ocpus);
    if (template.memoryGb > 0) setMemoryGb(template.memoryGb);
    if (template.bootVolumeGb > 0) setBootVolumeGb(template.bootVolumeGb);
    if (template.bootVolumeVpusPerGb > 0) setBootVolumeVpusPerGb(template.bootVolumeVpusPerGb);
    if (template.vcnId) setVcnId(template.vcnId);
    if (template.subnetId) setSubnetId(template.subnetId);
    setAssignPublicIp(Boolean(template.assignPublicIp));
    setEnableIpv6(Boolean(template.enableIpv6));
    if (template.reservedPublicIp) setReservedPublicIp(template.reservedPublicIp);
    if (template.sshKey) setSshKey(template.sshKey);
    if (template.cloudInit) setCloudInit(template.cloudInit);
    if (template.tags?.owner) setOwnerTag(template.tags.owner);
    if (template.tags?.purpose) setPurposeTag(template.tags.purpose);
    void refreshLaunchOptionsForContext({
      profileId: template.profileId || profileId,
      region: template.region || region,
      compartmentId: template.compartmentId || compartmentId,
      availabilityAd: template.availabilityAd || availabilityAd,
      vcnId: template.vcnId || vcnId,
      shape: template.shape || shape
    });
  }

  async function saveCurrentAsTemplate() {
    setIsSavingTemplate(true);
    setErrorMessage("");
    setResultMessage("");
    try {
      const templateName = isTemplateMode ? name.trim() : window.prompt("模板名称", name ? `${name} 模板` : `${shape} 模板`);
      if (!templateName) return;
      const payload = buildTemplatePayload(templateName);
      const saved = editTemplateId ? await updateTemplate(editTemplateId, payload) : await createTemplate(payload);
      setTemplates((current) => [saved, ...current.filter((item) => item.id !== saved.id)]);
      setSelectedTemplateId(saved.id);
      setAppliedTemplate(saved);
      setResultMessage(`${editTemplateId ? "已更新" : "已保存"}模板 ${saved.name}。`);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "保存模板失败");
    } finally {
      setIsSavingTemplate(false);
    }
  }

  function buildTemplatePayload(templateName: string) {
    const tags = compactTags({
      owner: ownerTag,
      purpose: purposeTag,
      managedBy: "oci-lifecycle-platform"
    });
    const config = {
      instance: {
        name
      },
      context: {
        profileId,
        region,
        compartment: selectedCompartment || compartmentId,
        compartmentId,
        availabilityAd
      },
      imageAndShape: {
        imageId,
        imageName: optionLabel(options?.images, imageId),
        shape,
        ocpus,
        memoryGb,
        bootVolumeGb,
        bootVolumeVpusPerGb
      },
      networkAndAccess: {
        vcnId,
        subnetId,
        assignPublicIp,
        enableIpv6,
        reservedPublicIp,
        sshKey,
        cloudInit
      },
      tags
    };
    return {
        name: templateName,
        description: isTemplateMode ? "从创建模板向导保存的预输入模板" : "从创建实例页面保存的预输入模板",
        version: "v1",
        profileId,
        region,
        compartment: selectedCompartment || compartmentId,
        compartmentId,
        availabilityAd,
        imageId,
        imageName: optionLabel(options?.images, imageId),
        shape,
        ocpus,
        memoryGb,
        bootVolumeGb,
        bootVolumeVpusPerGb,
        vcnId,
        subnetId,
        assignPublicIp,
        enableIpv6,
        reservedPublicIp,
        sshKey,
        cloudInit,
        tags,
        configFormat: "json",
        configText: JSON.stringify(config, null, 2),
        status: "ACTIVE"
    };
  }

  function updateOcpus(value: number) {
    setOcpus(clampShapeValue(value, selectedShape?.minOcpus, selectedShape?.maxOcpus));
  }

  function updateMemoryGb(value: number) {
    setMemoryGb(clampShapeValue(value, selectedShape?.minMemoryGb, selectedShape?.maxMemoryGb));
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
    if (isTemplateMode) {
      await saveCurrentAsTemplate();
      return;
    }
    setIsSubmitting(true);
    setResultMessage("");
    setErrorMessage("");
    try {
      const retryDelay = retryDelayBounds();
      const result = await createInstanceTask({
        name,
        templateId: appliedTemplate?.id || selectedTemplateId || undefined,
        profileId,
        region,
        compartment: selectedCompartment || compartmentId,
        compartmentId,
        availabilityAd,
        imageId,
        shape,
        ocpus,
        memoryGb,
        bootVolumeGb,
        bootVolumeVpusPerGb,
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
        eyebrow={isTemplateMode ? "模板" : "创建"}
        title={isTemplateMode ? "新建 OCI 实例模板" : "创建 OCI 实例"}
        description={
          isTemplateMode
            ? "复用创建实例的真实选项：上下文、镜像规格、网络访问和公网/IPv6 开关。配置会在后端保存为内部 JSON，不调用 OCI 创建机器。"
            : "从真实 OCI Profile 和 Launch Options 选择参数，提交后由后端任务系统执行 LaunchInstance。"
        }
      />

      <section className="wizard-layout">
        <div className="glass-panel section-card wizard-panel">
          <AsyncState isLoading={isLoading} error={loadError} empty={!isLoading && !loadError && !options} />
          {!isLoading && options ? (
            <>
              <div className="section-title-row">
                <div>
                  <h2>真实创建参数</h2>
                  <p>选择上下文后刷新可用选项；没有密钥或权限时会显示后端返回的真实错误。</p>
                </div>
                <button className="secondary-button" disabled={isRefreshingOptions} onClick={() => void refreshRealOptions()}>
                  <RefreshCw size={18} />
                  {isRefreshingOptions ? "刷新中..." : "刷新选项"}
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

              {!isTemplateMode ? (
              <div className="form-section">
                <div className="form-section-title">
                  <ClipboardList size={18} />
                  <span>模板预输入</span>
                </div>
                <div className="template-prefill-row">
                  <label>
                    选择模板
                    <select
                      value={selectedTemplateId}
                      onChange={(event) => {
                        setSelectedTemplateId(event.target.value);
                        setAppliedTemplate(null);
                      }}
                    >
                      <option value="">不使用模板</option>
                      {templates.map((template) => (
                        <option value={template.id} key={template.id}>
                          {template.name} / {template.shape} / {template.region || "未指定区域"}
                        </option>
                      ))}
                    </select>
                  </label>
                  <button
                    className="secondary-button"
                    disabled={!selectedTemplateId}
                    onClick={() => {
                      const template = templates.find((item) => item.id === selectedTemplateId);
                      if (template) applyTemplate(template);
                    }}
                  >
                    应用模板
                  </button>
                  <button
                    className="secondary-button"
                    disabled={!selectedTemplateId && !appliedTemplate}
                    onClick={() => {
                      setSelectedTemplateId("");
                      setAppliedTemplate(null);
                    }}
                  >
                    清空模板
                  </button>
                  <button className="secondary-button" disabled={isSavingTemplate || !shape || !profileId} onClick={() => void saveCurrentAsTemplate()}>
                    <Save size={18} />
                    {isSavingTemplate ? "保存中..." : "保存当前配置为模板"}
                  </button>
                </div>
                <div className="modal-warning">
                  <ClipboardList size={18} />
                  <span>
                    模板只会预填当前创建表单，不会调用 OCI API。最终创建机器仍以你点击“创建实例任务”时的表单参数为准。
                    {appliedTemplate ? ` 当前已应用：${appliedTemplate.name}` : ""}
                  </span>
                </div>
              </div>
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
                        if (nextProfile) {
                          setRegion(nextProfile.defaultRegion);
                          void refreshLaunchOptionsForContext({ profileId: event.target.value, region: nextProfile.defaultRegion });
                        } else {
                          void refreshLaunchOptionsForContext({ profileId: event.target.value });
                        }
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
                      <select
                        value={region}
                        onChange={(event) => {
                          setRegion(event.target.value);
                          void refreshLaunchOptionsForContext({ region: event.target.value });
                        }}
                      >
                        <option value="">请选择 Region</option>
                        {options.regions.map((item) => (
                          <option value={item.id} key={item.id}>
                            {item.label}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <select value={region} disabled={isTemplateMode}>
                        <option value="">{isTemplateMode ? "请先选择可用 Profile 并刷新选项" : "暂无可用 Region"}</option>
                      </select>
                    )}
                  </label>
                  <label>
                    Compartment
                    {options.compartments.length > 0 ? (
                      <select
                        value={compartmentId}
                        onChange={(event) => {
                          setCompartmentId(event.target.value);
                          void refreshLaunchOptionsForContext({ compartmentId: event.target.value });
                        }}
                      >
                        <option value="">留空使用 tenancy</option>
                        {options.compartments.map((item) => (
                          <option value={item.id} key={item.id}>
                            {item.label}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <select value={compartmentId} disabled={isTemplateMode}>
                        <option value="">{isTemplateMode ? "请先刷新真实 Compartment 选项" : "暂无可用 Compartment"}</option>
                      </select>
                    )}
                  </label>
                  <label>
                    Availability Domain
                    {options.availabilityAds.length > 0 ? (
                      <select
                        value={availabilityAd}
                        onChange={(event) => {
                          setAvailabilityAd(event.target.value);
                          void refreshLaunchOptionsForContext({ availabilityAd: event.target.value });
                        }}
                      >
                        <option value="">自动选择</option>
                        {options.availabilityAds.map((item) => (
                          <option value={item.id} key={item.id}>
                            {item.label}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <select value={availabilityAd} disabled={isTemplateMode}>
                        <option value="">{isTemplateMode ? "请先刷新真实 AD 选项" : "暂无可用 AD"}</option>
                      </select>
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
                <div className="form-grid create-spec-grid">
                  <label>
                    {isTemplateMode ? "模板名称" : "实例名称"}
                    <input value={name} onChange={(event) => setName(event.target.value)} placeholder={isTemplateMode ? "例如：韩国 E3 Flex 1C1G" : "例如：oci-worker-01"} />
                  </label>
                  <label className="span-two">
                    Image
                    {options.images.length > 0 ? (
                      <select value={imageId} disabled={isRefreshingOptions} onChange={(event) => setImageId(event.target.value)}>
                        <option value="">自动选择兼容镜像</option>
                        {options.images.map((item) => (
                          <option value={item.id} key={item.id}>
                            {item.label}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <select value={imageId} disabled={isTemplateMode}>
                        <option value="">{isTemplateMode ? "请先刷新真实兼容镜像选项" : "暂无兼容镜像"}</option>
                      </select>
                    )}
                    {isRefreshingOptions ? <span className="label-hint label-hint-left">正在刷新兼容镜像...</span> : null}
                  </label>
                  <label>
                    Shape
                    {options.shapes.length > 0 ? (
                      <select value={shape} disabled={isRefreshingOptions} onChange={(event) => applyShape(event.target.value)}>
                        <option value="">请选择 Shape</option>
                        {options.shapes.map((item) => (
                          <option value={item.name} key={item.name}>
                            {shapeOptionLabel(item)}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <select value={shape} disabled={isTemplateMode}>
                        <option value="">{isTemplateMode ? "请先刷新真实 Shape 选项" : "暂无可用 Shape"}</option>
                      </select>
                    )}
                  </label>
                  <label>
                    <span className="label-row">
                      <span>OCPU</span>
                      {selectedShape ? <span className="label-hint">{shapeRangeHint(selectedShape.minOcpus, selectedShape.maxOcpus, isOcpuFrozen)}</span> : null}
                    </span>
                    {isTemplateMode ? (
                      <select value={ocpus} disabled={!selectedShape || isOcpuFrozen} onChange={(event) => updateOcpus(Number(event.target.value))}>
                        {boundedIntegerOptions(selectedShape?.minOcpus, selectedShape?.maxOcpus, ocpus).map((value) => (
                          <option value={value} key={value}>
                            {value} OCPU
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input
                        type="number"
                        min={selectedShape?.minOcpus || 1}
                        max={selectedShape?.maxOcpus || undefined}
                        value={ocpus}
                        disabled={isOcpuFrozen}
                        onChange={(event) => updateOcpus(Number(event.target.value))}
                      />
                    )}
                  </label>
                  <label>
                    <span className="label-row">
                      <span>内存 GB</span>
                      {selectedShape ? <span className="label-hint">{shapeRangeHint(selectedShape.minMemoryGb, selectedShape.maxMemoryGb, isMemoryFrozen)}</span> : null}
                    </span>
                    {isTemplateMode ? (
                      <select value={memoryGb} disabled={!selectedShape || isMemoryFrozen} onChange={(event) => updateMemoryGb(Number(event.target.value))}>
                        {boundedIntegerOptions(selectedShape?.minMemoryGb, selectedShape?.maxMemoryGb, memoryGb).map((value) => (
                          <option value={value} key={value}>
                            {value} GB
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input
                        type="number"
                        min={selectedShape?.minMemoryGb || 1}
                        max={selectedShape?.maxMemoryGb || undefined}
                        value={memoryGb}
                        disabled={isMemoryFrozen}
                        onChange={(event) => updateMemoryGb(Number(event.target.value))}
                      />
                    )}
                  </label>
                  <div className="disk-config-field">
                    <label>
                      <span className="label-row">
                        <span>启动盘 GB</span>
                        <span className="label-hint">{bootVolumeUsageLabel}</span>
                      </span>
                      {isTemplateMode ? (
                        <select value={bootVolumeGb} onChange={(event) => setBootVolumeGb(Number(event.target.value))}>
                          {templateBootVolumeOptions(bootVolumeGb).map((value) => (
                            <option value={value} key={value}>
                              {value} GB
                            </option>
                          ))}
                        </select>
                      ) : (
                        <input type="number" min={50} value={bootVolumeGb} onChange={(event) => setBootVolumeGb(Number(event.target.value))} />
                      )}
                    </label>
                    <label>
                      <span className="label-row">
                        <span>硬盘性能</span>
                        <span className="label-hint">VPUs/GB</span>
                      </span>
                      <select value={bootVolumeVpusPerGb} onChange={(event) => setBootVolumeVpusPerGb(Number(event.target.value))}>
                        {BOOT_VOLUME_VPU_OPTIONS.map((value) => (
                          <option value={value} key={value}>
                            {value} VPUs/GB{value === 10 ? " / Balanced" : ""}
                          </option>
                        ))}
                      </select>
                    </label>
                  </div>
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
                      <select
                        value={vcnId}
                        onChange={(event) => {
                          setVcnId(event.target.value);
                          void refreshLaunchOptionsForContext({ vcnId: event.target.value });
                        }}
                      >
                        <option value="">自动选择</option>
                        {options.vcns.map((item) => (
                          <option value={item.id} key={item.id}>
                            {item.label}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <select value={vcnId} disabled={isTemplateMode}>
                        <option value="">{isTemplateMode ? "请先刷新真实 VCN 选项" : "暂无可用 VCN"}</option>
                      </select>
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
                      <select value={subnetId} disabled={isTemplateMode}>
                        <option value="">{isTemplateMode ? "请先刷新真实 Subnet 选项" : "暂无可用 Subnet"}</option>
                      </select>
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
                      <select value={reservedPublicIp} disabled={isTemplateMode}>
                        <option value="">不绑定保留公网 IP</option>
                      </select>
                    )}
                  </label>
                  <label>
                    SSH 公钥
                    {isTemplateMode ? (
                      <select value={sshKey ? "keep" : ""} onChange={(event) => event.target.value === "" && setSshKey("")}>
                        <option value="">创建实例时填写，模板不保存 SSH 公钥</option>
                        {sshKey ? <option value="keep">保留模板内已保存 SSH 公钥</option> : null}
                      </select>
                    ) : (
                      <input value={sshKey} onChange={(event) => setSshKey(event.target.value)} placeholder="ssh-ed25519 AAAA..." />
                    )}
                  </label>
                  <label>
                    cloud-init
                    {isTemplateMode ? (
                      <select value={cloudInit ? "keep" : ""} onChange={(event) => event.target.value === "" && setCloudInit("")}>
                        <option value="">创建实例时填写，模板不保存 cloud-init</option>
                        {cloudInit ? <option value="keep">保留模板内已保存 cloud-init</option> : null}
                      </select>
                    ) : (
                      <input value={cloudInit} onChange={(event) => setCloudInit(event.target.value)} placeholder="#cloud-config 或启动脚本摘要" />
                    )}
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
                {!isTemplateMode ? (
                  <>
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
                  </>
                ) : null}
              </div>

              {!isTemplateMode ? <div className="form-section">
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
              </div> : null}

              <div className="preflight-card">
                <strong>
                  <ShieldCheck size={16} /> {isTemplateMode ? "保存说明" : "提交说明"}
                </strong>
                <p>
                  {isTemplateMode
                    ? "保存模板只会写入由当前真实选项生成的内部 JSON 预输入配置，不创建实例、不调用 OCI API。后续使用模板创建机器时才进入真实任务。"
                    : "OCI 模式会创建真实 LaunchInstance 任务。没有密钥或权限时，任务会失败并保留真实错误码。"}
                </p>
              </div>

              {resultMessage ? (
                <div className="inline-success">
                  {resultMessage} <Link className="link-button" to={isTemplateMode ? "/templates" : "/jobs"}>{isTemplateMode ? "查看模板管理" : "查看任务中心"}</Link>
                </div>
              ) : null}
              {errorMessage ? <div className="inline-error">{errorMessage}</div> : null}

              <div className="button-row">
                <button className="primary-button" disabled={isSubmitting || isRefreshingOptions || !profileId || !name || !shape} onClick={handleSubmit}>
                  {isSubmitting || isSavingTemplate ? (isTemplateMode ? "保存中..." : "提交中...") : isTemplateMode ? editTemplateId ? "更新模板" : "创建模板" : "创建实例任务"}
                </button>
              </div>
            </>
          ) : null}
        </div>
        <BudgetPanel budget={budget} bootVolumeUsage={bootVolumeUsage} inventoryBootVolumeGb={inventoryBootVolumeGb} />
      </section>
    </div>
  );
}

type BudgetEstimate = {
  computeKnown: boolean;
  storageKnown: boolean;
  status: "free" | "estimated" | "rough" | "missing_price" | "waiting_usage";
  statusLabel: string;
  hourly: number | null;
  daily: number | null;
  monthly: number | null;
  computeHourly: number | null;
  storageHourly: number | null;
  storageCapacityHourly: number | null;
  storagePerformanceHourly: number | null;
  billableBootGb: number | null;
  bootVolumeVpusPerGb: number;
  freeBootRemainingGb: number | null;
  note: string;
  freeTierBlockers: string[];
};

function BudgetPanel({
  budget,
  bootVolumeUsage,
  inventoryBootVolumeGb
}: {
  budget: BudgetEstimate;
  bootVolumeUsage: BootVolumeUsage | null;
  inventoryBootVolumeGb: number | null;
}) {
  return (
    <aside className="glass-panel budget-panel">
      <div className="budget-heading">
        <div className="wizard-step-index">
          <HardDrive size={18} />
        </div>
        <div>
          <strong>预算估算</strong>
          <p>按当前创建参数实时计算</p>
        </div>
      </div>

      <div className="budget-total">
        <span>预计每月</span>
        <strong>{formatMoney(budget.monthly, "month")}</strong>
        <small>{budget.hourly === null ? budget.statusLabel : `${formatMoney(budget.hourly, "hour")} / 小时`}</small>
        <em className={`budget-status ${budget.status}`}>{budget.statusLabel}</em>
      </div>

      <div className="budget-metrics">
        <div>
          <span>每小时</span>
          <strong>{formatMoney(budget.hourly, "hour")}</strong>
        </div>
        <div>
          <span>每天</span>
          <strong>{formatMoney(budget.daily, "day")}</strong>
        </div>
        <div>
          <span>每月</span>
          <strong>{formatMoney(budget.monthly, "month")}</strong>
        </div>
      </div>

      <dl className="budget-breakdown">
        <div>
          <dt>计算资源</dt>
          <dd>{budget.computeHourly === null ? budget.statusLabel : `${formatMoney(budget.computeHourly, "hour")} / 小时`}</dd>
        </div>
        <div>
          <dt>启动盘超额</dt>
          <dd>
            {budget.billableBootGb === null ? "等待 OCI 用量" : `${budget.billableBootGb} GB`}
            {budget.storageCapacityHourly !== null ? ` · 容量 ${formatMoney(budget.storageCapacityHourly, "hour")} / 小时` : ""}
          </dd>
        </div>
        <div>
          <dt>硬盘性能</dt>
          <dd>
            {budget.billableBootGb === null ? "等待 OCI 用量" : `${budget.bootVolumeVpusPerGb} VPUs/GB`}
            {budget.storagePerformanceHourly !== null ? ` · ${formatMoney(budget.storagePerformanceHourly, "hour")} / 小时` : ""}
          </dd>
        </div>
        <div>
          <dt>已用启动盘</dt>
          <dd>{bootVolumeUsageText(bootVolumeUsage, inventoryBootVolumeGb)}</dd>
        </div>
        <div>
          <dt>免费剩余</dt>
          <dd>{budget.freeBootRemainingGb === null ? "-" : `${budget.freeBootRemainingGb} GB`}</dd>
        </div>
      </dl>

      <p className="budget-note">{budget.note}</p>
      {budget.freeTierBlockers.length > 0 ? (
        <ul className="budget-note warning">
          {budget.freeTierBlockers.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      ) : null}
      {bootVolumeUsage && !bootVolumeUsage.verified ? <p className="budget-note warning">{bootVolumeUsage.errorMessage}</p> : null}
    </aside>
  );
}

function ShapeHint({ shape }: { shape: ShapeOption }) {
  return (
    <div className="modal-warning">
      <Server size={18} />
      <span>
        {shape.name}
        {isAlwaysFreeShape(shape.name) ? <strong className="free-badge">Free</strong> : null}
        {shape.arch === "selection-preserved" ? (
          <>：刷新结果未返回该 Shape，已保留当前选择。请刷新选项或检查区域/AD。</>
        ) : (
          <>：OCPU {rangeLabel(shape.minOcpus, shape.maxOcpus)}，内存 {rangeLabel(shape.minMemoryGb, shape.maxMemoryGb)} GB，处理器 {shape.arch}</>
        )}
      </span>
    </div>
  );
}

function isAlwaysFreeShape(shapeName: string) {
  return ALWAYS_FREE_SHAPES.has(shapeName);
}

function shapeOptionLabel(shape: ShapeOption) {
  if (shape.arch === "selection-preserved") return `${shape.name} / 保留选择`;
  return isAlwaysFreeShape(shape.name) ? `${shape.name} / Free` : shape.name;
}

function keepSelectedShapeOption(
  launchOptions: LaunchOptions,
  previousOptions: LaunchOptions | null,
  selectedShapeName: string,
  catalogShapes: ShapeOption[]
): LaunchOptions {
  const normalizedShape = selectedShapeName.trim();
  let shapes = mergeShapeOptions(launchOptions.shapes, previousOptions?.shapes ?? [], catalogShapes, normalizedShape);
  if (!normalizedShape || shapes.some((item) => item.name === normalizedShape)) {
    return shapes === launchOptions.shapes ? launchOptions : { ...launchOptions, shapes };
  }
  const previousShape = shapes.find((item) => item.name === normalizedShape);
  const preservedShape =
    previousShape ??
    ({
      name: normalizedShape,
      arch: "selection-preserved",
      minOcpus: 0,
      maxOcpus: 0,
      minMemoryGb: 0,
      maxMemoryGb: 0
    } satisfies ShapeOption);
  shapes = [preservedShape, ...shapes];
  return {
    ...launchOptions,
    shapes
  };
}

function mergeShapeOptions(currentShapes: ShapeOption[], previousShapes: ShapeOption[], catalogShapes: ShapeOption[], selectedShapeName: string): ShapeOption[] {
  const bestKnownShapes = largestShapeList(currentShapes, previousShapes, catalogShapes);
  const shouldKeepPreviousFullList =
    selectedShapeName.trim() !== "" &&
    bestKnownShapes.length > currentShapes.length &&
    currentShapes.length <= 1 &&
    (currentShapes.length === 0 || currentShapes.some((item) => item.name === selectedShapeName));
  if (!shouldKeepPreviousFullList) {
    return currentShapes.length > 0 ? currentShapes : bestKnownShapes;
  }

  return mergeShapeLists(bestKnownShapes, currentShapes);
}

function largestShapeList(...shapeLists: ShapeOption[][]) {
  return shapeLists.reduce((best, list) => (list.length > best.length ? list : best), [] as ShapeOption[]);
}

function mergeShapeLists(...shapeLists: ShapeOption[][]): ShapeOption[] {
  const byName = new Map<string, ShapeOption>();
  shapeLists.forEach((shapeList) => {
    shapeList.forEach((item) => {
      if (item.name) byName.set(item.name, item);
    });
  });
  return Array.from(byName.values()).sort((a, b) => a.name.localeCompare(b.name));
}

function isFixedShapeRange(min?: number, max?: number) {
  return Number.isFinite(min) && Number.isFinite(max) && Number(min) > 0 && Number(min) === Number(max);
}

function clampShapeValue(value: number, min?: number, max?: number) {
  const safeValue = Number.isFinite(value) ? value : min || 1;
  const safeMin = Number.isFinite(min) && Number(min) > 0 ? Number(min) : 1;
  const safeMax = Number.isFinite(max) && Number(max) > 0 ? Number(max) : undefined;
  if (safeMax !== undefined && safeMin === safeMax) return safeMin;
  return Math.min(Math.max(safeValue, safeMin), safeMax ?? Number.MAX_SAFE_INTEGER);
}

function boundedIntegerOptions(min: number | undefined, max: number | undefined, current: number) {
  const safeCurrent = Number.isFinite(current) && current > 0 ? Math.floor(current) : 1;
  const safeMin = Number.isFinite(min) && Number(min) > 0 ? Math.floor(Number(min)) : safeCurrent;
  const safeMax = Number.isFinite(max) && Number(max) > 0 ? Math.floor(Number(max)) : safeCurrent;
  const lower = Math.min(safeMin, safeMax, safeCurrent);
  const upper = Math.max(safeMin, safeMax, safeCurrent);
  const values: number[] = [];
  for (let value = lower; value <= upper; value += 1) {
    values.push(value);
  }
  return values.length > 0 ? values : [safeCurrent];
}

function templateBootVolumeOptions(current: number) {
  const safeCurrent = Number.isFinite(current) && current >= 50 ? Math.floor(current) : 50;
  return Array.from(new Set([...TEMPLATE_BOOT_VOLUME_OPTIONS, safeCurrent])).filter((value) => value >= 50).sort((a, b) => a - b);
}

function shapeRangeHint(min: number, max: number, frozen: boolean) {
  if (frozen) return `固定 ${min}`;
  return `${rangeLabel(min, max)}`;
}

function sumBootVolumes(instances: Instance[]) {
  return instances.reduce((sum, instance) => {
    if (String(instance.status).toLowerCase().includes("terminat")) return sum;
    return sum + (Number(instance.bootVolumeGb) || 0);
  }, 0);
}

function estimateBudget(
  shape: string,
  ocpus: number,
  memoryGb: number,
  bootVolumeGb: number,
  bootVolumeVpusPerGb: number,
  usage: BootVolumeUsage | null,
  inventoryBootVolumeGb: number | null,
  instances: Instance[],
  shapeOption?: ShapeOption
): BudgetEstimate {
  const computeUsage = summarizeComputeFreeUsage(instances);
  const compute = estimateComputeHourly(shape, ocpus, memoryGb, computeUsage, shapeOption);
  const usedBootGb = usage?.verified ? usage.totalGb : inventoryBootVolumeGb;
  const freeBootRemainingGb = usedBootGb === null ? null : Math.max(0, ALWAYS_FREE_BOOT_VOLUME_GB - usedBootGb);
  const billableBootGb = freeBootRemainingGb === null ? null : Math.max(0, bootVolumeGb - freeBootRemainingGb);
  const safeVPUsPerGB = normalizeBootVolumeVpus(bootVolumeVpusPerGb);
  const billableBaseVpusGb = billableBootGb === null ? null : billableBootGb * Math.min(safeVPUsPerGB, ALWAYS_FREE_BOOT_VOLUME_VPUS_PER_GB);
  const billableUpliftVpusGb = safeVPUsPerGB > ALWAYS_FREE_BOOT_VOLUME_VPUS_PER_GB ? bootVolumeGb * (safeVPUsPerGB - ALWAYS_FREE_BOOT_VOLUME_VPUS_PER_GB) : 0;
  const storageCapacityHourly = billableBootGb === null ? null : (billableBootGb * BOOT_VOLUME_GB_MONTH) / HOURS_PER_MONTH;
  const storagePerformanceHourly =
    billableBaseVpusGb === null ? null : ((billableBaseVpusGb + billableUpliftVpusGb) * BOOT_VOLUME_VPU_GB_MONTH) / HOURS_PER_MONTH;
  const storageHourly = storageCapacityHourly === null || storagePerformanceHourly === null ? null : storageCapacityHourly + storagePerformanceHourly;
  const hourly = compute.hourly === null || storageHourly === null ? null : compute.hourly + storageHourly;
  const storageFree = billableBootGb === 0 && safeVPUsPerGB <= ALWAYS_FREE_BOOT_VOLUME_VPUS_PER_GB;
  const freeTierBlockers = [
    ...compute.freeTierBlockers,
    ...(billableBootGb === null ? ["等待 OCI 启动盘用量，暂不能确认是否仍在 200GB 免费额度内"] : []),
    ...(billableBootGb !== null && billableBootGb > 0 ? [`启动盘容量将超出 Always Free 剩余额度 ${billableBootGb} GB`] : []),
    ...(safeVPUsPerGB > ALWAYS_FREE_BOOT_VOLUME_VPUS_PER_GB ? [`硬盘性能 ${safeVPUsPerGB} VPUs/GB 超过免费 Balanced ${ALWAYS_FREE_BOOT_VOLUME_VPUS_PER_GB} VPUs/GB`] : [])
  ];
  const status =
    compute.hourly === null
      ? "missing_price"
      : storageHourly === null
        ? "waiting_usage"
        : hourly === 0 && storageFree
          ? "free"
          : compute.status === "free"
            ? "estimated"
            : compute.status;
  const statusLabel =
    compute.hourly === null ? "价格未接入" : storageHourly === null ? "等待启动盘用量" : compute.hourly === 0 && storageFree ? "免费额度内" : "已估算";

  return {
    computeKnown: compute.hourly !== null,
    storageKnown: storageHourly !== null,
    status,
    statusLabel,
    hourly,
    daily: hourly === null ? null : hourly * HOURS_PER_DAY,
    monthly: hourly === null ? null : hourly * HOURS_PER_MONTH,
    computeHourly: compute.hourly,
    storageHourly,
    storageCapacityHourly,
    storagePerformanceHourly,
    billableBootGb,
    bootVolumeVpusPerGb: safeVPUsPerGB,
    freeBootRemainingGb,
    note: compute.note,
    freeTierBlockers
  };
}

function normalizeBootVolumeVpus(value: number) {
  if (!Number.isFinite(value) || value <= 0) return 10;
  return Math.min(120, Math.max(10, Math.round(value)));
}

function estimateComputeHourly(shape: string, ocpus: number, memoryGb: number, usage: ComputeFreeUsage, shapeOption?: ShapeOption) {
  const normalizedShape = shape.trim();
  const safeOcpus = Math.max(0, Number(ocpus) || 0);
  const safeMemoryGb = Math.max(0, Number(memoryGb) || 0);

  if (normalizedShape === "VM.Standard.E2.1.Micro") {
    const isFree = usage.e2MicroCount < ALWAYS_FREE_E2_MICRO_COUNT;
    return {
      hourly: isFree ? 0 : STANDARD_FLEX_PRICE.ocpuHour + STANDARD_FLEX_PRICE.memoryGbHour,
      status: isFree ? ("free" as const) : ("estimated" as const),
      statusLabel: isFree ? "免费额度内" : "已估算",
      note: "E2.1.Micro 按 Always Free 两台候选实例判断；最终仍以 OCI Usage / Cost Analysis 为准。",
      freeTierBlockers: isFree ? [] : [`当前已有 ${usage.e2MicroCount} 台 E2.1.Micro，可能超过 Always Free 两台额度`]
    };
  }
  if (
    normalizedShape === "VM.Standard.A1.Flex" &&
    usage.a1Ocpus + safeOcpus <= ALWAYS_FREE_A1_OCPUS &&
    usage.a1MemoryGb + safeMemoryGb <= ALWAYS_FREE_A1_MEMORY_GB
  ) {
    return {
      hourly: 0,
      status: "free" as const,
      statusLabel: "免费额度内",
      note: `A1.Flex 按账号级 Always Free ${ALWAYS_FREE_A1_OCPUS} OCPU / ${ALWAYS_FREE_A1_MEMORY_GB} GB 内估算为 0；最终以 OCI 用量为准。`,
      freeTierBlockers: []
    };
  }
  if (normalizedShape === "VM.Standard.A1.Flex") {
    const billableOcpus = Math.max(0, usage.a1Ocpus + safeOcpus - ALWAYS_FREE_A1_OCPUS);
    const billableMemoryGb = Math.max(0, usage.a1MemoryGb + safeMemoryGb - ALWAYS_FREE_A1_MEMORY_GB);
    return {
      hourly: billableOcpus * A1_FLEX_PRICE.ocpuHour + billableMemoryGb * A1_FLEX_PRICE.memoryGbHour,
      status: "estimated" as const,
      statusLabel: "已估算",
      note: "A1.Flex 当前配置按账号级剩余额度扣减后估算超出部分。",
      freeTierBlockers: [
        ...(usage.a1Ocpus + safeOcpus > ALWAYS_FREE_A1_OCPUS
          ? [`A1 OCPU 将达到 ${usage.a1Ocpus + safeOcpus}，超过免费 ${ALWAYS_FREE_A1_OCPUS}`]
          : []),
        ...(usage.a1MemoryGb + safeMemoryGb > ALWAYS_FREE_A1_MEMORY_GB
          ? [`A1 内存将达到 ${usage.a1MemoryGb + safeMemoryGb} GB，超过免费 ${ALWAYS_FREE_A1_MEMORY_GB} GB`]
          : [])
      ]
    };
  }
  if (
    normalizedShape.includes("VM.Standard.E3.Flex") ||
    normalizedShape.includes("VM.Standard.E4.Flex") ||
    normalizedShape.includes("VM.Standard.E5.Flex") ||
    normalizedShape.includes("VM.Standard.E6.Flex") ||
    normalizedShape.includes("VM.Standard3.Flex")
  ) {
    return {
      hourly: safeOcpus * STANDARD_FLEX_PRICE.ocpuHour + safeMemoryGb * STANDARD_FLEX_PRICE.memoryGbHour,
      status: "estimated" as const,
      statusLabel: "已估算",
      note: "标准 Flex Shape 按本地价格表估算，最终金额以 OCI Cost Analysis / Usage 报表为准。",
      freeTierBlockers: [`${normalizedShape} 不属于当前 Always Free 计算 Shape`]
    };
  }
  const inferredOcpus = shapeOption?.minOcpus || safeOcpus;
  const inferredMemoryGb = shapeOption?.minMemoryGb || safeMemoryGb;
  if (/^(VM|BM)\.Standard/.test(normalizedShape) && inferredOcpus > 0 && inferredMemoryGb > 0) {
    return {
      hourly: inferredOcpus * STANDARD_FLEX_PRICE.ocpuHour + inferredMemoryGb * STANDARD_FLEX_PRICE.memoryGbHour,
      status: "rough" as const,
      statusLabel: "粗估",
      note: "该固定规格 Shape 暂未接入精确 SKU 价格，当前按 OCPU/内存通用单价粗估；最终金额以 OCI Cost Analysis / Usage 报表为准。",
      freeTierBlockers: [`${normalizedShape} 不属于当前 Always Free 计算 Shape`]
    };
  }
  return {
    hourly: null,
    status: "missing_price" as const,
    statusLabel: "价格未接入",
    note: "当前 Shape 未接入价格表，预算不硬算；需要接入 OCI Price List、Cost Report 或手工价格表后再显示金额。",
    freeTierBlockers: [`${normalizedShape || "当前 Shape"} 无法判断免费额度`]
  };
}

type ComputeFreeUsage = {
  e2MicroCount: number;
  a1Ocpus: number;
  a1MemoryGb: number;
};

function summarizeComputeFreeUsage(instances: Instance[]): ComputeFreeUsage {
  return activeInstances(instances).reduce(
    (usage, instance) => {
      if (instance.shape === "VM.Standard.E2.1.Micro") {
        usage.e2MicroCount += 1;
      }
      if (instance.shape === "VM.Standard.A1.Flex") {
        usage.a1Ocpus += Number(instance.ocpus) || 0;
        usage.a1MemoryGb += Number(instance.memoryGb) || 0;
      }
      return usage;
    },
    { e2MicroCount: 0, a1Ocpus: 0, a1MemoryGb: 0 }
  );
}

function activeInstances(instances: Instance[]) {
  return instances.filter((instance) => !String(instance.status).toLowerCase().includes("terminat"));
}

function formatBootVolumeUsageLabel(usage: BootVolumeUsage | null, inventoryBootVolumeGb: number | null) {
  if (usage?.verified) {
    return `已用 ${usage.totalGb} GB / Always Free ${ALWAYS_FREE_BOOT_VOLUME_GB} GB`;
  }
  if (inventoryBootVolumeGb !== null && inventoryBootVolumeGb > 0) {
    return `库存估算 ${inventoryBootVolumeGb} GB / 等待 OCI 启动卷查询`;
  }
  return `已用 - GB / Always Free ${ALWAYS_FREE_BOOT_VOLUME_GB} GB`;
}

function bootVolumeUsageText(usage: BootVolumeUsage | null, inventoryBootVolumeGb: number | null) {
  if (usage?.verified) {
    return `${usage.totalGb} GB · ${usage.bootVolumeCount} 个启动卷`;
  }
  if (inventoryBootVolumeGb !== null && inventoryBootVolumeGb > 0) {
    return `库存估算 ${inventoryBootVolumeGb} GB`;
  }
  return "等待 OCI 启动卷查询";
}

function formatMoney(value: number | null, unit: "hour" | "day" | "month") {
  if (value === null || Number.isNaN(value)) return "待计算";
  const digits = unit === "hour" ? 4 : 2;
  return `$${value.toFixed(digits)}`;
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
