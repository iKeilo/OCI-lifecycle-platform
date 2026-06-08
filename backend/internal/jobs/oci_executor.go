package jobs

import (
	"context"
	"encoding/json"
	"errors"

	"a-series-oracle/backend/internal/domain"
	"a-series-oracle/backend/internal/oci"
	"a-series-oracle/backend/internal/store"
)

var ErrOCIExecutorNotImplemented = errors.New("oci executor is not implemented")

type OCIExecutor struct {
	store     *store.Store
	readiness oci.ReadinessConfig
	resolver  OCIProfileResolver
}

type OCIProfileResolver interface {
	Resolve(profileID string, region string) (oci.ReadinessConfig, domain.Profile, error)
}

func NewOCIExecutor(store *store.Store, readiness oci.ReadinessConfig) *OCIExecutor {
	return &OCIExecutor{
		store:     store,
		readiness: readiness,
	}
}

func NewOCIExecutorWithResolver(store *store.Store, readiness oci.ReadinessConfig, resolver OCIProfileResolver) *OCIExecutor {
	return &OCIExecutor{
		store:     store,
		readiness: readiness,
		resolver:  resolver,
	}
}

func (e *OCIExecutor) Execute(ctx context.Context, jobID string) error {
	if _, err := e.store.StartJob(jobID); err != nil {
		return ignoreConflict(err)
	}

	job, ok := e.store.GetJob(jobID)
	if !ok {
		return store.ErrNotFound
	}

	cfg := e.readiness
	if e.resolver != nil {
		resolved, _, err := e.resolver.Resolve(job.ProfileID, job.Region)
		if err != nil {
			if _, failErr := e.store.FailJob(jobID, "OCI_PROFILE_RESOLVE_FAILED", err.Error()); failErr != nil {
				return failErr
			}
			return err
		}
		cfg = resolved
	}

	readiness := oci.CheckReadiness(cfg)
	if !readiness.Ready {
		if _, err := e.store.FailJob(jobID, "OCI_NOT_READY", readiness.Message); err != nil {
			return err
		}
		return oci.ErrNotReady
	}

	if _, err := oci.NewClients(cfg); err != nil {
		if _, failErr := e.store.FailJob(jobID, "OCI_CLIENT_INIT_FAILED", err.Error()); failErr != nil {
			return failErr
		}
		return err
	}

	if job.ResourceType == "instance" {
		if operation, _ := job.Input["operation"].(string); operation == "launch" {
			return e.executeLaunchJob(ctx, cfg, job)
		} else if operation == "ip-management" {
			return e.executeIPManagementJob(ctx, cfg, job)
		}
		return e.executeInstanceJob(ctx, cfg, job)
	}

	if _, err := e.store.FailJob(jobID, "OCI_EXECUTOR_NOT_IMPLEMENTED", "real OCI SDK execution is not implemented for this job type yet; no local state was changed"); err != nil {
		return err
	}
	return ErrOCIExecutorNotImplemented
}

func (e *OCIExecutor) executeLaunchJob(ctx context.Context, cfg oci.ReadinessConfig, job domain.Job) error {
	req, err := oci.CreateRequestFromJobInput(job.Input)
	if err != nil {
		if _, failErr := e.store.FailJob(job.ID, "OCI_LAUNCH_INPUT_INVALID", err.Error()); failErr != nil {
			return failErr
		}
		return err
	}
	result := oci.LaunchInstanceFromRequest(ctx, cfg, req, job.ID)
	if _, err := e.store.SetJobOCIRefs(job.ID, result.RequestID, result.WorkRequestID); err != nil {
		return err
	}
	if _, err := e.store.MarkJobWaitingOCI(job.ID); err != nil {
		return ignoreConflict(err)
	}
	if _, err := e.store.MarkJobVerifying(job.ID); err != nil {
		return ignoreConflict(err)
	}
	if !result.Verified {
		if _, err := e.store.FailJob(job.ID, result.ErrorCode, result.ErrorMessage); err != nil {
			return err
		}
		return errors.New(result.ErrorCode + ": " + result.ErrorMessage)
	}
	payload, err := structToMap(result)
	if err != nil {
		if _, failErr := e.store.FailJob(job.ID, "OCI_RESULT_ENCODE_FAILED", err.Error()); failErr != nil {
			return failErr
		}
		return err
	}
	if _, err := e.store.CompleteJob(job.ID, payload); err != nil {
		return ignoreConflict(err)
	}
	return nil
}

func (e *OCIExecutor) executeIPManagementJob(ctx context.Context, cfg oci.ReadinessConfig, job domain.Job) error {
	instanceID, _ := job.Input["ociInstanceId"].(string)
	if instanceID == "" {
		instanceID = job.ResourceID
	}
	result := oci.ExecuteIPManagement(ctx, cfg, oci.IPManagementExecutionRequest{
		InstanceID: instanceID,
		VNICID:     stringFromInput(job.Input["vnicId"]),
		EnableIPv6: boolFromInput(job.Input["enableIpv6"]),
		JobID:      job.ID,
	})
	if _, err := e.store.SetJobOCIRefs(job.ID, result.RequestID, result.WorkRequestID); err != nil {
		return err
	}
	if _, err := e.store.MarkJobWaitingOCI(job.ID); err != nil {
		return ignoreConflict(err)
	}
	if _, err := e.store.MarkJobVerifying(job.ID); err != nil {
		return ignoreConflict(err)
	}

	if !result.Verified {
		if _, err := e.store.FailJob(job.ID, result.ErrorCode, result.ErrorMessage); err != nil {
			return err
		}
		return errors.New(result.ErrorCode + ": " + result.ErrorMessage)
	}

	payload, err := structToMap(result)
	if err != nil {
		if _, failErr := e.store.FailJob(job.ID, "OCI_RESULT_ENCODE_FAILED", err.Error()); failErr != nil {
			return failErr
		}
		return err
	}
	if _, err := e.store.CompleteJob(job.ID, payload); err != nil {
		return ignoreConflict(err)
	}
	return nil
}

func (e *OCIExecutor) executeInstanceJob(ctx context.Context, cfg oci.ReadinessConfig, job domain.Job) error {
	action, _ := job.Input["action"].(string)
	instanceID, _ := job.Input["ociInstanceId"].(string)
	if instanceID == "" {
		instanceID = job.ResourceID
	}

	result := oci.ExecuteInstanceLifecycleAction(ctx, cfg, oci.InstanceActionExecutionRequest{
		InstanceID:         instanceID,
		Action:             domain.InstanceLifecycleAction(action),
		Graceful:           boolFromInput(job.Input["graceful"]),
		PreserveBootVolume: boolFromInput(job.Input["preserveBootVolume"]),
		TargetShape:        stringFromInput(job.Input["targetShape"]),
		TargetOCPUs:        intFromInput(job.Input["targetOcpus"]),
		TargetMemoryGB:     intFromInput(job.Input["targetMemoryGb"]),
		JobID:              job.ID,
	})
	if _, err := e.store.SetJobOCIRefs(job.ID, result.RequestID, result.WorkRequestID); err != nil {
		return err
	}
	if _, err := e.store.MarkJobWaitingOCI(job.ID); err != nil {
		return ignoreConflict(err)
	}
	if _, err := e.store.MarkJobVerifying(job.ID); err != nil {
		return ignoreConflict(err)
	}

	if !result.Verified {
		if _, err := e.store.FailJob(job.ID, result.ErrorCode, result.ErrorMessage); err != nil {
			return err
		}
		return errors.New(result.ErrorCode + ": " + result.ErrorMessage)
	}

	payload, err := structToMap(result)
	if err != nil {
		if _, failErr := e.store.FailJob(job.ID, "OCI_RESULT_ENCODE_FAILED", err.Error()); failErr != nil {
			return failErr
		}
		return err
	}
	if _, err := e.store.CompleteJob(job.ID, payload); err != nil {
		return ignoreConflict(err)
	}
	return nil
}

func structToMap(value any) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func stringFromInput(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func boolFromInput(value any) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	return false
}

func intFromInput(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}
