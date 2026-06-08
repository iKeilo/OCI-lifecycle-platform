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
  ocpus: number;
  memoryGb: number;
  bootVolumeGb: number;
  status: "Running" | "Stopped" | "Provisioning" | "Terminated";
  protected: boolean;
  ociInstanceId: string;
  profileId: string;
  compartmentId: string;
  lastSyncedAt: string;
};

export type InstanceTemplate = {
  id: string;
  name: string;
  version: string;
  profileId: string;
  region: string;
  compartment: string;
  imageId: string;
  imageName: string;
  shape: string;
  ocpus: number;
  memoryGb: number;
  bootVolumeGb: number;
  vcnId: string;
  subnetId: string;
  assignPublicIp: boolean;
  tags: Record<string, string>;
  status: string;
  createdBy: string;
  createdAt: string;
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

export type LaunchOptions = {
  verified?: boolean;
  profileId?: string;
  region?: string;
  compartmentId?: string;
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
  vcns: LaunchOption[];
  subnets: LaunchOption[];
  reservedIps: LaunchOption[];
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
  profileId: string;
  region: string;
  compartment: string;
  compartmentId: string;
  availabilityAd: string;
  templateId: string;
  imageId: string;
  shape: string;
  ocpus: number;
  memoryGb: number;
  bootVolumeGb: number;
  assignPublicIp: boolean;
  enableIpv6: boolean;
  reservedPublicIp: string;
  vcnId: string;
  subnetId: string;
  sshKey: string;
  cloudInit: string;
  tags: Record<string, string>;
  maxRetries: number;
  requireApproval: boolean;
  snapshotBefore: boolean;
};

export type CreateInstanceResponse = {
  instance: Instance;
  job: Job;
};

export type AuthStatus = {
  authEnabled: boolean;
  authenticated: boolean;
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
  snapshotBefore: boolean;
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

export async function createInstanceTask(payload: CreateInstancePayload): Promise<CreateInstanceResponse | Job> {
  return request<CreateInstanceResponse | Job>("/api/instances", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function listTemplates(): Promise<InstanceTemplate[]> {
  const response = await request<ListResponse<InstanceTemplate>>("/api/templates");
  return response.items;
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

export async function getOCIReadiness(): Promise<OCIReadiness> {
  return request<OCIReadiness>("/api/oci/readiness");
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

export async function listInstances(status?: string): Promise<Instance[]> {
  const query = status ? `?status=${encodeURIComponent(status)}` : "";
  const response = await request<ListResponse<Instance>>(`/api/instances${query}`);
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

export async function login(password: string): Promise<AuthStatus> {
  return request<AuthStatus>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({ password })
  });
}

export async function logout(): Promise<AuthStatus> {
  return request<AuthStatus>("/api/auth/logout", {
    method: "POST",
    body: JSON.stringify({})
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
