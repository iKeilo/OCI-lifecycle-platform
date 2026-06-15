import { ArrowLeft, CloudCog, HardDrive, Network, RefreshCw, Save, Server } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { getSelectedOCIContext, onOCIContextChange } from "../app/ociContext";
import { AsyncState } from "../components/AsyncState";
import { PageHeader } from "../components/PageHeader";
import {
  createTemplate,
  getLaunchOptionsForContext,
  getTemplate,
  updateTemplate,
  type InstanceTemplate,
  type LaunchOption,
  type LaunchOptions,
  type ShapeOption,
  type TemplatePayload
} from "../services/api";

const BOOT_VOLUME_VPU_OPTIONS = [10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120];
const BOOT_VOLUME_GB_OPTIONS = [50, 60, 75, 100, 150, 200, 256, 512, 1024];
const ALWAYS_FREE_SHAPES = new Set(["VM.Standard.E2.1.Micro", "VM.Standard.A1.Flex"]);

const emptyOptions: LaunchOptions = {
  profiles: [],
  templates: [],
  regions: [],
  compartments: [],
  availabilityAds: [],
  images: [],
  shapes: [],
  vcns: [],
  subnets: [],
  reservedIps: [],
  bootVolumeUsage: {
    verified: false,
    totalGb: 0,
    bootVolumeCount: 0,
    compartmentCount: 0,
    availabilityDomainCount: 0
  }
};

export function TemplateEditorPage() {
  const { templateId = "" } = useParams();
  const isEdit = Boolean(templateId);
  const navigate = useNavigate();
  const [options, setOptions] = useState<LaunchOptions>(emptyOptions);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [errorMessage, setErrorMessage] = useState("");
  const [resultMessage, setResultMessage] = useState("");
  const [name, setName] = useState("");
  const [instanceName, setInstanceName] = useState("");
  const [description, setDescription] = useState("");
  const [profileId, setProfileId] = useState("");
  const [region, setRegion] = useState("");
  const [compartmentId, setCompartmentId] = useState("");
  const [availabilityAd, setAvailabilityAd] = useState("");
  const [imageId, setImageId] = useState("");
  const [shape, setShape] = useState("VM.Standard.E3.Flex");
  const [ocpus, setOcpus] = useState(1);
  const [memoryGb, setMemoryGb] = useState(1);
  const [bootVolumeGb, setBootVolumeGb] = useState(50);
  const [bootVolumeVpusPerGb, setBootVolumeVpusPerGb] = useState(10);
  const [vcnId, setVcnId] = useState("");
  const [subnetId, setSubnetId] = useState("");
  const [reservedPublicIp, setReservedPublicIp] = useState("");
  const [assignPublicIp, setAssignPublicIp] = useState(true);
  const [enableIpv6, setEnableIpv6] = useState(false);
  const refreshSeq = useRef(0);
  const shapeCatalogRef = useRef<ShapeOption[]>([]);

  const selectedProfile = useMemo(() => options.profiles.find((item) => item.id === profileId), [options.profiles, profileId]);
  const selectedShape = useMemo(() => options.shapes.find((item) => item.name === shape), [options.shapes, shape]);
  const selectedCompartment = useMemo(() => optionLabel(options.compartments, compartmentId), [options.compartments, compartmentId]);
  const selectedImage = useMemo(() => optionLabel(options.images, imageId), [options.images, imageId]);
  const isOcpuFrozen = isFixedShapeRange(selectedShape?.minOcpus, selectedShape?.maxOcpus);
  const isMemoryFrozen = isFixedShapeRange(selectedShape?.minMemoryGb, selectedShape?.maxMemoryGb);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setIsLoading(true);
      setLoadError("");
      try {
        const context = getSelectedOCIContext();
        let template: InstanceTemplate | null = null;
        if (templateId) {
          template = await getTemplate(templateId);
          applyTemplate(template);
        } else {
          if (context.profileId) setProfileId(context.profileId);
          if (context.region) setRegion(context.region);
        }
        const launchOptions = await getLaunchOptionsForContext({
          profileId: template?.profileId || context.profileId,
          region: template?.region || context.region,
          compartmentId: template?.compartmentId || "",
          availabilityDomain: template?.availabilityAd || "",
          vcnId: template?.vcnId || "",
          shape: template?.shape || shape
        });
        if (!cancelled) applyLoadedOptions(launchOptions, template?.shape || shape);
      } catch (error) {
        if (!cancelled) setLoadError(error instanceof Error ? error.message : "加载模板选项失败");
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    }
    void load();
    const unsubscribe = onOCIContextChange((context) => {
      setProfileId(context.profileId);
      setRegion(context.region);
      void refreshOptions({ profileId: context.profileId, region: context.region });
    });
    return () => {
      cancelled = true;
      unsubscribe();
    };
  }, [templateId]);

  useEffect(() => {
    if (!selectedShape) return;
    setOcpus((current) => clampShapeValue(current, selectedShape.minOcpus, selectedShape.maxOcpus));
    setMemoryGb((current) => clampShapeValue(current, selectedShape.minMemoryGb, selectedShape.maxMemoryGb));
  }, [selectedShape?.name, selectedShape?.minOcpus, selectedShape?.maxOcpus, selectedShape?.minMemoryGb, selectedShape?.maxMemoryGb]);

  function applyLoadedOptions(launchOptions: LaunchOptions, preferredShape = shape) {
    const normalizedOptions = keepSelectedShapeOption(launchOptions, options, preferredShape, shapeCatalogRef.current);
    shapeCatalogRef.current = mergeShapeLists(shapeCatalogRef.current, normalizedOptions.shapes);
    setOptions(normalizedOptions);
    const firstProfile = normalizedOptions.profiles[0];
    if (!profileId && firstProfile) {
      setProfileId(firstProfile.id);
      setRegion(firstProfile.defaultRegion);
    }
    if (!region && normalizedOptions.regions[0]) setRegion(normalizedOptions.regions[0].id);
    if (!compartmentId && normalizedOptions.compartments[0]) setCompartmentId(normalizedOptions.compartments[0].id);
    if (!availabilityAd && normalizedOptions.availabilityAds[0]) setAvailabilityAd(normalizedOptions.availabilityAds[0].id);
    if (!shape && normalizedOptions.shapes[0]) setShape(normalizedOptions.shapes[0].name);
    const currentImageStillCompatible = imageId && normalizedOptions.images.some((item) => item.id === imageId);
    if ((!imageId || !currentImageStillCompatible) && normalizedOptions.images[0]) setImageId(normalizedOptions.images[0].id);
    if (!vcnId && normalizedOptions.vcns[0]) setVcnId(normalizedOptions.vcns[0].id);
    if (!subnetId && normalizedOptions.subnets[0]) {
      setSubnetId(normalizedOptions.subnets[0].id);
      setAssignPublicIp(Boolean(normalizedOptions.subnets[0].public));
      setEnableIpv6(Boolean(normalizedOptions.subnets[0].ipv6Enabled));
    }
  }

  function applyTemplate(template: InstanceTemplate) {
    setName(template.name);
    setInstanceName(templateInstanceName(template));
    setDescription(template.description || "");
    setProfileId(template.profileId || "");
    setRegion(template.region || "");
    setCompartmentId(template.compartmentId || "");
    setAvailabilityAd(template.availabilityAd || "");
    setImageId(template.imageId || "");
    setShape(template.shape || "VM.Standard.E3.Flex");
    setOcpus(template.ocpus || 1);
    setMemoryGb(template.memoryGb || 1);
    setBootVolumeGb(template.bootVolumeGb || 50);
    setBootVolumeVpusPerGb(template.bootVolumeVpusPerGb || 10);
    setVcnId(template.vcnId || "");
    setSubnetId(template.subnetId || "");
    setReservedPublicIp(template.reservedPublicIp || "");
    setAssignPublicIp(Boolean(template.assignPublicIp));
    setEnableIpv6(Boolean(template.enableIpv6));
  }

  async function refreshOptions(overrides: Partial<{
    profileId: string;
    region: string;
    compartmentId: string;
    availabilityAd: string;
    vcnId: string;
    shape: string;
  }> = {}) {
    const sequence = refreshSeq.current + 1;
    refreshSeq.current = sequence;
    setIsRefreshing(true);
    setLoadError("");
    try {
      const context = { profileId, region, compartmentId, availabilityAd, vcnId, shape, ...overrides };
      const launchOptions = await getLaunchOptionsForContext({
        profileId: context.profileId,
        region: context.region,
        compartmentId: context.compartmentId,
        availabilityDomain: context.availabilityAd,
        vcnId: context.vcnId,
        shape: context.shape
      });
      if (sequence !== refreshSeq.current) return;
      applyLoadedOptions(launchOptions, context.shape);
    } catch (error) {
      if (sequence !== refreshSeq.current) return;
      setLoadError(error instanceof Error ? error.message : "刷新选项失败");
    } finally {
      if (sequence === refreshSeq.current) setIsRefreshing(false);
    }
  }

  function applyShape(nextShape: string) {
    setShape(nextShape);
    const shapeOption = options.shapes.find((item) => item.name === nextShape);
    if (shapeOption) {
      setOcpus((current) => clampShapeValue(current, shapeOption.minOcpus, shapeOption.maxOcpus));
      setMemoryGb((current) => clampShapeValue(current, shapeOption.minMemoryGb, shapeOption.maxMemoryGb));
    }
    void refreshOptions({ shape: nextShape });
  }

  async function handleSave() {
    setIsSaving(true);
    setErrorMessage("");
    setResultMessage("");
    try {
      const payload = buildPayload();
      const saved = isEdit ? await updateTemplate(templateId, payload) : await createTemplate(payload);
      setResultMessage(`${isEdit ? "已更新" : "已创建"}模板 ${saved.name}`);
      navigate("/templates");
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "保存模板失败");
    } finally {
      setIsSaving(false);
    }
  }

  function buildPayload(): TemplatePayload {
    const templateName = name.trim();
    const defaultInstanceName = instanceName.trim();
    const config = {
      instance: {
        name: defaultInstanceName
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
        imageName: selectedImage,
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
        reservedPublicIp
      },
      tags: {
        managedBy: "oci-lifecycle-platform"
      }
    };
    return {
      name: templateName,
      description: description.trim() || "从模板管理选项页保存的 OCI 创建预输入模板",
      version: "v1",
      profileId,
      region,
      compartment: selectedCompartment || compartmentId,
      compartmentId,
      availabilityAd,
      imageId,
      imageName: selectedImage,
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
      sshKey: "",
      cloudInit: "",
      tags: { managedBy: "oci-lifecycle-platform" },
      configFormat: "json",
      configText: JSON.stringify(config, null, 2),
      status: "ACTIVE"
    };
  }

  const canSave = Boolean(name.trim() && profileId && region && shape && ocpus > 0 && memoryGb > 0 && bootVolumeGb >= 50);

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="模板"
        title={isEdit ? "编辑 OCI 实例模板" : "新建 OCI 实例模板"}
        description="这里调用真实 Launch Options 生成选项，不跳转创建实例，不创建 OCI 资源。模板只保存后续创建机器时的预输入。"
        actions={
          <Link className="secondary-button" to="/templates">
            <ArrowLeft size={18} />
            返回模板管理
          </Link>
        }
      />

      <section className="wizard-layout template-editor-layout">
        <div className="glass-panel form-card template-editor-card">
          <div className="wizard-heading">
            <div>
              <h2>模板选项</h2>
              <p>所有 OCI 资源配置都来自下拉、开关和 Shape 约束，不要求手写 OCID 或配置文件。</p>
            </div>
            <button className="secondary-button" disabled={isRefreshing} onClick={() => void refreshOptions()}>
              <RefreshCw size={18} />
              {isRefreshing ? "刷新中..." : "刷新选项"}
            </button>
          </div>

          <AsyncState isLoading={isLoading} error={loadError} empty={false} />
          {!isLoading ? (
            <>
              <div className="form-section">
                <div className="form-section-title">
                  <CloudCog size={18} />
                  <span>模板信息</span>
                </div>
                <div className="form-grid">
                  <label>
                    模板名称
                    <input value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：韩国 E3 Flex 1C1G" />
                  </label>
                  <label>
                    实例名称
                    <input value={instanceName} onChange={(event) => setInstanceName(event.target.value)} placeholder="例如：oci-worker-01" />
                  </label>
                  <label>
                    模板说明
                    <input value={description} onChange={(event) => setDescription(event.target.value)} placeholder="可选，用于说明用途" />
                  </label>
                </div>
              </div>

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
                        const nextProfile = options.profiles.find((item) => item.id === event.target.value);
                        setProfileId(event.target.value);
                        if (nextProfile) {
                          setRegion(nextProfile.defaultRegion);
                          void refreshOptions({ profileId: event.target.value, region: nextProfile.defaultRegion });
                        } else {
                          void refreshOptions({ profileId: event.target.value });
                        }
                      }}
                    >
                      <option value="">请选择 Profile</option>
                      {options.profiles.map((item) => (
                        <option value={item.id} key={item.id}>
                          {item.name} / {item.defaultRegion}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Region
                    <select
                      value={region}
                      onChange={(event) => {
                        setRegion(event.target.value);
                        void refreshOptions({ region: event.target.value });
                      }}
                    >
                      <option value="">请选择 Region</option>
                      {options.regions.map((item) => (
                        <option value={item.id} key={item.id}>
                          {item.label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Compartment
                    <select
                      value={compartmentId}
                      onChange={(event) => {
                        setCompartmentId(event.target.value);
                        void refreshOptions({ compartmentId: event.target.value });
                      }}
                    >
                      <option value="">留空使用 tenancy</option>
                      {options.compartments.map((item) => (
                        <option value={item.id} key={item.id}>
                          {item.label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Availability Domain
                    <select
                      value={availabilityAd}
                      onChange={(event) => {
                        setAvailabilityAd(event.target.value);
                        void refreshOptions({ availabilityAd: event.target.value });
                      }}
                    >
                      <option value="">自动选择</option>
                      {options.availabilityAds.map((item) => (
                        <option value={item.id} key={item.id}>
                          {item.label}
                        </option>
                      ))}
                    </select>
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
                  <label className="span-two">
                    Image
                    <select value={imageId} disabled={isRefreshing} onChange={(event) => setImageId(event.target.value)}>
                      <option value="">自动选择兼容镜像</option>
                      {options.images.map((item) => (
                        <option value={item.id} key={item.id}>
                          {item.label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Shape
                    <select value={shape} disabled={isRefreshing} onChange={(event) => applyShape(event.target.value)}>
                      <option value="">请选择 Shape</option>
                      {options.shapes.map((item) => (
                        <option value={item.name} key={item.name}>
                          {shapeOptionLabel(item)}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    <span className="label-row">
                      <span>OCPU</span>
                      {selectedShape ? <span className="label-hint">{shapeRangeHint(selectedShape.minOcpus, selectedShape.maxOcpus, isOcpuFrozen)}</span> : null}
                    </span>
                    <select value={ocpus} disabled={!selectedShape || isOcpuFrozen} onChange={(event) => setOcpus(clampShapeValue(Number(event.target.value), selectedShape?.minOcpus, selectedShape?.maxOcpus))}>
                      {boundedIntegerOptions(selectedShape?.minOcpus, selectedShape?.maxOcpus, ocpus).map((value) => (
                        <option value={value} key={value}>
                          {value} OCPU
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    <span className="label-row">
                      <span>内存 GB</span>
                      {selectedShape ? <span className="label-hint">{shapeRangeHint(selectedShape.minMemoryGb, selectedShape.maxMemoryGb, isMemoryFrozen)}</span> : null}
                    </span>
                    <select value={memoryGb} disabled={!selectedShape || isMemoryFrozen} onChange={(event) => setMemoryGb(clampShapeValue(Number(event.target.value), selectedShape?.minMemoryGb, selectedShape?.maxMemoryGb))}>
                      {boundedIntegerOptions(selectedShape?.minMemoryGb, selectedShape?.maxMemoryGb, memoryGb).map((value) => (
                        <option value={value} key={value}>
                          {value} GB
                        </option>
                      ))}
                    </select>
                  </label>
                  <div className="disk-config-field">
                    <label>
                      <span className="label-row">
                        <span>启动盘 GB</span>
                        <span className="label-hint">最低 50 GB</span>
                      </span>
                      <select value={bootVolumeGb} onChange={(event) => setBootVolumeGb(Number(event.target.value))}>
                        {bootVolumeOptions(bootVolumeGb).map((value) => (
                          <option value={value} key={value}>
                            {value} GB
                          </option>
                        ))}
                      </select>
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
                {selectedShape ? <p className="muted-line">{shapeSummary(selectedShape)}</p> : null}
              </div>

              <div className="form-section">
                <div className="form-section-title">
                  <Network size={18} />
                  <span>网络与访问</span>
                </div>
                <div className="form-grid">
                  <label>
                    VCN
                    <select
                      value={vcnId}
                      onChange={(event) => {
                        setVcnId(event.target.value);
                        void refreshOptions({ vcnId: event.target.value });
                      }}
                    >
                      <option value="">自动选择</option>
                      {options.vcns.map((item) => (
                        <option value={item.id} key={item.id}>
                          {item.label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Subnet
                    <select
                      value={subnetId}
                      onChange={(event) => {
                        const subnet = options.subnets.find((item) => item.id === event.target.value);
                        setSubnetId(event.target.value);
                        if (subnet) {
                          setAssignPublicIp(Boolean(subnet.public));
                          setEnableIpv6(Boolean(subnet.ipv6Enabled));
                        }
                      }}
                    >
                      <option value="">自动选择</option>
                      {options.subnets.map((item) => (
                        <option value={item.id} key={item.id}>
                          {item.label} / {item.public ? "public" : "private"}{item.ipv6Enabled ? " / IPv6" : ""}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    保留公网 IP
                    <select value={reservedPublicIp} onChange={(event) => setReservedPublicIp(event.target.value)}>
                      <option value="">不绑定保留公网 IP</option>
                      {options.reservedIps.map((item) => (
                        <option value={item.id} key={item.id}>
                          {item.label}
                        </option>
                      ))}
                    </select>
                  </label>
                </div>
              </div>

              <div className="switch-panel">
                <div className="switch-row">
                  <div>
                    <strong>分配公网 IPv4</strong>
                    <p>保存为模板默认值；最终创建时仍以创建实例页提交值为准。</p>
                  </div>
                  <button className={`toggle-switch ${assignPublicIp ? "on" : ""}`} onClick={() => setAssignPublicIp((value) => !value)} />
                </div>
                <div className="switch-row">
                  <div>
                    <strong>启用 IPv6</strong>
                    <p>仅当所选子网支持 IPv6 时创建实例才会成功；模板页不调用 OCI 创建资源。</p>
                  </div>
                  <button className={`toggle-switch ${enableIpv6 ? "on" : ""}`} onClick={() => setEnableIpv6((value) => !value)} />
                </div>
              </div>

              <div className="preflight-card">
                <strong>
                  <HardDrive size={16} /> 保存说明
                </strong>
                <p>模板管理只调用真实选项接口并保存预输入，不进入创建实例页，不提交 LaunchInstance 任务。</p>
              </div>

              {resultMessage ? <div className="inline-success">{resultMessage}</div> : null}
              {errorMessage ? <div className="inline-error">{errorMessage}</div> : null}

              <div className="button-row">
                <button className="primary-button" disabled={isSaving || isRefreshing || !canSave} onClick={() => void handleSave()}>
                  <Save size={18} />
                  {isSaving ? "保存中..." : isEdit ? "更新模板" : "创建模板"}
                </button>
              </div>
            </>
          ) : null}
        </div>
      </section>
    </div>
  );
}

function optionLabel(options: LaunchOption[], id: string) {
  return options.find((item) => item.id === id)?.label || "";
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
  for (let value = lower; value <= upper; value += 1) values.push(value);
  return values.length > 0 ? values : [safeCurrent];
}

function bootVolumeOptions(current: number) {
  const safeCurrent = Number.isFinite(current) && current >= 50 ? Math.floor(current) : 50;
  return Array.from(new Set([...BOOT_VOLUME_GB_OPTIONS, safeCurrent])).filter((value) => value >= 50).sort((a, b) => a - b);
}

function shapeRangeHint(min?: number, max?: number, frozen?: boolean) {
  if (!Number.isFinite(min) || !Number.isFinite(max)) return "未知";
  if (frozen) return `固定 ${min}`;
  return `${min}-${max}`;
}

function shapeOptionLabel(shape: ShapeOption) {
  if (shape.arch === "selection-preserved") return `${shape.name} / 保留选择`;
  return ALWAYS_FREE_SHAPES.has(shape.name) ? `${shape.name} / Free` : shape.name;
}

function shapeSummary(shape: ShapeOption) {
  if (shape.arch === "selection-preserved") return `${shape.name}：刷新结果未返回该 Shape，已保留当前选择。请刷新选项或检查区域/AD。`;
  return `${shape.name}：OCPU ${shapeRangeHint(shape.minOcpus, shape.maxOcpus)}，内存 ${shapeRangeHint(shape.minMemoryGb, shape.maxMemoryGb)} GB，处理器 ${shape.arch}`;
}

function keepSelectedShapeOption(
  launchOptions: LaunchOptions,
  previousOptions: LaunchOptions,
  selectedShapeName: string,
  catalogShapes: ShapeOption[]
): LaunchOptions {
  const normalizedShape = selectedShapeName.trim();
  let shapes = mergeShapeOptions(launchOptions.shapes, previousOptions.shapes, catalogShapes, normalizedShape);
  if (!normalizedShape || shapes.some((item) => item.name === normalizedShape)) return { ...launchOptions, shapes };
  shapes = [
    {
      name: normalizedShape,
      arch: "selection-preserved",
      minOcpus: 1,
      maxOcpus: Math.max(1, Number.MAX_SAFE_INTEGER),
      minMemoryGb: 1,
      maxMemoryGb: Math.max(1, Number.MAX_SAFE_INTEGER)
    },
    ...shapes
  ];
  return { ...launchOptions, shapes };
}

function mergeShapeOptions(currentShapes: ShapeOption[], previousShapes: ShapeOption[], catalogShapes: ShapeOption[], selectedShapeName: string) {
  const bestKnownShapes = largestShapeList(currentShapes, previousShapes, catalogShapes);
  const onlySelectedReturned =
    selectedShapeName.trim() !== "" &&
    bestKnownShapes.length > currentShapes.length &&
    currentShapes.length <= 1 &&
    (currentShapes.length === 0 || currentShapes.some((item) => item.name === selectedShapeName));
  if (onlySelectedReturned) return currentShapes.length > 0 ? currentShapes : bestKnownShapes;
  return mergeShapeLists(bestKnownShapes, currentShapes);
}

function largestShapeList(...shapeLists: ShapeOption[][]) {
  return shapeLists.reduce((best, list) => (list.length > best.length ? list : best), [] as ShapeOption[]);
}

function mergeShapeLists(...shapeLists: ShapeOption[][]) {
  const byName = new Map<string, ShapeOption>();
  shapeLists.forEach((shapeList) => {
    shapeList.forEach((item) => {
      if (item.name) byName.set(item.name, item);
    });
  });
  return Array.from(byName.values()).sort((a, b) => a.name.localeCompare(b.name));
}
