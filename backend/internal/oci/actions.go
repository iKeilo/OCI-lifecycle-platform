package oci

import (
	"context"
	"fmt"
	"strings"
	"time"

	"a-series-oracle/backend/internal/domain"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type InstanceActionExecutionRequest struct {
	InstanceID         string
	Action             domain.InstanceLifecycleAction
	Graceful           bool
	PreserveBootVolume bool
	TargetShape        string
	TargetOCPUs        int
	TargetMemoryGB     int
	TargetBootVolumeGB int
	ExpandBootVolume   bool
	JobID              string
}

type InstanceActionExecutionResult struct {
	Verified            bool      `json:"verified"`
	ExecutionMode       string    `json:"executionMode"`
	InstanceID          string    `json:"instanceId"`
	Action              string    `json:"action"`
	RequestID           string    `json:"requestId,omitempty"`
	WorkRequestID       string    `json:"workRequestId,omitempty"`
	InitialState        string    `json:"initialState,omitempty"`
	FinalState          string    `json:"finalState,omitempty"`
	TargetShape         string    `json:"targetShape,omitempty"`
	TargetOCPUs         int       `json:"targetOcpus,omitempty"`
	TargetMemoryGB      int       `json:"targetMemoryGb,omitempty"`
	TargetBootVolumeGB  int       `json:"targetBootVolumeGb,omitempty"`
	CurrentBootVolumeGB int       `json:"currentBootVolumeGb,omitempty"`
	BootVolumeID        string    `json:"bootVolumeId,omitempty"`
	BootVolumeExpanded  bool      `json:"bootVolumeExpanded"`
	ErrorCode           string    `json:"errorCode,omitempty"`
	ErrorMessage        string    `json:"errorMessage,omitempty"`
	ExecutedAt          time.Time `json:"executedAt"`
	WaitedForState      bool      `json:"waitedForState"`
	PreserveBootDisk    bool      `json:"preserveBootVolume"`
}

func ExecuteInstanceLifecycleAction(ctx context.Context, cfg ReadinessConfig, req InstanceActionExecutionRequest) InstanceActionExecutionResult {
	result := InstanceActionExecutionResult{
		ExecutionMode:      cfg.ExecutionMode,
		InstanceID:         req.InstanceID,
		Action:             string(req.Action),
		TargetShape:        req.TargetShape,
		TargetOCPUs:        req.TargetOCPUs,
		TargetMemoryGB:     req.TargetMemoryGB,
		TargetBootVolumeGB: req.TargetBootVolumeGB,
		ExecutedAt:         time.Now().UTC(),
		PreserveBootDisk:   req.PreserveBootVolume,
	}
	readiness := CheckReadiness(cfg)
	if !readiness.Ready {
		result.ErrorCode = "OCI_NOT_READY"
		result.ErrorMessage = readiness.Message
		return result
	}
	if strings.TrimSpace(req.InstanceID) == "" {
		result.ErrorCode = "OCI_INSTANCE_ID_REQUIRED"
		result.ErrorMessage = "OCI instance OCID is required"
		return result
	}

	clients, err := NewClients(cfg)
	if err != nil {
		result.ErrorCode = "OCI_CLIENT_INIT_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}

	current, err := clients.Compute.GetInstance(ctx, core.GetInstanceRequest{InstanceId: common.String(req.InstanceID)})
	if current.OpcRequestId != nil && result.RequestID == "" {
		result.RequestID = *current.OpcRequestId
	}
	if err != nil {
		result.ErrorCode = "OCI_GET_INSTANCE_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	result.InitialState = string(current.Instance.LifecycleState)

	switch req.Action {
	case domain.InstanceActionStart:
		return executePowerAction(ctx, clients, req, result, core.InstanceActionActionStart, core.InstanceLifecycleStateRunning)
	case domain.InstanceActionStop:
		action := core.InstanceActionActionStop
		if req.Graceful {
			action = core.InstanceActionActionSoftstop
		}
		return executePowerAction(ctx, clients, req, result, action, core.InstanceLifecycleStateStopped)
	case domain.InstanceActionReboot:
		action := core.InstanceActionActionReset
		if req.Graceful {
			action = core.InstanceActionActionSoftreset
		}
		return executePowerAction(ctx, clients, req, result, action, core.InstanceLifecycleStateRunning)
	case domain.InstanceActionTerminate:
		response, err := clients.Compute.TerminateInstance(ctx, core.TerminateInstanceRequest{
			InstanceId:         common.String(req.InstanceID),
			PreserveBootVolume: common.Bool(req.PreserveBootVolume),
		})
		if response.OpcRequestId != nil {
			result.RequestID = *response.OpcRequestId
		}
		if err != nil {
			result.ErrorCode = "OCI_TERMINATE_INSTANCE_FAILED"
			result.ErrorMessage = err.Error()
			return result
		}
		return waitForActionState(ctx, clients, req.InstanceID, result, core.InstanceLifecycleStateTerminated)
	case domain.InstanceActionResize:
		return executeResize(ctx, clients, req, result, current.Instance)
	default:
		result.ErrorCode = "OCI_ACTION_UNSUPPORTED"
		result.ErrorMessage = fmt.Sprintf("unsupported action: %s", req.Action)
		return result
	}
}

func executePowerAction(ctx context.Context, clients Clients, req InstanceActionExecutionRequest, result InstanceActionExecutionResult, action core.InstanceActionActionEnum, target core.InstanceLifecycleStateEnum) InstanceActionExecutionResult {
	response, err := clients.Compute.InstanceAction(ctx, core.InstanceActionRequest{
		InstanceId:    common.String(req.InstanceID),
		Action:        action,
		OpcRetryToken: retryToken("action", req.JobID),
		OpcRequestId:  requestID("codex-action", req.JobID),
	})
	if response.OpcRequestId != nil {
		result.RequestID = *response.OpcRequestId
	}
	if err != nil {
		result.ErrorCode = "OCI_INSTANCE_ACTION_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	return waitForActionState(ctx, clients, req.InstanceID, result, target)
}

func executeResize(ctx context.Context, clients Clients, req InstanceActionExecutionRequest, result InstanceActionExecutionResult, instance core.Instance) InstanceActionExecutionResult {
	if strings.TrimSpace(req.TargetShape) == "" {
		result.ErrorCode = "OCI_RESIZE_TARGET_SHAPE_REQUIRED"
		result.ErrorMessage = "targetShape is required"
		return result
	}
	if req.TargetOCPUs <= 0 || req.TargetMemoryGB <= 0 {
		result.ErrorCode = "OCI_RESIZE_SHAPE_CONFIG_REQUIRED"
		result.ErrorMessage = "targetOcpus and targetMemoryGb must be greater than zero"
		return result
	}

	response, err := clients.Compute.UpdateInstance(ctx, core.UpdateInstanceRequest{
		InstanceId: common.String(req.InstanceID),
		UpdateInstanceDetails: core.UpdateInstanceDetails{
			Shape: common.String(req.TargetShape),
			ShapeConfig: &core.UpdateInstanceShapeConfigDetails{
				Ocpus:       common.Float32(float32(req.TargetOCPUs)),
				MemoryInGBs: common.Float32(float32(req.TargetMemoryGB)),
			},
			UpdateOperationConstraint: core.UpdateInstanceDetailsUpdateOperationConstraintAllowDowntime,
		},
		OpcRetryToken: retryToken("resize", req.JobID),
		OpcRequestId:  requestID("codex-resize", req.JobID),
	})
	if response.OpcRequestId != nil {
		result.RequestID = *response.OpcRequestId
	}
	if response.OpcWorkRequestId != nil {
		result.WorkRequestID = *response.OpcWorkRequestId
	}
	if err != nil {
		result.ErrorCode = "OCI_UPDATE_INSTANCE_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	result = waitForActionState(ctx, clients, req.InstanceID, result, core.InstanceLifecycleStateRunning, core.InstanceLifecycleStateStopped)
	if !result.Verified {
		return result
	}
	if req.ExpandBootVolume || req.TargetBootVolumeGB > 0 {
		return executeBootVolumeExpansion(ctx, clients, req, result, instance)
	}
	return result
}

func executeBootVolumeExpansion(ctx context.Context, clients Clients, req InstanceActionExecutionRequest, result InstanceActionExecutionResult, instance core.Instance) InstanceActionExecutionResult {
	if req.TargetBootVolumeGB <= 0 {
		return result
	}
	availabilityDomain := strings.TrimSpace(stringValue(instance.AvailabilityDomain))
	compartmentID := strings.TrimSpace(stringValue(instance.CompartmentId))
	if availabilityDomain == "" || compartmentID == "" {
		result.ErrorCode = "OCI_BOOT_VOLUME_CONTEXT_MISSING"
		result.ErrorMessage = "availabilityDomain and compartmentId are required for boot volume expansion"
		result.Verified = false
		return result
	}

	attachments, err := clients.Compute.ListBootVolumeAttachments(ctx, core.ListBootVolumeAttachmentsRequest{
		AvailabilityDomain: common.String(availabilityDomain),
		CompartmentId:      common.String(compartmentID),
		InstanceId:         common.String(req.InstanceID),
		Limit:              common.Int(25),
		OpcRequestId:       requestID("codex-list-boot-volume", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, attachments.OpcRequestId)
	if err != nil {
		result.ErrorCode = "OCI_LIST_BOOT_VOLUME_ATTACHMENTS_FAILED"
		result.ErrorMessage = err.Error()
		result.Verified = false
		return result
	}

	bootVolumeID := ""
	for _, attachment := range attachments.Items {
		if attachment.LifecycleState == core.BootVolumeAttachmentLifecycleStateDetached || attachment.LifecycleState == core.BootVolumeAttachmentLifecycleStateDetaching {
			continue
		}
		if attachment.BootVolumeId != nil && *attachment.BootVolumeId != "" {
			bootVolumeID = *attachment.BootVolumeId
			break
		}
	}
	if bootVolumeID == "" {
		result.ErrorCode = "OCI_BOOT_VOLUME_ATTACHMENT_NOT_FOUND"
		result.ErrorMessage = "no attached boot volume found for instance"
		result.Verified = false
		return result
	}
	result.BootVolumeID = bootVolumeID

	bootVolume, err := clients.Blockstorage.GetBootVolume(ctx, core.GetBootVolumeRequest{
		BootVolumeId: common.String(bootVolumeID),
		OpcRequestId: requestID("codex-get-boot-volume", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, bootVolume.OpcRequestId)
	if err != nil {
		result.ErrorCode = "OCI_GET_BOOT_VOLUME_FAILED"
		result.ErrorMessage = err.Error()
		result.Verified = false
		return result
	}
	if bootVolume.BootVolume.SizeInGBs != nil {
		result.CurrentBootVolumeGB = int(*bootVolume.BootVolume.SizeInGBs)
	}
	if result.CurrentBootVolumeGB > req.TargetBootVolumeGB {
		result.ErrorCode = "OCI_BOOT_VOLUME_CANNOT_SHRINK"
		result.ErrorMessage = "boot volume expansion cannot decrease disk size"
		result.Verified = false
		return result
	}
	if result.CurrentBootVolumeGB == req.TargetBootVolumeGB {
		result.TargetBootVolumeGB = result.CurrentBootVolumeGB
		return result
	}

	update, err := clients.Blockstorage.UpdateBootVolume(ctx, core.UpdateBootVolumeRequest{
		BootVolumeId: common.String(bootVolumeID),
		UpdateBootVolumeDetails: core.UpdateBootVolumeDetails{
			SizeInGBs: common.Int64(int64(req.TargetBootVolumeGB)),
		},
		OpcRequestId: requestID("codex-expand-boot-volume", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, update.OpcRequestId)
	if err != nil {
		result.ErrorCode = "OCI_UPDATE_BOOT_VOLUME_FAILED"
		result.ErrorMessage = err.Error()
		result.Verified = false
		return result
	}
	result.BootVolumeExpanded = true
	result.TargetBootVolumeGB = req.TargetBootVolumeGB

	if err := waitBootVolumeSize(ctx, clients, bootVolumeID, req.TargetBootVolumeGB, 10*time.Minute); err != nil {
		result.ErrorCode = "OCI_WAIT_BOOT_VOLUME_SIZE_FAILED"
		result.ErrorMessage = err.Error()
		result.Verified = false
		return result
	}
	return result
}

func waitBootVolumeSize(ctx context.Context, clients Clients, bootVolumeID string, targetGB int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		response, err := clients.Blockstorage.GetBootVolume(ctx, core.GetBootVolumeRequest{BootVolumeId: common.String(bootVolumeID)})
		if err != nil {
			return err
		}
		currentGB := 0
		if response.BootVolume.SizeInGBs != nil {
			currentGB = int(*response.BootVolume.SizeInGBs)
		}
		if currentGB >= targetGB && response.BootVolume.LifecycleState == core.BootVolumeLifecycleStateAvailable {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("boot volume %s did not reach %d GB before timeout; current size is %d GB", bootVolumeID, targetGB, currentGB)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

func waitForActionState(ctx context.Context, clients Clients, instanceID string, result InstanceActionExecutionResult, accepted ...core.InstanceLifecycleStateEnum) InstanceActionExecutionResult {
	result.WaitedForState = true
	state, err := waitInstanceState(ctx, clients, instanceID, 10*time.Minute, accepted...)
	result.FinalState = string(state)
	if err != nil {
		result.ErrorCode = "OCI_WAIT_INSTANCE_STATE_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	result.Verified = true
	return result
}

func retryToken(prefix string, jobID string) *string {
	value := strings.TrimSpace(jobID)
	if value == "" {
		value = time.Now().UTC().Format("20060102150405")
	}
	return common.String("codex-" + prefix + "-" + value + "-" + time.Now().UTC().Format("20060102150405.000000000"))
}

func requestID(prefix string, jobID string) *string {
	value := strings.TrimSpace(jobID)
	if value == "" {
		value = time.Now().UTC().Format("20060102150405")
	}
	return common.String(prefix + "-" + value)
}
