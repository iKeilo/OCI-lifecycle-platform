package jobs

import (
	"context"
	"encoding/json"
	"errors"

	"a-series-oracle/backend/internal/domain"
	"a-series-oracle/backend/internal/lifecyclenotify"
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
		} else if operation == "reinstall" {
			return e.executeReinstallJob(ctx, cfg, job)
		}
		return e.executeInstanceJob(ctx, cfg, job)
	}
	if job.ResourceType == "network" {
		if operation, _ := job.Input["operation"].(string); operation == "public-ip-batch" {
			return e.executePublicIPBatchJob(ctx, cfg, job)
		}
	}

	if _, err := e.store.FailJob(jobID, "OCI_EXECUTOR_NOT_IMPLEMENTED", "real OCI SDK execution is not implemented for this job type yet; no local state was changed"); err != nil {
		return err
	}
	return ErrOCIExecutorNotImplemented
}

func (e *OCIExecutor) executePublicIPBatchJob(ctx context.Context, cfg oci.ReadinessConfig, job domain.Job) error {
	result := oci.ExecutePublicIPBatch(ctx, cfg, oci.PublicIPBatchExecutionRequest{
		Action:        stringFromInput(job.Input["action"]),
		CompartmentID: job.CompartmentID,
		Count:         intFromInput(job.Input["count"]),
		DisplayPrefix: stringFromInput(job.Input["displayPrefix"]),
		PublicIPIDs:   stringSliceFromInput(job.Input["publicIpIds"]),
		JobID:         job.ID,
	})
	if _, err := e.store.SetJobOCIRefs(job.ID, result.RequestID, ""); err != nil {
		return err
	}
	if _, err := e.store.MarkJobWaitingOCI(job.ID); err != nil {
		return ignoreConflict(err)
	}
	if _, err := e.store.MarkJobVerifying(job.ID); err != nil {
		return ignoreConflict(err)
	}
	payload, err := structToMap(result)
	if err != nil {
		if _, failErr := e.store.FailJob(job.ID, "OCI_RESULT_ENCODE_FAILED", err.Error()); failErr != nil {
			return failErr
		}
		return err
	}
	if !result.Verified {
		if _, err := e.store.FailJob(job.ID, result.ErrorCode, result.ErrorMessage); err != nil {
			return err
		}
		return errors.New(result.ErrorCode + ": " + result.ErrorMessage)
	}
	if _, err := e.store.CompleteJob(job.ID, payload); err != nil {
		return ignoreConflict(err)
	}
	return nil
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
		InstanceID:               instanceID,
		VNICID:                   stringFromInput(job.Input["vnicId"]),
		EnableIPv6:               boolFromInput(job.Input["enableIpv6"]),
		DisableIPv6:              boolFromInput(job.Input["disableIpv6"]),
		AutoConfigureIPv6:        boolFromInput(job.Input["autoConfigureIpv6"]),
		IPv6Strategy:             stringFromInput(job.Input["ipv6Strategy"]),
		NetworkChangeMode:        stringFromInput(job.Input["networkChangeMode"]),
		RouteTableMode:           stringFromInput(job.Input["routeTableMode"]),
		SecurityMode:             stringFromInput(job.Input["securityMode"]),
		AllowIrreversibleVCNIPv6: boolFromInput(job.Input["allowIrreversibleVcnIpv6"]),
		AllowPublicIPv4Change:    boolFromInput(job.Input["allowPublicIpv4Change"]),
		OpenSSHIPv6:              boolFromInput(job.Input["openSshIpv6"]),
		OpenHTTPIPv6:             boolFromInput(job.Input["openHttpIpv6"]),
		OpenHTTPSIPv6:            boolFromInput(job.Input["openHttpsIpv6"]),
		JobID:                    job.ID,
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

func (e *OCIExecutor) executeReinstallJob(ctx context.Context, cfg oci.ReadinessConfig, job domain.Job) error {
	instanceID, _ := job.Input["ociInstanceId"].(string)
	if instanceID == "" {
		instanceID = job.ResourceID
	}
	result := oci.ExecuteInstanceReinstall(ctx, cfg, oci.InstanceReinstallExecutionRequest{
		InstanceID:             instanceID,
		ImageID:                stringFromInput(job.Input["imageId"]),
		ImageName:              stringFromInput(job.Input["imageName"]),
		BootVolumeSizeGB:       intFromInput(job.Input["bootVolumeSizeGb"]),
		BootVolumeVPUsPerGB:    intFromInput(job.Input["bootVolumeVpusPerGb"]),
		PreserveOldBootVolume:  boolFromInput(job.Input["preserveOldBootVolume"]),
		CreateBootVolumeBackup: boolFromInput(job.Input["createBootVolumeBackup"]),
		JobID:                  job.ID,
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
		payload, _ := structToMap(result)
		failed, err := e.store.FailJob(job.ID, result.ErrorCode, result.ErrorMessage)
		if err != nil {
			return err
		}
		lifecyclenotify.SendReinstallNotification(ctx, e.store, lifecyclenotify.ReinstallFailed, failed, payload)
		return errors.New(result.ErrorCode + ": " + result.ErrorMessage)
	}

	payload, err := structToMap(result)
	if err != nil {
		failed, failErr := e.store.FailJob(job.ID, "OCI_RESULT_ENCODE_FAILED", err.Error())
		if failErr != nil {
			return failErr
		}
		lifecyclenotify.SendReinstallNotification(ctx, e.store, lifecyclenotify.ReinstallFailed, failed, map[string]any{
			"errorCode":    "OCI_RESULT_ENCODE_FAILED",
			"errorMessage": err.Error(),
		})
		return err
	}
	completed, err := e.store.CompleteJob(job.ID, payload)
	if err != nil {
		return ignoreConflict(err)
	}
	lifecyclenotify.SendReinstallNotification(ctx, e.store, lifecyclenotify.ReinstallSucceeded, completed, payload)
	return nil
}

func (e *OCIExecutor) executeInstanceJob(ctx context.Context, cfg oci.ReadinessConfig, job domain.Job) error {
	action, _ := job.Input["action"].(string)
	instanceID, _ := job.Input["ociInstanceId"].(string)
	if instanceID == "" {
		instanceID = job.ResourceID
	}

	result := oci.ExecuteInstanceLifecycleAction(ctx, cfg, oci.InstanceActionExecutionRequest{
		InstanceID:                instanceID,
		Action:                    domain.InstanceLifecycleAction(action),
		Graceful:                  boolFromInput(job.Input["graceful"]),
		PreserveBootVolume:        boolFromInput(job.Input["preserveBootVolume"]),
		TargetShape:               stringFromInput(job.Input["targetShape"]),
		TargetOCPUs:               intFromInput(job.Input["targetOcpus"]),
		TargetMemoryGB:            intFromInput(job.Input["targetMemoryGb"]),
		TargetBootVolumeGB:        intFromInput(job.Input["targetBootVolumeGb"]),
		TargetBootVolumeVPUsPerGB: intFromInput(job.Input["targetBootVolumeVpusPerGb"]),
		ExpandBootVolume:          boolFromInput(job.Input["expandBootVolume"]),
		JobID:                     job.ID,
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

func stringSliceFromInput(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}
