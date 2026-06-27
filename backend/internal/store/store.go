package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"a-series-oracle/backend/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation failed")
	ErrConflict   = errors.New("conflict")
)

type Store struct {
	mu                 sync.RWMutex
	now                func() time.Time
	sink               PersistenceSink
	nextInst           int
	nextJob            int
	nextRule           int
	nextNotice         int
	nextAudit          int64
	profiles           map[string]domain.Profile
	instances          map[string]domain.Instance
	templates          map[string]domain.InstanceTemplate
	jobs               map[string]domain.Job
	automations        map[string]domain.AutomationRule
	notifications      map[string]domain.Notification
	auditLogs          []domain.AuditLog
	emailSettings      domain.EmailSettings
	webhookSettings    domain.WebhookSettings
	accountSettings    domain.AccountSettings
	appearanceSettings domain.AppearanceSettings
	budgetSettings     domain.BudgetSettings
	accessSettings     domain.AccessControlSettings
	guardrailSettings  domain.SecurityGuardrailSettings
}

type PersistenceSink interface {
	SaveProfile(profile domain.Profile, secret domain.ProfileSecret) error
	SaveJob(job domain.Job) error
	SaveInstance(instance domain.Instance) error
	RecordAudit(entry domain.AuditLog) error
}

type jobDeleteSink interface {
	DeleteJobs(jobIDs []string) error
}

type profileSecretReader interface {
	GetProfileSecret(profileID string) (domain.ProfileSecret, error)
}

type profileDeleteSink interface {
	DeleteProfile(profileID string) error
}

type auditLogReader interface {
	ListAuditLogs(filter domain.AuditLogFilter) ([]domain.AuditLog, error)
}

type settingsSink interface {
	SaveEmailSettings(settings domain.EmailSettings) error
	SaveWebhookSettings(settings domain.WebhookSettings) error
	SaveAccountSettings(settings domain.AccountSettings) error
	SaveAppearanceSettings(settings domain.AppearanceSettings) error
	SaveBudgetSettings(settings domain.BudgetSettings) error
	SaveAccessControlSettings(settings domain.AccessControlSettings) error
	SaveSecurityGuardrailSettings(settings domain.SecurityGuardrailSettings) error
}

type settingsReader interface {
	GetEmailSettings() (domain.EmailSettings, error)
	GetWebhookSettings() (domain.WebhookSettings, error)
	GetAccountSettings() (domain.AccountSettings, error)
	GetAppearanceSettings() (domain.AppearanceSettings, error)
	GetBudgetSettings() (domain.BudgetSettings, error)
	GetAccessControlSettings() (domain.AccessControlSettings, error)
	GetSecurityGuardrailSettings() (domain.SecurityGuardrailSettings, error)
}

type notificationSink interface {
	SaveNotification(notification domain.Notification) error
	DeleteNotification(notificationID string) error
}

type notificationReader interface {
	ListNotifications() ([]domain.Notification, error)
}

type templateSink interface {
	SaveTemplate(template domain.InstanceTemplate) error
	DeleteTemplate(templateID string) error
}

type templateReader interface {
	ListTemplates() ([]domain.InstanceTemplate, error)
}

func New() *Store {
	return &Store{
		now:           time.Now,
		nextInst:      1,
		nextJob:       1,
		nextRule:      1,
		nextNotice:    1,
		nextAudit:     1,
		profiles:      map[string]domain.Profile{},
		instances:     map[string]domain.Instance{},
		templates:     map[string]domain.InstanceTemplate{},
		jobs:          map[string]domain.Job{},
		automations:   map[string]domain.AutomationRule{},
		notifications: map[string]domain.Notification{},
		accountSettings: domain.AccountSettings{
			DisplayName:   "Administrator",
			AvatarInitial: "A",
		},
		appearanceSettings: domain.AppearanceSettings{
			Theme:          "light",
			BackgroundMode: "aurora",
			Language:       "zh-CN",
		},
		budgetSettings: domain.BudgetSettings{
			MonthlyBudgetUSD: 10,
			ActualSpendUSD:   0,
			ForecastSpendUSD: 0,
			ThresholdPercent: 90,
			ScopeMode:        "tag",
			ProfileID:        "DEFAULT",
			Region:           "ap-chuncheon-1",
			CompartmentID:    "Root tenancy",
			TagKey:           "budget.autoAction",
			TagValue:         "enabled",
			ActionMode:       "downgrade",
			DowngradePreset:  "free-first",
			RequireApproval:  true,
		},
		accessSettings:    defaultAccessControlSettings(time.Now().UTC()),
		guardrailSettings: defaultSecurityGuardrails(time.Now().UTC()),
	}
}

func NewSeeded() *Store {
	now := time.Now().UTC()
	s := New()
	s.nextInst = 4
	s.nextJob = 1043
	s.nextRule = 3

	s.profiles["profile-default"] = domain.Profile{
		ID:            "profile-default",
		Name:          "DEFAULT",
		TenancyOCID:   "ocid1.tenancy.oc1..placeholder",
		UserOCID:      "ocid1.user.oc1..admin",
		Fingerprint:   "7d:33:8c:4b:11:98",
		DefaultRegion: "ap-singapore-1",
		Status:        "Healthy",
		LastCheckedAt: now.Add(-2 * time.Minute),
	}
	s.profiles["profile-capacity-lab"] = domain.Profile{
		ID:            "profile-capacity-lab",
		Name:          "capacity-lab",
		TenancyOCID:   "ocid1.tenancy.oc1..lab",
		UserOCID:      "ocid1.user.oc1..automation",
		Fingerprint:   "19:aa:02:b8:73:21",
		DefaultRegion: "ap-seoul-1",
		Status:        "Limited",
		LastCheckedAt: now.Add(-21 * time.Minute),
	}

	for _, instance := range []domain.Instance{
		{
			ID:            "inst-prod-web-01",
			Name:          "prod-web-server-01",
			Created:       "创建于 2 天前",
			Shape:         "VM.Standard.A1.Flex",
			Region:        "ap-singapore-1",
			Compartment:   "production",
			PrimaryIP:     "152.69.228.112",
			PrivateIP:     "10.0.0.42",
			OCPUs:         1,
			MemoryGB:      6,
			BootVolumeGB:  100,
			Status:        domain.InstanceRunning,
			Protected:     true,
			OCIInstanceID: "ocid1.instance.oc1.ap-singapore-1.placeholder001",
			ProfileID:     "profile-default",
			CompartmentID: "ocid1.compartment.oc1..production",
			LastSyncedAt:  now.Add(-30 * time.Second),
		},
		{
			ID:            "inst-dev-db",
			Name:          "dev-database-node",
			Created:       "创建于 1 周前",
			Shape:         "VM.Standard.E2.1.Micro",
			Region:        "ap-singapore-1",
			Compartment:   "development",
			PrimaryIP:     "168.110.108.114",
			PrivateIP:     "10.0.1.25",
			OCPUs:         1,
			MemoryGB:      1,
			BootVolumeGB:  47,
			Status:        domain.InstanceStopped,
			OCIInstanceID: "ocid1.instance.oc1.ap-singapore-1.placeholder002",
			ProfileID:     "profile-default",
			CompartmentID: "ocid1.compartment.oc1..development",
			LastSyncedAt:  now.Add(-90 * time.Second),
		},
		{
			ID:            "inst-api-gateway-02",
			Name:          "api-gateway-02",
			Created:       "创建于 1 个月前",
			Shape:         "VM.Standard3.Flex",
			Region:        "ap-seoul-1",
			Compartment:   "edge",
			PrimaryIP:     "130.61.88.2",
			PrivateIP:     "10.0.4.88",
			OCPUs:         2,
			MemoryGB:      16,
			BootVolumeGB:  80,
			Status:        domain.InstanceRunning,
			OCIInstanceID: "ocid1.instance.oc1.ap-seoul-1.placeholder003",
			ProfileID:     "profile-capacity-lab",
			CompartmentID: "ocid1.compartment.oc1..edge",
			LastSyncedAt:  now.Add(-45 * time.Second),
		},
	} {
		s.instances[instance.ID] = instance
	}

	s.automations["auto-a1-capacity"] = domain.AutomationRule{
		ID:              "auto-a1-capacity",
		Name:            "A1 容量自动创建",
		Type:            "容量重试",
		TargetPool:      "a1-free-pool",
		Action:          "创建 1 台实例",
		TriggerInterval: "每 5 分钟",
		Cooldown:        "30 分钟",
		MaxRetries:      10,
		FailurePolicy:   "达到上限后暂停并通知",
		MaxInstances:    4,
		MaxDailyRuns:    24,
		RegionScope:     "仅当前区域",
		NotifyChannel:   "邮件 + Webhook",
		Enabled:         true,
		CreatedBy:       "system",
		CreatedAt:       now.Add(-24 * time.Hour),
	}
	s.automations["auto-dev-nightly-stop"] = domain.AutomationRule{
		ID:              "auto-dev-nightly-stop",
		Name:            "开发环境夜间关机",
		Type:            "定时规则",
		TargetPool:      "development",
		Action:          "停止实例",
		TriggerInterval: "每天 23:30",
		Cooldown:        "10 分钟",
		MaxRetries:      3,
		FailurePolicy:   "仅通知管理员",
		MaxInstances:    20,
		MaxDailyRuns:    1,
		RegionScope:     "仅当前区域",
		NotifyChannel:   "邮件",
		Enabled:         false,
		CreatedBy:       "admin",
		CreatedAt:       now.Add(-72 * time.Hour),
	}

	for _, job := range []domain.Job{
		s.seedJob("JOB-1042", "创建实例", domain.JobSuccess, "prod-web-server-01", "inst-prod-web-01", now.Add(-10*time.Minute)),
		s.seedJob("JOB-1041", "变更实例规格", domain.JobWaitingOCI, "dev-database-node", "inst-dev-db", now.Add(-18*time.Minute)),
		s.seedJob("JOB-1040", "容量重试", domain.JobFailed, "a1-free-pool", "auto-a1-capacity", now.Add(-time.Hour)),
	} {
		s.jobs[job.ID] = job
	}

	return s
}

func (s *Store) SetPersistenceSink(sink PersistenceSink) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sink = sink
}

func (s *Store) ReplaceProfiles(profiles []domain.Profile) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := make(map[string]domain.Profile, len(profiles))
	for _, profile := range profiles {
		if strings.TrimSpace(profile.ID) == "" {
			continue
		}
		next[profile.ID] = profile
	}
	s.profiles = next
}

func (s *Store) ReplaceInstances(instances []domain.Instance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := make(map[string]domain.Instance, len(instances))
	for _, instance := range instances {
		if strings.TrimSpace(instance.ID) == "" {
			continue
		}
		next[instance.ID] = instance
	}
	s.instances = next
}

func (s *Store) ReplaceJobs(jobs []domain.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := make(map[string]domain.Job, len(jobs))
	for _, job := range jobs {
		if strings.TrimSpace(job.ID) == "" {
			continue
		}
		next[job.ID] = job
	}
	s.jobs = next
}

func (s *Store) ReplaceTemplates(templates []domain.InstanceTemplate) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := make(map[string]domain.InstanceTemplate, len(templates))
	for _, template := range templates {
		template = normalizeTemplate(template, s.now().UTC())
		if strings.TrimSpace(template.ID) == "" {
			continue
		}
		next[template.ID] = template
	}
	s.templates = next
}

func (s *Store) ReplaceAuditLogs(entries []domain.AuditLog) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.auditLogs = append([]domain.AuditLog(nil), entries...)
	var maxID int64
	for _, entry := range entries {
		if entry.ID > maxID {
			maxID = entry.ID
		}
	}
	s.nextAudit = maxID + 1
	if s.nextAudit < 1 {
		s.nextAudit = 1
	}
}

func (s *Store) ReplaceNotifications(notifications []domain.Notification) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := make(map[string]domain.Notification, len(notifications))
	maxID := 0
	for _, notification := range notifications {
		if strings.TrimSpace(notification.ID) == "" {
			continue
		}
		next[notification.ID] = notification
		if suffix := numericIDSuffix(notification.ID, "notice-"); suffix > maxID {
			maxID = suffix
		}
	}
	s.notifications = next
	s.nextNotice = maxID + 1
	if s.nextNotice < 1 {
		s.nextNotice = 1
	}
}

func (s *Store) RecoverRunnableJobs() ([]domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var runnable []domain.Job
	for _, job := range s.jobs {
		switch job.Status {
		case domain.JobPending, domain.JobRetrying:
			runnable = append(runnable, job)
		case domain.JobRunning, domain.JobWaitingOCI, domain.JobVerifying:
			previousStatus := job.Status
			job.Status = domain.JobRetrying
			job.StartedAt = nil
			job.FinishedAt = nil
			job.ErrorCode = ""
			job.ErrorMessage = ""
			job.Input = cloneMap(job.Input)
			job.Input["recoveredFromStatus"] = string(previousStatus)
			job.Input["recoveredAt"] = s.now().UTC().Format(time.RFC3339)
			if _, err := s.saveJobLocked(job); err != nil {
				return nil, err
			}
			runnable = append(runnable, job)
		}
	}
	sort.Slice(runnable, func(i, j int) bool { return runnable[i].CreatedAt.Before(runnable[j].CreatedAt) })
	return runnable, nil
}

func (s *Store) SyncInstances(instances []domain.Instance) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, instance := range instances {
		if strings.TrimSpace(instance.ID) == "" {
			continue
		}
		s.instances[instance.ID] = instance
		if err := s.saveInstanceLocked(instance); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateProfile(req domain.CreateProfileRequest, actor string) (domain.Profile, error) {
	if strings.TrimSpace(req.Name) == "" {
		return domain.Profile{}, fmt.Errorf("%w: profile name is required", ErrValidation)
	}
	if strings.TrimSpace(req.TenancyOCID) == "" {
		return domain.Profile{}, fmt.Errorf("%w: tenancyOcid is required", ErrValidation)
	}
	if strings.TrimSpace(req.UserOCID) == "" {
		return domain.Profile{}, fmt.Errorf("%w: userOcid is required", ErrValidation)
	}
	if strings.TrimSpace(req.Fingerprint) == "" {
		return domain.Profile{}, fmt.Errorf("%w: fingerprint is required", ErrValidation)
	}
	if strings.TrimSpace(req.DefaultRegion) == "" {
		return domain.Profile{}, fmt.Errorf("%w: defaultRegion is required", ErrValidation)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	profile := domain.Profile{
		ID:            s.nextProfileIDLocked(req.Name),
		Name:          strings.TrimSpace(req.Name),
		TenancyOCID:   strings.TrimSpace(req.TenancyOCID),
		UserOCID:      strings.TrimSpace(req.UserOCID),
		Fingerprint:   strings.TrimSpace(req.Fingerprint),
		DefaultRegion: strings.TrimSpace(req.DefaultRegion),
		Status:        "Pending",
		LastCheckedAt: now,
	}
	secret := domain.ProfileSecret{
		PrivateKey:     req.PrivateKey,
		PrivateKeyFile: strings.TrimSpace(req.PrivateKeyFile),
	}
	if err := s.saveProfileLocked(profile, secret); err != nil {
		return domain.Profile{}, err
	}
	_ = actor
	return profile, nil
}

func (s *Store) seedJob(id, typ string, status domain.JobStatus, resourceName, resourceID string, createdAt time.Time) domain.Job {
	return domain.Job{
		ID:               id,
		Type:             typ,
		Status:           status,
		ProfileID:        "profile-default",
		Region:           "ap-singapore-1",
		CompartmentID:    "ocid1.compartment.oc1..production",
		ResourceType:     "seed",
		ResourceID:       resourceID,
		OCIRequestID:     "req-" + strings.ToLower(id),
		OCIWorkRequestID: "wr-" + strings.TrimPrefix(id, "JOB-"),
		Input:            map[string]any{"resourceName": resourceName},
		RetryCount:       0,
		MaxRetries:       3,
		CreatedBy:        "admin",
		CreatedAt:        createdAt,
	}
}

func (s *Store) ListProfiles() []domain.Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	profiles := make([]domain.Profile, 0, len(s.profiles))
	for _, profile := range s.profiles {
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles
}

func (s *Store) GetProfile(id string) (domain.Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.profileByIDOrNameLocked(id)
}

func (s *Store) SetProfileStatus(id string, status string, checkedAt time.Time) (domain.Profile, error) {
	if strings.TrimSpace(status) == "" {
		return domain.Profile{}, fmt.Errorf("%w: status is required", ErrValidation)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	profile, ok := s.profileByIDOrNameLocked(id)
	if !ok {
		return domain.Profile{}, ErrNotFound
	}
	profile.Status = strings.TrimSpace(status)
	if checkedAt.IsZero() {
		checkedAt = s.now().UTC()
	}
	profile.LastCheckedAt = checkedAt.UTC()
	if err := s.saveProfileLocked(profile, domain.ProfileSecret{}); err != nil {
		return domain.Profile{}, err
	}
	return profile, nil
}

func (s *Store) DeleteProfile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	profile, ok := s.profileByIDOrNameLocked(id)
	if !ok {
		return ErrNotFound
	}
	if sink, ok := s.sink.(profileDeleteSink); ok {
		if err := sink.DeleteProfile(profile.ID); err != nil {
			return err
		}
	}
	delete(s.profiles, profile.ID)
	return nil
}

func (s *Store) GetProfileSecret(id string) (domain.ProfileSecret, error) {
	s.mu.RLock()
	profile, ok := s.profileByIDOrNameLocked(id)
	sink := s.sink
	s.mu.RUnlock()
	if !ok {
		return domain.ProfileSecret{}, ErrNotFound
	}
	reader, ok := sink.(profileSecretReader)
	if !ok {
		return domain.ProfileSecret{}, ErrNotFound
	}
	return reader.GetProfileSecret(profile.ID)
}

func (s *Store) ListInstances(status string) []domain.Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	instances := make([]domain.Instance, 0, len(s.instances))
	for _, instance := range s.instances {
		if status != "" && !strings.EqualFold(string(instance.Status), status) {
			continue
		}
		instances = append(instances, instance)
	}
	sort.Slice(instances, func(i, j int) bool { return instances[i].Name < instances[j].Name })
	return instances
}

func (s *Store) GetInstance(id string) (domain.Instance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	instance, ok := s.instances[id]
	return instance, ok
}

func (s *Store) ListTemplates() []domain.InstanceTemplate {
	return s.listTemplatesFiltered(domain.TemplateFilter{})
}

func (s *Store) ListTemplatesFiltered(filter domain.TemplateFilter) []domain.InstanceTemplate {
	return s.listTemplatesFiltered(filter)
}

func (s *Store) listTemplatesFiltered(filter domain.TemplateFilter) []domain.InstanceTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	templates := make([]domain.InstanceTemplate, 0, len(s.templates))
	for _, template := range s.templates {
		if !filter.IncludeDeleted && strings.EqualFold(template.Status, "deleted") {
			continue
		}
		if strings.TrimSpace(filter.ProfileID) != "" && !strings.EqualFold(template.ProfileID, filter.ProfileID) {
			continue
		}
		if strings.TrimSpace(filter.Region) != "" && !strings.EqualFold(template.Region, filter.Region) {
			continue
		}
		if strings.TrimSpace(filter.CompartmentID) != "" && !strings.EqualFold(template.CompartmentID, filter.CompartmentID) {
			continue
		}
		if strings.TrimSpace(filter.Status) != "" && !strings.EqualFold(template.Status, filter.Status) {
			continue
		}
		if strings.TrimSpace(filter.ValidationStatus) != "" && !strings.EqualFold(template.ValidationStatus, filter.ValidationStatus) {
			continue
		}
		if strings.TrimSpace(filter.Query) != "" {
			query := strings.ToLower(strings.TrimSpace(filter.Query))
			haystack := strings.ToLower(strings.Join([]string{
				template.ID,
				template.Name,
				template.Description,
				template.Version,
				template.Region,
				template.Compartment,
				template.CompartmentID,
				template.ImageName,
				template.ImageID,
				template.Shape,
			}, " "))
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		templates = append(templates, template)
	}
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].Region == templates[j].Region {
			return templates[i].Name < templates[j].Name
		}
		return templates[i].Region < templates[j].Region
	})
	if filter.Limit > 0 && filter.Limit < len(templates) {
		templates = templates[:filter.Limit]
	}
	return templates
}

func (s *Store) GetTemplate(id string) (domain.InstanceTemplate, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	template, ok := s.templates[id]
	return template, ok
}

func (s *Store) CreateTemplate(req domain.CreateTemplateRequest, actor string) (domain.InstanceTemplate, error) {
	template, err := templateFromCreateRequest(req)
	if err != nil {
		return domain.InstanceTemplate{}, err
	}
	template.CreatedBy = defaultString(actor, "admin")
	template.CreatedAt = s.now().UTC()
	template.UpdatedAt = template.CreatedAt
	template.Status = normalizeTemplateStatus(template.Status)
	template.ValidationStatus = normalizeValidationStatus(template.ValidationStatus)
	s.mu.Lock()
	defer s.mu.Unlock()
	template.ID = s.nextTemplateIDLocked(template.Name)
	s.templates[template.ID] = template
	if err := s.saveTemplateLocked(template); err != nil {
		return domain.InstanceTemplate{}, err
	}
	return template, nil
}

func (s *Store) UpdateTemplate(id string, req domain.UpdateTemplateRequest, actor string) (domain.InstanceTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	template, ok := s.templates[id]
	if !ok {
		return domain.InstanceTemplate{}, ErrNotFound
	}
	updated, err := mergeTemplateUpdate(template, req)
	if err != nil {
		return domain.InstanceTemplate{}, err
	}
	updated.ID = template.ID
	updated.CreatedBy = template.CreatedBy
	updated.CreatedAt = template.CreatedAt
	updated.UpdatedAt = s.now().UTC()
	updated.ValidationStatus = markTemplateStale(updated.ValidationStatus, template)
	s.templates[id] = updated
	if err := s.saveTemplateLocked(updated); err != nil {
		return domain.InstanceTemplate{}, err
	}
	return updated, nil
}

func (s *Store) DeleteTemplate(id string, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	template, ok := s.templates[id]
	if !ok {
		return ErrNotFound
	}
	template.Status = "Deleted"
	template.UpdatedAt = s.now().UTC()
	template.ValidationStatus = "UNVERIFIED"
	s.templates[id] = template
	if err := s.saveTemplateLocked(template); err != nil {
		return err
	}
	_ = actor
	return nil
}

func (s *Store) ValidateTemplate(id string) (domain.TemplateValidationResult, error) {
	s.mu.RLock()
	template, ok := s.templates[id]
	s.mu.RUnlock()
	if !ok {
		return domain.TemplateValidationResult{}, ErrNotFound
	}
	missing := templateMissingFields(template)
	result := domain.TemplateValidationResult{
		TemplateID:      template.ID,
		ProfileID:       template.ProfileID,
		Region:          template.Region,
		CompartmentID:   template.CompartmentID,
		Status:          template.Status,
		LastValidatedAt: s.now().UTC(),
		CheckedFields: []string{
			"profile", "region", "image", "shape", "compute", "bootVolume", "network",
		},
	}
	if len(missing) == 0 {
		result.Verified = true
		result.Status = "VALID"
		template.ValidationStatus = "VALID"
		template.ValidationErrorCode = ""
		template.ValidationMessage = "template fields are complete for form prefill"
	} else {
		result.Verified = false
		result.Status = "INVALID"
		result.ErrorCode = "TEMPLATE_FIELDS_INCOMPLETE"
		result.ErrorMessage = "template is missing required prefill fields: " + strings.Join(missing, ", ")
		result.IncompatibleKeys = missing
		template.ValidationStatus = "INVALID"
		template.ValidationErrorCode = result.ErrorCode
		template.ValidationMessage = result.ErrorMessage
	}
	template.LastValidatedAt = result.LastValidatedAt
	s.mu.Lock()
	s.templates[id] = template
	_ = s.saveTemplateLocked(template)
	s.mu.Unlock()
	return result, nil
}

func (s *Store) GetLaunchOptions() domain.LaunchOptions {
	profiles := s.ListProfiles()
	templates := s.ListTemplates()
	return domain.LaunchOptions{
		Profiles:        profiles,
		Templates:       templates,
		Regions:         launchRegions(profiles, templates),
		Compartments:    launchCompartments(templates),
		AvailabilityADs: []domain.LaunchOption{},
		Images:          []domain.LaunchOption{},
		Shapes:          launchShapes(templates),
		VCNs:            []domain.LaunchOption{},
		Subnets:         []domain.LaunchOption{},
		ReservedIPs:     []domain.LaunchOption{},
	}
}

func (s *Store) ListJobs() []domain.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]domain.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	return jobs
}

func (s *Store) ClearCompletedJobs(actor string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	jobIDs := make([]string, 0)
	for id, job := range s.jobs {
		if isCompletedJobStatus(job.Status) {
			jobIDs = append(jobIDs, id)
		}
	}
	if len(jobIDs) == 0 {
		return 0, nil
	}
	sort.Strings(jobIDs)
	if sink, ok := s.sink.(jobDeleteSink); ok {
		if err := sink.DeleteJobs(jobIDs); err != nil {
			return 0, err
		}
	}
	for _, id := range jobIDs {
		delete(s.jobs, id)
	}
	_ = s.recordAuditLocked(domain.AuditLog{
		Actor:        defaultString(actor, "system"),
		Action:       "jobs.clear_completed",
		ResourceType: "job",
		RequestPayload: map[string]any{
			"jobIds": jobIDs,
			"count":  len(jobIDs),
		},
		ResultPayload: map[string]any{
			"deletedCount": len(jobIDs),
		},
	})
	return len(jobIDs), nil
}

func (s *Store) ListAuditLogs(filter domain.AuditLogFilter) ([]domain.AuditLog, error) {
	s.mu.RLock()
	reader, useReader := s.sink.(auditLogReader)
	entries := append([]domain.AuditLog(nil), s.auditLogs...)
	s.mu.RUnlock()

	if useReader {
		return reader.ListAuditLogs(filter)
	}
	return filterAuditLogs(entries, filter), nil
}

func (s *Store) ListNotifications(unreadOnly bool) []domain.Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Notification, 0, len(s.notifications))
	for _, notification := range s.notifications {
		if unreadOnly && notification.Read {
			continue
		}
		items = append(items, notification)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items
}

func (s *Store) CreateNotification(req domain.NotificationRequest, actor string) (domain.Notification, error) {
	if strings.TrimSpace(req.Title) == "" {
		return domain.Notification{}, fmt.Errorf("%w: notification title is required", ErrValidation)
	}
	if strings.TrimSpace(req.Message) == "" {
		return domain.Notification{}, fmt.Errorf("%w: notification message is required", ErrValidation)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	severity := req.Severity
	if severity == "" {
		severity = domain.NotificationInfo
	}
	notification := domain.Notification{
		ID:             s.nextNotificationIDLocked(),
		Title:          strings.TrimSpace(req.Title),
		Message:        strings.TrimSpace(req.Message),
		Severity:       severity,
		Category:       defaultString(req.Category, "system"),
		ResourceType:   strings.TrimSpace(req.ResourceType),
		ResourceID:     strings.TrimSpace(req.ResourceID),
		ProfileID:      strings.TrimSpace(req.ProfileID),
		Region:         strings.TrimSpace(req.Region),
		CompartmentID:  strings.TrimSpace(req.CompartmentID),
		Sensitive:      req.Sensitive,
		EmailRequested: req.EmailRequested,
		CreatedBy:      defaultString(actor, "system"),
		CreatedAt:      now,
	}
	s.notifications[notification.ID] = notification
	if sink, ok := s.sink.(notificationSink); ok {
		if err := sink.SaveNotification(notification); err != nil {
			return domain.Notification{}, err
		}
	}
	return notification, nil
}

func (s *Store) UpdateNotificationEmailStatus(id string, sent bool, message string) (domain.Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	notification, ok := s.notifications[id]
	if !ok {
		return domain.Notification{}, ErrNotFound
	}
	notification.EmailSent = sent
	notification.EmailError = ""
	if !sent {
		notification.EmailError = strings.TrimSpace(message)
	}
	s.notifications[id] = notification
	if sink, ok := s.sink.(notificationSink); ok {
		if err := sink.SaveNotification(notification); err != nil {
			return domain.Notification{}, err
		}
	}
	return notification, nil
}

func (s *Store) UpdateNotificationWebhookStatus(id string, sent bool, message string) (domain.Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	notification, ok := s.notifications[id]
	if !ok {
		return domain.Notification{}, ErrNotFound
	}
	notification.WebhookSent = sent
	notification.WebhookError = ""
	if !sent {
		notification.WebhookError = strings.TrimSpace(message)
	}
	s.notifications[id] = notification
	if sink, ok := s.sink.(notificationSink); ok {
		if err := sink.SaveNotification(notification); err != nil {
			return domain.Notification{}, err
		}
	}
	return notification, nil
}

func (s *Store) MarkNotificationRead(id string) (domain.Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	notification, ok := s.notifications[id]
	if !ok {
		return domain.Notification{}, ErrNotFound
	}
	if !notification.Read {
		now := s.now().UTC()
		notification.Read = true
		notification.ReadAt = &now
		s.notifications[id] = notification
		if sink, ok := s.sink.(notificationSink); ok {
			if err := sink.SaveNotification(notification); err != nil {
				return domain.Notification{}, err
			}
		}
	}
	return notification, nil
}

func (s *Store) MarkAllNotificationsRead() []domain.Notification {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	items := make([]domain.Notification, 0, len(s.notifications))
	for id, notification := range s.notifications {
		if !notification.Read {
			notification.Read = true
			notification.ReadAt = &now
			s.notifications[id] = notification
			if sink, ok := s.sink.(notificationSink); ok {
				_ = sink.SaveNotification(notification)
			}
		}
		items = append(items, notification)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items
}

func (s *Store) DeleteNotification(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.notifications[id]; !ok {
		return ErrNotFound
	}
	delete(s.notifications, id)
	if sink, ok := s.sink.(notificationSink); ok {
		if err := sink.DeleteNotification(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetEmailSettings() domain.EmailSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return redactEmailSettings(s.emailSettings)
}

func (s *Store) GetEmailSettingsForSend() domain.EmailSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.emailSettings
}

func (s *Store) SetEmailSettings(settings domain.EmailSettings) (domain.EmailSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if settings.Port == 0 {
		settings.Port = 587
	}
	if strings.TrimSpace(settings.Password) == "" && s.emailSettings.PasswordSet {
		settings.Password = s.emailSettings.Password
		settings.PasswordSet = true
	} else if strings.TrimSpace(settings.Password) != "" {
		settings.PasswordSet = true
	}
	settings.Host = strings.TrimSpace(settings.Host)
	settings.Username = strings.TrimSpace(settings.Username)
	settings.From = strings.TrimSpace(settings.From)
	settings.To = cleanRecipients(settings.To)
	s.emailSettings = settings
	if sink, ok := s.sink.(settingsSink); ok {
		if err := sink.SaveEmailSettings(s.emailSettings); err != nil {
			return domain.EmailSettings{}, err
		}
	}
	return redactEmailSettings(s.emailSettings), nil
}

func (s *Store) GetWebhookSettings() domain.WebhookSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return redactWebhookSettings(s.webhookSettings)
}

func (s *Store) GetWebhookSettingsForSend() domain.WebhookSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.webhookSettings
}

func (s *Store) SetWebhookSettings(settings domain.WebhookSettings) (domain.WebhookSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings.URL = strings.TrimSpace(settings.URL)
	if settings.Headers == nil {
		settings.Headers = map[string]string{}
	}
	if strings.TrimSpace(settings.Secret) == "" && s.webhookSettings.SecretSet {
		settings.Secret = s.webhookSettings.Secret
		settings.SecretSet = true
	} else if strings.TrimSpace(settings.Secret) != "" {
		settings.SecretSet = true
	}
	s.webhookSettings = settings
	if sink, ok := s.sink.(settingsSink); ok {
		if err := sink.SaveWebhookSettings(s.webhookSettings); err != nil {
			return domain.WebhookSettings{}, err
		}
	}
	return redactWebhookSettings(s.webhookSettings), nil
}

func (s *Store) GetAccountSettings() domain.AccountSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return redactAccountSettings(normalizeAccountSettings(s.accountSettings))
}

func (s *Store) GetAccountSettingsForAuth() domain.AccountSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return normalizeAccountSettings(s.accountSettings)
}

func (s *Store) SetAccountProfile(req domain.AccountProfileRequest) (domain.AccountSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings := normalizeAccountSettings(s.accountSettings)
	if strings.TrimSpace(req.DisplayName) != "" {
		settings.DisplayName = strings.TrimSpace(req.DisplayName)
	}
	settings.Email = strings.TrimSpace(req.Email)
	settings.Avatar = strings.TrimSpace(req.Avatar)
	settings.AvatarInitial = accountInitial(settings.DisplayName, settings.Email)
	settings.UpdatedAt = s.now().UTC()
	s.accountSettings = settings
	if sink, ok := s.sink.(settingsSink); ok {
		if err := sink.SaveAccountSettings(s.accountSettings); err != nil {
			return domain.AccountSettings{}, err
		}
	}
	_ = s.recordAuditLocked(domain.AuditLog{
		Actor:          "admin",
		Action:         "account.profile.updated",
		ResourceType:   "account",
		ResourceID:     "panel",
		RequestPayload: map[string]any{"displayName": settings.DisplayName, "email": settings.Email, "avatarSet": settings.Avatar != ""},
		CreatedAt:      s.now().UTC(),
	})
	return redactAccountSettings(s.accountSettings), nil
}

func (s *Store) SetAccountPasswordHash(hash string) (domain.AccountSettings, error) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return domain.AccountSettings{}, fmt.Errorf("%w: password hash is required", ErrValidation)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	settings := normalizeAccountSettings(s.accountSettings)
	settings.PasswordHash = hash
	settings.PasswordSet = true
	settings.UpdatedAt = s.now().UTC()
	s.accountSettings = settings
	if sink, ok := s.sink.(settingsSink); ok {
		if err := sink.SaveAccountSettings(s.accountSettings); err != nil {
			return domain.AccountSettings{}, err
		}
	}
	_ = s.recordAuditLocked(domain.AuditLog{
		Actor:        "admin",
		Action:       "account.password.updated",
		ResourceType: "account",
		ResourceID:   "panel",
		CreatedAt:    s.now().UTC(),
	})
	return redactAccountSettings(s.accountSettings), nil
}

func (s *Store) GetAppearanceSettings() domain.AppearanceSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return normalizeAppearanceSettings(s.appearanceSettings)
}

func (s *Store) SetAppearanceSettings(settings domain.AppearanceSettings) (domain.AppearanceSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := normalizeAppearanceSettings(settings)
	next.UpdatedAt = s.now().UTC()
	s.appearanceSettings = next
	if sink, ok := s.sink.(settingsSink); ok {
		if err := sink.SaveAppearanceSettings(s.appearanceSettings); err != nil {
			return domain.AppearanceSettings{}, err
		}
	}
	_ = s.recordAuditLocked(domain.AuditLog{
		Actor:        "admin",
		Action:       "settings.appearance.updated",
		ResourceType: "settings",
		ResourceID:   "appearance",
		RequestPayload: map[string]any{
			"theme":          next.Theme,
			"backgroundMode": next.BackgroundMode,
			"backgroundSet":  next.BackgroundImage != "",
		},
		CreatedAt: s.now().UTC(),
	})
	return s.appearanceSettings, nil
}

func (s *Store) GetBudgetSettings() domain.BudgetSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return normalizeBudgetSettings(s.budgetSettings, s.now())
}

func (s *Store) SetBudgetSettings(settings domain.BudgetSettings) (domain.BudgetSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := normalizeBudgetSettings(settings, s.now())
	next.UpdatedAt = s.now().UTC()
	s.budgetSettings = next
	if sink, ok := s.sink.(settingsSink); ok {
		if err := sink.SaveBudgetSettings(s.budgetSettings); err != nil {
			return domain.BudgetSettings{}, err
		}
	}
	_ = s.recordAuditLocked(domain.AuditLog{
		Actor:        "admin",
		Action:       "settings.budget.updated",
		ResourceType: "settings",
		ResourceID:   "budget",
		RequestPayload: map[string]any{
			"enabled":           next.Enabled,
			"monthlyBudgetUsd":  next.MonthlyBudgetUSD,
			"thresholdPercent":  next.ThresholdPercent,
			"scopeMode":         next.ScopeMode,
			"manualInstanceIds": len(next.ManualInstanceIDs),
			"actionMode":        next.ActionMode,
			"requireApproval":   next.RequireApproval,
		},
		CreatedAt: s.now().UTC(),
	})
	return s.budgetSettings, nil
}

func (s *Store) GetAccessControlSettings() domain.AccessControlSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return redactAccessSettings(normalizeAccessControlSettings(s.accessSettings, s.now().UTC()))
}

func (s *Store) SetAccessControlSettings(settings domain.AccessControlSettings, actor string) (domain.AccessControlSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentPasswords := map[string]string{}
	for _, user := range s.accessSettings.Users {
		currentPasswords[user.ID] = user.PasswordHash
	}
	next := normalizeAccessControlSettings(settings, s.now().UTC())
	for index := range next.Users {
		next.Users[index].PasswordHash = currentPasswords[next.Users[index].ID]
		next.Users[index].PasswordSet = strings.TrimSpace(next.Users[index].PasswordHash) != ""
	}
	next.UpdatedAt = s.now().UTC()
	s.accessSettings = next
	if sink, ok := s.sink.(settingsSink); ok {
		if err := sink.SaveAccessControlSettings(s.accessSettings); err != nil {
			return domain.AccessControlSettings{}, err
		}
	}
	_ = s.recordAuditLocked(domain.AuditLog{
		Actor:        defaultString(actor, "admin"),
		Action:       "settings.access.updated",
		ResourceType: "settings",
		ResourceID:   "access",
		RequestPayload: map[string]any{
			"enabled": next.Enabled,
			"users":   len(next.Users),
		},
		CreatedAt: s.now().UTC(),
	})
	return redactAccessSettings(s.accessSettings), nil
}

func (s *Store) SetAccessUserPassword(userID, password, actor string) (domain.AccessControlSettings, error) {
	userID = sanitizeIdentifier(userID)
	if userID == "" {
		return domain.AccessControlSettings{}, fmt.Errorf("%w: userId is required", ErrValidation)
	}
	password = strings.TrimSpace(password)
	if len(password) < 8 {
		return domain.AccessControlSettings{}, fmt.Errorf("%w: password must be at least 8 characters", ErrValidation)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.AccessControlSettings{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := normalizeAccessControlSettings(s.accessSettings, s.now().UTC())
	found := false
	for index := range next.Users {
		if next.Users[index].ID == userID {
			next.Users[index].PasswordHash = string(hash)
			next.Users[index].PasswordSet = true
			next.Users[index].UpdatedAt = s.now().UTC()
			found = true
			break
		}
	}
	if !found {
		return domain.AccessControlSettings{}, ErrNotFound
	}
	next.UpdatedAt = s.now().UTC()
	s.accessSettings = next
	if sink, ok := s.sink.(settingsSink); ok {
		if err := sink.SaveAccessControlSettings(s.accessSettings); err != nil {
			return domain.AccessControlSettings{}, err
		}
	}
	_ = s.recordAuditLocked(domain.AuditLog{
		Actor:        defaultString(actor, "admin"),
		Action:       "access.user.password.updated",
		ResourceType: "user",
		ResourceID:   userID,
		CreatedAt:    s.now().UTC(),
	})
	return redactAccessSettings(s.accessSettings), nil
}

func (s *Store) VerifyAccessUser(userID, password string) (domain.AccessUser, bool) {
	userID = sanitizeIdentifier(userID)
	if userID == "" {
		return domain.AccessUser{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	settings := normalizeAccessControlSettings(s.accessSettings, s.now().UTC())
	for index, user := range settings.Users {
		if user.ID != userID || !strings.EqualFold(user.Status, "active") || strings.TrimSpace(user.PasswordHash) == "" {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
			return domain.AccessUser{}, false
		}
		now := s.now().UTC()
		settings.Users[index].LastLoginAt = now
		settings.Users[index].PasswordSet = true
		s.accessSettings = settings
		if sink, ok := s.sink.(settingsSink); ok {
			_ = sink.SaveAccessControlSettings(s.accessSettings)
		}
		return redactAccessUser(settings.Users[index]), true
	}
	return domain.AccessUser{}, false
}

func (s *Store) GetAccessUser(userID string) (domain.AccessUser, bool) {
	userID = sanitizeIdentifier(userID)
	if userID == "" {
		userID = "admin"
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	settings := normalizeAccessControlSettings(s.accessSettings, s.now().UTC())
	for _, user := range settings.Users {
		if user.ID == userID {
			return redactAccessUser(user), true
		}
	}
	return domain.AccessUser{}, false
}

func (s *Store) Authorize(userID, permission, profileID, region, compartmentID string) error {
	settings := s.GetAccessControlSettings()
	if !settings.Enabled {
		return nil
	}
	user, ok := s.GetAccessUser(userID)
	if !ok || !strings.EqualFold(user.Status, "active") {
		return fmt.Errorf("%w: user is disabled or not found", ErrConflict)
	}
	role := accessRoleByID(settings.Roles, user.RoleID)
	if role.ID == "" || !permissionAllowed(role.Permissions, permission) {
		return fmt.Errorf("%w: permission %s denied", ErrConflict, permission)
	}
	if !scopeAllowed(user.AllowedProfiles, profileID) || !scopeAllowed(user.AllowedRegions, region) || !scopeAllowed(user.AllowedCompartments, compartmentID) {
		return fmt.Errorf("%w: resource scope denied", ErrConflict)
	}
	return nil
}

func (s *Store) GetSecurityGuardrailSettings() domain.SecurityGuardrailSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return normalizeSecurityGuardrails(s.guardrailSettings, s.now().UTC())
}

func (s *Store) SetSecurityGuardrailSettings(settings domain.SecurityGuardrailSettings, actor string) (domain.SecurityGuardrailSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := normalizeSecurityGuardrails(settings, s.now().UTC())
	next.UpdatedAt = s.now().UTC()
	s.guardrailSettings = next
	if sink, ok := s.sink.(settingsSink); ok {
		if err := sink.SaveSecurityGuardrailSettings(s.guardrailSettings); err != nil {
			return domain.SecurityGuardrailSettings{}, err
		}
	}
	_ = s.recordAuditLocked(domain.AuditLog{
		Actor:        defaultString(actor, "admin"),
		Action:       "settings.guardrails.updated",
		ResourceType: "settings",
		ResourceID:   "guardrails",
		RequestPayload: map[string]any{
			"enabled":                     next.Enabled,
			"maxOcpusPerInstance":         next.MaxOCPUsPerInstance,
			"maxMemoryGbPerInstance":      next.MaxMemoryGBPerInstance,
			"maxBootVolumeGb":             next.MaxBootVolumeGB,
			"maxPublicIpBatchCount":       next.MaxPublicIPBatchCount,
			"blockPublicIpv6RouteChanges": next.BlockPublicIPv6RouteChanges,
		},
		CreatedAt: s.now().UTC(),
	})
	return s.guardrailSettings, nil
}

func (s *Store) LoadPersistedSettings() error {
	s.mu.RLock()
	reader, ok := s.sink.(settingsReader)
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	emailSettings, err := reader.GetEmailSettings()
	if err != nil {
		return err
	}
	webhookSettings, err := reader.GetWebhookSettings()
	if err != nil {
		return err
	}
	accountSettings, err := reader.GetAccountSettings()
	if err != nil {
		return err
	}
	appearanceSettings, err := reader.GetAppearanceSettings()
	if err != nil {
		return err
	}
	budgetSettings, err := reader.GetBudgetSettings()
	if err != nil {
		return err
	}
	accessSettings, err := reader.GetAccessControlSettings()
	if err != nil {
		return err
	}
	guardrailSettings, err := reader.GetSecurityGuardrailSettings()
	if err != nil {
		return err
	}
	s.mu.Lock()
	if emailSettings.Host != "" || emailSettings.Enabled || emailSettings.PasswordSet {
		s.emailSettings = emailSettings
	}
	if webhookSettings.URL != "" || webhookSettings.Enabled || webhookSettings.SecretSet {
		s.webhookSettings = webhookSettings
	}
	if accountSettings.DisplayName != "" || accountSettings.PasswordSet || accountSettings.PasswordHash != "" {
		s.accountSettings = normalizeAccountSettings(accountSettings)
	}
	if appearanceSettings.Theme != "" || appearanceSettings.BackgroundMode != "" || appearanceSettings.BackgroundImage != "" {
		s.appearanceSettings = normalizeAppearanceSettings(appearanceSettings)
	}
	if budgetSettings.MonthlyBudgetUSD > 0 || budgetSettings.Enabled || budgetSettings.ScopeMode != "" {
		s.budgetSettings = normalizeBudgetSettings(budgetSettings, s.now())
	}
	if len(accessSettings.Users) > 0 || len(accessSettings.Roles) > 0 {
		s.accessSettings = normalizeAccessControlSettings(accessSettings, s.now().UTC())
	}
	if guardrailSettings.MaxOCPUsPerInstance > 0 || guardrailSettings.Enabled || len(guardrailSettings.AllowedRegions) > 0 || len(guardrailSettings.DeniedRegions) > 0 {
		s.guardrailSettings = normalizeSecurityGuardrails(guardrailSettings, s.now().UTC())
	}
	s.mu.Unlock()
	return nil
}

func (s *Store) GetJob(id string) (domain.Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	return job, ok
}

func (s *Store) StartJob(id string) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, ErrNotFound
	}
	if job.Status != domain.JobPending && job.Status != domain.JobRetrying {
		return domain.Job{}, fmt.Errorf("%w: job is not pending", ErrConflict)
	}

	now := s.now().UTC()
	job.Status = domain.JobRunning
	job.StartedAt = &now
	job.ErrorCode = ""
	job.ErrorMessage = ""
	return s.saveJobLocked(job)
}

func (s *Store) MarkJobWaitingOCI(id string) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, ErrNotFound
	}
	if !isActiveStatus(job.Status) {
		return domain.Job{}, fmt.Errorf("%w: job is not active", ErrConflict)
	}

	job.Status = domain.JobWaitingOCI
	return s.saveJobLocked(job)
}

func (s *Store) SetJobOCIRefs(id, requestID, workRequestID string) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, ErrNotFound
	}
	if strings.TrimSpace(requestID) != "" {
		job.OCIRequestID = requestID
	}
	if strings.TrimSpace(workRequestID) != "" {
		job.OCIWorkRequestID = workRequestID
	}
	return s.saveJobLocked(job)
}

func (s *Store) MarkJobVerifying(id string) (domain.Job, error) {
	return s.setActiveJobStatus(id, domain.JobVerifying)
}

func (s *Store) CompleteJob(id string, result map[string]any) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, ErrNotFound
	}
	if !isActiveStatus(job.Status) {
		return domain.Job{}, fmt.Errorf("%w: job is not active", ErrConflict)
	}

	now := s.now().UTC()
	job.Status = domain.JobSuccess
	job.Result = result
	job.FinishedAt = &now
	redactSensitiveJobInput(&job)
	if job.ResourceType == "instance" {
		if operation, _ := job.Input["operation"].(string); operation == "launch" {
			instance := instanceFromLaunchJob(job, result, now)
			if instance.ID != "" {
				job.ResourceID = instance.ID
				if err := s.saveInstanceLocked(instance); err != nil {
					return domain.Job{}, err
				}
			}
		} else if operation == "ip-management" {
			if instance, ok := s.instances[job.ResourceID]; ok {
				instance.LastSyncedAt = now
				if err := s.saveInstanceLocked(instance); err != nil {
					return domain.Job{}, err
				}
			}
		} else if operation == "reinstall" {
			if instance, ok := s.instances[job.ResourceID]; ok {
				instance.LastSyncedAt = now
				instance.Status = statusFromOCIState(stringFromMap(result, "finalState"), instance.Status)
				if bootVolumeGB := intFromMap(result, "targetBootVolumeGb"); bootVolumeGB > 0 {
					instance.BootVolumeGB = bootVolumeGB
				} else if bootVolumeGB := intFromMap(job.Input, "bootVolumeSizeGb"); bootVolumeGB > 0 {
					instance.BootVolumeGB = bootVolumeGB
				}
				if vpus := intFromMap(result, "targetBootVolumeVpusPerGb"); vpus > 0 {
					instance.BootVolumeVPUsPerGB = vpus
				} else if vpus := intFromMap(job.Input, "bootVolumeVpusPerGb"); vpus > 0 {
					instance.BootVolumeVPUsPerGB = vpus
				}
				if err := s.saveInstanceLocked(instance); err != nil {
					return domain.Job{}, err
				}
			}
		} else if instance, ok := s.instances[job.ResourceID]; ok {
			instance.LastSyncedAt = now
			instance.OCIInstanceID = defaultString(instance.OCIInstanceID, "ocid1.instance.oc1."+instance.Region+"."+strings.ToLower(strings.ReplaceAll(job.ID, "-", "")))
			if instance.Status == domain.InstanceProvisioning {
				instance.Status = domain.InstanceRunning
				if instance.PrimaryIP == "-" || instance.PrimaryIP == "" {
					instance.PrimaryIP = "无公网 IP"
				}
			}
			if action, ok := job.Input["action"].(string); ok {
				applyInstanceAction(&instance, action, job.Input)
			}
			if err := s.saveInstanceLocked(instance); err != nil {
				return domain.Job{}, err
			}
		} else if instance := instanceFromActionJob(job, result, now); instance.ID != "" {
			if err := s.saveInstanceLocked(instance); err != nil {
				return domain.Job{}, err
			}
		}
	}
	return s.saveJobLocked(job)
}

func (s *Store) FailJob(id, code, message string) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, ErrNotFound
	}

	now := s.now().UTC()
	job.Status = domain.JobFailed
	job.ErrorCode = defaultString(code, "JOB_FAILED")
	job.ErrorMessage = defaultString(message, "job failed")
	job.FinishedAt = &now
	redactSensitiveJobInput(&job)
	return s.saveJobLocked(job)
}

func (s *Store) CancelJob(id string) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, ErrNotFound
	}
	if !isCancelableStatus(job.Status) {
		return domain.Job{}, fmt.Errorf("%w: job cannot be cancelled", ErrConflict)
	}

	now := s.now().UTC()
	job.Status = domain.JobCancelled
	job.ErrorCode = "JOB_CANCELLED"
	job.ErrorMessage = "cancelled by user"
	job.FinishedAt = &now
	redactSensitiveJobInput(&job)
	return s.saveJobLocked(job)
}

func (s *Store) RetryJob(id, actor string) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, ErrNotFound
	}
	if !isRetryableStatus(job.Status) {
		return domain.Job{}, fmt.Errorf("%w: job cannot be retried", ErrConflict)
	}
	if job.RetryCount >= job.MaxRetries {
		return domain.Job{}, fmt.Errorf("%w: retry limit reached", ErrConflict)
	}

	retry := job
	retry.ID = s.nextJobIDLocked()
	retry.Status = domain.JobPending
	retry.OCIRequestID = ""
	retry.OCIWorkRequestID = ""
	retry.Result = nil
	retry.ErrorCode = ""
	retry.ErrorMessage = ""
	retry.RetryCount = job.RetryCount + 1
	retry.CreatedBy = defaultString(actor, job.CreatedBy)
	retry.CreatedAt = s.now().UTC()
	retry.StartedAt = nil
	retry.FinishedAt = nil
	retry.Input = cloneMap(job.Input)
	retry.Input["retryOf"] = job.ID
	return s.saveJobLocked(retry)
}

func (s *Store) ListAutomations() []domain.AutomationRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rules := make([]domain.AutomationRule, 0, len(s.automations))
	for _, rule := range s.automations {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].Name < rules[j].Name })
	return rules
}

func (s *Store) CreateIPTask(instanceID string, req domain.IPTaskRequest, actor string) (domain.Job, error) {
	if strings.TrimSpace(req.Mode) == "" {
		return domain.Job{}, fmt.Errorf("%w: mode is required", ErrValidation)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	instance, ok := s.instances[instanceID]
	if !ok {
		return domain.Job{}, ErrNotFound
	}

	job := s.newJobLocked("IP 管理", "instance", instance.ID, actor)
	job.ProfileID = instance.ProfileID
	job.Region = instance.Region
	job.CompartmentID = instance.CompartmentID
	job.MaxRetries = 2
	job.Input = map[string]any{
		"operation":                "ip-management",
		"mode":                     req.Mode,
		"reservedPublicIp":         req.ReservedPublicIP,
		"dnsLabel":                 req.DNSLabel,
		"vnicId":                   req.VNICID,
		"note":                     req.Note,
		"enableIpv6":               req.EnableIPv6,
		"autoConfigureIpv6":        req.AutoConfigureIPv6,
		"ipv6Strategy":             defaultString(req.IPv6Strategy, "assign_only"),
		"networkChangeMode":        defaultString(req.NetworkChangeMode, req.IPv6Strategy),
		"routeTableMode":           req.RouteTableMode,
		"securityMode":             req.SecurityMode,
		"allowIrreversibleVcnIpv6": req.AllowIrreversibleVCNIPv6,
		"allowPublicIpv4Change":    req.AllowPublicIPv4Change,
		"openSshIpv6":              req.OpenSSHIPv6,
		"openHttpIpv6":             req.OpenHTTPIPv6,
		"openHttpsIpv6":            req.OpenHTTPSIPv6,
		"mayReplacePublicIPv4":     req.AllowPublicIPv4Change || strings.EqualFold(req.NetworkChangeMode, "replace_public_path"),
		"snapshotBefore":           req.SnapshotBefore,
		"instanceName":             instance.Name,
		"currentPublicIp":          instance.PrimaryIP,
		"currentPrivateIp":         instance.PrivateIP,
	}
	return s.saveJobLocked(job)
}

func (s *Store) CreateOCIIPTask(instanceID string, req domain.IPTaskRequest, actor, profileID, region, compartmentID string) (domain.Job, error) {
	if strings.TrimSpace(instanceID) == "" {
		return domain.Job{}, fmt.Errorf("%w: instance OCID is required", ErrValidation)
	}
	if !strings.HasPrefix(instanceID, "ocid1.instance.") {
		return domain.Job{}, fmt.Errorf("%w: instance id must be an OCI instance OCID", ErrValidation)
	}
	if strings.TrimSpace(req.Mode) == "" && !req.EnableIPv6 && !req.DisableIPv6 {
		return domain.Job{}, fmt.Errorf("%w: mode, enableIpv6, or disableIpv6 is required", ErrValidation)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	job := s.newJobLocked("IP 管理", "instance", instanceID, actor)
	profileID = defaultString(profileID, "DEFAULT")
	if profile, ok := s.profileByIDOrNameLocked(profileID); ok {
		profileID = profile.ID
		if strings.TrimSpace(region) == "" {
			region = profile.DefaultRegion
		}
	}
	job.ProfileID = profileID
	job.Region = region
	job.CompartmentID = compartmentID
	job.MaxRetries = 0
	job.Input = map[string]any{
		"operation":                "ip-management",
		"mode":                     req.Mode,
		"reservedPublicIp":         req.ReservedPublicIP,
		"dnsLabel":                 req.DNSLabel,
		"vnicId":                   req.VNICID,
		"note":                     req.Note,
		"enableIpv6":               req.EnableIPv6,
		"disableIpv6":              req.DisableIPv6,
		"autoConfigureIpv6":        req.AutoConfigureIPv6,
		"ipv6Strategy":             defaultString(req.IPv6Strategy, "assign_only"),
		"networkChangeMode":        defaultString(req.NetworkChangeMode, req.IPv6Strategy),
		"routeTableMode":           req.RouteTableMode,
		"securityMode":             req.SecurityMode,
		"allowIrreversibleVcnIpv6": req.AllowIrreversibleVCNIPv6,
		"allowPublicIpv4Change":    req.AllowPublicIPv4Change,
		"openSshIpv6":              req.OpenSSHIPv6,
		"openHttpIpv6":             req.OpenHTTPIPv6,
		"openHttpsIpv6":            req.OpenHTTPSIPv6,
		"mayReplacePublicIPv4":     req.AllowPublicIPv4Change || strings.EqualFold(req.NetworkChangeMode, "replace_public_path"),
		"snapshotBefore":           req.SnapshotBefore,
		"ociInstanceId":            instanceID,
		"executionMode":            "oci",
	}
	return s.saveJobLocked(job)
}

func (s *Store) CreatePublicIPBatchTask(req domain.PublicIPBatchTaskRequest, actor string) (domain.Job, error) {
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action != "create" && action != "delete" {
		return domain.Job{}, fmt.Errorf("%w: action must be create or delete", ErrValidation)
	}
	if action == "create" {
		if req.Count <= 0 {
			return domain.Job{}, fmt.Errorf("%w: count must be greater than zero", ErrValidation)
		}
		if req.Count > 50 {
			return domain.Job{}, fmt.Errorf("%w: count cannot exceed 50 per batch", ErrValidation)
		}
	} else if len(cleanStringList(req.PublicIPIDs)) == 0 {
		return domain.Job{}, fmt.Errorf("%w: publicIpIds is required for delete", ErrValidation)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	profileID := defaultString(req.ProfileID, "DEFAULT")
	region := strings.TrimSpace(req.Region)
	if profile, ok := s.profileByIDOrNameLocked(profileID); ok {
		profileID = profile.ID
		if region == "" {
			region = profile.DefaultRegion
		}
	}
	job := s.newJobLocked("批量公网 IP", "network", "reserved-public-ip", actor)
	job.ProfileID = profileID
	job.Region = region
	job.CompartmentID = strings.TrimSpace(req.CompartmentID)
	job.MaxRetries = 1
	job.Input = map[string]any{
		"operation":     "public-ip-batch",
		"action":        action,
		"count":         req.Count,
		"displayPrefix": strings.TrimSpace(req.DisplayPrefix),
		"publicIpIds":   cleanStringList(req.PublicIPIDs),
		"note":          strings.TrimSpace(req.Note),
	}
	return s.saveJobLocked(job)
}

func (s *Store) CreateRebootTask(instanceID string, req domain.RebootInstanceRequest, actor string) (domain.Job, error) {
	return s.CreateInstanceActionTask(instanceID, domain.InstanceActionRequest{
		Action:         domain.InstanceActionReboot,
		Graceful:       req.Graceful,
		SnapshotBefore: true,
		Note:           req.Note,
	}, actor)
}

func (s *Store) CreateInstanceActionTask(instanceID string, req domain.InstanceActionRequest, actor string) (domain.Job, error) {
	if req.Action == "" {
		return domain.Job{}, fmt.Errorf("%w: action is required", ErrValidation)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	instance, ok := s.instances[instanceID]
	if !ok {
		return domain.Job{}, ErrNotFound
	}
	if instance.Status == domain.InstanceTerminated {
		return domain.Job{}, fmt.Errorf("%w: terminated instance cannot be changed", ErrConflict)
	}
	if req.Action == domain.InstanceActionResize {
		if strings.TrimSpace(req.TargetShape) == "" {
			return domain.Job{}, fmt.Errorf("%w: targetShape is required", ErrValidation)
		}
		targetIsFlexible := isFlexibleShapeName(req.TargetShape)
		if targetIsFlexible && req.TargetOCPUs <= 0 {
			return domain.Job{}, fmt.Errorf("%w: targetOcpus must be greater than zero", ErrValidation)
		}
		if targetIsFlexible && req.TargetMemoryGB <= 0 {
			return domain.Job{}, fmt.Errorf("%w: targetMemoryGb must be greater than zero", ErrValidation)
		}
		if req.TargetBootVolumeGB > 0 && instance.BootVolumeGB > 0 && req.TargetBootVolumeGB < instance.BootVolumeGB {
			return domain.Job{}, fmt.Errorf("%w: boot volume cannot be decreased", ErrValidation)
		}
		if req.ExpandBootVolume {
			if req.TargetBootVolumeGB <= 0 {
				return domain.Job{}, fmt.Errorf("%w: targetBootVolumeGb must be greater than zero", ErrValidation)
			}
			if req.TargetBootVolumeGB < instance.BootVolumeGB {
				return domain.Job{}, fmt.Errorf("%w: boot volume cannot be decreased", ErrValidation)
			}
		}
		if req.TargetBootVolumeVPUsPerGB != 0 && !validBootVolumeVPUs(req.TargetBootVolumeVPUsPerGB) {
			return domain.Job{}, fmt.Errorf("%w: targetBootVolumeVpusPerGb must be between 10 and 120", ErrValidation)
		}
	}

	job := s.newJobLocked(instanceActionJobType(req.Action), "instance", instance.ID, actor)
	job.ProfileID = instance.ProfileID
	job.Region = instance.Region
	job.CompartmentID = instance.CompartmentID
	job.MaxRetries = 1
	job.Input = map[string]any{
		"action":                     string(req.Action),
		"graceful":                   req.Graceful,
		"preserveBootVolume":         req.PreserveBootVolume,
		"targetShape":                req.TargetShape,
		"targetOcpus":                req.TargetOCPUs,
		"targetMemoryGb":             req.TargetMemoryGB,
		"targetBootVolumeGb":         req.TargetBootVolumeGB,
		"targetBootVolumeVpusPerGb":  req.TargetBootVolumeVPUsPerGB,
		"expandBootVolume":           req.ExpandBootVolume,
		"currentBootVolumeGb":        instance.BootVolumeGB,
		"currentBootVolumeVpusPerGb": defaultInt(instance.BootVolumeVPUsPerGB, 10),
		"snapshotBefore":             req.SnapshotBefore,
		"note":                       req.Note,
		"instanceName":               instance.Name,
		"currentStatus":              instance.Status,
		"currentShape":               instance.Shape,
		"currentOcpus":               instance.OCPUs,
		"currentMemoryGb":            instance.MemoryGB,
	}
	if req.Action == domain.InstanceActionTerminate {
		instance.Status = domain.InstanceTerminating
		instance.LastSyncedAt = s.now().UTC()
		if err := s.saveInstanceLocked(instance); err != nil {
			return domain.Job{}, err
		}
	}
	return s.saveJobLocked(job)
}

func (s *Store) CreateOCIInstanceLaunchTask(req domain.CreateInstanceRequest, actor string) (domain.Job, error) {
	req, template, err := s.applyTemplateToCreateRequest(req)
	if err != nil {
		return domain.Job{}, err
	}
	if strings.TrimSpace(req.Name) == "" {
		return domain.Job{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(req.Shape) == "" {
		return domain.Job{}, fmt.Errorf("%w: shape is required", ErrValidation)
	}
	if req.OCPUs <= 0 {
		return domain.Job{}, fmt.Errorf("%w: ocpus must be greater than zero", ErrValidation)
	}
	if req.MemoryGB <= 0 {
		return domain.Job{}, fmt.Errorf("%w: memoryGb must be greater than zero", ErrValidation)
	}
	if req.BootVolumeGB <= 0 {
		req.BootVolumeGB = 50
	}
	if req.BootVolumeVPUsPerGB == 0 {
		req.BootVolumeVPUsPerGB = 10
	}
	if !validBootVolumeVPUs(req.BootVolumeVPUsPerGB) {
		return domain.Job{}, fmt.Errorf("%w: bootVolumeVpusPerGb must be between 10 and 120", ErrValidation)
	}
	if req.MaxRetries < 0 {
		return domain.Job{}, fmt.Errorf("%w: maxRetries cannot be negative", ErrValidation)
	}
	if err := validateRetryPolicy(req); err != nil {
		return domain.Job{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	job := s.newJobLocked("创建实例", "instance", "", actor)
	profileID := defaultString(req.ProfileID, "DEFAULT")
	if profile, ok := s.profileByIDOrNameLocked(profileID); ok {
		profileID = profile.ID
		if strings.TrimSpace(req.Region) == "" {
			req.Region = profile.DefaultRegion
		}
	}
	job.ProfileID = profileID
	job.Region = req.Region
	job.CompartmentID = req.CompartmentID
	job.MaxRetries = req.MaxRetries
	job.Input = map[string]any{
		"operation":            "launch",
		"name":                 req.Name,
		"profileId":            job.ProfileID,
		"region":               req.Region,
		"compartment":          req.Compartment,
		"compartmentId":        req.CompartmentID,
		"availabilityAd":       req.AvailabilityAD,
		"templateId":           req.TemplateID,
		"imageId":              req.ImageID,
		"shape":                req.Shape,
		"ocpus":                req.OCPUs,
		"memoryGb":             req.MemoryGB,
		"bootVolumeGb":         req.BootVolumeGB,
		"bootVolumeVpusPerGb":  defaultInt(req.BootVolumeVPUsPerGB, 10),
		"assignPublicIp":       req.AssignPublicIP,
		"enableIpv6":           req.EnableIPv6,
		"reservedPublicIp":     req.ReservedPublicIP,
		"vcnId":                req.VCNID,
		"subnetId":             req.SubnetID,
		"sshKey":               req.SSHKey,
		"cloudInit":            req.CloudInit,
		"tags":                 req.Tags,
		"retryMode":            normalizedRetryMode(req),
		"retryMaxAttempts":     effectiveRetryMaxAttempts(req),
		"retryDelayMinSeconds": req.RetryDelayMinSec,
		"retryDelayMaxSeconds": req.RetryDelayMaxSec,
		"requireApproval":      req.RequireApproval,
		"snapshotBefore":       req.SnapshotBefore,
		"generateRootPassword": req.GenerateRootPassword,
		"notifyRootPassword":   req.NotifyRootPassword,
		"cloudInitSensitive":   req.GenerateRootPassword,
		"executionMode":        "oci",
	}
	if strings.TrimSpace(template.ID) != "" {
		job.Input["templateVersion"] = template.Version
		job.Input["templateOverrides"] = templateOverrides(template, req)
	}
	return s.saveJobLocked(job)
}

func (s *Store) CreateOCIInstanceActionTask(instanceID string, req domain.InstanceActionRequest, actor, profileID, region, compartmentID string) (domain.Job, error) {
	if strings.TrimSpace(instanceID) == "" {
		return domain.Job{}, fmt.Errorf("%w: instance OCID is required", ErrValidation)
	}
	if !strings.HasPrefix(instanceID, "ocid1.instance.") {
		return domain.Job{}, fmt.Errorf("%w: instance id must be an OCI instance OCID", ErrValidation)
	}
	if req.Action == "" {
		return domain.Job{}, fmt.Errorf("%w: action is required", ErrValidation)
	}
	if req.Action == domain.InstanceActionResize {
		if strings.TrimSpace(req.TargetShape) == "" {
			return domain.Job{}, fmt.Errorf("%w: targetShape is required", ErrValidation)
		}
		targetIsFlexible := isFlexibleShapeName(req.TargetShape)
		if targetIsFlexible && req.TargetOCPUs <= 0 {
			return domain.Job{}, fmt.Errorf("%w: targetOcpus must be greater than zero", ErrValidation)
		}
		if targetIsFlexible && req.TargetMemoryGB <= 0 {
			return domain.Job{}, fmt.Errorf("%w: targetMemoryGb must be greater than zero", ErrValidation)
		}
		if req.ExpandBootVolume {
			if req.TargetBootVolumeGB <= 0 {
				return domain.Job{}, fmt.Errorf("%w: targetBootVolumeGb must be greater than zero", ErrValidation)
			}
		}
		if req.TargetBootVolumeVPUsPerGB != 0 && !validBootVolumeVPUs(req.TargetBootVolumeVPUsPerGB) {
			return domain.Job{}, fmt.Errorf("%w: targetBootVolumeVpusPerGb must be between 10 and 120", ErrValidation)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if req.Action == domain.InstanceActionResize {
		if instance, ok := s.instances[instanceID]; ok && req.TargetBootVolumeGB > 0 && instance.BootVolumeGB > 0 && req.TargetBootVolumeGB < instance.BootVolumeGB {
			return domain.Job{}, fmt.Errorf("%w: boot volume cannot be decreased", ErrValidation)
		}
	}

	job := s.newJobLocked(instanceActionJobType(req.Action), "instance", instanceID, actor)
	profileID = defaultString(profileID, "DEFAULT")
	if profile, ok := s.profileByIDOrNameLocked(profileID); ok {
		profileID = profile.ID
		if strings.TrimSpace(region) == "" {
			region = profile.DefaultRegion
		}
	}
	job.ProfileID = profileID
	job.Region = region
	job.CompartmentID = compartmentID
	job.MaxRetries = 0
	job.Input = map[string]any{
		"action":                    string(req.Action),
		"graceful":                  req.Graceful,
		"preserveBootVolume":        req.PreserveBootVolume,
		"targetShape":               req.TargetShape,
		"targetOcpus":               req.TargetOCPUs,
		"targetMemoryGb":            req.TargetMemoryGB,
		"targetBootVolumeGb":        req.TargetBootVolumeGB,
		"targetBootVolumeVpusPerGb": req.TargetBootVolumeVPUsPerGB,
		"expandBootVolume":          req.ExpandBootVolume,
		"snapshotBefore":            req.SnapshotBefore,
		"note":                      req.Note,
		"ociInstanceId":             instanceID,
		"executionMode":             "oci",
	}
	if req.Action == domain.InstanceActionTerminate {
		if instance, ok := s.instances[instanceID]; ok {
			instance.Status = domain.InstanceTerminating
			instance.LastSyncedAt = s.now().UTC()
			if err := s.saveInstanceLocked(instance); err != nil {
				return domain.Job{}, err
			}
		}
	}
	return s.saveJobLocked(job)
}

func (s *Store) CreateOCIInstanceReinstallTask(instanceID string, req domain.InstanceReinstallRequest, actor, profileID, region, compartmentID string) (domain.Job, error) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return domain.Job{}, fmt.Errorf("%w: instance OCID is required", ErrValidation)
	}
	if !strings.HasPrefix(instanceID, "ocid1.instance.") {
		return domain.Job{}, fmt.Errorf("%w: instance id must be an OCI instance OCID", ErrValidation)
	}
	if strings.TrimSpace(req.ImageID) == "" {
		return domain.Job{}, fmt.Errorf("%w: imageId is required", ErrValidation)
	}
	if !strings.HasPrefix(strings.TrimSpace(req.ImageID), "ocid1.image.") {
		return domain.Job{}, fmt.Errorf("%w: imageId must be an OCI image OCID", ErrValidation)
	}
	if req.BootVolumeSizeGB > 0 && req.BootVolumeSizeGB < 50 {
		return domain.Job{}, fmt.Errorf("%w: bootVolumeSizeGb must be at least 50", ErrValidation)
	}
	if req.BootVolumeVPUsPerGB != 0 && !validBootVolumeVPUs(req.BootVolumeVPUsPerGB) {
		return domain.Job{}, fmt.Errorf("%w: bootVolumeVpusPerGb must be between 10 and 120", ErrValidation)
	}
	if req.GenerateRootPassword || strings.TrimSpace(req.CloudInit) != "" || strings.TrimSpace(req.SSHAuthorizedKey) != "" {
		return domain.Job{}, fmt.Errorf("%w: root password, cloud-init and SSH key injection are not supported by OCI UpdateInstance reinstall because instance user_data and ssh_authorized_keys metadata cannot be changed after launch", ErrValidation)
	}
	if req.CreateBootVolumeBackup {
		return domain.Job{}, fmt.Errorf("%w: boot volume backup before reinstall is not implemented yet", ErrValidation)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if instance, ok := s.instances[instanceID]; ok {
		if instance.Status == domain.InstanceTerminated || instance.Status == domain.InstanceTerminating {
			return domain.Job{}, fmt.Errorf("%w: terminated instance cannot be reinstalled", ErrConflict)
		}
		if strings.TrimSpace(req.ConfirmationName) != "" && strings.TrimSpace(req.ConfirmationName) != strings.TrimSpace(instance.Name) {
			return domain.Job{}, fmt.Errorf("%w: confirmationName must match instance name", ErrValidation)
		}
		if req.BootVolumeSizeGB > 0 && instance.BootVolumeGB > 0 && req.BootVolumeSizeGB < instance.BootVolumeGB {
			return domain.Job{}, fmt.Errorf("%w: boot volume cannot be decreased", ErrValidation)
		}
		if strings.TrimSpace(profileID) == "" {
			profileID = instance.ProfileID
		}
		if strings.TrimSpace(region) == "" {
			region = instance.Region
		}
		if strings.TrimSpace(compartmentID) == "" {
			compartmentID = instance.CompartmentID
		}
	}

	job := s.newJobLocked("重装系统", "instance", instanceID, actor)
	profileID = defaultString(profileID, "DEFAULT")
	if profile, ok := s.profileByIDOrNameLocked(profileID); ok {
		profileID = profile.ID
		if strings.TrimSpace(region) == "" {
			region = profile.DefaultRegion
		}
	}
	job.ProfileID = profileID
	job.Region = strings.TrimSpace(region)
	job.CompartmentID = strings.TrimSpace(compartmentID)
	job.MaxRetries = 0
	job.Input = map[string]any{
		"operation":              "reinstall",
		"ociInstanceId":          instanceID,
		"imageId":                strings.TrimSpace(req.ImageID),
		"imageName":              strings.TrimSpace(req.ImageName),
		"bootVolumeSizeGb":       req.BootVolumeSizeGB,
		"bootVolumeVpusPerGb":    req.BootVolumeVPUsPerGB,
		"preserveOldBootVolume":  req.PreserveOldBootVolume,
		"createBootVolumeBackup": false,
		"generateRootPassword":   false,
		"notifyPasswordInApp":    false,
		"notifyPasswordByEmail":  false,
		"confirmationName":       strings.TrimSpace(req.ConfirmationName),
		"note":                   strings.TrimSpace(req.Note),
		"cloudInitSensitive":     false,
		"executionMode":          "oci",
	}
	return s.saveJobLocked(job)
}

func (s *Store) CreateInstanceTask(req domain.CreateInstanceRequest, actor string) (domain.CreateInstanceResponse, error) {
	req, template, err := s.applyTemplateToCreateRequest(req)
	if err != nil {
		return domain.CreateInstanceResponse{}, err
	}
	if strings.TrimSpace(req.Name) == "" {
		return domain.CreateInstanceResponse{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(req.Shape) == "" {
		return domain.CreateInstanceResponse{}, fmt.Errorf("%w: shape is required", ErrValidation)
	}
	if req.OCPUs <= 0 {
		return domain.CreateInstanceResponse{}, fmt.Errorf("%w: ocpus must be greater than zero", ErrValidation)
	}
	if req.MemoryGB <= 0 {
		return domain.CreateInstanceResponse{}, fmt.Errorf("%w: memoryGb must be greater than zero", ErrValidation)
	}
	if req.BootVolumeGB <= 0 {
		req.BootVolumeGB = 50
	}
	if req.BootVolumeVPUsPerGB == 0 {
		req.BootVolumeVPUsPerGB = 10
	}
	if !validBootVolumeVPUs(req.BootVolumeVPUsPerGB) {
		return domain.CreateInstanceResponse{}, fmt.Errorf("%w: bootVolumeVpusPerGb must be between 10 and 120", ErrValidation)
	}
	if req.MaxRetries < 0 {
		return domain.CreateInstanceResponse{}, fmt.Errorf("%w: maxRetries cannot be negative", ErrValidation)
	}
	if err := validateRetryPolicy(req); err != nil {
		return domain.CreateInstanceResponse{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	profileID := defaultString(req.ProfileID, "profile-default")
	profile, ok := s.profileByIDOrNameLocked(profileID)
	if !ok {
		return domain.CreateInstanceResponse{}, fmt.Errorf("%w: profile not found", ErrValidation)
	}
	profileID = profile.ID
	if strings.TrimSpace(req.Region) == "" {
		req.Region = profile.DefaultRegion
	}

	now := s.now().UTC()
	instance := domain.Instance{
		ID:                  s.nextInstanceIDLocked(req.Name),
		Name:                req.Name,
		Created:             "刚刚创建",
		Shape:               req.Shape,
		Region:              strings.TrimSpace(req.Region),
		Compartment:         strings.TrimSpace(req.Compartment),
		PrimaryIP:           "-",
		PrivateIP:           fmt.Sprintf("10.0.%d.%d", s.nextInst, 20+s.nextInst),
		OCPUs:               req.OCPUs,
		MemoryGB:            req.MemoryGB,
		BootVolumeGB:        req.BootVolumeGB,
		BootVolumeVPUsPerGB: req.BootVolumeVPUsPerGB,
		Status:              domain.InstanceProvisioning,
		Protected:           req.RequireApproval,
		OCIInstanceID:       "",
		ProfileID:           profileID,
		CompartmentID:       strings.TrimSpace(req.CompartmentID),
		LastSyncedAt:        now,
	}
	if req.AssignPublicIP {
		instance.PrimaryIP = fmt.Sprintf("203.0.113.%d", 20+s.nextInst)
	}
	if err := s.saveInstanceLocked(instance); err != nil {
		return domain.CreateInstanceResponse{}, err
	}

	job := s.newJobLocked("创建实例", "instance", instance.ID, actor)
	job.ProfileID = instance.ProfileID
	job.Region = instance.Region
	job.CompartmentID = instance.CompartmentID
	job.MaxRetries = req.MaxRetries
	job.Input = map[string]any{
		"name":                 req.Name,
		"profileId":            profileID,
		"region":               instance.Region,
		"compartment":          instance.Compartment,
		"compartmentId":        instance.CompartmentID,
		"availabilityAd":       req.AvailabilityAD,
		"templateId":           req.TemplateID,
		"imageId":              req.ImageID,
		"shape":                req.Shape,
		"ocpus":                req.OCPUs,
		"memoryGb":             req.MemoryGB,
		"bootVolumeGb":         req.BootVolumeGB,
		"bootVolumeVpusPerGb":  req.BootVolumeVPUsPerGB,
		"assignPublicIp":       req.AssignPublicIP,
		"enableIpv6":           req.EnableIPv6,
		"reservedPublicIp":     req.ReservedPublicIP,
		"vcnId":                req.VCNID,
		"subnetId":             req.SubnetID,
		"sshKey":               req.SSHKey,
		"cloudInit":            req.CloudInit,
		"tags":                 req.Tags,
		"retryMode":            normalizedRetryMode(req),
		"retryMaxAttempts":     effectiveRetryMaxAttempts(req),
		"retryDelayMinSeconds": req.RetryDelayMinSec,
		"retryDelayMaxSeconds": req.RetryDelayMaxSec,
		"requireApproval":      req.RequireApproval,
		"snapshotBefore":       req.SnapshotBefore,
	}
	if strings.TrimSpace(template.ID) != "" {
		job.Input["templateVersion"] = template.Version
		job.Input["templateOverrides"] = templateOverrides(template, req)
	}
	if _, err := s.saveJobLocked(job); err != nil {
		return domain.CreateInstanceResponse{}, err
	}

	return domain.CreateInstanceResponse{Instance: instance, Job: job}, nil
}

func (s *Store) CreateAutomationTask(req domain.AutomationTaskRequest, actor string) (domain.AutomationTaskResponse, error) {
	if strings.TrimSpace(req.Name) == "" {
		return domain.AutomationTaskResponse{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(req.Type) == "" {
		return domain.AutomationTaskResponse{}, fmt.Errorf("%w: type is required", ErrValidation)
	}
	if req.MaxRetries < 0 {
		return domain.AutomationTaskResponse{}, fmt.Errorf("%w: maxRetries cannot be negative", ErrValidation)
	}
	if req.MaxInstances <= 0 {
		req.MaxInstances = 1
	}
	if req.MaxDailyRuns <= 0 {
		req.MaxDailyRuns = 1
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	rule := domain.AutomationRule{
		ID:               s.nextRuleIDLocked(),
		Name:             req.Name,
		Type:             req.Type,
		TargetPool:       defaultString(req.TargetPool, "manual-instances"),
		Action:           defaultString(req.Action, "创建 1 台实例"),
		TriggerInterval:  defaultString(req.TriggerInterval, "每 5 分钟"),
		Cooldown:         defaultString(req.Cooldown, "30 分钟"),
		MaxRetries:       req.MaxRetries,
		FailurePolicy:    defaultString(req.FailurePolicy, "达到上限后暂停并通知"),
		MaxInstances:     req.MaxInstances,
		MaxDailyRuns:     req.MaxDailyRuns,
		RegionScope:      defaultString(req.RegionScope, "仅当前区域"),
		NotifyChannel:    defaultString(req.NotifyChannel, "邮件 + Webhook"),
		Enabled:          req.EnableImmediately,
		ApprovalRequired: req.ApprovalRequired,
		CreatedBy:        actor,
		CreatedAt:        now,
	}
	if rule.Enabled {
		next := now.Add(5 * time.Minute)
		rule.NextRunAt = &next
	}
	s.automations[rule.ID] = rule

	job := s.newJobLocked("添加自动化任务", "automation", rule.ID, actor)
	job.MaxRetries = 0
	job.Input = map[string]any{
		"ruleId":            rule.ID,
		"name":              rule.Name,
		"type":              rule.Type,
		"targetPool":        rule.TargetPool,
		"action":            rule.Action,
		"triggerInterval":   rule.TriggerInterval,
		"cooldown":          rule.Cooldown,
		"maxRetries":        rule.MaxRetries,
		"failurePolicy":     rule.FailurePolicy,
		"maxInstances":      rule.MaxInstances,
		"maxDailyRuns":      rule.MaxDailyRuns,
		"regionScope":       rule.RegionScope,
		"notifyChannel":     rule.NotifyChannel,
		"enableImmediately": rule.Enabled,
		"approvalRequired":  rule.ApprovalRequired,
	}
	if _, err := s.saveJobLocked(job); err != nil {
		return domain.AutomationTaskResponse{}, err
	}

	return domain.AutomationTaskResponse{Rule: rule, Job: job}, nil
}

func (s *Store) newJobLocked(typ, resourceType, resourceID, actor string) domain.Job {
	id := s.nextJobIDLocked()
	now := s.now().UTC()
	return domain.Job{
		ID:               id,
		Type:             typ,
		Status:           domain.JobPending,
		ProfileID:        "",
		Region:           "",
		CompartmentID:    "",
		ResourceType:     resourceType,
		ResourceID:       resourceID,
		OCIRequestID:     "",
		OCIWorkRequestID: "",
		Input:            map[string]any{},
		RetryCount:       0,
		MaxRetries:       3,
		CreatedBy:        defaultString(actor, "admin"),
		CreatedAt:        now,
	}
}

func (s *Store) saveJobLocked(job domain.Job) (domain.Job, error) {
	s.jobs[job.ID] = job
	entry := domain.AuditLog{
		Actor:            defaultString(job.CreatedBy, "system"),
		Action:           "job." + string(job.Status),
		ResourceType:     job.ResourceType,
		ResourceID:       job.ResourceID,
		ProfileID:        job.ProfileID,
		Region:           job.Region,
		CompartmentID:    job.CompartmentID,
		OCIRequestID:     job.OCIRequestID,
		OCIWorkRequestID: job.OCIWorkRequestID,
		RequestPayload:   cloneMap(job.Input),
		ResultPayload:    cloneMap(job.Result),
		ErrorCode:        job.ErrorCode,
		ErrorMessage:     job.ErrorMessage,
		CreatedAt:        s.now().UTC(),
	}
	if s.sink != nil {
		if err := s.sink.SaveJob(job); err != nil {
			return job, err
		}
	}
	if err := s.recordAuditLocked(entry); err != nil {
		return job, err
	}
	return job, nil
}

func (s *Store) saveInstanceLocked(instance domain.Instance) error {
	if strings.TrimSpace(instance.ID) == "" {
		return nil
	}
	s.instances[instance.ID] = instance
	if s.sink == nil {
		return nil
	}
	return s.sink.SaveInstance(instance)
}

func (s *Store) saveProfileLocked(profile domain.Profile, secret domain.ProfileSecret) error {
	if strings.TrimSpace(profile.ID) == "" {
		return nil
	}
	s.profiles[profile.ID] = profile
	if s.sink == nil {
		if strings.TrimSpace(secret.PrivateKey) != "" || strings.TrimSpace(secret.PrivateKeyFile) != "" {
			return fmt.Errorf("%w: database persistence is required to store OCI profile keys", ErrValidation)
		}
		return nil
	}
	return s.sink.SaveProfile(profile, secret)
}

func (s *Store) saveTemplateLocked(template domain.InstanceTemplate) error {
	if strings.TrimSpace(template.ID) == "" {
		return nil
	}
	template = normalizeTemplate(template, s.now().UTC())
	s.templates[template.ID] = template
	if s.sink == nil {
		return nil
	}
	sink, ok := s.sink.(templateSink)
	if !ok {
		return nil
	}
	return sink.SaveTemplate(template)
}

func (s *Store) deleteTemplateLocked(templateID string) error {
	if strings.TrimSpace(templateID) == "" {
		return nil
	}
	delete(s.templates, templateID)
	if s.sink == nil {
		return nil
	}
	sink, ok := s.sink.(templateSink)
	if !ok {
		return nil
	}
	return sink.DeleteTemplate(templateID)
}

func instanceFromLaunchJob(job domain.Job, result map[string]any, syncedAt time.Time) domain.Instance {
	instanceID := defaultString(stringFromMap(result, "instanceId"), job.ResourceID)
	if strings.TrimSpace(instanceID) == "" {
		return domain.Instance{}
	}
	status := statusFromOCIState(stringFromMap(result, "finalState"), domain.InstanceRunning)
	return domain.Instance{
		ID:                  instanceID,
		Name:                defaultString(stringFromMap(result, "displayName"), stringFromMap(job.Input, "name")),
		Created:             syncedAt.Format(time.RFC3339),
		Shape:               defaultString(stringFromMap(result, "shape"), stringFromMap(job.Input, "shape")),
		Region:              defaultString(job.Region, stringFromMap(job.Input, "region")),
		Compartment:         defaultString(stringFromMap(job.Input, "compartment"), defaultString(stringFromMap(result, "compartmentId"), job.CompartmentID)),
		PrimaryIP:           "",
		PrivateIP:           "",
		OCPUs:               defaultInt(intFromMap(result, "ocpus"), intFromMap(job.Input, "ocpus")),
		MemoryGB:            defaultInt(intFromMap(result, "memoryGb"), intFromMap(job.Input, "memoryGb")),
		BootVolumeGB:        defaultInt(intFromMap(result, "bootVolumeGb"), intFromMap(job.Input, "bootVolumeGb")),
		BootVolumeVPUsPerGB: defaultInt(intFromMap(result, "bootVolumeVpusPerGb"), defaultInt(intFromMap(job.Input, "bootVolumeVpusPerGb"), 10)),
		Status:              status,
		Protected:           boolFromMap(job.Input, "requireApproval"),
		OCIInstanceID:       instanceID,
		ProfileID:           defaultString(job.ProfileID, stringFromMap(job.Input, "profileId")),
		CompartmentID:       defaultString(stringFromMap(result, "compartmentId"), job.CompartmentID),
		LastSyncedAt:        syncedAt,
	}
}

func instanceFromActionJob(job domain.Job, result map[string]any, syncedAt time.Time) domain.Instance {
	instanceID := defaultString(stringFromMap(result, "instanceId"), defaultString(stringFromMap(job.Input, "ociInstanceId"), job.ResourceID))
	if strings.TrimSpace(instanceID) == "" {
		return domain.Instance{}
	}
	instance := domain.Instance{
		ID:            instanceID,
		Name:          instanceID,
		Created:       syncedAt.Format(time.RFC3339),
		Region:        job.Region,
		Compartment:   job.CompartmentID,
		Status:        statusFromOCIState(stringFromMap(result, "finalState"), domain.InstanceProvisioning),
		OCIInstanceID: instanceID,
		ProfileID:     job.ProfileID,
		CompartmentID: job.CompartmentID,
		LastSyncedAt:  syncedAt,
	}
	if action, _ := job.Input["action"].(string); domain.InstanceLifecycleAction(action) == domain.InstanceActionResize {
		instance.Shape = defaultString(stringFromMap(result, "targetShape"), stringFromMap(job.Input, "targetShape"))
		instance.OCPUs = defaultInt(intFromMap(result, "targetOcpus"), intFromMap(job.Input, "targetOcpus"))
		instance.MemoryGB = defaultInt(intFromMap(result, "targetMemoryGb"), intFromMap(job.Input, "targetMemoryGb"))
	}
	return instance
}

func statusFromOCIState(value string, fallback domain.InstanceStatus) domain.InstanceStatus {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "RUNNING":
		return domain.InstanceRunning
	case "STOPPED", "STOPPING":
		return domain.InstanceStopped
	case "TERMINATING":
		return domain.InstanceTerminating
	case "TERMINATED":
		return domain.InstanceTerminated
	case "PROVISIONING", "STARTING", "MOVING":
		return domain.InstanceProvisioning
	default:
		return fallback
	}
}

func (s *Store) setActiveJobStatus(id string, status domain.JobStatus) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, ErrNotFound
	}
	if !isActiveStatus(job.Status) {
		return domain.Job{}, fmt.Errorf("%w: job is not active", ErrConflict)
	}

	job.Status = status
	return s.saveJobLocked(job)
}

func isActiveStatus(status domain.JobStatus) bool {
	return status == domain.JobRunning || status == domain.JobWaitingOCI || status == domain.JobVerifying
}

func isCancelableStatus(status domain.JobStatus) bool {
	return status == domain.JobPending || status == domain.JobRetrying || isActiveStatus(status)
}

func isRetryableStatus(status domain.JobStatus) bool {
	return status == domain.JobFailed || status == domain.JobCancelled || status == domain.JobManualNeeded || status == domain.JobRollbackNeeded
}

func isCompletedJobStatus(status domain.JobStatus) bool {
	return status == domain.JobSuccess ||
		status == domain.JobFailed ||
		status == domain.JobCancelled ||
		status == domain.JobRollbackNeeded ||
		status == domain.JobManualNeeded
}

func instanceActionJobType(action domain.InstanceLifecycleAction) string {
	switch action {
	case domain.InstanceActionStart:
		return "启动实例"
	case domain.InstanceActionStop:
		return "停止实例"
	case domain.InstanceActionReboot:
		return "重启实例"
	case domain.InstanceActionTerminate:
		return "终止实例"
	case domain.InstanceActionResize:
		return "升降级实例"
	default:
		return "实例操作"
	}
}

func applyInstanceAction(instance *domain.Instance, action string, input map[string]any) {
	switch domain.InstanceLifecycleAction(action) {
	case domain.InstanceActionStart, domain.InstanceActionReboot:
		instance.Status = domain.InstanceRunning
	case domain.InstanceActionStop:
		instance.Status = domain.InstanceStopped
	case domain.InstanceActionTerminate:
		instance.Status = domain.InstanceTerminated
		instance.PrimaryIP = "已释放"
	case domain.InstanceActionResize:
		if shape, ok := input["targetShape"].(string); ok && strings.TrimSpace(shape) != "" {
			instance.Shape = shape
		}
		if ocpus, ok := intFromAny(input["targetOcpus"]); ok && ocpus > 0 {
			instance.OCPUs = ocpus
		}
		if memoryGB, ok := intFromAny(input["targetMemoryGb"]); ok && memoryGB > 0 {
			instance.MemoryGB = memoryGB
		}
		if bootVolumeGB, ok := intFromAny(input["targetBootVolumeGb"]); ok && bootVolumeGB > instance.BootVolumeGB {
			instance.BootVolumeGB = bootVolumeGB
		}
		if bootVolumeVPUsPerGB, ok := intFromAny(input["targetBootVolumeVpusPerGb"]); ok && bootVolumeVPUsPerGB > 0 {
			instance.BootVolumeVPUsPerGB = bootVolumeVPUsPerGB
		}
		instance.Status = domain.InstanceRunning
	}
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}

func stringFromMap(in map[string]any, key string) string {
	if in == nil {
		return ""
	}
	if value, ok := in[key].(string); ok {
		return value
	}
	return ""
}

func intFromMap(in map[string]any, key string) int {
	if in == nil {
		return 0
	}
	if value, ok := intFromAny(in[key]); ok {
		return value
	}
	return 0
}

func boolFromMap(in map[string]any, key string) bool {
	if in == nil {
		return false
	}
	if value, ok := in[key].(bool); ok {
		return value
	}
	return false
}

func (s *Store) applyTemplateToCreateRequest(req domain.CreateInstanceRequest) (domain.CreateInstanceRequest, domain.InstanceTemplate, error) {
	templateID := strings.TrimSpace(req.TemplateID)
	if templateID == "" {
		return req, domain.InstanceTemplate{}, nil
	}
	s.mu.RLock()
	template, ok := s.templates[templateID]
	s.mu.RUnlock()
	if !ok || strings.EqualFold(template.Status, "DELETED") {
		return domain.CreateInstanceRequest{}, domain.InstanceTemplate{}, fmt.Errorf("%w: template not found", ErrValidation)
	}
	if strings.EqualFold(template.Status, "DISABLED") || strings.EqualFold(template.Status, "ARCHIVED") {
		return domain.CreateInstanceRequest{}, domain.InstanceTemplate{}, fmt.Errorf("%w: template is not active", ErrValidation)
	}
	if parsed, err := templateFromConfig(template); err == nil {
		parsed.ID = template.ID
		parsed.Name = template.Name
		parsed.Description = template.Description
		parsed.Version = template.Version
		parsed.Status = template.Status
		parsed.ValidationStatus = template.ValidationStatus
		parsed.CreatedBy = template.CreatedBy
		parsed.CreatedAt = template.CreatedAt
		parsed.UpdatedAt = template.UpdatedAt
		template = parsed
	} else {
		return domain.CreateInstanceRequest{}, domain.InstanceTemplate{}, fmt.Errorf("%w: template config parse failed: %v", ErrValidation, err)
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = templateDefaultInstanceName(template)
	}
	if strings.TrimSpace(req.ProfileID) == "" {
		req.ProfileID = template.ProfileID
	}
	if strings.TrimSpace(req.Region) == "" {
		req.Region = template.Region
	}
	if strings.TrimSpace(req.Compartment) == "" {
		req.Compartment = template.Compartment
	}
	if strings.TrimSpace(req.CompartmentID) == "" {
		req.CompartmentID = template.CompartmentID
	}
	if strings.TrimSpace(req.AvailabilityAD) == "" {
		req.AvailabilityAD = template.AvailabilityAD
	}
	if strings.TrimSpace(req.ImageID) == "" {
		req.ImageID = template.ImageID
	}
	if strings.TrimSpace(req.Shape) == "" {
		req.Shape = template.Shape
	}
	if req.OCPUs <= 0 {
		req.OCPUs = template.OCPUs
	}
	if req.MemoryGB <= 0 {
		req.MemoryGB = template.MemoryGB
	}
	if req.BootVolumeGB <= 0 {
		req.BootVolumeGB = template.BootVolumeGB
	}
	if req.BootVolumeVPUsPerGB <= 0 {
		req.BootVolumeVPUsPerGB = template.BootVolumeVPUsPerGB
	}
	if strings.TrimSpace(req.VCNID) == "" {
		req.VCNID = template.VCNID
	}
	if strings.TrimSpace(req.SubnetID) == "" {
		req.SubnetID = template.SubnetID
	}
	if !req.AssignPublicIP {
		req.AssignPublicIP = template.AssignPublicIP
	}
	if !req.EnableIPv6 {
		req.EnableIPv6 = template.EnableIPv6
	}
	if strings.TrimSpace(req.ReservedPublicIP) == "" {
		req.ReservedPublicIP = template.ReservedPublicIP
	}
	if strings.TrimSpace(req.SSHKey) == "" {
		req.SSHKey = template.SSHKey
	}
	if strings.TrimSpace(req.CloudInit) == "" {
		req.CloudInit = template.CloudInit
	}
	if len(req.Tags) == 0 {
		req.Tags = cleanTags(template.Tags)
	} else {
		merged := cleanTags(template.Tags)
		for key, value := range cleanTags(req.Tags) {
			merged[key] = value
		}
		req.Tags = merged
	}
	return req, template, nil
}

func templateOverrides(template domain.InstanceTemplate, req domain.CreateInstanceRequest) map[string]any {
	overrides := map[string]any{}
	addString := func(key, templateValue, reqValue string) {
		if strings.TrimSpace(reqValue) != "" && strings.TrimSpace(reqValue) != strings.TrimSpace(templateValue) {
			overrides[key] = reqValue
		}
	}
	addInt := func(key string, templateValue, reqValue int) {
		if reqValue > 0 && reqValue != templateValue {
			overrides[key] = reqValue
		}
	}
	addBool := func(key string, templateValue, reqValue bool) {
		if reqValue != templateValue {
			overrides[key] = reqValue
		}
	}
	addString("profileId", template.ProfileID, req.ProfileID)
	addString("region", template.Region, req.Region)
	addString("compartmentId", template.CompartmentID, req.CompartmentID)
	addString("availabilityAd", template.AvailabilityAD, req.AvailabilityAD)
	addString("imageId", template.ImageID, req.ImageID)
	addString("shape", template.Shape, req.Shape)
	addInt("ocpus", template.OCPUs, req.OCPUs)
	addInt("memoryGb", template.MemoryGB, req.MemoryGB)
	addInt("bootVolumeGb", template.BootVolumeGB, req.BootVolumeGB)
	addInt("bootVolumeVpusPerGb", template.BootVolumeVPUsPerGB, req.BootVolumeVPUsPerGB)
	addString("vcnId", template.VCNID, req.VCNID)
	addString("subnetId", template.SubnetID, req.SubnetID)
	addBool("assignPublicIp", template.AssignPublicIP, req.AssignPublicIP)
	addBool("enableIpv6", template.EnableIPv6, req.EnableIPv6)
	addString("reservedPublicIp", template.ReservedPublicIP, req.ReservedPublicIP)
	return overrides
}

func templateMissingFields(template domain.InstanceTemplate) []string {
	var missing []string
	requireString := func(key, value string) {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	requireInt := func(key string, value int) {
		if value <= 0 {
			missing = append(missing, key)
		}
	}
	requireString("profileId", template.ProfileID)
	requireString("region", template.Region)
	requireString("shape", template.Shape)
	requireInt("ocpus", template.OCPUs)
	requireInt("memoryGb", template.MemoryGB)
	requireInt("bootVolumeGb", template.BootVolumeGB)
	if template.BootVolumeGB > 0 && template.BootVolumeGB < 50 {
		missing = append(missing, "bootVolumeGb>=50")
	}
	if template.BootVolumeVPUsPerGB <= 0 || !validBootVolumeVPUs(template.BootVolumeVPUsPerGB) {
		missing = append(missing, "bootVolumeVpusPerGb")
	}
	return missing
}

func templateFromCreateRequest(req domain.CreateTemplateRequest) (domain.InstanceTemplate, error) {
	template := domain.InstanceTemplate{
		Name:                strings.TrimSpace(req.Name),
		Description:         strings.TrimSpace(req.Description),
		Version:             strings.TrimSpace(req.Version),
		ProfileID:           strings.TrimSpace(req.ProfileID),
		Region:              strings.TrimSpace(req.Region),
		Compartment:         strings.TrimSpace(req.Compartment),
		CompartmentID:       strings.TrimSpace(req.CompartmentID),
		AvailabilityAD:      strings.TrimSpace(req.AvailabilityAD),
		ImageID:             strings.TrimSpace(req.ImageID),
		ImageName:           strings.TrimSpace(req.ImageName),
		Shape:               strings.TrimSpace(req.Shape),
		OCPUs:               req.OCPUs,
		MemoryGB:            req.MemoryGB,
		BootVolumeGB:        req.BootVolumeGB,
		BootVolumeVPUsPerGB: req.BootVolumeVPUsPerGB,
		VCNID:               strings.TrimSpace(req.VCNID),
		SubnetID:            strings.TrimSpace(req.SubnetID),
		AssignPublicIP:      req.AssignPublicIP,
		EnableIPv6:          req.EnableIPv6,
		ReservedPublicIP:    strings.TrimSpace(req.ReservedPublicIP),
		SSHKey:              strings.TrimSpace(req.SSHKey),
		CloudInit:           strings.TrimSpace(req.CloudInit),
		Tags:                cleanTags(req.Tags),
		ConfigFormat:        strings.TrimSpace(req.ConfigFormat),
		ConfigText:          strings.TrimSpace(req.ConfigText),
		Status:              req.Status,
	}
	return normalizeTemplateForSave(template, time.Time{})
}

func mergeTemplateUpdate(current domain.InstanceTemplate, req domain.UpdateTemplateRequest) (domain.InstanceTemplate, error) {
	updated := current
	updated.Name = strings.TrimSpace(req.Name)
	updated.Description = strings.TrimSpace(req.Description)
	if strings.TrimSpace(req.Version) != "" {
		updated.Version = strings.TrimSpace(req.Version)
	}
	updated.ProfileID = strings.TrimSpace(req.ProfileID)
	updated.Region = strings.TrimSpace(req.Region)
	updated.Compartment = strings.TrimSpace(req.Compartment)
	updated.CompartmentID = strings.TrimSpace(req.CompartmentID)
	updated.AvailabilityAD = strings.TrimSpace(req.AvailabilityAD)
	updated.ImageID = strings.TrimSpace(req.ImageID)
	updated.ImageName = strings.TrimSpace(req.ImageName)
	updated.Shape = strings.TrimSpace(req.Shape)
	if req.OCPUs > 0 {
		updated.OCPUs = req.OCPUs
	}
	if req.MemoryGB > 0 {
		updated.MemoryGB = req.MemoryGB
	}
	if req.BootVolumeGB > 0 {
		updated.BootVolumeGB = req.BootVolumeGB
	}
	if req.BootVolumeVPUsPerGB > 0 {
		updated.BootVolumeVPUsPerGB = req.BootVolumeVPUsPerGB
	}
	updated.VCNID = strings.TrimSpace(req.VCNID)
	updated.SubnetID = strings.TrimSpace(req.SubnetID)
	updated.AssignPublicIP = req.AssignPublicIP
	updated.EnableIPv6 = req.EnableIPv6
	updated.ReservedPublicIP = strings.TrimSpace(req.ReservedPublicIP)
	updated.SSHKey = strings.TrimSpace(req.SSHKey)
	updated.CloudInit = strings.TrimSpace(req.CloudInit)
	if req.Tags != nil {
		updated.Tags = cleanTags(req.Tags)
	}
	updated.ConfigFormat = strings.TrimSpace(req.ConfigFormat)
	updated.ConfigText = strings.TrimSpace(req.ConfigText)
	if strings.TrimSpace(req.Status) != "" {
		updated.Status = req.Status
	}
	return normalizeTemplateForSave(updated, current.CreatedAt)
}

func normalizeTemplateForSave(template domain.InstanceTemplate, createdAt time.Time) (domain.InstanceTemplate, error) {
	if strings.TrimSpace(template.Name) == "" {
		return domain.InstanceTemplate{}, fmt.Errorf("%w: template name is required", ErrValidation)
	}
	if strings.TrimSpace(template.Version) == "" {
		template.Version = "v1"
	}
	if template.BootVolumeGB <= 0 {
		template.BootVolumeGB = 50
	}
	if template.BootVolumeGB < 50 {
		return domain.InstanceTemplate{}, fmt.Errorf("%w: bootVolumeGb cannot be less than 50", ErrValidation)
	}
	if template.BootVolumeVPUsPerGB == 0 {
		template.BootVolumeVPUsPerGB = 10
	}
	if !validBootVolumeVPUs(template.BootVolumeVPUsPerGB) {
		return domain.InstanceTemplate{}, fmt.Errorf("%w: bootVolumeVpusPerGb must be between 10 and 120", ErrValidation)
	}
	if strings.TrimSpace(template.ConfigFormat) == "" {
		template.ConfigFormat = "json"
	}
	if strings.TrimSpace(template.ConfigText) != "" {
		parsed, err := templateFromConfig(template)
		if err != nil {
			return domain.InstanceTemplate{}, err
		}
		template = mergeTemplateConfigFields(template, parsed)
	} else {
		configText, err := templateConfigJSON(template)
		if err != nil {
			return domain.InstanceTemplate{}, err
		}
		template.ConfigFormat = "json"
		template.ConfigText = configText
	}
	template = normalizeTemplate(template, createdAt)
	return template, nil
}

func normalizeTemplate(template domain.InstanceTemplate, now time.Time) domain.InstanceTemplate {
	template.ID = strings.TrimSpace(template.ID)
	template.Name = strings.TrimSpace(template.Name)
	template.Description = strings.TrimSpace(template.Description)
	template.Version = defaultString(strings.TrimSpace(template.Version), "v1")
	template.ProfileID = strings.TrimSpace(template.ProfileID)
	template.Region = strings.TrimSpace(template.Region)
	template.Compartment = strings.TrimSpace(template.Compartment)
	template.CompartmentID = strings.TrimSpace(template.CompartmentID)
	template.AvailabilityAD = strings.TrimSpace(template.AvailabilityAD)
	template.ImageID = strings.TrimSpace(template.ImageID)
	template.ImageName = strings.TrimSpace(template.ImageName)
	template.Shape = strings.TrimSpace(template.Shape)
	template.VCNID = strings.TrimSpace(template.VCNID)
	template.SubnetID = strings.TrimSpace(template.SubnetID)
	template.ReservedPublicIP = strings.TrimSpace(template.ReservedPublicIP)
	template.SSHKey = strings.TrimSpace(template.SSHKey)
	template.CloudInit = strings.TrimSpace(template.CloudInit)
	template.ConfigFormat = normalizeTemplateConfigFormat(template.ConfigFormat)
	template.ConfigText = strings.TrimSpace(template.ConfigText)
	template.CloudInitSet = template.CloudInitSet || template.CloudInit != ""
	if template.Tags == nil {
		template.Tags = map[string]string{}
	} else {
		template.Tags = cleanTags(template.Tags)
	}
	template.Status = normalizeTemplateStatus(template.Status)
	template.ValidationStatus = normalizeValidationStatus(template.ValidationStatus)
	if template.CreatedAt.IsZero() && !now.IsZero() {
		template.CreatedAt = now.UTC()
	}
	if template.UpdatedAt.IsZero() {
		if !now.IsZero() {
			template.UpdatedAt = now.UTC()
		} else {
			template.UpdatedAt = template.CreatedAt
		}
	}
	return template
}

func normalizeTemplateStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "DRAFT":
		return "DRAFT"
	case "DISABLED":
		return "DISABLED"
	case "ARCHIVED":
		return "ARCHIVED"
	case "DELETED":
		return "DELETED"
	default:
		return "ACTIVE"
	}
}

func normalizeValidationStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "VALIDATING", "VALID", "INVALID", "STALE":
		return strings.ToUpper(strings.TrimSpace(status))
	default:
		return "UNVERIFIED"
	}
}

func normalizeTemplateConfigFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "yaml", "yml":
		return "yaml"
	default:
		return "json"
	}
}

func templateConfigJSON(template domain.InstanceTemplate) (string, error) {
	payload := map[string]any{
		"context": map[string]any{
			"profileId":      template.ProfileID,
			"region":         template.Region,
			"compartment":    template.Compartment,
			"compartmentId":  template.CompartmentID,
			"availabilityAd": template.AvailabilityAD,
		},
		"imageAndShape": map[string]any{
			"imageId":             template.ImageID,
			"imageName":           template.ImageName,
			"shape":               template.Shape,
			"ocpus":               template.OCPUs,
			"memoryGb":            template.MemoryGB,
			"bootVolumeGb":        template.BootVolumeGB,
			"bootVolumeVpusPerGb": template.BootVolumeVPUsPerGB,
		},
		"networkAndAccess": map[string]any{
			"vcnId":            template.VCNID,
			"subnetId":         template.SubnetID,
			"assignPublicIp":   template.AssignPublicIP,
			"enableIpv6":       template.EnableIPv6,
			"reservedPublicIp": template.ReservedPublicIP,
			"sshKey":           template.SSHKey,
			"cloudInit":        template.CloudInit,
		},
		"tags": cleanTags(template.Tags),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func templateFromConfig(template domain.InstanceTemplate) (domain.InstanceTemplate, error) {
	raw, err := templateConfigMap(template)
	if err != nil {
		return domain.InstanceTemplate{}, err
	}
	if len(raw) == 0 {
		return template, nil
	}
	parsed := template
	applyTemplateConfigMap(&parsed, raw)
	return normalizeTemplate(parsed, template.CreatedAt), nil
}

func templateConfigMap(template domain.InstanceTemplate) (map[string]any, error) {
	configText := strings.TrimSpace(template.ConfigText)
	if configText == "" {
		return map[string]any{}, nil
	}
	var raw map[string]any
	var err error
	switch normalizeTemplateConfigFormat(template.ConfigFormat) {
	case "yaml":
		raw, err = parseSimpleYAML(configText)
	default:
		err = json.Unmarshal([]byte(configText), &raw)
	}
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func templateDefaultInstanceName(template domain.InstanceTemplate) string {
	raw, err := templateConfigMap(template)
	if err != nil {
		return ""
	}
	instance := mapFromAny(raw["instance"])
	return defaultString(stringFromMap(instance, "name"), stringFromMap(raw, "instanceName"))
}

func mergeTemplateConfigFields(base, parsed domain.InstanceTemplate) domain.InstanceTemplate {
	base.ProfileID = parsed.ProfileID
	base.Region = parsed.Region
	base.Compartment = parsed.Compartment
	base.CompartmentID = parsed.CompartmentID
	base.AvailabilityAD = parsed.AvailabilityAD
	base.ImageID = parsed.ImageID
	base.ImageName = parsed.ImageName
	base.Shape = parsed.Shape
	base.OCPUs = parsed.OCPUs
	base.MemoryGB = parsed.MemoryGB
	base.BootVolumeGB = parsed.BootVolumeGB
	base.BootVolumeVPUsPerGB = parsed.BootVolumeVPUsPerGB
	base.VCNID = parsed.VCNID
	base.SubnetID = parsed.SubnetID
	base.AssignPublicIP = parsed.AssignPublicIP
	base.EnableIPv6 = parsed.EnableIPv6
	base.ReservedPublicIP = parsed.ReservedPublicIP
	base.SSHKey = parsed.SSHKey
	base.CloudInit = parsed.CloudInit
	base.Tags = cleanTags(parsed.Tags)
	return base
}

func applyTemplateConfigMap(template *domain.InstanceTemplate, raw map[string]any) {
	context := mapFromAny(raw["context"])
	compute := mapFromAny(raw["imageAndShape"])
	if len(compute) == 0 {
		compute = mapFromAny(raw["compute"])
	}
	network := mapFromAny(raw["networkAndAccess"])
	if len(network) == 0 {
		network = mapFromAny(raw["network"])
	}
	tags := mapFromAny(raw["tags"])

	template.ProfileID = defaultString(defaultString(stringFromMap(context, "profileId"), stringFromMap(raw, "profileId")), template.ProfileID)
	template.Region = defaultString(defaultString(stringFromMap(context, "region"), stringFromMap(raw, "region")), template.Region)
	template.Compartment = defaultString(defaultString(stringFromMap(context, "compartment"), stringFromMap(raw, "compartment")), template.Compartment)
	template.CompartmentID = defaultString(defaultString(stringFromMap(context, "compartmentId"), stringFromMap(raw, "compartmentId")), template.CompartmentID)
	template.AvailabilityAD = defaultString(defaultString(stringFromMap(context, "availabilityAd"), stringFromMap(raw, "availabilityAd")), template.AvailabilityAD)
	template.ImageID = defaultString(defaultString(stringFromMap(compute, "imageId"), stringFromMap(raw, "imageId")), template.ImageID)
	template.ImageName = defaultString(defaultString(stringFromMap(compute, "imageName"), stringFromMap(raw, "imageName")), template.ImageName)
	template.Shape = defaultString(defaultString(stringFromMap(compute, "shape"), stringFromMap(raw, "shape")), template.Shape)
	template.OCPUs = defaultInt(defaultInt(intFromMap(compute, "ocpus"), intFromMap(raw, "ocpus")), template.OCPUs)
	template.MemoryGB = defaultInt(defaultInt(intFromMap(compute, "memoryGb"), intFromMap(raw, "memoryGb")), template.MemoryGB)
	template.BootVolumeGB = defaultInt(defaultInt(intFromMap(compute, "bootVolumeGb"), intFromMap(raw, "bootVolumeGb")), template.BootVolumeGB)
	template.BootVolumeVPUsPerGB = defaultInt(defaultInt(intFromMap(compute, "bootVolumeVpusPerGb"), intFromMap(raw, "bootVolumeVpusPerGb")), template.BootVolumeVPUsPerGB)
	template.VCNID = defaultString(defaultString(stringFromMap(network, "vcnId"), stringFromMap(raw, "vcnId")), template.VCNID)
	template.SubnetID = defaultString(defaultString(stringFromMap(network, "subnetId"), stringFromMap(raw, "subnetId")), template.SubnetID)
	if value, ok := boolValueFromMaps(network, raw, "assignPublicIp"); ok {
		template.AssignPublicIP = value
	}
	if value, ok := boolValueFromMaps(network, raw, "enableIpv6"); ok {
		template.EnableIPv6 = value
	}
	template.ReservedPublicIP = defaultString(defaultString(stringFromMap(network, "reservedPublicIp"), stringFromMap(raw, "reservedPublicIp")), template.ReservedPublicIP)
	template.SSHKey = defaultString(defaultString(stringFromMap(network, "sshKey"), stringFromMap(raw, "sshKey")), template.SSHKey)
	template.CloudInit = defaultString(defaultString(stringFromMap(network, "cloudInit"), stringFromMap(raw, "cloudInit")), template.CloudInit)
	if len(tags) > 0 {
		template.Tags = stringMapFromAny(tags)
	}
}

func mapFromAny(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func stringMapFromAny(in map[string]any) map[string]string {
	out := map[string]string{}
	for key, value := range in {
		if str, ok := value.(string); ok {
			out[key] = str
		}
	}
	return out
}

func boolValueFromMaps(primary, fallback map[string]any, key string) (bool, bool) {
	if value, ok := primary[key].(bool); ok {
		return value, true
	}
	if value, ok := fallback[key].(bool); ok {
		return value, true
	}
	return false, false
}

func parseSimpleYAML(input string) (map[string]any, error) {
	root := map[string]any{}
	var current map[string]any
	for _, line := range strings.Split(input, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid yaml line: %s", strings.TrimSpace(line))
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if indent == 0 && value == "" {
			child := map[string]any{}
			root[key] = child
			current = child
			continue
		}
		target := root
		if indent > 0 && current != nil {
			target = current
		}
		target[key] = parseScalar(value)
	}
	return root, nil
}

func parseScalar(value string) any {
	value = strings.Trim(strings.TrimSpace(value), `"'`)
	switch strings.ToLower(value) {
	case "true":
		return true
	case "false":
		return false
	}
	var number int
	if _, err := fmt.Sscanf(value, "%d", &number); err == nil && fmt.Sprintf("%d", number) == value {
		return number
	}
	return value
}

func markTemplateStale(currentStatus string, previous domain.InstanceTemplate) string {
	status := normalizeValidationStatus(currentStatus)
	if status == "VALID" {
		return "STALE"
	}
	_ = previous
	return status
}

func cleanTags(tags map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range tags {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func validateRetryPolicy(req domain.CreateInstanceRequest) error {
	if req.RetryDelayMinSec < 0 || req.RetryDelayMaxSec < 0 {
		return fmt.Errorf("%w: retry delay cannot be negative", ErrValidation)
	}
	if req.RetryDelayMaxSec > 0 && req.RetryDelayMaxSec < req.RetryDelayMinSec {
		return fmt.Errorf("%w: retryDelayMaxSeconds cannot be less than retryDelayMinSeconds", ErrValidation)
	}
	switch normalizedRetryMode(req) {
	case "none", "success_stop":
		return nil
	case "count":
		if effectiveRetryMaxAttempts(req) <= 0 {
			return fmt.Errorf("%w: retryMaxAttempts must be greater than zero", ErrValidation)
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported retryMode", ErrValidation)
	}
}

func normalizedRetryMode(req domain.CreateInstanceRequest) string {
	mode := strings.TrimSpace(req.RetryMode)
	if mode == "" {
		if req.MaxRetries > 0 {
			return "count"
		}
		return "none"
	}
	return mode
}

func effectiveRetryMaxAttempts(req domain.CreateInstanceRequest) int {
	if req.RetryMaxAttempts > 0 {
		return req.RetryMaxAttempts
	}
	return req.MaxRetries
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func validBootVolumeVPUs(value int) bool {
	return value >= 10 && value <= 120
}

func launchRegions(profiles []domain.Profile, templates []domain.InstanceTemplate) []domain.LaunchOption {
	seen := map[string]bool{}
	out := []domain.LaunchOption{}
	add := func(region string) {
		region = strings.TrimSpace(region)
		if region == "" || seen[region] {
			return
		}
		seen[region] = true
		out = append(out, domain.LaunchOption{ID: region, Label: region, Region: region})
	}
	for _, profile := range profiles {
		add(profile.DefaultRegion)
	}
	for _, template := range templates {
		add(template.Region)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func launchCompartments(templates []domain.InstanceTemplate) []domain.LaunchOption {
	seen := map[string]bool{}
	out := []domain.LaunchOption{}
	for _, template := range templates {
		compartment := strings.TrimSpace(template.Compartment)
		if compartment == "" || seen[compartment] {
			continue
		}
		seen[compartment] = true
		out = append(out, domain.LaunchOption{ID: compartment, Label: compartment, Compartment: compartment})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func launchShapes(templates []domain.InstanceTemplate) []domain.ShapeOption {
	seen := map[string]domain.ShapeOption{}
	for _, template := range templates {
		shape := strings.TrimSpace(template.Shape)
		if shape == "" {
			continue
		}
		option := seen[shape]
		if option.Name == "" {
			option = domain.ShapeOption{Name: shape, Arch: "unknown"}
		}
		if option.MinOCPUs == 0 || template.OCPUs < option.MinOCPUs {
			option.MinOCPUs = template.OCPUs
		}
		if template.OCPUs > option.MaxOCPUs {
			option.MaxOCPUs = template.OCPUs
		}
		if option.MinMemoryGB == 0 || template.MemoryGB < option.MinMemoryGB {
			option.MinMemoryGB = template.MemoryGB
		}
		if template.MemoryGB > option.MaxMemoryGB {
			option.MaxMemoryGB = template.MemoryGB
		}
		seen[shape] = option
	}
	out := make([]domain.ShapeOption, 0, len(seen))
	for _, option := range seen {
		out = append(out, option)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (s *Store) recordAuditLocked(entry domain.AuditLog) error {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = s.now().UTC()
	}
	if entry.ID == 0 {
		entry.ID = s.nextAuditIDLocked()
	}
	s.auditLogs = append(s.auditLogs, entry)
	if len(s.auditLogs) > 1000 {
		s.auditLogs = append([]domain.AuditLog(nil), s.auditLogs[len(s.auditLogs)-1000:]...)
	}
	if s.sink == nil {
		return nil
	}
	return s.sink.RecordAudit(entry)
}

func filterAuditLogs(entries []domain.AuditLog, filter domain.AuditLogFilter) []domain.AuditLog {
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	status := strings.ToLower(strings.TrimSpace(filter.Status))
	out := make([]domain.AuditLog, 0, min(limit, len(entries)))
	sort.Slice(entries, func(i, j int) bool { return entries[i].CreatedAt.After(entries[j].CreatedAt) })
	for _, entry := range entries {
		if !auditMatches(entry.Actor, filter.Actor) ||
			!auditMatches(entry.Action, filter.Action) ||
			!auditMatches(entry.ResourceType, filter.ResourceType) ||
			!auditMatches(entry.ResourceID, filter.ResourceID) ||
			!auditMatches(entry.ProfileID, filter.ProfileID) ||
			!auditMatches(entry.Region, filter.Region) ||
			!auditMatches(entry.CompartmentID, filter.CompartmentID) ||
			!auditMatches(entry.OCIRequestID, filter.OCIRequestID) ||
			!auditMatches(entry.OCIWorkRequestID, filter.OCIWorkRequestID) {
			continue
		}
		if status == "failed" && entry.ErrorCode == "" && entry.ErrorMessage == "" {
			continue
		}
		if status == "success" && (entry.ErrorCode != "" || entry.ErrorMessage != "") {
			continue
		}
		out = append(out, entry)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func auditMatches(value, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(want))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *Store) nextJobIDLocked() string {
	id := fmt.Sprintf("JOB-%d", s.nextJob)
	s.nextJob++
	return id
}

func (s *Store) nextProfileIDLocked(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.ReplaceAll(slug, " ", "-")
	if slug == "" {
		slug = "profile"
	}
	base := "profile-" + slug
	id := base
	for i := 2; ; i++ {
		if _, exists := s.profiles[id]; !exists {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
}

func (s *Store) nextTemplateIDLocked(name string) string {
	slug := slugifyIDPart(name)
	if slug == "" {
		slug = "template"
	}
	base := "tpl-" + slug
	id := base
	for i := 2; ; i++ {
		if _, exists := s.templates[id]; !exists {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
}

func (s *Store) profileByIDOrNameLocked(id string) (domain.Profile, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Profile{}, false
	}
	if profile, ok := s.profiles[id]; ok {
		return profile, true
	}
	for _, profile := range s.profiles {
		if strings.EqualFold(profile.Name, id) {
			return profile, true
		}
	}
	return domain.Profile{}, false
}

func (s *Store) nextInstanceIDLocked(name string) string {
	slug := slugifyIDPart(name)
	if slug == "" {
		slug = "instance"
	}
	id := fmt.Sprintf("inst-%s-%02d", slug, s.nextInst)
	s.nextInst++
	return id
}

func (s *Store) nextRuleIDLocked() string {
	id := fmt.Sprintf("auto-%d", s.nextRule)
	s.nextRule++
	return id
}

func (s *Store) nextNotificationIDLocked() string {
	id := fmt.Sprintf("notice-%d", s.nextNotice)
	s.nextNotice++
	return id
}

func slugifyIDPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		switch {
		case isAlphaNum:
			builder.WriteRune(r)
			lastDash = false
		case !lastDash:
			builder.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func (s *Store) nextAuditIDLocked() int64 {
	id := s.nextAudit
	s.nextAudit++
	return id
}

func numericIDSuffix(id string, prefix string) int {
	id = strings.TrimSpace(id)
	prefix = strings.TrimSpace(prefix)
	if prefix != "" && !strings.HasPrefix(id, prefix) {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimPrefix(id, prefix))
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func redactEmailSettings(settings domain.EmailSettings) domain.EmailSettings {
	passwordSet := settings.PasswordSet || strings.TrimSpace(settings.Password) != ""
	settings.Password = ""
	settings.PasswordSet = passwordSet
	return settings
}

func redactWebhookSettings(settings domain.WebhookSettings) domain.WebhookSettings {
	secretSet := settings.SecretSet || strings.TrimSpace(settings.Secret) != ""
	settings.Secret = ""
	settings.SecretSet = secretSet
	return settings
}

func redactAccountSettings(settings domain.AccountSettings) domain.AccountSettings {
	settings = normalizeAccountSettings(settings)
	settings.PasswordHash = ""
	return settings
}

func normalizeAccountSettings(settings domain.AccountSettings) domain.AccountSettings {
	settings.DisplayName = strings.TrimSpace(settings.DisplayName)
	settings.Email = strings.TrimSpace(settings.Email)
	settings.Avatar = strings.TrimSpace(settings.Avatar)
	if settings.DisplayName == "" {
		settings.DisplayName = "Administrator"
	}
	settings.AvatarInitial = accountInitial(settings.DisplayName, settings.Email)
	settings.PasswordSet = settings.PasswordSet || strings.TrimSpace(settings.PasswordHash) != ""
	return settings
}

func accountInitial(name, email string) string {
	source := strings.TrimSpace(name)
	if source == "" {
		source = strings.TrimSpace(email)
	}
	if source == "" {
		return "A"
	}
	return strings.ToUpper(string([]rune(source)[0]))
}

func normalizeAppearanceSettings(settings domain.AppearanceSettings) domain.AppearanceSettings {
	settings.Theme = strings.ToLower(strings.TrimSpace(settings.Theme))
	if settings.Theme != "dark" {
		settings.Theme = "light"
	}
	settings.BackgroundMode = strings.ToLower(strings.TrimSpace(settings.BackgroundMode))
	switch settings.BackgroundMode {
	case "aurora", "plain", "image":
	default:
		settings.BackgroundMode = "aurora"
	}
	settings.BackgroundImage = strings.TrimSpace(settings.BackgroundImage)
	if settings.BackgroundMode != "image" {
		settings.BackgroundImage = ""
	}
	switch strings.TrimSpace(settings.Language) {
	case "en-US":
		settings.Language = "en-US"
	default:
		settings.Language = "zh-CN"
	}
	return settings
}

func normalizeBudgetSettings(settings domain.BudgetSettings, now time.Time) domain.BudgetSettings {
	if settings.MonthlyBudgetUSD <= 0 {
		settings.MonthlyBudgetUSD = 10
	}
	if settings.ThresholdPercent <= 0 {
		settings.ThresholdPercent = 90
	}
	settings.ActualSpendUSD = maxFloat(0, settings.ActualSpendUSD)
	settings.ForecastSpendUSD = maxFloat(0, settings.ForecastSpendUSD)
	settings.ScopeMode = strings.ToLower(strings.TrimSpace(settings.ScopeMode))
	switch settings.ScopeMode {
	case "tag", "compartment", "pool", "manual":
	default:
		settings.ScopeMode = "tag"
	}
	settings.ProfileID = strings.TrimSpace(settings.ProfileID)
	settings.Region = strings.TrimSpace(settings.Region)
	settings.CompartmentID = strings.TrimSpace(settings.CompartmentID)
	settings.ResourcePool = strings.TrimSpace(settings.ResourcePool)
	settings.TagKey = strings.TrimSpace(settings.TagKey)
	settings.TagValue = strings.TrimSpace(settings.TagValue)
	settings.ManualInstanceIDs = cleanStringList(settings.ManualInstanceIDs)
	settings.ActionMode = strings.ToLower(strings.TrimSpace(settings.ActionMode))
	switch settings.ActionMode {
	case "notify", "downgrade", "delete":
	default:
		settings.ActionMode = "downgrade"
	}
	settings.DowngradePreset = strings.ToLower(strings.TrimSpace(settings.DowngradePreset))
	switch settings.DowngradePreset {
	case "free-first", "min-flex", "custom", "stop-only":
	default:
		settings.DowngradePreset = "free-first"
	}
	if settings.ProfileID == "" {
		settings.ProfileID = "DEFAULT"
	}
	if settings.TagKey == "" {
		settings.TagKey = "budget.autoAction"
	}
	if settings.TagValue == "" {
		settings.TagValue = "enabled"
	}
	if settings.UpdatedAt.IsZero() && !now.IsZero() {
		settings.UpdatedAt = now.UTC()
	}
	return settings
}

func cleanStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" || seen[strings.ToLower(part)] {
				continue
			}
			seen[strings.ToLower(part)] = true
			out = append(out, part)
		}
	}
	return out
}

func defaultAccessControlSettings(now time.Time) domain.AccessControlSettings {
	return domain.AccessControlSettings{
		Enabled:   true,
		Roles:     defaultAccessRoles(),
		Users:     []domain.AccessUser{{ID: "admin", DisplayName: "Administrator", RoleID: "super_admin", Status: "active", UpdatedAt: now}},
		UpdatedAt: now,
	}
}

func defaultAccessRoles() []domain.AccessRole {
	return []domain.AccessRole{
		{ID: "super_admin", Name: "超级管理员", Description: "拥有平台全部权限，可管理密钥、用户、护栏和所有 OCI 操作。", Permissions: []string{"*"}, System: true},
		{ID: "ops_admin", Name: "运维管理员", Description: "可管理实例、网络、模板、任务和自动化，但不能修改用户权限。", Permissions: []string{"profile:read", "instance:*", "network:*", "template:*", "job:*", "automation:*", "notification:*", "audit:read", "budget:read", "guardrail:read"}, System: true},
		{ID: "operator", Name: "普通操作员", Description: "可查看资源并执行启动、停止、重启、任务查看等低风险操作。", Permissions: []string{"profile:read", "instance:read", "instance:operate", "job:read", "notification:read"}, System: true},
		{ID: "auditor", Name: "审计员", Description: "只读访问任务、通知和审计日志。", Permissions: []string{"job:read", "notification:read", "audit:read", "guardrail:read"}, System: true},
	}
}

func defaultSecurityGuardrails(now time.Time) domain.SecurityGuardrailSettings {
	return domain.SecurityGuardrailSettings{
		Enabled:                       true,
		MaxOCPUsPerInstance:           4,
		MaxMemoryGBPerInstance:        24,
		MaxBootVolumeGB:               200,
		MaxRetryAttempts:              20,
		MaxPublicIPBatchCount:         10,
		RequireApprovalForTerminate:   true,
		BlockBootVolumeDeletion:       false,
		BlockPublicIPv6RouteChanges:   false,
		BlockRootPasswordWithoutEmail: true,
		RequireTemplateForLaunch:      false,
		UpdatedAt:                     now,
	}
}

func normalizeAccessControlSettings(settings domain.AccessControlSettings, now time.Time) domain.AccessControlSettings {
	if len(settings.Roles) == 0 {
		settings.Roles = defaultAccessRoles()
	}
	if len(settings.Users) == 0 {
		settings.Users = defaultAccessControlSettings(now.UTC()).Users
	}
	settings.Enabled = true
	for index := range settings.Users {
		settings.Users[index].ID = sanitizeIdentifier(settings.Users[index].ID)
		if settings.Users[index].ID == "" {
			settings.Users[index].ID = fmt.Sprintf("user-%d", index+1)
		}
		settings.Users[index].DisplayName = strings.TrimSpace(settings.Users[index].DisplayName)
		if settings.Users[index].DisplayName == "" {
			settings.Users[index].DisplayName = settings.Users[index].ID
		}
		settings.Users[index].Email = strings.TrimSpace(settings.Users[index].Email)
		settings.Users[index].RoleID = sanitizeIdentifier(defaultString(settings.Users[index].RoleID, "operator"))
		settings.Users[index].Status = strings.ToLower(strings.TrimSpace(defaultString(settings.Users[index].Status, "active")))
		if settings.Users[index].Status != "active" && settings.Users[index].Status != "disabled" {
			settings.Users[index].Status = "active"
		}
		settings.Users[index].AllowedProfiles = cleanStringList(settings.Users[index].AllowedProfiles)
		settings.Users[index].AllowedRegions = cleanStringList(settings.Users[index].AllowedRegions)
		settings.Users[index].AllowedCompartments = cleanStringList(settings.Users[index].AllowedCompartments)
		settings.Users[index].PasswordSet = strings.TrimSpace(settings.Users[index].PasswordHash) != ""
		if settings.Users[index].UpdatedAt.IsZero() {
			settings.Users[index].UpdatedAt = now.UTC()
		}
	}
	if settings.UpdatedAt.IsZero() {
		settings.UpdatedAt = now.UTC()
	}
	return settings
}

func normalizeSecurityGuardrails(settings domain.SecurityGuardrailSettings, now time.Time) domain.SecurityGuardrailSettings {
	defaults := defaultSecurityGuardrails(now.UTC())
	if settings.MaxOCPUsPerInstance <= 0 {
		settings.MaxOCPUsPerInstance = defaults.MaxOCPUsPerInstance
	}
	if settings.MaxMemoryGBPerInstance <= 0 {
		settings.MaxMemoryGBPerInstance = defaults.MaxMemoryGBPerInstance
	}
	if settings.MaxBootVolumeGB <= 0 {
		settings.MaxBootVolumeGB = defaults.MaxBootVolumeGB
	}
	if settings.MaxRetryAttempts <= 0 {
		settings.MaxRetryAttempts = defaults.MaxRetryAttempts
	}
	if settings.MaxPublicIPBatchCount <= 0 {
		settings.MaxPublicIPBatchCount = defaults.MaxPublicIPBatchCount
	}
	settings.AllowedRegions = cleanStringList(settings.AllowedRegions)
	settings.DeniedRegions = cleanStringList(settings.DeniedRegions)
	if settings.UpdatedAt.IsZero() {
		settings.UpdatedAt = now.UTC()
	}
	return settings
}

func redactAccessSettings(settings domain.AccessControlSettings) domain.AccessControlSettings {
	users := make([]domain.AccessUser, len(settings.Users))
	copy(users, settings.Users)
	settings.Users = users
	for index := range settings.Users {
		settings.Users[index] = redactAccessUser(settings.Users[index])
	}
	return settings
}

func redactAccessUser(user domain.AccessUser) domain.AccessUser {
	user.PasswordSet = strings.TrimSpace(user.PasswordHash) != ""
	user.PasswordHash = ""
	return user
}

func accessRoleByID(roles []domain.AccessRole, roleID string) domain.AccessRole {
	for _, role := range roles {
		if role.ID == roleID {
			return role
		}
	}
	return domain.AccessRole{}
}

func permissionAllowed(permissions []string, permission string) bool {
	permission = strings.TrimSpace(permission)
	for _, candidate := range permissions {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == permission {
			return true
		}
		if strings.HasSuffix(candidate, ":*") && strings.HasPrefix(permission, strings.TrimSuffix(candidate, "*")) {
			return true
		}
	}
	return false
}

func scopeAllowed(allowed []string, value string) bool {
	if len(allowed) == 0 {
		return true
	}
	value = strings.TrimSpace(value)
	for _, candidate := range allowed {
		if candidate == "*" || strings.EqualFold(candidate, value) {
			return true
		}
	}
	return false
}

func sanitizeIdentifier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "\\", "-")
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, "..", "-")
	var out strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '@' {
			out.WriteRune(r)
		}
	}
	return strings.Trim(out.String(), ".-_")
}

func maxFloat(minValue float64, value float64) float64 {
	if value < minValue {
		return minValue
	}
	return value
}

func isFlexibleShapeName(shape string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(shape)), ".flex")
}

func cleanRecipients(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" || seen[strings.ToLower(part)] {
				continue
			}
			seen[strings.ToLower(part)] = true
			out = append(out, part)
		}
	}
	return out
}

func redactSensitiveJobInput(job *domain.Job) {
	if job == nil || job.Input == nil {
		return
	}
	if !boolFromMap(job.Input, "cloudInitSensitive") {
		return
	}
	delete(job.Input, "cloudInit")
	job.Input["cloudInitRedacted"] = true
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
