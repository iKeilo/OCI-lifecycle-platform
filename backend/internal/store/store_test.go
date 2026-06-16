package store

import (
	"testing"

	"a-series-oracle/backend/internal/domain"
)

type fakeSink struct {
	profiles  []domain.Profile
	jobs      []domain.Job
	instances []domain.Instance
	audits    []domain.AuditLog
}

func (s *fakeSink) SaveProfile(profile domain.Profile, secret domain.ProfileSecret) error {
	s.profiles = append(s.profiles, profile)
	return nil
}

func (s *fakeSink) SaveJob(job domain.Job) error {
	s.jobs = append(s.jobs, job)
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
