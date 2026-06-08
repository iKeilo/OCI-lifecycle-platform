package domain

import "time"

type Profile struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	TenancyOCID   string    `json:"tenancyOcid"`
	UserOCID      string    `json:"userOcid"`
	Fingerprint   string    `json:"fingerprint"`
	DefaultRegion string    `json:"defaultRegion"`
	Status        string    `json:"status"`
	LastCheckedAt time.Time `json:"lastCheckedAt"`
}

type CreateProfileRequest struct {
	Name           string `json:"name"`
	TenancyOCID    string `json:"tenancyOcid"`
	UserOCID       string `json:"userOcid"`
	Fingerprint    string `json:"fingerprint"`
	DefaultRegion  string `json:"defaultRegion"`
	PrivateKey     string `json:"privateKey"`
	PrivateKeyFile string `json:"privateKeyFile"`
}

type ProfileTestRequest struct {
	ProfileID     string `json:"profileId"`
	Region        string `json:"region"`
	CompartmentID string `json:"compartmentId"`
}

type ProfileSecret struct {
	PrivateKey     string
	PrivateKeyFile string
}

type InstanceStatus string

const (
	InstanceRunning      InstanceStatus = "Running"
	InstanceStopped      InstanceStatus = "Stopped"
	InstanceProvisioning InstanceStatus = "Provisioning"
	InstanceTerminated   InstanceStatus = "Terminated"
)

type Instance struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Created        string         `json:"created"`
	Shape          string         `json:"shape"`
	Region         string         `json:"region"`
	Compartment    string         `json:"compartment"`
	PrimaryIP      string         `json:"primaryIp"`
	PrivateIP      string         `json:"privateIp"`
	OCPUs          int            `json:"ocpus"`
	MemoryGB       int            `json:"memoryGb"`
	BootVolumeGB   int            `json:"bootVolumeGb"`
	Status         InstanceStatus `json:"status"`
	Protected      bool           `json:"protected"`
	OCIInstanceID  string         `json:"ociInstanceId"`
	ProfileID      string         `json:"profileId"`
	CompartmentID  string         `json:"compartmentId"`
	LastSyncedAt   time.Time      `json:"lastSyncedAt"`
	ReservedIPName string         `json:"reservedIpName,omitempty"`
}

type JobStatus string

const (
	JobPending        JobStatus = "PENDING"
	JobRunning        JobStatus = "RUNNING"
	JobWaitingOCI     JobStatus = "WAITING_OCI"
	JobVerifying      JobStatus = "VERIFYING"
	JobSuccess        JobStatus = "SUCCESS"
	JobFailed         JobStatus = "FAILED"
	JobRetrying       JobStatus = "RETRYING"
	JobCancelled      JobStatus = "CANCELLED"
	JobRollbackNeeded JobStatus = "ROLLBACK_REQUIRED"
	JobManualNeeded   JobStatus = "MANUAL_REQUIRED"
)

type Job struct {
	ID               string         `json:"id"`
	Type             string         `json:"type"`
	Status           JobStatus      `json:"status"`
	ProfileID        string         `json:"profileId"`
	Region           string         `json:"region"`
	CompartmentID    string         `json:"compartmentId"`
	ResourceType     string         `json:"resourceType"`
	ResourceID       string         `json:"resourceId"`
	OCIRequestID     string         `json:"ociRequestId"`
	OCIWorkRequestID string         `json:"ociWorkRequestId"`
	Input            map[string]any `json:"input"`
	Result           map[string]any `json:"result,omitempty"`
	ErrorCode        string         `json:"errorCode,omitempty"`
	ErrorMessage     string         `json:"errorMessage,omitempty"`
	RetryCount       int            `json:"retryCount"`
	MaxRetries       int            `json:"maxRetries"`
	CreatedBy        string         `json:"createdBy"`
	CreatedAt        time.Time      `json:"createdAt"`
	StartedAt        *time.Time     `json:"startedAt,omitempty"`
	FinishedAt       *time.Time     `json:"finishedAt,omitempty"`
}

type AuditLog struct {
	ID               int64          `json:"id"`
	Actor            string         `json:"actor"`
	Action           string         `json:"action"`
	ResourceType     string         `json:"resourceType"`
	ResourceID       string         `json:"resourceId"`
	ProfileID        string         `json:"profileId"`
	Region           string         `json:"region"`
	CompartmentID    string         `json:"compartmentId"`
	OCIRequestID     string         `json:"ociRequestId"`
	OCIWorkRequestID string         `json:"ociWorkRequestId"`
	RequestPayload   map[string]any `json:"requestPayload,omitempty"`
	ResultPayload    map[string]any `json:"resultPayload,omitempty"`
	ErrorCode        string         `json:"errorCode,omitempty"`
	ErrorMessage     string         `json:"errorMessage,omitempty"`
	CreatedAt        time.Time      `json:"createdAt"`
}

type InstanceTemplate struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Version        string            `json:"version"`
	ProfileID      string            `json:"profileId"`
	Region         string            `json:"region"`
	Compartment    string            `json:"compartment"`
	ImageID        string            `json:"imageId"`
	ImageName      string            `json:"imageName"`
	Shape          string            `json:"shape"`
	OCPUs          int               `json:"ocpus"`
	MemoryGB       int               `json:"memoryGb"`
	BootVolumeGB   int               `json:"bootVolumeGb"`
	VCNID          string            `json:"vcnId"`
	SubnetID       string            `json:"subnetId"`
	AssignPublicIP bool              `json:"assignPublicIp"`
	Tags           map[string]string `json:"tags"`
	Status         string            `json:"status"`
	CreatedBy      string            `json:"createdBy"`
	CreatedAt      time.Time         `json:"createdAt"`
}

type LaunchOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Region      string `json:"region,omitempty"`
	Compartment string `json:"compartment,omitempty"`
	Public      bool   `json:"public,omitempty"`
	IPv6Enabled bool   `json:"ipv6Enabled,omitempty"`
}

type ShapeOption struct {
	Name        string `json:"name"`
	Arch        string `json:"arch"`
	MinOCPUs    int    `json:"minOcpus"`
	MaxOCPUs    int    `json:"maxOcpus"`
	MinMemoryGB int    `json:"minMemoryGb"`
	MaxMemoryGB int    `json:"maxMemoryGb"`
}

type LaunchOptions struct {
	Verified        bool               `json:"verified"`
	ProfileID       string             `json:"profileId,omitempty"`
	Region          string             `json:"region,omitempty"`
	CompartmentID   string             `json:"compartmentId,omitempty"`
	RequestIDs      []string           `json:"requestIds,omitempty"`
	ErrorCode       string             `json:"errorCode,omitempty"`
	ErrorMessage    string             `json:"errorMessage,omitempty"`
	LastSyncedAt    time.Time          `json:"lastSyncedAt,omitempty"`
	Profiles        []Profile          `json:"profiles"`
	Templates       []InstanceTemplate `json:"templates"`
	Regions         []LaunchOption     `json:"regions"`
	Compartments    []LaunchOption     `json:"compartments"`
	AvailabilityADs []LaunchOption     `json:"availabilityAds"`
	Images          []LaunchOption     `json:"images"`
	Shapes          []ShapeOption      `json:"shapes"`
	VCNs            []LaunchOption     `json:"vcns"`
	Subnets         []LaunchOption     `json:"subnets"`
	ReservedIPs     []LaunchOption     `json:"reservedIps"`
}

type InstanceLifecycleAction string

const (
	InstanceActionStart     InstanceLifecycleAction = "START"
	InstanceActionStop      InstanceLifecycleAction = "STOP"
	InstanceActionReboot    InstanceLifecycleAction = "REBOOT"
	InstanceActionTerminate InstanceLifecycleAction = "TERMINATE"
	InstanceActionResize    InstanceLifecycleAction = "RESIZE"
)

type InstanceActionRequest struct {
	Action             InstanceLifecycleAction `json:"action"`
	Graceful           bool                    `json:"graceful"`
	PreserveBootVolume bool                    `json:"preserveBootVolume"`
	TargetShape        string                  `json:"targetShape"`
	TargetOCPUs        int                     `json:"targetOcpus"`
	TargetMemoryGB     int                     `json:"targetMemoryGb"`
	SnapshotBefore     bool                    `json:"snapshotBefore"`
	Note               string                  `json:"note"`
}

type AutomationRule struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Type             string     `json:"type"`
	TargetPool       string     `json:"targetPool"`
	Action           string     `json:"action"`
	TriggerInterval  string     `json:"triggerInterval"`
	Cooldown         string     `json:"cooldown"`
	MaxRetries       int        `json:"maxRetries"`
	FailurePolicy    string     `json:"failurePolicy"`
	MaxInstances     int        `json:"maxInstances"`
	MaxDailyRuns     int        `json:"maxDailyRuns"`
	RegionScope      string     `json:"regionScope"`
	NotifyChannel    string     `json:"notifyChannel"`
	Enabled          bool       `json:"enabled"`
	ApprovalRequired bool       `json:"approvalRequired"`
	LastRunAt        *time.Time `json:"lastRunAt,omitempty"`
	NextRunAt        *time.Time `json:"nextRunAt,omitempty"`
	CreatedBy        string     `json:"createdBy"`
	CreatedAt        time.Time  `json:"createdAt"`
}

type IPTaskRequest struct {
	Mode             string `json:"mode"`
	ReservedPublicIP string `json:"reservedPublicIp"`
	DNSLabel         string `json:"dnsLabel"`
	VNICID           string `json:"vnicId"`
	Note             string `json:"note"`
	EnableIPv6       bool   `json:"enableIpv6"`
	SnapshotBefore   bool   `json:"snapshotBefore"`
}

type RebootInstanceRequest struct {
	Graceful bool   `json:"graceful"`
	Note     string `json:"note"`
}

type CreateInstanceRequest struct {
	Name             string            `json:"name"`
	ProfileID        string            `json:"profileId"`
	Region           string            `json:"region"`
	Compartment      string            `json:"compartment"`
	CompartmentID    string            `json:"compartmentId"`
	AvailabilityAD   string            `json:"availabilityAd"`
	TemplateID       string            `json:"templateId"`
	ImageID          string            `json:"imageId"`
	Shape            string            `json:"shape"`
	OCPUs            int               `json:"ocpus"`
	MemoryGB         int               `json:"memoryGb"`
	BootVolumeGB     int               `json:"bootVolumeGb"`
	AssignPublicIP   bool              `json:"assignPublicIp"`
	EnableIPv6       bool              `json:"enableIpv6"`
	ReservedPublicIP string            `json:"reservedPublicIp"`
	VCNID            string            `json:"vcnId"`
	SubnetID         string            `json:"subnetId"`
	SSHKey           string            `json:"sshKey"`
	CloudInit        string            `json:"cloudInit"`
	Tags             map[string]string `json:"tags"`
	MaxRetries       int               `json:"maxRetries"`
	RequireApproval  bool              `json:"requireApproval"`
	SnapshotBefore   bool              `json:"snapshotBefore"`
}

type CreateInstanceResponse struct {
	Instance Instance `json:"instance"`
	Job      Job      `json:"job"`
}

type AutomationTaskRequest struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	TargetPool        string `json:"targetPool"`
	Action            string `json:"action"`
	TriggerInterval   string `json:"triggerInterval"`
	Cooldown          string `json:"cooldown"`
	MaxRetries        int    `json:"maxRetries"`
	FailurePolicy     string `json:"failurePolicy"`
	MaxInstances      int    `json:"maxInstances"`
	MaxDailyRuns      int    `json:"maxDailyRuns"`
	RegionScope       string `json:"regionScope"`
	NotifyChannel     string `json:"notifyChannel"`
	EnableImmediately bool   `json:"enableImmediately"`
	ApprovalRequired  bool   `json:"approvalRequired"`
}

type AutomationTaskResponse struct {
	Rule AutomationRule `json:"rule"`
	Job  Job            `json:"job"`
}
