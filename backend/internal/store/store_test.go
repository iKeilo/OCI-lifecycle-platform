package store

import (
	"strings"
	"testing"

	"a-series-oracle/backend/internal/domain"
)

type fakeSink struct {
	profiles           []domain.Profile
	jobs               []domain.Job
	deletedJobs        []string
	instances          []domain.Instance
	audits             []domain.AuditLog
	notifications      []domain.Notification
	deletedNotices     []string
	emailSettings      domain.EmailSettings
	webhookSettings    domain.WebhookSettings
	accountSettings    domain.AccountSettings
	appearanceSettings domain.AppearanceSettings
	budgetSettings     domain.BudgetSettings
	accessSettings     domain.AccessControlSettings
	guardrailSettings  domain.SecurityGuardrailSettings
}

func (s *fakeSink) SaveProfile(profile domain.Profile, secret domain.ProfileSecret) error {
	s.profiles = append(s.profiles, profile)
	return nil
}

func (s *fakeSink) SaveJob(job domain.Job) error {
	s.jobs = append(s.jobs, job)
	return nil
}

func (s *fakeSink) DeleteJobs(jobIDs []string) error {
	s.deletedJobs = append(s.deletedJobs, jobIDs...)
	return nil
}

func (s *fakeSink) SaveInstance(instance domain.Instance) error {
	s.instances = append(s.instances, instance)
	return nil
}

func (s *fakeSink) RecordAudit(entry domain.AuditLog) error {
	s.audits = append(s.audits, entry)
	return nil
}

func (s *fakeSink) SaveNotification(notification domain.Notification) error {
	for index, existing := range s.notifications {
		if existing.ID == notification.ID {
			s.notifications[index] = notification
			return nil
		}
	}
	s.notifications = append(s.notifications, notification)
	return nil
}

func (s *fakeSink) DeleteNotification(notificationID string) error {
	s.deletedNotices = append(s.deletedNotices, notificationID)
	for index, existing := range s.notifications {
		if existing.ID == notificationID {
			s.notifications = append(s.notifications[:index], s.notifications[index+1:]...)
			return nil
		}
	}
	return nil
}

func (s *fakeSink) ListNotifications() ([]domain.Notification, error) {
	return append([]domain.Notification(nil), s.notifications...), nil
}

func (s *fakeSink) SaveEmailSettings(settings domain.EmailSettings) error {
	s.emailSettings = settings
	return nil
}

func (s *fakeSink) GetEmailSettings() (domain.EmailSettings, error) {
	return s.emailSettings, nil
}

func (s *fakeSink) SaveWebhookSettings(settings domain.WebhookSettings) error {
	s.webhookSettings = settings
	return nil
}

func (s *fakeSink) GetWebhookSettings() (domain.WebhookSettings, error) {
	return s.webhookSettings, nil
}

func (s *fakeSink) SaveAccountSettings(settings domain.AccountSettings) error {
	s.accountSettings = settings
	return nil
}

func (s *fakeSink) GetAccountSettings() (domain.AccountSettings, error) {
	return s.accountSettings, nil
}

func (s *fakeSink) SaveAppearanceSettings(settings domain.AppearanceSettings) error {
	s.appearanceSettings = settings
	return nil
}

func (s *fakeSink) GetAppearanceSettings() (domain.AppearanceSettings, error) {
	return s.appearanceSettings, nil
}

func (s *fakeSink) SaveBudgetSettings(settings domain.BudgetSettings) error {
	s.budgetSettings = settings
	return nil
}

func (s *fakeSink) GetBudgetSettings() (domain.BudgetSettings, error) {
	return s.budgetSettings, nil
}

func (s *fakeSink) SaveAccessControlSettings(settings domain.AccessControlSettings) error {
	s.accessSettings = settings
	return nil
}

func (s *fakeSink) GetAccessControlSettings() (domain.AccessControlSettings, error) {
	return s.accessSettings, nil
}

func (s *fakeSink) SaveSecurityGuardrailSettings(settings domain.SecurityGuardrailSettings) error {
	s.guardrailSettings = settings
	return nil
}

func (s *fakeSink) GetSecurityGuardrailSettings() (domain.SecurityGuardrailSettings, error) {
	return s.guardrailSettings, nil
}

func TestCompleteStopJobUpdatesInstanceStatus(t *testing.T) {
	s := NewSeeded()
	job, err := s.CreateInstanceActionTask("inst-prod-web-01", domain.InstanceActionRequest{
		Action:             domain.InstanceActionStop,
		Graceful:           true,
		PreserveBootVolume: true,
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartJob(job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MarkJobWaitingOCI(job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MarkJobVerifying(job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteJob(job.ID, map[string]any{"verified": true}); err != nil {
		t.Fatal(err)
	}

	instance, ok := s.GetInstance("inst-prod-web-01")
	if !ok {
		t.Fatal("expected instance")
	}
	if instance.Status != domain.InstanceStopped {
		t.Fatalf("expected stopped instance, got %s", instance.Status)
	}
}

func TestCompleteResizeJobUpdatesInstanceShape(t *testing.T) {
	s := NewSeeded()
	job, err := s.CreateInstanceActionTask("inst-prod-web-01", domain.InstanceActionRequest{
		Action:             domain.InstanceActionResize,
		TargetShape:        "VM.Standard3.Flex",
		TargetOCPUs:        2,
		TargetMemoryGB:     16,
		TargetBootVolumeGB: 110,
		ExpandBootVolume:   true,
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartJob(job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MarkJobWaitingOCI(job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MarkJobVerifying(job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteJob(job.ID, map[string]any{"verified": true}); err != nil {
		t.Fatal(err)
	}

	instance, ok := s.GetInstance("inst-prod-web-01")
	if !ok {
		t.Fatal("expected instance")
	}
	if instance.Shape != "VM.Standard3.Flex" || instance.OCPUs != 2 || instance.MemoryGB != 16 || instance.BootVolumeGB != 110 {
		t.Fatalf("unexpected resized instance: %#v", instance)
	}
}

func TestCreateResizeTaskRejectsBootVolumeShrink(t *testing.T) {
	s := NewSeeded()
	_, err := s.CreateInstanceActionTask("inst-prod-web-01", domain.InstanceActionRequest{
		Action:             domain.InstanceActionResize,
		TargetShape:        "VM.Standard3.Flex",
		TargetOCPUs:        2,
		TargetMemoryGB:     16,
		TargetBootVolumeGB: 90,
		ExpandBootVolume:   true,
	}, "tester")
	if err == nil {
		t.Fatal("expected boot volume shrink to be rejected")
	}
}

func TestCreateOCIInstanceActionTaskDoesNotRequireLocalInstance(t *testing.T) {
	s := NewSeeded()
	instanceID := "ocid1.instance.oc1.ap-chuncheon-1.example"
	job, err := s.CreateOCIInstanceActionTask(instanceID, domain.InstanceActionRequest{
		Action:             domain.InstanceActionStop,
		Graceful:           true,
		PreserveBootVolume: true,
	}, "tester", "DEFAULT", "ap-chuncheon-1", "ocid1.tenancy.oc1..example")
	if err != nil {
		t.Fatal(err)
	}

	if job.ResourceID != instanceID || job.Input["ociInstanceId"] != instanceID {
		t.Fatalf("unexpected OCI job: %#v", job)
	}
	if _, err := s.SetJobOCIRefs(job.ID, "request-1", "work-1"); err != nil {
		t.Fatal(err)
	}
	updated, ok := s.GetJob(job.ID)
	if !ok {
		t.Fatal("expected job")
	}
	if updated.OCIRequestID != "request-1" || updated.OCIWorkRequestID != "work-1" {
		t.Fatalf("expected OCI refs to be persisted, got %#v", updated)
	}
}

func TestCreateOCIInstanceReinstallTaskRecordsSafeInput(t *testing.T) {
	s := NewSeeded()
	instanceID := "ocid1.instance.oc1.ap-chuncheon-1.example"
	job, err := s.CreateOCIInstanceReinstallTask(instanceID, domain.InstanceReinstallRequest{
		ImageID:               "ocid1.image.oc1.ap-chuncheon-1.example",
		ImageName:             "Oracle-Linux-9",
		BootVolumeSizeGB:      60,
		BootVolumeVPUsPerGB:   20,
		PreserveOldBootVolume: true,
		ConfirmationName:      "not-required-for-uncached-oci-instance",
	}, "tester", "DEFAULT", "ap-chuncheon-1", "ocid1.tenancy.oc1..example")
	if err != nil {
		t.Fatal(err)
	}
	if job.Type != "重装系统" || job.ResourceID != instanceID || job.Input["operation"] != "reinstall" {
		t.Fatalf("unexpected reinstall job: %#v", job)
	}
	if job.Input["generateRootPassword"] != false || job.Input["cloudInitSensitive"] != false {
		t.Fatalf("reinstall job must not claim password injection support: %#v", job.Input)
	}
}

func TestCreateOCIInstanceReinstallTaskRejectsPasswordInjection(t *testing.T) {
	s := NewSeeded()
	_, err := s.CreateOCIInstanceReinstallTask("ocid1.instance.oc1.ap-chuncheon-1.example", domain.InstanceReinstallRequest{
		ImageID:              "ocid1.image.oc1.ap-chuncheon-1.example",
		BootVolumeSizeGB:     50,
		GenerateRootPassword: true,
	}, "tester", "DEFAULT", "ap-chuncheon-1", "ocid1.tenancy.oc1..example")
	if err == nil {
		t.Fatal("expected password injection to be rejected")
	}
}

func TestPersistenceSinkReceivesJobCreatesAndUpdates(t *testing.T) {
	s := NewSeeded()
	sink := &fakeSink{}
	s.SetPersistenceSink(sink)

	job, err := s.CreateOCIInstanceLaunchTask(domain.CreateInstanceRequest{
		Name:         "codex-test",
		Shape:        "VM.Standard.E3.Flex",
		OCPUs:        1,
		MemoryGB:     1,
		BootVolumeGB: 50,
		Region:       "ap-chuncheon-1",
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if len(sink.jobs) != 1 {
		t.Fatalf("expected create to persist one job, got %d", len(sink.jobs))
	}
	if sink.jobs[0].ID != job.ID || sink.jobs[0].Input["operation"] != "launch" {
		t.Fatalf("unexpected persisted launch job: %#v", sink.jobs[0])
	}

	if _, err := s.StartJob(job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetJobOCIRefs(job.ID, "request-1", "work-1"); err != nil {
		t.Fatal(err)
	}
	if len(sink.jobs) != 3 {
		t.Fatalf("expected create/start/oci-ref persistence, got %d writes", len(sink.jobs))
	}
	last := sink.jobs[len(sink.jobs)-1]
	if last.OCIRequestID != "request-1" || last.OCIWorkRequestID != "work-1" {
		t.Fatalf("expected OCI refs in persisted job, got %#v", last)
	}
	if len(sink.audits) != len(sink.jobs) {
		t.Fatalf("expected matching audit writes, got jobs=%d audits=%d", len(sink.jobs), len(sink.audits))
	}
}

func TestCreateInstanceTaskPersistsInstance(t *testing.T) {
	s := NewSeeded()
	sink := &fakeSink{}
	s.SetPersistenceSink(sink)

	result, err := s.CreateInstanceTask(domain.CreateInstanceRequest{
		Name:         "local-created",
		ProfileID:    "profile-default",
		Shape:        "VM.Standard.E3.Flex",
		OCPUs:        1,
		MemoryGB:     1,
		BootVolumeGB: 50,
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if len(sink.instances) != 1 {
		t.Fatalf("expected one persisted instance, got %d", len(sink.instances))
	}
	if sink.instances[0].ID != result.Instance.ID || sink.instances[0].Status != domain.InstanceProvisioning {
		t.Fatalf("unexpected persisted instance: %#v", sink.instances[0])
	}
	if result.Job.MaxRetries != 0 || result.Job.Input["retryMode"] != "none" {
		t.Fatalf("expected no retry defaults to be preserved, got job=%#v", result.Job)
	}
}

func TestCreateInstanceTaskMergesTemplatePrefillAndRecordsSource(t *testing.T) {
	s := NewSeeded()
	template, err := s.CreateTemplate(domain.CreateTemplateRequest{
		Name:                "template-prefill",
		ProfileID:           "profile-default",
		Region:              "ap-chuncheon-1",
		Compartment:         "development",
		CompartmentID:       "ocid1.compartment.oc1..template",
		AvailabilityAD:      "AD-1",
		ImageID:             "ocid1.image.oc1..template",
		Shape:               "VM.Standard.E3.Flex",
		OCPUs:               1,
		MemoryGB:            1,
		BootVolumeGB:        50,
		BootVolumeVPUsPerGB: 20,
		VCNID:               "ocid1.vcn.oc1..template",
		SubnetID:            "ocid1.subnet.oc1..template",
		AssignPublicIP:      true,
		EnableIPv6:          true,
		SSHKey:              "ssh-rsa template",
		Tags: map[string]string{
			"owner": "template",
		},
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if template.ConfigFormat != "json" || template.ConfigText == "" {
		t.Fatalf("expected generated json template config, got format=%q text=%q", template.ConfigFormat, template.ConfigText)
	}

	result, err := s.CreateInstanceTask(domain.CreateInstanceRequest{
		TemplateID: template.ID,
		Name:       "from-template",
		MemoryGB:   2,
		Tags: map[string]string{
			"owner": "request",
		},
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}

	if result.Instance.Shape != template.Shape || result.Instance.OCPUs != template.OCPUs || result.Instance.MemoryGB != 2 {
		t.Fatalf("expected template fields with request override, got instance=%#v template=%#v", result.Instance, template)
	}
	if result.Job.Input["templateId"] != template.ID || result.Job.Input["templateVersion"] != template.Version {
		t.Fatalf("expected template source in job input, got %#v", result.Job.Input)
	}
	if result.Job.Input["region"] != template.Region || result.Job.Input["imageId"] != template.ImageID || result.Job.Input["subnetId"] != template.SubnetID {
		t.Fatalf("expected template prefill in job input, got %#v", result.Job.Input)
	}
	overrides, ok := result.Job.Input["templateOverrides"].(map[string]any)
	if !ok {
		t.Fatalf("expected templateOverrides map, got %#v", result.Job.Input["templateOverrides"])
	}
	if overrides["memoryGb"] != 2 {
		t.Fatalf("expected request overrides to be recorded, got %#v", overrides)
	}
}

func TestCreateInstanceTaskParsesYAMLTemplateConfig(t *testing.T) {
	s := NewSeeded()
	template, err := s.CreateTemplate(domain.CreateTemplateRequest{
		Name:         "yaml-prefill",
		ConfigFormat: "yaml",
		ConfigText: `
context:
  profileId: profile-default
  region: ap-chuncheon-1
  compartmentId: ocid1.compartment.oc1..yaml
imageAndShape:
  imageId: ocid1.image.oc1..yaml
  shape: VM.Standard.E3.Flex
  ocpus: 1
  memoryGb: 2
  bootVolumeGb: 50
  bootVolumeVpusPerGb: 20
networkAndAccess:
  vcnId: ocid1.vcn.oc1..yaml
  subnetId: ocid1.subnet.oc1..yaml
  assignPublicIp: true
  enableIpv6: true
tags:
  owner: yaml
`,
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}

	result, err := s.CreateInstanceTask(domain.CreateInstanceRequest{
		TemplateID: template.ID,
		Name:       "from-yaml-template",
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if result.Instance.Shape != "VM.Standard.E3.Flex" || result.Instance.MemoryGB != 2 || result.Instance.PrimaryIP == "-" {
		t.Fatalf("expected yaml template to drive instance prefill, got %#v", result.Instance)
	}
	if result.Job.Input["enableIpv6"] != true || result.Job.Input["subnetId"] != "ocid1.subnet.oc1..yaml" {
		t.Fatalf("expected yaml network config in job input, got %#v", result.Job.Input)
	}
}

func TestCreateInstanceTaskUsesTemplateDefaultInstanceName(t *testing.T) {
	s := NewSeeded()
	template, err := s.CreateTemplate(domain.CreateTemplateRequest{
		Name:         "named-template",
		ConfigFormat: "json",
		ConfigText: `{
  "instance": {
    "name": "prefilled-instance-name"
  },
  "context": {
    "profileId": "profile-default",
    "region": "ap-chuncheon-1",
    "compartmentId": "ocid1.compartment.oc1..named"
  },
  "imageAndShape": {
    "imageId": "ocid1.image.oc1..named",
    "shape": "VM.Standard.E3.Flex",
    "ocpus": 1,
    "memoryGb": 2,
    "bootVolumeGb": 50,
    "bootVolumeVpusPerGb": 10
  },
  "networkAndAccess": {
    "vcnId": "ocid1.vcn.oc1..named",
    "subnetId": "ocid1.subnet.oc1..named",
    "assignPublicIp": true
  }
}`,
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}

	result, err := s.CreateInstanceTask(domain.CreateInstanceRequest{
		TemplateID: template.ID,
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if result.Instance.Name != "prefilled-instance-name" || result.Job.Input["name"] != "prefilled-instance-name" {
		t.Fatalf("expected default instance name from template config, got instance=%#v job=%#v", result.Instance, result.Job.Input)
	}
}

func TestCreateTemplatePartialConfigPreservesPayloadFields(t *testing.T) {
	s := NewSeeded()
	template, err := s.CreateTemplate(domain.CreateTemplateRequest{
		Name:                "partial-config",
		ProfileID:           "profile-default",
		Region:              "ap-chuncheon-1",
		Compartment:         "root",
		CompartmentID:       "ocid1.tenancy.oc1..partial",
		AvailabilityAD:      "AD-1",
		ImageID:             "ocid1.image.oc1..partial",
		ImageName:           "Oracle Linux",
		Shape:               "VM.Standard.E3.Flex",
		OCPUs:               1,
		MemoryGB:            1,
		BootVolumeGB:        50,
		BootVolumeVPUsPerGB: 10,
		VCNID:               "ocid1.vcn.oc1..partial",
		SubnetID:            "ocid1.subnet.oc1..partial",
		AssignPublicIP:      true,
		ConfigFormat:        "json",
		ConfigText:          `{"instance":{"name":"partial-instance"}}`,
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if template.ProfileID != "profile-default" || template.Region != "ap-chuncheon-1" || template.ImageID == "" {
		t.Fatalf("expected partial config to preserve request context, got %#v", template)
	}
	if template.OCPUs != 1 || template.MemoryGB != 1 || template.BootVolumeGB != 50 || template.BootVolumeVPUsPerGB != 10 {
		t.Fatalf("expected partial config to preserve compute fields, got %#v", template)
	}
	if template.VCNID == "" || template.SubnetID == "" || !template.AssignPublicIP {
		t.Fatalf("expected partial config to preserve network fields, got %#v", template)
	}
}

func TestCreateOCIInstanceLaunchTaskPersistsRetryPolicy(t *testing.T) {
	s := NewSeeded()
	job, err := s.CreateOCIInstanceLaunchTask(domain.CreateInstanceRequest{
		Name:             "retry-created",
		ProfileID:        "DEFAULT",
		Region:           "ap-chuncheon-1",
		CompartmentID:    "ocid1.tenancy.oc1..example",
		Shape:            "VM.Standard.E3.Flex",
		OCPUs:            1,
		MemoryGB:         1,
		BootVolumeGB:     50,
		MaxRetries:       4,
		RetryMode:        "count",
		RetryMaxAttempts: 4,
		RetryDelayMinSec: 5,
		RetryDelayMaxSec: 9,
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if job.MaxRetries != 4 || job.Input["retryMode"] != "count" || job.Input["retryDelayMinSeconds"] != 5 || job.Input["retryDelayMaxSeconds"] != 9 {
		t.Fatalf("unexpected retry policy input: %#v", job)
	}
}

func TestCompleteOCILaunchJobCreatesPersistedInstance(t *testing.T) {
	s := NewSeeded()
	sink := &fakeSink{}
	s.SetPersistenceSink(sink)

	job, err := s.CreateOCIInstanceLaunchTask(domain.CreateInstanceRequest{
		Name:          "oci-created",
		ProfileID:     "DEFAULT",
		Region:        "ap-chuncheon-1",
		CompartmentID: "ocid1.tenancy.oc1..example",
		Shape:         "VM.Standard.E3.Flex",
		OCPUs:         1,
		MemoryGB:      1,
		BootVolumeGB:  50,
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartJob(job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MarkJobWaitingOCI(job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MarkJobVerifying(job.ID); err != nil {
		t.Fatal(err)
	}
	completed, err := s.CompleteJob(job.ID, map[string]any{
		"instanceId":    "ocid1.instance.oc1.ap-chuncheon-1.example",
		"displayName":   "oci-created",
		"shape":         "VM.Standard.E3.Flex",
		"ocpus":         1,
		"memoryGb":      1,
		"bootVolumeGb":  50,
		"compartmentId": "ocid1.tenancy.oc1..example",
		"finalState":    "RUNNING",
	})
	if err != nil {
		t.Fatal(err)
	}

	if completed.ResourceID != "ocid1.instance.oc1.ap-chuncheon-1.example" {
		t.Fatalf("expected completed job to reference OCI instance, got %#v", completed)
	}
	instance, ok := s.GetInstance(completed.ResourceID)
	if !ok {
		t.Fatal("expected launch completion to create instance inventory")
	}
	if instance.Status != domain.InstanceRunning || instance.OCIInstanceID != completed.ResourceID {
		t.Fatalf("unexpected launch inventory instance: %#v", instance)
	}
	if len(sink.instances) == 0 || sink.instances[len(sink.instances)-1].ID != completed.ResourceID {
		t.Fatalf("expected persisted launch instance, got %#v", sink.instances)
	}
}

func TestRecoverRunnableJobsMovesActiveJobsToRetrying(t *testing.T) {
	s := NewSeeded()
	sink := &fakeSink{}
	s.SetPersistenceSink(sink)

	job, err := s.CreateOCIInstanceLaunchTask(domain.CreateInstanceRequest{
		Name:         "recover-me",
		Shape:        "VM.Standard.E3.Flex",
		OCPUs:        1,
		MemoryGB:     1,
		BootVolumeGB: 50,
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartJob(job.ID); err != nil {
		t.Fatal(err)
	}

	runnable, err := s.RecoverRunnableJobs()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, recovered := range runnable {
		if recovered.ID == job.ID {
			found = true
			if recovered.Status != domain.JobRetrying {
				t.Fatalf("expected recovered job to be retrying, got %#v", recovered)
			}
			if recovered.Input["recoveredFromStatus"] != string(domain.JobRunning) {
				t.Fatalf("expected recovery marker, got %#v", recovered.Input)
			}
		}
	}
	if !found {
		t.Fatalf("expected recovered job %s in runnable list: %#v", job.ID, runnable)
	}
}

func TestCreateProfilePersistsWithoutReturningSecret(t *testing.T) {
	s := New()
	sink := &fakeSink{}
	s.SetPersistenceSink(sink)

	profile, err := s.CreateProfile(domain.CreateProfileRequest{
		Name:           "primary",
		TenancyOCID:    "ocid1.tenancy.oc1..example",
		UserOCID:       "ocid1.user.oc1..example",
		Fingerprint:    "11:22:33",
		DefaultRegion:  "ap-chuncheon-1",
		PrivateKeyFile: "E:\\keys\\oci.pem",
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if profile.ID == "" || profile.Name != "primary" || profile.DefaultRegion != "ap-chuncheon-1" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
	if len(sink.profiles) != 1 || sink.profiles[0].ID != profile.ID {
		t.Fatalf("expected profile persistence, got %#v", sink.profiles)
	}
}

func TestLoadPersistedSettingsRestoresEmailForSend(t *testing.T) {
	sink := &fakeSink{}
	s := New()
	s.SetPersistenceSink(sink)
	saved, err := s.SetEmailSettings(domain.EmailSettings{
		Enabled:  true,
		Host:     "mail.example.com",
		Port:     465,
		Username: "panel@example.com",
		Password: "smtp-secret",
		From:     "panel@example.com",
		To:       []string{"ops@example.com"},
		UseTLS:   true,
		StartTLS: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Password != "" || !saved.PasswordSet {
		t.Fatalf("expected redacted saved settings with password marker, got %#v", saved)
	}

	reloaded := New()
	reloaded.SetPersistenceSink(sink)
	if err := reloaded.LoadPersistedSettings(); err != nil {
		t.Fatal(err)
	}
	forSend := reloaded.GetEmailSettingsForSend()
	if !forSend.Enabled || forSend.Host != "mail.example.com" || forSend.Port != 465 || !forSend.UseTLS || forSend.StartTLS {
		t.Fatalf("expected persisted SMTP settings to load, got %#v", forSend)
	}
	if forSend.Password != "smtp-secret" || !forSend.PasswordSet {
		t.Fatalf("expected persisted SMTP password to be available for delivery without exposing it in API, got %#v", forSend)
	}
	forAPI := reloaded.GetEmailSettings()
	if forAPI.Password != "" || !forAPI.PasswordSet {
		t.Fatalf("expected API settings to remain redacted, got %#v", forAPI)
	}
}

func TestListAuditLogsFiltersExtendedFields(t *testing.T) {
	s := New()
	s.ReplaceAuditLogs([]domain.AuditLog{
		{
			ID:               1,
			Actor:            "admin",
			Action:           "instance.launch",
			ResourceType:     "instance",
			ResourceID:       "inst-a",
			ProfileID:        "profile-a",
			Region:           "ap-chuncheon-1",
			CompartmentID:    "compartment-a",
			OCIRequestID:     "request-a",
			OCIWorkRequestID: "work-a",
		},
		{
			ID:               2,
			Actor:            "admin",
			Action:           "instance.terminate",
			ResourceType:     "instance",
			ResourceID:       "inst-b",
			ProfileID:        "profile-b",
			Region:           "us-ashburn-1",
			CompartmentID:    "compartment-b",
			OCIRequestID:     "request-b",
			OCIWorkRequestID: "work-b",
			ErrorCode:        "Conflict",
			ErrorMessage:     "instance is busy",
		},
	})

	logs, err := s.ListAuditLogs(domain.AuditLogFilter{
		Region:           "ashburn",
		CompartmentID:    "compartment-b",
		OCIRequestID:     "request-b",
		OCIWorkRequestID: "work-b",
		Status:           "failed",
		Limit:            10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].ID != 2 {
		t.Fatalf("expected extended audit filter to return failed ashburn entry, got %#v", logs)
	}
}

func TestClearCompletedJobsPersistsDeleteAndKeepsRunnableJobs(t *testing.T) {
	sink := &fakeSink{}
	s := New()
	s.SetPersistenceSink(sink)
	s.ReplaceJobs([]domain.Job{
		{ID: "JOB-success", Status: domain.JobSuccess, CreatedBy: "tester"},
		{ID: "JOB-failed", Status: domain.JobFailed, CreatedBy: "tester"},
		{ID: "JOB-cancelled", Status: domain.JobCancelled, CreatedBy: "tester"},
		{ID: "JOB-manual", Status: domain.JobManualNeeded, CreatedBy: "tester"},
		{ID: "JOB-pending", Status: domain.JobPending, CreatedBy: "tester"},
		{ID: "JOB-running", Status: domain.JobRunning, CreatedBy: "tester"},
		{ID: "JOB-waiting", Status: domain.JobWaitingOCI, CreatedBy: "tester"},
		{ID: "JOB-retrying", Status: domain.JobRetrying, CreatedBy: "tester"},
	})

	deleted, err := s.ClearCompletedJobs("admin")
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 4 {
		t.Fatalf("expected 4 completed jobs to be deleted, got %d", deleted)
	}
	if len(sink.deletedJobs) != 4 {
		t.Fatalf("expected persisted job deletes, got %#v", sink.deletedJobs)
	}
	remaining := s.ListJobs()
	remainingIDs := map[string]bool{}
	for _, job := range remaining {
		remainingIDs[job.ID] = true
	}
	for _, id := range []string{"JOB-pending", "JOB-running", "JOB-waiting", "JOB-retrying"} {
		if !remainingIDs[id] {
			t.Fatalf("expected runnable job %s to remain, remaining=%#v", id, remainingIDs)
		}
	}
	for _, id := range []string{"JOB-success", "JOB-failed", "JOB-cancelled", "JOB-manual"} {
		if remainingIDs[id] {
			t.Fatalf("expected completed job %s to be removed, remaining=%#v", id, remainingIDs)
		}
	}
	if len(sink.audits) != 1 || sink.audits[0].Action != "jobs.clear_completed" {
		t.Fatalf("expected clear completed audit record, got %#v", sink.audits)
	}
}

func TestAccessControlSanitizesUserIDsAndScopesPermissions(t *testing.T) {
	s := New()
	settings, err := s.SetAccessControlSettings(domain.AccessControlSettings{
		Users: []domain.AccessUser{
			{
				ID:                  "../Audit/User",
				DisplayName:         "Audit User",
				RoleID:              "auditor",
				Status:              "active",
				AllowedRegions:      []string{"ap-chuncheon-1"},
				AllowedCompartments: []string{"ocid1.compartment.oc1..allowed"},
			},
		},
	}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if settings.Users[0].ID != "audit-user" {
		t.Fatalf("expected path-like user id to be sanitized, got %#v", settings.Users[0].ID)
	}
	if err := s.Authorize("audit-user", "audit:read", "", "ap-chuncheon-1", "ocid1.compartment.oc1..allowed"); err != nil {
		t.Fatalf("expected auditor read to pass: %v", err)
	}
	if err := s.Authorize("audit-user", "instance:write", "", "ap-chuncheon-1", "ocid1.compartment.oc1..allowed"); err == nil {
		t.Fatal("expected auditor instance write to be denied")
	}
	if err := s.Authorize("audit-user", "audit:read", "", "us-ashburn-1", "ocid1.compartment.oc1..allowed"); err == nil {
		t.Fatal("expected region scope escape to be denied")
	}
}

func TestAccessUserPasswordPersistsWithoutExposure(t *testing.T) {
	sink := &fakeSink{}
	s := New()
	s.SetPersistenceSink(sink)

	settings, err := s.SetAccessUserPassword("admin", "secret-password", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(settings.Users) == 0 || !settings.Users[0].PasswordSet || settings.Users[0].PasswordHash != "" {
		t.Fatalf("expected redacted password state, got %#v", settings.Users)
	}
	if len(sink.accessSettings.Users) == 0 || strings.TrimSpace(sink.accessSettings.Users[0].PasswordHash) == "" {
		t.Fatalf("expected persisted password hash, got %#v", sink.accessSettings.Users)
	}

	reloaded := New()
	reloaded.SetPersistenceSink(sink)
	if err := reloaded.LoadPersistedSettings(); err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.VerifyAccessUser("admin", "secret-password"); !ok {
		t.Fatal("expected persisted admin password to verify after reload")
	}

	injected, err := reloaded.SetAccessControlSettings(domain.AccessControlSettings{
		Users: []domain.AccessUser{{
			ID:           "admin",
			DisplayName:  "Administrator",
			RoleID:       "super_admin",
			Status:       "active",
			PasswordHash: "attacker-controlled-hash",
		}},
	}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if injected.Users[0].PasswordHash != "" {
		t.Fatalf("expected access settings response to remain redacted, got %#v", injected.Users[0])
	}
	if _, ok := reloaded.VerifyAccessUser("admin", "secret-password"); !ok {
		t.Fatal("expected existing password hash to survive access settings update")
	}
	if _, ok := reloaded.VerifyAccessUser("admin", "attacker-controlled-hash"); ok {
		t.Fatal("access settings update must not accept caller supplied passwordHash")
	}
}

func TestNotificationsPersistStatusAndDelete(t *testing.T) {
	sink := &fakeSink{}
	s := New()
	s.SetPersistenceSink(sink)
	notification, err := s.CreateNotification(domain.NotificationRequest{
		Title:          "root password generated",
		Message:        "root password is available in secure notice",
		Severity:       domain.NotificationWarning,
		Category:       "credential",
		ResourceType:   "instance",
		ResourceID:     "inst-test",
		EmailRequested: true,
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if len(sink.notifications) != 1 || sink.notifications[0].ID != notification.ID {
		t.Fatalf("expected notification persistence, got %#v", sink.notifications)
	}
	updated, err := s.UpdateNotificationEmailStatus(notification.ID, false, "smtp unavailable")
	if err != nil {
		t.Fatal(err)
	}
	if !updated.EmailRequested || updated.EmailSent || updated.EmailError != "smtp unavailable" {
		t.Fatalf("unexpected updated notification: %#v", updated)
	}
	if sink.notifications[0].EmailError != "smtp unavailable" {
		t.Fatalf("expected persisted email status, got %#v", sink.notifications[0])
	}
	if err := s.DeleteNotification(notification.ID); err != nil {
		t.Fatal(err)
	}
	if len(sink.deletedNotices) != 1 || sink.deletedNotices[0] != notification.ID {
		t.Fatalf("expected delete persistence, got %#v", sink.deletedNotices)
	}
	if len(sink.notifications) != 0 {
		t.Fatalf("expected notification removed from persistence, got %#v", sink.notifications)
	}
}
