export type Job = {
  id: string;
  type: string;
  status: string;
  profileId: string;
  region: string;
  compartmentId: string;
  resourceType: string;
  resourceId: string;
  ociRequestId: string;
  ociWorkRequestId: string;
  input?: Record<string, unknown>;
  result?: Record<string, unknown>;
  errorCode?: string;
  errorMessage?: string;
  retryCount: number;
  maxRetries: number;
  createdBy: string;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
};

export type Profile = {
  id: string;
  name: string;
  tenancyOcid: string;
  userOcid: string;
  fingerprint: string;
  defaultRegion: string;
  status: string;
  lastCheckedAt: string;
};

export type CreateProfilePayload = {
  name: string;
  tenancyOcid: string;
  userOcid: string;
  fingerprint: string;
  defaultRegion: string;
  privateKey: string;
  privateKeyFile: string;
};

export type Instance = {
  id: string;
  name: string;
  created: string;
  shape: string;
  region: string;
  compartment: string;
  primaryIp: string;
  privateIp: string;
  primaryIpv6: string;
  ipv6Addresses: string[];
  ipv6Enabled: boolean;
  ocpus: number;
  memoryGb: number;
  bootVolumeGb: number;
  bootVolumeVpusPerGb: number;
  status: "Running" | "Stopped" | "Provisioning" | "Terminating" | "Terminated";
  protected: boolean;
  ociInstanceId: string;
  profileId: string;
  compartmentId: string;
  lastSyncedAt: string;
};

export type InstanceTemplate = {
  id: string;
  name: string;
  description: string;
  version: string;
  profileId: string;
  region: string;
  compartment: string;
  compartmentId: string;
  availabilityAd: string;
  imageId: string;
  imageName: string;
  shape: string;
  ocpus: number;
  memoryGb: number;
  bootVolumeGb: number;
  bootVolumeVpusPerGb: number;
  vcnId: string;
  subnetId: string;
  assignPublicIp: boolean;
  enableIpv6: boolean;
  reservedPublicIp: string;
  sshKey: string;
  cloudInit?: string;
  cloudInitSet: boolean;
  tags: Record<string, string>;
  configFormat: string;
  configText?: string;
  status: string;
  validationStatus: string;
  validationErrorCode?: string;
  validationMessage?: string;
  lastValidatedAt?: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
};

export type TemplatePayload = {
  name: string;
  description: string;
  version: string;
  profileId: string;
  region: string;
  compartment: string;
  compartmentId: string;
  availabilityAd: string;
  imageId: string;
  imageName: string;
  shape: string;
  ocpus: number;
  memoryGb: number;
  bootVolumeGb: number;
  bootVolumeVpusPerGb: number;
  vcnId: string;
  subnetId: string;
  assignPublicIp: boolean;
  enableIpv6: boolean;
  reservedPublicIp: string;
  sshKey: string;
  cloudInit: string;
  tags: Record<string, string>;
  configFormat: string;
  configText: string;
  status: string;
};

export type TemplateValidationResult = {
  verified: boolean;
  templateId: string;
  profileId: string;
  region: string;
  compartmentId: string;
  status: string;
  errorCode?: string;
  errorMessage?: string;
  lastValidatedAt: string;
  checkedFields: string[];
  incompatibleKeys?: string[];
};

export type LaunchOption = {
  id: string;
  label: string;
  region?: string;
  compartment?: string;
  public?: boolean;
  ipv6Enabled?: boolean;
};

export type ShapeOption = {
  name: string;
  arch: string;
  minOcpus: number;
  maxOcpus: number;
  minMemoryGb: number;
  maxMemoryGb: number;
};

export type BootVolumeUsage = {
  verified: boolean;
  region?: string;
  totalGb: number;
  bootVolumeCount: number;
  compartmentCount: number;
  availabilityDomainCount: number;
  requestIds?: string[];
  errorCode?: string;
  errorMessage?: string;
  lastSyncedAt?: string;
};

export type LaunchOptions = {
  verified?: boolean;
  profileId?: string;
  region?: string;
  compartmentId?: string;
  cacheState?: string;
  cacheCheckedAt?: string;
  cacheChangedAt?: string;
  shapeFingerprint?: string;
  requestIds?: string[];
  errorCode?: string;
  errorMessage?: string;
  lastSyncedAt?: string;
  profiles: Profile[];
  templates: InstanceTemplate[];
  regions: LaunchOption[];
  compartments: LaunchOption[];
  availabilityAds: LaunchOption[];
  images: LaunchOption[];
  shapes: ShapeOption[];
  shapeImages: Record<string, LaunchOption[]>;
  vcns: LaunchOption[];
  subnets: LaunchOption[];
  reservedIps: LaunchOption[];
  bootVolumeUsage: BootVolumeUsage;
};

export type OCIReadiness = {
  executionMode: string;
  ready: boolean;
  missing?: string[];
  message: string;
};

export type OCIReadOnlyValidationResult = {
  verified: boolean;
  executionMode: string;
  region: string;
  tenancyOcid: string;
  compartmentId: string;
  regionRequestId?: string;
  instancesRequestId?: string;
  regions: Array<{ regionName: string; status: string }>;
  instances: Array<{ id: string; displayName: string; lifecycleState: string; shape: string }>;
  errorCode?: string;
  errorMessage?: string;
  validatedAt: string;
};

export type AutomationRule = {
  id: string;
  name: string;
  type: string;
  targetPool: string;
  action: string;
  triggerInterval: string;
  cooldown: string;
  maxRetries: number;
  failurePolicy: string;
  maxInstances: number;
  maxDailyRuns: number;
  regionScope: string;
  notifyChannel: string;
  enabled: boolean;
  approvalRequired: boolean;
  lastRunAt?: string;
  nextRunAt?: string;
  createdBy: string;
  createdAt: string;
};

export type AuditLog = {
  id: number;
  actor: string;
  action: string;
  resourceType: string;
  resourceId: string;
  profileId: string;
  region: string;
  compartmentId: string;
  ociRequestId: string;
  ociWorkRequestId: string;
  requestPayload?: Record<string, unknown>;
  resultPayload?: Record<string, unknown>;
  errorCode?: string;
  errorMessage?: string;
  createdAt: string;
};

export type AuditLogFilter = {
  actor?: string;
  action?: string;
  resourceType?: string;
  resourceId?: string;
  profileId?: string;
  region?: string;
  compartmentId?: string;
  ociRequestId?: string;
  ociWorkRequestId?: string;
  status?: "success" | "failed" | "";
  limit?: number;
};

type ListResponse<T> = {
  items: T[];
};

export type IPTaskPayload = {
  mode: string;
  reservedPublicIp: string;
  dnsLabel: string;
  vnicId: string;
  note: string;
  enableIpv6: boolean;
  disableIpv6?: boolean;
  autoConfigureIpv6: boolean;
  ipv6Strategy: "assign_only" | "additive" | "clone_route_table" | "replace_public_path";
  networkChangeMode: "assign_only" | "additive" | "clone_route_table" | "replace_public_path";
  routeTableMode: "merge_existing" | "clone";
  securityMode: "append" | "none";
  allowIrreversibleVcnIpv6: boolean;
  allowPublicIpv4Change: boolean;
  openSshIpv6: boolean;
  openHttpIpv6: boolean;
  openHttpsIpv6: boolean;
  snapshotBefore: boolean;
};

export type AutomationTaskPayload = {
  name: string;
  type: string;
  targetPool: string;
  action: string;
  triggerInterval: string;
  cooldown: string;
  maxRetries: number;
  failurePolicy: string;
  maxInstances: number;
  maxDailyRuns: number;
  regionScope: string;
  notifyChannel: string;
  enableImmediately: boolean;
  approvalRequired: boolean;
};

export type CreateInstancePayload = {
  name: string;
  templateId?: string;
  profileId: string;
  region: string;
  compartment: string;
  compartmentId: string;
  availabilityAd: string;
  imageId: string;
  shape: string;
  ocpus: number;
  memoryGb: number;
  bootVolumeGb: number;
  bootVolumeVpusPerGb: number;
  assignPublicIp: boolean;
  enableIpv6: boolean;
  reservedPublicIp: string;
  vcnId: string;
  subnetId: string;
  sshKey: string;
  cloudInit: string;
  tags: Record<string, string>;
  maxRetries: number;
  retryMode: "success_stop" | "count" | "none";
  retryMaxAttempts: number;
  retryDelayMinSeconds: number;
  retryDelayMaxSeconds: number;
  requireApproval: boolean;
  snapshotBefore: boolean;
  generateRootPassword: boolean;
  notifyRootPassword: boolean;
};

export type Notification = {
  id: string;
  title: string;
  message: string;
  severity: "info" | "success" | "warning" | "error";
  category: string;
  resourceType?: string;
  resourceId?: string;
  profileId?: string;
  region?: string;
  compartmentId?: string;
  sensitive: boolean;
  read: boolean;
  emailRequested: boolean;
  emailSent: boolean;
  emailError?: string;
  webhookSent: boolean;
  webhookError?: string;
  createdBy: string;
  createdAt: string;
  readAt?: string;
};

export type NotificationListResponse = {
  items: Notification[];
  unreadCount: number;
};

export type EmailSettings = {
  enabled: boolean;
  host: string;
  port: number;
  username: string;
  password?: string;
  passwordSet: boolean;
  from: string;
  to: string[];
  useTls: boolean;
  startTls: boolean;
};

export type EmailTestResult = {
  verified: boolean;
  message: string;
};

export type WebhookSettings = {
  enabled: boolean;
  url: string;
  secret?: string;
  secretSet: boolean;
  headers?: Record<string, string>;
};

export type WebhookTestResult = {
  verified: boolean;
  message: string;
};

export type AccountSettings = {
  displayName: string;
  email: string;
  avatar: string;
  avatarInitial: string;
  passwordSet: boolean;
  updatedAt?: string;
};

export type AccountProfilePayload = {
  displayName: string;
  email: string;
  avatar: string;
};

export type AccountPasswordPayload = {
  currentPassword: string;
  newPassword: string;
};

export type AppearanceSettings = {
  theme: "light" | "dark";
  backgroundMode: "aurora" | "plain" | "image";
  backgroundImage: string;
  language: "zh-CN" | "en-US";
  updatedAt?: string;
};

export type BudgetSettings = {
  enabled: boolean;
  monthlyBudgetUsd: number;
  actualSpendUsd: number;
  forecastSpendUsd: number;
  thresholdPercent: number;
  scopeMode: "tag" | "compartment" | "manual";
  profileId: string;
  region: string;
  compartmentId: string;
  resourcePool: string;
  tagKey: string;
  tagValue: string;
  manualInstanceIds: string[];
  actionMode: "notify" | "downgrade" | "delete";
  downgradePreset: string;
  deleteBootVolume: boolean;
  requireApproval: boolean;
  updatedAt?: string;
};

export type NetworkInventory = {
  verified: boolean;
  executionMode: string;
  profileId?: string;
  region?: string;
  compartmentId?: string;
  errorCode?: string;
  errorMessage?: string;
  requestIds?: string[];
  lastSyncedAt?: string;
  publicIps: PublicIPResource[];
  privateIps: PrivateIPResource[];
  ipv6s: IPv6Resource[];
  vcns: VCNResource[];
  subnets: SubnetResource[];
};

export type PublicIPResource = {
  id: string;
  displayName: string;
  ipAddress: string;
  lifetime: string;
  scope: string;
  lifecycleState: string;
  assignedEntityId: string;
  compartmentId: string;
  region: string;
  timeCreated?: string;
};

export type PublicIPBatchTaskPayload = {
  action: "create" | "delete";
  profileId?: string;
  region?: string;
  compartmentId?: string;
  count?: number;
  displayPrefix?: string;
  publicIpIds?: string[];
  note?: string;
};

export type PrivateIPResource = {
  id: string;
  displayName: string;
  ipAddress: string;
  hostnameLabel: string;
  vnicId: string;
  subnetId: string;
  compartmentId: string;
  lifecycleState: string;
  timeCreated?: string;
};

export type IPv6Resource = {
  id: string;
  displayName: string;
  ipAddress: string;
  vnicId: string;
  subnetId: string;
  compartmentId: string;
  lifecycleState: string;
  timeCreated?: string;
};

export type VCNResource = {
  id: string;
  displayName: string;
  cidrBlock: string;
  ipv6CidrBlocks: string[];
  lifecycleState: string;
  compartmentId: string;
};

export type SubnetResource = {
  id: string;
  displayName: string;
  vcnId: string;
  cidrBlock: string;
  ipv6CidrBlocks: string[];
  public: boolean;
  compartmentId: string;
  lifecycleState: string;
};

export type CreateInstanceResponse = {
  instance: Instance;
  job: Job;
};

export type AuthStatus = {
  authEnabled: boolean;
  authenticated: boolean;
  user?: AccessUser;
};

export type AccessRole = {
  id: string;
  name: string;
  description: string;
  permissions: string[];
  system: boolean;
};

export type AccessUser = {
  id: string;
  displayName: string;
  email: string;
  roleId: string;
  status: string;
  allowedProfiles: string[];
  allowedRegions: string[];
  allowedCompartments: string[];
  passwordSet: boolean;
  lastLoginAt?: string;
  updatedAt?: string;
};

export type AccessControlSettings = {
  enabled: boolean;
  users: AccessUser[];
  roles: AccessRole[];
  updatedAt?: string;
};

export type SecurityGuardrails = {
  enabled: boolean;
  allowedRegions: string[];
  deniedRegions: string[];
  maxOcpusPerInstance: number;
  maxMemoryGbPerInstance: number;
  maxBootVolumeGb: number;
  maxRetryAttempts: number;
  maxPublicIpBatchCount: number;
  requireApprovalForTerminate: boolean;
  blockBootVolumeDeletion: boolean;
  blockPublicIpv6RouteChanges: boolean;
  blockRootPasswordWithoutEmail: boolean;
  requireTemplateForLaunch: boolean;
  updatedAt?: string;
};

export type AutomationTaskResponse = {
  rule: {
    id: string;
    name: string;
    enabled: boolean;
  };
  job: Job;
};

export type InstanceActionPayload = {
  action: "START" | "STOP" | "REBOOT" | "TERMINATE" | "RESIZE";
  graceful: boolean;
  preserveBootVolume: boolean;
  targetShape: string;
  targetOcpus: number;
  targetMemoryGb: number;
  targetBootVolumeGb: number;
  targetBootVolumeVpusPerGb: number;
  expandBootVolume: boolean;
  snapshotBefore: boolean;
  note: string;
};

export type InstanceReinstallPayload = {
  profileId: string;
  region: string;
  compartmentId: string;
  imageId: string;
  imageName: string;
  bootVolumeSizeGb: number;
  bootVolumeVpusPerGb: number;
  preserveOldBootVolume: boolean;
  createBootVolumeBackup: boolean;
  confirmationName: string;
  note: string;
};

export async function createIPTask(instanceId: string, payload: IPTaskPayload): Promise<Job> {
  return request<Job>(`/api/instances/${encodeURIComponent(instanceId)}/ip-tasks`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function createInstanceAction(instanceId: string, payload: InstanceActionPayload): Promise<Job> {
  return request<Job>(`/api/instances/${encodeURIComponent(instanceId)}/actions`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function createInstanceReinstallTask(instanceId: string, payload: InstanceReinstallPayload): Promise<Job> {
  return request<Job>(`/api/instances/${encodeURIComponent(instanceId)}/system/reinstall`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function createInstanceTask(payload: CreateInstancePayload): Promise<CreateInstanceResponse | Job> {
  return request<CreateInstanceResponse | Job>("/api/instances", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function listTemplates(params: {
  profileId?: string;
  region?: string;
  compartmentId?: string;
  status?: string;
  validationStatus?: string;
  q?: string;
} = {}): Promise<InstanceTemplate[]> {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value) query.set(key, value);
  });
  const response = await request<ListResponse<InstanceTemplate>>(`/api/templates${query.toString() ? `?${query.toString()}` : ""}`);
  return response.items;
}

export async function createTemplate(payload: TemplatePayload): Promise<InstanceTemplate> {
  return request<InstanceTemplate>("/api/templates", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function getTemplate(templateId: string): Promise<InstanceTemplate> {
  return request<InstanceTemplate>(`/api/templates/${encodeURIComponent(templateId)}`);
}

export async function updateTemplate(templateId: string, payload: Partial<TemplatePayload>): Promise<InstanceTemplate> {
  return request<InstanceTemplate>(`/api/templates/${encodeURIComponent(templateId)}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

export async function deleteTemplate(templateId: string): Promise<void> {
  await request<unknown>(`/api/templates/${encodeURIComponent(templateId)}`, {
    method: "DELETE"
  });
}

export async function validateTemplate(templateId: string): Promise<TemplateValidationResult> {
  return request<TemplateValidationResult>(`/api/templates/${encodeURIComponent(templateId)}/validate`, {
    method: "POST",
    body: JSON.stringify({})
  });
}

export async function getLaunchOptions(): Promise<LaunchOptions> {
  return request<LaunchOptions>("/api/launch-options");
}

export async function getLaunchOptionsForContext(params: {
  profileId?: string;
  region?: string;
  compartmentId?: string;
  availabilityDomain?: string;
  vcnId?: string;
  shape?: string;
}): Promise<LaunchOptions> {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value) query.set(key, value);
  }
  return request<LaunchOptions>(`/api/launch-options${query.toString() ? `?${query.toString()}` : ""}`);
}

export async function getOCIReadiness(params: { profileId?: string; region?: string } = {}): Promise<OCIReadiness> {
  const query = new URLSearchParams();
  if (params.profileId) query.set("profileId", params.profileId);
  if (params.region) query.set("region", params.region);
  return request<OCIReadiness>(`/api/oci/readiness${query.toString() ? `?${query.toString()}` : ""}`);
}

export async function validateOCIReadOnly(compartmentId = ""): Promise<OCIReadOnlyValidationResult> {
  return request<OCIReadOnlyValidationResult>("/api/oci/validate-readonly", {
    method: "POST",
    body: JSON.stringify({ compartmentId })
  });
}

export async function listProfiles(): Promise<Profile[]> {
  const response = await request<ListResponse<Profile>>("/api/profiles");
  return response.items;
}

export async function createProfile(payload: CreateProfilePayload): Promise<Profile> {
  return request<Profile>("/api/profiles", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function getProfile(profileId: string): Promise<Profile> {
  return request<Profile>(`/api/profiles/${encodeURIComponent(profileId)}`);
}

export async function testProfile(profileId: string, payload: { region?: string; compartmentId?: string } = {}): Promise<OCIReadOnlyValidationResult> {
  return request<OCIReadOnlyValidationResult>(`/api/profiles/${encodeURIComponent(profileId)}/test`, {
    method: "POST",
    body: JSON.stringify({ profileId, ...payload })
  });
}

export async function enableProfile(profileId: string): Promise<Profile> {
  return request<Profile>(`/api/profiles/${encodeURIComponent(profileId)}/enable`, {
    method: "POST",
    body: JSON.stringify({})
  });
}

export async function disableProfile(profileId: string): Promise<Profile> {
  return request<Profile>(`/api/profiles/${encodeURIComponent(profileId)}/disable`, {
    method: "POST",
    body: JSON.stringify({})
  });
}

export async function deleteProfile(profileId: string): Promise<void> {
  await request<unknown>(`/api/profiles/${encodeURIComponent(profileId)}`, {
    method: "DELETE"
  });
}

export async function listInstances(params: string | { status?: string; profileId?: string; region?: string; compartmentId?: string } = {}): Promise<Instance[]> {
  const normalized = typeof params === "string" ? { status: params } : params;
  const query = new URLSearchParams();
  Object.entries(normalized).forEach(([key, value]) => {
    if (value) query.set(key, value);
  });
  const response = await request<ListResponse<Instance>>(`/api/instances${query.toString() ? `?${query.toString()}` : ""}`);
  return response.items;
}

export async function listJobs(): Promise<Job[]> {
  const response = await request<ListResponse<Job>>("/api/jobs");
  return response.items;
}

export async function getJob(jobId: string): Promise<Job> {
  return request<Job>(`/api/jobs/${encodeURIComponent(jobId)}`);
}

export async function cancelJob(jobId: string): Promise<Job> {
  return request<Job>(`/api/jobs/${encodeURIComponent(jobId)}/cancel`, {
    method: "POST",
    body: JSON.stringify({})
  });
}

export async function retryJob(jobId: string): Promise<Job> {
  return request<Job>(`/api/jobs/${encodeURIComponent(jobId)}/retry`, {
    method: "POST",
    body: JSON.stringify({})
  });
}

export async function clearCompletedJobs(): Promise<{ deletedCount: number; items: Job[] }> {
  return request<{ deletedCount: number; items: Job[] }>("/api/jobs/clear-completed", {
    method: "POST",
    body: JSON.stringify({})
  });
}

export async function listNotifications(unreadOnly = false): Promise<NotificationListResponse> {
  return request<NotificationListResponse>(`/api/notifications${unreadOnly ? "?unread=true" : ""}`);
}

export async function markNotificationRead(id: string): Promise<Notification> {
  return request<Notification>(`/api/notifications/${encodeURIComponent(id)}/read`, {
    method: "POST",
    body: JSON.stringify({})
  });
}

export async function markAllNotificationsRead(): Promise<ListResponse<Notification>> {
  return request<ListResponse<Notification>>("/api/notifications/read-all", {
    method: "POST",
    body: JSON.stringify({})
  });
}

export async function deleteNotification(id: string): Promise<void> {
  await request<unknown>(`/api/notifications/${encodeURIComponent(id)}`, {
    method: "DELETE"
  });
}

export async function listAuditLogs(filter: AuditLogFilter = {}): Promise<AuditLog[]> {
  const query = new URLSearchParams();
  Object.entries(filter).forEach(([key, value]) => {
    if (value !== undefined && value !== "") {
      query.set(key, String(value));
    }
  });
  const response = await request<ListResponse<AuditLog>>(`/api/audit-logs${query.toString() ? `?${query.toString()}` : ""}`);
  return response.items;
}

export async function getEmailSettings(): Promise<EmailSettings> {
  return request<EmailSettings>("/api/email/settings");
}

export async function updateEmailSettings(payload: EmailSettings): Promise<EmailSettings> {
  return request<EmailSettings>("/api/email/settings", {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}

export async function testEmail(to: string): Promise<EmailTestResult> {
  return request<EmailTestResult>("/api/email/test", {
    method: "POST",
    body: JSON.stringify({ to })
  });
}

export async function getWebhookSettings(): Promise<WebhookSettings> {
  return request<WebhookSettings>("/api/webhook/settings");
}

export async function updateWebhookSettings(payload: WebhookSettings): Promise<WebhookSettings> {
  return request<WebhookSettings>("/api/webhook/settings", {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}

export async function testWebhook(): Promise<WebhookTestResult> {
  return request<WebhookTestResult>("/api/webhook/test", {
    method: "POST",
    body: JSON.stringify({})
  });
}

export async function getAccountSettings(): Promise<AccountSettings> {
  return request<AccountSettings>("/api/account");
}

export async function updateAccountProfile(payload: AccountProfilePayload): Promise<AccountSettings> {
  return request<AccountSettings>("/api/account/profile", {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}

export async function updateAccountPassword(payload: AccountPasswordPayload): Promise<AccountSettings> {
  return request<AccountSettings>("/api/account/password", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function getAppearanceSettings(): Promise<AppearanceSettings> {
  return request<AppearanceSettings>("/api/settings/appearance");
}

export async function updateAppearanceSettings(payload: AppearanceSettings): Promise<AppearanceSettings> {
  return request<AppearanceSettings>("/api/settings/appearance", {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}

export async function getBudgetSettings(): Promise<BudgetSettings> {
  return request<BudgetSettings>("/api/budget/settings");
}

export async function updateBudgetSettings(payload: BudgetSettings): Promise<BudgetSettings> {
  return request<BudgetSettings>("/api/budget/settings", {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}

export async function getNetworkInventory(params: {
  profileId?: string;
  region?: string;
  compartmentId?: string;
  vcnId?: string;
} = {}): Promise<NetworkInventory> {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value) query.set(key, value);
  });
  return request<NetworkInventory>(`/api/network/inventory${query.toString() ? `?${query.toString()}` : ""}`);
}

export async function createPublicIPBatchTask(payload: PublicIPBatchTaskPayload): Promise<Job> {
  return request<Job>("/api/network/public-ips/batch", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function listAutomations(): Promise<AutomationRule[]> {
  const response = await request<ListResponse<AutomationRule>>("/api/automations");
  return response.items;
}

export async function createAutomationTask(payload: AutomationTaskPayload): Promise<AutomationTaskResponse> {
  return request<AutomationTaskResponse>("/api/automations/tasks", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function getAuthStatus(): Promise<AuthStatus> {
  return request<AuthStatus>("/api/auth/me");
}

export async function login(password: string, username = ""): Promise<AuthStatus> {
  return request<AuthStatus>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password })
  });
}

export async function logout(): Promise<AuthStatus> {
  return request<AuthStatus>("/api/auth/logout", {
    method: "POST",
    body: JSON.stringify({})
  });
}

export async function getAccessControlSettings(): Promise<AccessControlSettings> {
  return request<AccessControlSettings>("/api/access-control");
}

export async function updateAccessControlSettings(payload: AccessControlSettings): Promise<AccessControlSettings> {
  return request<AccessControlSettings>("/api/access-control", {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}

export async function setAccessUserPassword(userId: string, newPassword: string): Promise<AccessControlSettings> {
  return request<AccessControlSettings>(`/api/access-control/users/${encodeURIComponent(userId)}/password`, {
    method: "POST",
    body: JSON.stringify({ userId, newPassword })
  });
}

export async function getSecurityGuardrails(): Promise<SecurityGuardrails> {
  return request<SecurityGuardrails>("/api/security/guardrails");
}

export async function updateSecurityGuardrails(payload: SecurityGuardrails): Promise<SecurityGuardrails> {
  return request<SecurityGuardrails>("/api/security/guardrails", {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  let response: Response;
  try {
    response = await fetch(path, {
      ...init,
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        "X-Actor": "admin",
        ...init.headers
      }
    });
  } catch (error) {
    throw new Error(`无法连接后端 API：${error instanceof Error ? error.message : String(error)}`);
  }

  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    const message = payload?.error?.message ?? `请求失败：HTTP ${response.status}`;
    throw new Error(message);
  }
  return payload as T;
}
