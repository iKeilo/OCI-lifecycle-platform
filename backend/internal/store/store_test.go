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
		Action:         domain.InstanceActionResize,
		TargetShape:    "VM.Standard3.Flex",
		TargetOCPUs:    2,
		TargetMemoryGB: 16,
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
	if instance.Shape != "VM.Standard3.Flex" || instance.OCPUs != 2 || instance.MemoryGB != 16 {
		t.Fatalf("unexpected resized instance: %#v", instance)
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
