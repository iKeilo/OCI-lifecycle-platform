package store

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"a-series-oracle/backend/internal/domain"
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
}

type PersistenceSink interface {
	SaveProfile(profile domain.Profile, secret domain.ProfileSecret) error
	SaveJob(job domain.Job) error
	SaveInstance(instance domain.Instance) error
	RecordAudit(entry domain.AuditLog) error
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
}

type settingsReader interface {
	GetEmailSettings() (domain.EmailSettings, error)
	GetWebhookSettings() (domain.WebhookSettings, error)
	GetAccountSettings() (domain.AccountSettings, error)
	GetAppearanceSettings() (domain.AppearanceSettings, error)
	GetBudgetSettings() (domain.BudgetSettings, error)
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

	for _, template := range []domain.InstanceTemplate{
		{
			ID:             "tpl-ubuntu-a1-small-v1",
			Name:           "Ubuntu A1 小型实例",
			Version:        "v1",
			ProfileID:      "profile-default",
			Region:         "ap-singapore-1",
			Compartment:    "production",
			ImageID:        "ocid1.image.oc1.ap-singapore-1.ubuntu2204",
			ImageName:      "Canonical Ubuntu 22.04",
			Shape:          "VM.Standard.A1.Flex",
			OCPUs:          1,
			MemoryGB:       6,
			BootVolumeGB:   80,
			VCNID:          "vcn-production",
			SubnetID:       "subnet-public-a",
			AssignPublicIP: true,
			Tags:           map[string]string{"owner": "ops", "purpose": "compute"},
			Status:         "Active",
			CreatedBy:      "system",
			CreatedAt:      now.Add(-7 * 24 * time.Hour),
		},
		{
			ID:             "tpl-oracle-micro-v1",
			Name:           "Oracle Linux 微型实例",
			Version:        "v1",
			ProfileID:      "profile-default",
			Region:         "ap-singapore-1",
			Compartment:    "development",
			ImageID:        "ocid1.image.oc1.ap-singapore-1.oraclelinux9",
			ImageName:      "Oracle Linux 9",
			Shape:          "VM.Standard.E2.1.Micro",
			OCPUs:          1,
			MemoryGB:       1,
			BootVolumeGB:   50,
			VCNID:          "vcn-development",
			SubnetID:       "subnet-public-a",
			AssignPublicIP: true,
			Tags:           map[string]string{"owner": "dev", "purpose": "test"},
			Status:         "Active",
			CreatedBy:      "system",
			CreatedAt:      now.Add(-5 * 24 * time.Hour),
		},
		{
			ID:             "tpl-edge-flex-v1",
			Name:           "边缘服务 Flex 实例",
			Version:        "v1",
			ProfileID:      "profile-capacity-lab",
			Region:         "ap-seoul-1",
			Compartment:    "edge",
			ImageID:        "ocid1.image.oc1.ap-seoul-1.ubuntu2204",
			ImageName:      "Canonical Ubuntu 22.04",
			Shape:          "VM.Standard3.Flex",
			OCPUs:          2,
			MemoryGB:       16,
			BootVolumeGB:   100,
			VCNID:          "vcn-edge",
			SubnetID:       "subnet-public-a",
			AssignPublicIP: true,
			Tags:           map[string]string{"owner": "edge", "purpose": "gateway"},
			Status:         "Active",
			CreatedBy:      "system",
			CreatedAt:      now.Add(-3 * 24 * time.Hour),
		},
	} {
		s.templates[template.ID] = template
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	templates := make([]domain.InstanceTemplate, 0, len(s.templates))
	for _, template := range s.templates {
		templates = append(templates, template)
	}
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].Region == templates[j].Region {
			return templates[i].Name < templates[j].Name
		}
		return templates[i].Region < templates[j].Region
	})
	return templates
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
		}
		items = append(items, notification)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items
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

func (s *Store) CreateInstanceTask(req domain.CreateInstanceRequest, actor string) (domain.CreateInstanceResponse, error) {
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
	if strings.TrimSpace(req.TemplateID) != "" {
		template, ok := s.templates[req.TemplateID]
		if !ok {
			return domain.CreateInstanceResponse{}, fmt.Errorf("%w: template not found", ErrValidation)
		}
		if strings.TrimSpace(req.ImageID) == "" {
			req.ImageID = template.ImageID
		}
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
			!auditMatches(entry.ProfileID, filter.ProfileID) {
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
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.ReplaceAll(slug, " ", "-")
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

func (s *Store) nextAuditIDLocked() int64 {
	id := s.nextAudit
	s.nextAudit++
	return id
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
