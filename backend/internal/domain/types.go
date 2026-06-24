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
	InstanceTerminating  InstanceStatus = "Terminating"
	InstanceTerminated   InstanceStatus = "Terminated"
)

type Instance struct {
	ID                  string         `json:"id"`
	Name                string         `json:"name"`
	Created             string         `json:"created"`
	Shape               string         `json:"shape"`
	Region              string         `json:"region"`
	Compartment         string         `json:"compartment"`
	PrimaryIP           string         `json:"primaryIp"`
	PrivateIP           string         `json:"privateIp"`
	PrimaryIPv6         string         `json:"primaryIpv6"`
	IPv6Addresses       []string       `json:"ipv6Addresses"`
	IPv6Enabled         bool           `json:"ipv6Enabled"`
	OCPUs               int            `json:"ocpus"`
	MemoryGB            int            `json:"memoryGb"`
	BootVolumeGB        int            `json:"bootVolumeGb"`
	BootVolumeVPUsPerGB int            `json:"bootVolumeVpusPerGb"`
	Status              InstanceStatus `json:"status"`
	Protected           bool           `json:"protected"`
	OCIInstanceID       string         `json:"ociInstanceId"`
	ProfileID           string         `json:"profileId"`
	CompartmentID       string         `json:"compartmentId"`
	LastSyncedAt        time.Time      `json:"lastSyncedAt"`
	ReservedIPName      string         `json:"reservedIpName,omitempty"`
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

type AuditLogFilter struct {
	Actor            string `json:"actor"`
	Action           string `json:"action"`
	ResourceType     string `json:"resourceType"`
	ResourceID       string `json:"resourceId"`
	ProfileID        string `json:"profileId"`
	Region           string `json:"region"`
	CompartmentID    string `json:"compartmentId"`
	OCIRequestID     string `json:"ociRequestId"`
	OCIWorkRequestID string `json:"ociWorkRequestId"`
	Status           string `json:"status"`
	Limit            int    `json:"limit"`
}

type InstanceTemplate struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	Version             string            `json:"version"`
	ProfileID           string            `json:"profileId"`
	Region              string            `json:"region"`
	Compartment         string            `json:"compartment"`
	CompartmentID       string            `json:"compartmentId"`
	AvailabilityAD      string            `json:"availabilityAd"`
	ImageID             string            `json:"imageId"`
	ImageName           string            `json:"imageName"`
	Shape               string            `json:"shape"`
	OCPUs               int               `json:"ocpus"`
	MemoryGB            int               `json:"memoryGb"`
	BootVolumeGB        int               `json:"bootVolumeGb"`
	BootVolumeVPUsPerGB int               `json:"bootVolumeVpusPerGb"`
	VCNID               string            `json:"vcnId"`
	SubnetID            string            `json:"subnetId"`
	AssignPublicIP      bool              `json:"assignPublicIp"`
	EnableIPv6          bool              `json:"enableIpv6"`
	ReservedPublicIP    string            `json:"reservedPublicIp"`
	SSHKey              string            `json:"sshKey"`
	CloudInit           string            `json:"cloudInit,omitempty"`
	CloudInitSet        bool              `json:"cloudInitSet"`
	Tags                map[string]string `json:"tags"`
	ConfigFormat        string            `json:"configFormat"`
	ConfigText          string            `json:"configText,omitempty"`
	Status              string            `json:"status"`
	ValidationStatus    string            `json:"validationStatus"`
	ValidationErrorCode string            `json:"validationErrorCode,omitempty"`
	ValidationMessage   string            `json:"validationMessage,omitempty"`
	LastValidatedAt     time.Time         `json:"lastValidatedAt,omitempty"`
	CreatedBy           string            `json:"createdBy"`
	CreatedAt           time.Time         `json:"createdAt"`
	UpdatedAt           time.Time         `json:"updatedAt"`
}

type TemplateFilter struct {
	ProfileID        string `json:"profileId"`
	Region           string `json:"region"`
	CompartmentID    string `json:"compartmentId"`
	Status           string `json:"status"`
	ValidationStatus string `json:"validationStatus"`
	Query            string `json:"q"`
	Limit            int    `json:"limit"`
	IncludeDeleted   bool   `json:"includeDeleted"`
}

type CreateTemplateRequest struct {
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	Version             string            `json:"version"`
	ProfileID           string            `json:"profileId"`
	Region              string            `json:"region"`
	Compartment         string            `json:"compartment"`
	CompartmentID       string            `json:"compartmentId"`
	AvailabilityAD      string            `json:"availabilityAd"`
	ImageID             string            `json:"imageId"`
	ImageName           string            `json:"imageName"`
	Shape               string            `json:"shape"`
	OCPUs               int               `json:"ocpus"`
	MemoryGB            int               `json:"memoryGb"`
	BootVolumeGB        int               `json:"bootVolumeGb"`
	BootVolumeVPUsPerGB int               `json:"bootVolumeVpusPerGb"`
	VCNID               string            `json:"vcnId"`
	SubnetID            string            `json:"subnetId"`
	AssignPublicIP      bool              `json:"assignPublicIp"`
	EnableIPv6          bool              `json:"enableIpv6"`
	ReservedPublicIP    string            `json:"reservedPublicIp"`
	SSHKey              string            `json:"sshKey"`
	CloudInit           string            `json:"cloudInit"`
	Tags                map[string]string `json:"tags"`
	ConfigFormat        string            `json:"configFormat"`
	ConfigText          string            `json:"configText"`
	Status              string            `json:"status"`
}

type UpdateTemplateRequest struct {
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	Version             string            `json:"version"`
	ProfileID           string            `json:"profileId"`
	Region              string            `json:"region"`
	Compartment         string            `json:"compartment"`
	CompartmentID       string            `json:"compartmentId"`
	AvailabilityAD      string            `json:"availabilityAd"`
	ImageID             string            `json:"imageId"`
	ImageName           string            `json:"imageName"`
	Shape               string            `json:"shape"`
	OCPUs               int               `json:"ocpus"`
	MemoryGB            int               `json:"memoryGb"`
	BootVolumeGB        int               `json:"bootVolumeGb"`
	BootVolumeVPUsPerGB int               `json:"bootVolumeVpusPerGb"`
	VCNID               string            `json:"vcnId"`
	SubnetID            string            `json:"subnetId"`
	AssignPublicIP      bool              `json:"assignPublicIp"`
	EnableIPv6          bool              `json:"enableIpv6"`
	ReservedPublicIP    string            `json:"reservedPublicIp"`
	SSHKey              string            `json:"sshKey"`
	CloudInit           string            `json:"cloudInit"`
	Tags                map[string]string `json:"tags"`
	ConfigFormat        string            `json:"configFormat"`
	ConfigText          string            `json:"configText"`
	Status              string            `json:"status"`
}

type TemplateValidationResult struct {
	Verified         bool      `json:"verified"`
	TemplateID       string    `json:"templateId"`
	ProfileID        string    `json:"profileId"`
	Region           string    `json:"region"`
	CompartmentID    string    `json:"compartmentId"`
	Status           string    `json:"status"`
	ErrorCode        string    `json:"errorCode,omitempty"`
	ErrorMessage     string    `json:"errorMessage,omitempty"`
	RequestIDs       []string  `json:"requestIds,omitempty"`
	LastValidatedAt  time.Time `json:"lastValidatedAt"`
	CheckedFields    []string  `json:"checkedFields"`
	IncompatibleKeys []string  `json:"incompatibleKeys,omitempty"`
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

type BootVolumeUsage struct {
	Verified                bool      `json:"verified"`
	Region                  string    `json:"region,omitempty"`
	TotalGB                 int       `json:"totalGb"`
	BootVolumeCount         int       `json:"bootVolumeCount"`
	CompartmentCount        int       `json:"compartmentCount"`
	AvailabilityDomainCount int       `json:"availabilityDomainCount"`
	RequestIDs              []string  `json:"requestIds,omitempty"`
	ErrorCode               string    `json:"errorCode,omitempty"`
	ErrorMessage            string    `json:"errorMessage,omitempty"`
	LastSyncedAt            time.Time `json:"lastSyncedAt,omitempty"`
}

type LaunchOptions struct {
	Verified         bool                      `json:"verified"`
	ProfileID        string                    `json:"profileId,omitempty"`
	Region           string                    `json:"region,omitempty"`
	CompartmentID    string                    `json:"compartmentId,omitempty"`
	CacheState       string                    `json:"cacheState,omitempty"`
	CacheCheckedAt   time.Time                 `json:"cacheCheckedAt,omitempty"`
	CacheChangedAt   time.Time                 `json:"cacheChangedAt,omitempty"`
	ShapeFingerprint string                    `json:"shapeFingerprint,omitempty"`
	RequestIDs       []string                  `json:"requestIds,omitempty"`
	ErrorCode        string                    `json:"errorCode,omitempty"`
	ErrorMessage     string                    `json:"errorMessage,omitempty"`
	LastSyncedAt     time.Time                 `json:"lastSyncedAt,omitempty"`
	Profiles         []Profile                 `json:"profiles"`
	Templates        []InstanceTemplate        `json:"templates"`
	Regions          []LaunchOption            `json:"regions"`
	Compartments     []LaunchOption            `json:"compartments"`
	AvailabilityADs  []LaunchOption            `json:"availabilityAds"`
	Images           []LaunchOption            `json:"images"`
	Shapes           []ShapeOption             `json:"shapes"`
	ShapeImages      map[string][]LaunchOption `json:"shapeImages"`
	VCNs             []LaunchOption            `json:"vcns"`
	Subnets          []LaunchOption            `json:"subnets"`
	ReservedIPs      []LaunchOption            `json:"reservedIps"`
	BootVolumeUsage  BootVolumeUsage           `json:"bootVolumeUsage"`
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
	Action                    InstanceLifecycleAction `json:"action"`
	Graceful                  bool                    `json:"graceful"`
	PreserveBootVolume        bool                    `json:"preserveBootVolume"`
	TargetShape               string                  `json:"targetShape"`
	TargetOCPUs               int                     `json:"targetOcpus"`
	TargetMemoryGB            int                     `json:"targetMemoryGb"`
	TargetBootVolumeGB        int                     `json:"targetBootVolumeGb"`
	TargetBootVolumeVPUsPerGB int                     `json:"targetBootVolumeVpusPerGb"`
	ExpandBootVolume          bool                    `json:"expandBootVolume"`
	SnapshotBefore            bool                    `json:"snapshotBefore"`
	Note                      string                  `json:"note"`
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
	Mode                     string `json:"mode"`
	ReservedPublicIP         string `json:"reservedPublicIp"`
	DNSLabel                 string `json:"dnsLabel"`
	VNICID                   string `json:"vnicId"`
	Note                     string `json:"note"`
	EnableIPv6               bool   `json:"enableIpv6"`
	DisableIPv6              bool   `json:"disableIpv6"`
	AutoConfigureIPv6        bool   `json:"autoConfigureIpv6"`
	IPv6Strategy             string `json:"ipv6Strategy"`
	NetworkChangeMode        string `json:"networkChangeMode"`
	RouteTableMode           string `json:"routeTableMode"`
	SecurityMode             string `json:"securityMode"`
	AllowIrreversibleVCNIPv6 bool   `json:"allowIrreversibleVcnIpv6"`
	AllowPublicIPv4Change    bool   `json:"allowPublicIpv4Change"`
	OpenSSHIPv6              bool   `json:"openSshIpv6"`
	OpenHTTPIPv6             bool   `json:"openHttpIpv6"`
	OpenHTTPSIPv6            bool   `json:"openHttpsIpv6"`
	SnapshotBefore           bool   `json:"snapshotBefore"`
}

type RebootInstanceRequest struct {
	Graceful bool   `json:"graceful"`
	Note     string `json:"note"`
}

type InstanceReinstallRequest struct {
	ProfileID              string `json:"profileId"`
	Region                 string `json:"region"`
	CompartmentID          string `json:"compartmentId"`
	ImageID                string `json:"imageId"`
	ImageName              string `json:"imageName"`
	BootVolumeSizeGB       int    `json:"bootVolumeSizeGb"`
	BootVolumeVPUsPerGB    int    `json:"bootVolumeVpusPerGb"`
	PreserveOldBootVolume  bool   `json:"preserveOldBootVolume"`
	CreateBootVolumeBackup bool   `json:"createBootVolumeBackup"`
	GenerateRootPassword   bool   `json:"generateRootPassword"`
	NotifyPasswordInApp    bool   `json:"notifyPasswordInApp"`
	NotifyPasswordByEmail  bool   `json:"notifyPasswordByEmail"`
	SSHAuthorizedKey       string `json:"sshAuthorizedKey,omitempty"`
	CloudInit              string `json:"cloudInit,omitempty"`
	ConfirmationName       string `json:"confirmationName"`
	Note                   string `json:"note"`
}

type CreateInstanceRequest struct {
	Name                 string            `json:"name"`
	ProfileID            string            `json:"profileId"`
	Region               string            `json:"region"`
	Compartment          string            `json:"compartment"`
	CompartmentID        string            `json:"compartmentId"`
	AvailabilityAD       string            `json:"availabilityAd"`
	TemplateID           string            `json:"templateId"`
	ImageID              string            `json:"imageId"`
	Shape                string            `json:"shape"`
	OCPUs                int               `json:"ocpus"`
	MemoryGB             int               `json:"memoryGb"`
	BootVolumeGB         int               `json:"bootVolumeGb"`
	BootVolumeVPUsPerGB  int               `json:"bootVolumeVpusPerGb"`
	AssignPublicIP       bool              `json:"assignPublicIp"`
	EnableIPv6           bool              `json:"enableIpv6"`
	ReservedPublicIP     string            `json:"reservedPublicIp"`
	VCNID                string            `json:"vcnId"`
	SubnetID             string            `json:"subnetId"`
	SSHKey               string            `json:"sshKey"`
	CloudInit            string            `json:"cloudInit"`
	Tags                 map[string]string `json:"tags"`
	MaxRetries           int               `json:"maxRetries"`
	RetryMode            string            `json:"retryMode"`
	RetryMaxAttempts     int               `json:"retryMaxAttempts"`
	RetryDelayMinSec     int               `json:"retryDelayMinSeconds"`
	RetryDelayMaxSec     int               `json:"retryDelayMaxSeconds"`
	RequireApproval      bool              `json:"requireApproval"`
	SnapshotBefore       bool              `json:"snapshotBefore"`
	GenerateRootPassword bool              `json:"generateRootPassword"`
	NotifyRootPassword   bool              `json:"notifyRootPassword"`
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

type NotificationSeverity string

const (
	NotificationInfo    NotificationSeverity = "info"
	NotificationSuccess NotificationSeverity = "success"
	NotificationWarning NotificationSeverity = "warning"
	NotificationError   NotificationSeverity = "error"
)

type Notification struct {
	ID             string               `json:"id"`
	Title          string               `json:"title"`
	Message        string               `json:"message"`
	Severity       NotificationSeverity `json:"severity"`
	Category       string               `json:"category"`
	ResourceType   string               `json:"resourceType,omitempty"`
	ResourceID     string               `json:"resourceId,omitempty"`
	ProfileID      string               `json:"profileId,omitempty"`
	Region         string               `json:"region,omitempty"`
	CompartmentID  string               `json:"compartmentId,omitempty"`
	Sensitive      bool                 `json:"sensitive"`
	Read           bool                 `json:"read"`
	EmailRequested bool                 `json:"emailRequested"`
	EmailSent      bool                 `json:"emailSent"`
	EmailError     string               `json:"emailError,omitempty"`
	WebhookSent    bool                 `json:"webhookSent"`
	WebhookError   string               `json:"webhookError,omitempty"`
	CreatedBy      string               `json:"createdBy"`
	CreatedAt      time.Time            `json:"createdAt"`
	ReadAt         *time.Time           `json:"readAt,omitempty"`
}

type NotificationRequest struct {
	Title          string               `json:"title"`
	Message        string               `json:"message"`
	Severity       NotificationSeverity `json:"severity"`
	Category       string               `json:"category"`
	ResourceType   string               `json:"resourceType"`
	ResourceID     string               `json:"resourceId"`
	ProfileID      string               `json:"profileId"`
	Region         string               `json:"region"`
	CompartmentID  string               `json:"compartmentId"`
	Sensitive      bool                 `json:"sensitive"`
	EmailRequested bool                 `json:"emailRequested"`
}

type EmailSettings struct {
	Enabled     bool     `json:"enabled"`
	Host        string   `json:"host"`
	Port        int      `json:"port"`
	Username    string   `json:"username"`
	Password    string   `json:"password,omitempty"`
	PasswordSet bool     `json:"passwordSet"`
	From        string   `json:"from"`
	To          []string `json:"to"`
	UseTLS      bool     `json:"useTls"`
	StartTLS    bool     `json:"startTls"`
}

type WebhookSettings struct {
	Enabled   bool              `json:"enabled"`
	URL       string            `json:"url"`
	Secret    string            `json:"secret,omitempty"`
	SecretSet bool              `json:"secretSet"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type EmailTestRequest struct {
	To string `json:"to"`
}

type EmailTestResult struct {
	Verified bool   `json:"verified"`
	Message  string `json:"message"`
}

type WebhookTestResult struct {
	Verified bool   `json:"verified"`
	Message  string `json:"message"`
}

type AccountSettings struct {
	DisplayName   string    `json:"displayName"`
	Email         string    `json:"email"`
	Avatar        string    `json:"avatar"`
	AvatarInitial string    `json:"avatarInitial"`
	PasswordHash  string    `json:"-"`
	PasswordSet   bool      `json:"passwordSet"`
	UpdatedAt     time.Time `json:"updatedAt,omitempty"`
}

type AccountProfileRequest struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Avatar      string `json:"avatar"`
}

type AccountPasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type AppearanceSettings struct {
	Theme           string    `json:"theme"`
	BackgroundMode  string    `json:"backgroundMode"`
	BackgroundImage string    `json:"backgroundImage"`
	Language        string    `json:"language"`
	UpdatedAt       time.Time `json:"updatedAt,omitempty"`
}

type BudgetSettings struct {
	Enabled           bool      `json:"enabled"`
	MonthlyBudgetUSD  float64   `json:"monthlyBudgetUsd"`
	ActualSpendUSD    float64   `json:"actualSpendUsd"`
	ForecastSpendUSD  float64   `json:"forecastSpendUsd"`
	ThresholdPercent  float64   `json:"thresholdPercent"`
	ScopeMode         string    `json:"scopeMode"`
	ProfileID         string    `json:"profileId"`
	Region            string    `json:"region"`
	CompartmentID     string    `json:"compartmentId"`
	ResourcePool      string    `json:"resourcePool"`
	TagKey            string    `json:"tagKey"`
	TagValue          string    `json:"tagValue"`
	ManualInstanceIDs []string  `json:"manualInstanceIds"`
	ActionMode        string    `json:"actionMode"`
	DowngradePreset   string    `json:"downgradePreset"`
	DeleteBootVolume  bool      `json:"deleteBootVolume"`
	RequireApproval   bool      `json:"requireApproval"`
	UpdatedAt         time.Time `json:"updatedAt,omitempty"`
}

type AccessUser struct {
	ID                  string    `json:"id"`
	DisplayName         string    `json:"displayName"`
	Email               string    `json:"email"`
	RoleID              string    `json:"roleId"`
	Status              string    `json:"status"`
	AllowedProfiles     []string  `json:"allowedProfiles"`
	AllowedRegions      []string  `json:"allowedRegions"`
	AllowedCompartments []string  `json:"allowedCompartments"`
	PasswordHash        string    `json:"passwordHash,omitempty"`
	PasswordSet         bool      `json:"passwordSet"`
	LastLoginAt         time.Time `json:"lastLoginAt,omitempty"`
	UpdatedAt           time.Time `json:"updatedAt,omitempty"`
}

type AccessRole struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	System      bool     `json:"system"`
}

type AccessControlSettings struct {
	Enabled   bool         `json:"enabled"`
	Users     []AccessUser `json:"users"`
	Roles     []AccessRole `json:"roles"`
	UpdatedAt time.Time    `json:"updatedAt,omitempty"`
}

type AccessControlUpdateRequest struct {
	Enabled bool         `json:"enabled"`
	Users   []AccessUser `json:"users"`
}

type AccessPasswordRequest struct {
	UserID      string `json:"userId"`
	NewPassword string `json:"newPassword"`
}

type SecurityGuardrailSettings struct {
	Enabled                       bool      `json:"enabled"`
	AllowedRegions                []string  `json:"allowedRegions"`
	DeniedRegions                 []string  `json:"deniedRegions"`
	MaxOCPUsPerInstance           int       `json:"maxOcpusPerInstance"`
	MaxMemoryGBPerInstance        int       `json:"maxMemoryGbPerInstance"`
	MaxBootVolumeGB               int       `json:"maxBootVolumeGb"`
	MaxRetryAttempts              int       `json:"maxRetryAttempts"`
	MaxPublicIPBatchCount         int       `json:"maxPublicIpBatchCount"`
	RequireApprovalForTerminate   bool      `json:"requireApprovalForTerminate"`
	BlockBootVolumeDeletion       bool      `json:"blockBootVolumeDeletion"`
	BlockPublicIPv6RouteChanges   bool      `json:"blockPublicIpv6RouteChanges"`
	BlockRootPasswordWithoutEmail bool      `json:"blockRootPasswordWithoutEmail"`
	RequireTemplateForLaunch      bool      `json:"requireTemplateForLaunch"`
	UpdatedAt                     time.Time `json:"updatedAt,omitempty"`
}

type NetworkInventoryRequest struct {
	ProfileID     string `json:"profileId"`
	Region        string `json:"region"`
	CompartmentID string `json:"compartmentId"`
	VCNID         string `json:"vcnId"`
}

type PublicIPResource struct {
	ID               string    `json:"id"`
	DisplayName      string    `json:"displayName"`
	IPAddress        string    `json:"ipAddress"`
	Lifetime         string    `json:"lifetime"`
	Scope            string    `json:"scope"`
	LifecycleState   string    `json:"lifecycleState"`
	AssignedEntityID string    `json:"assignedEntityId"`
	CompartmentID    string    `json:"compartmentId"`
	Region           string    `json:"region"`
	TimeCreated      time.Time `json:"timeCreated,omitempty"`
}

type PublicIPBatchTaskRequest struct {
	Action        string   `json:"action"`
	ProfileID     string   `json:"profileId"`
	Region        string   `json:"region"`
	CompartmentID string   `json:"compartmentId"`
	Count         int      `json:"count"`
	DisplayPrefix string   `json:"displayPrefix"`
	PublicIPIDs   []string `json:"publicIpIds"`
	Note          string   `json:"note"`
}

type PrivateIPResource struct {
	ID             string    `json:"id"`
	DisplayName    string    `json:"displayName"`
	IPAddress      string    `json:"ipAddress"`
	HostnameLabel  string    `json:"hostnameLabel"`
	VNICID         string    `json:"vnicId"`
	SubnetID       string    `json:"subnetId"`
	CompartmentID  string    `json:"compartmentId"`
	LifecycleState string    `json:"lifecycleState"`
	TimeCreated    time.Time `json:"timeCreated,omitempty"`
}

type IPv6Resource struct {
	ID             string    `json:"id"`
	DisplayName    string    `json:"displayName"`
	IPAddress      string    `json:"ipAddress"`
	VNICID         string    `json:"vnicId"`
	SubnetID       string    `json:"subnetId"`
	CompartmentID  string    `json:"compartmentId"`
	LifecycleState string    `json:"lifecycleState"`
	TimeCreated    time.Time `json:"timeCreated,omitempty"`
}

type VCNResource struct {
	ID             string   `json:"id"`
	DisplayName    string   `json:"displayName"`
	CIDRBlock      string   `json:"cidrBlock"`
	IPv6CIDRBlocks []string `json:"ipv6CidrBlocks"`
	LifecycleState string   `json:"lifecycleState"`
	CompartmentID  string   `json:"compartmentId"`
}

type SubnetResource struct {
	ID             string   `json:"id"`
	DisplayName    string   `json:"displayName"`
	VCNID          string   `json:"vcnId"`
	CIDRBlock      string   `json:"cidrBlock"`
	IPv6CIDRBlocks []string `json:"ipv6CidrBlocks"`
	Public         bool     `json:"public"`
	CompartmentID  string   `json:"compartmentId"`
	LifecycleState string   `json:"lifecycleState"`
}

type NetworkInventory struct {
	Verified      bool                `json:"verified"`
	ExecutionMode string              `json:"executionMode"`
	ProfileID     string              `json:"profileId,omitempty"`
	Region        string              `json:"region,omitempty"`
	CompartmentID string              `json:"compartmentId,omitempty"`
	ErrorCode     string              `json:"errorCode,omitempty"`
	ErrorMessage  string              `json:"errorMessage,omitempty"`
	RequestIDs    []string            `json:"requestIds,omitempty"`
	LastSyncedAt  time.Time           `json:"lastSyncedAt,omitempty"`
	PublicIPs     []PublicIPResource  `json:"publicIps"`
	PrivateIPs    []PrivateIPResource `json:"privateIps"`
	IPv6s         []IPv6Resource      `json:"ipv6s"`
	VCNs          []VCNResource       `json:"vcns"`
	Subnets       []SubnetResource    `json:"subnets"`
}
