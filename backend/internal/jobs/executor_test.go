package jobs

import (
	"context"
	"errors"
	"testing"

	"a-series-oracle/backend/internal/domain"
	"a-series-oracle/backend/internal/oci"
	"a-series-oracle/backend/internal/store"
)

func TestLocalExecutorCompletesEngineeringJob(t *testing.T) {
	s := store.NewSeeded()
	job, err := s.CreateInstanceActionTask("inst-prod-web-01", domain.InstanceActionRequest{
		Action:             domain.InstanceActionStop,
		Graceful:           true,
		PreserveBootVolume: true,
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}

	executor := NewLocalExecutor(s)
	executor.step = 0
	if err := executor.Execute(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}

	detail, ok := s.GetJob(job.ID)
	if !ok {
		t.Fatal("expected job")
	}
	if detail.Status != domain.JobSuccess {
		t.Fatalf("expected success, got %s", detail.Status)
	}
	instance, _ := s.GetInstance("inst-prod-web-01")
	if instance.Status != domain.InstanceStopped {
		t.Fatalf("expected local executor to update engineering inventory, got %s", instance.Status)
	}
}

func TestOCIExecutorFailsWhenNotReadyAndDoesNotMutateInstance(t *testing.T) {
	s := store.NewSeeded()
	job, err := s.CreateInstanceActionTask("inst-prod-web-01", domain.InstanceActionRequest{
		Action:             domain.InstanceActionStop,
		Graceful:           true,
		PreserveBootVolume: true,
	}, "tester")
	if err != nil {
		t.Fatal(err)
	}

	executor := NewOCIExecutor(s, oci.ReadinessConfig{ExecutionMode: "oci"})
	err = executor.Execute(context.Background(), job.ID)
	if !errors.Is(err, oci.ErrNotReady) {
		t.Fatalf("expected ErrNotReady, got %v", err)
	}

	detail, ok := s.GetJob(job.ID)
	if !ok {
		t.Fatal("expected job")
	}
	if detail.Status != domain.JobFailed || detail.ErrorCode != "OCI_NOT_READY" {
		t.Fatalf("expected OCI_NOT_READY failure, got %#v", detail)
	}
	instance, _ := s.GetInstance("inst-prod-web-01")
	if instance.Status != domain.InstanceRunning {
		t.Fatalf("OCI executor must not mutate local inventory without real OCI result, got %s", instance.Status)
	}
}
