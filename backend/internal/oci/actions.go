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
	InstanceID                string
	Action                    domain.InstanceLifecycleAction
	Graceful                  bool
	PreserveBootVolume        bool
	TargetShape               string
	TargetOCPUs               int
	TargetMemoryGB            int
	TargetBootVolumeGB        int
	TargetBootVolumeVPUsPerGB int
	ExpandBootVolume          bool
	JobID                     string
}

type InstanceActionExecutionResult struct {
	Verified                     bool      `json:"verified"`
	ExecutionMode                string    `json:"executionMode"`
	InstanceID                   string    `json:"instanceId"`
	Action                       string    `json:"action"`
	RequestID                    string    `json:"requestId,omitempty"`
	WorkRequestID                string    `json:"workRequestId,omitempty"`
	InitialState                 string    `json:"initialState,omitempty"`
	FinalState                   string    `json:"finalState,omitempty"`
	TargetShape                  string    `json:"targetShape,omitempty"`
	TargetOCPUs                  int       `json:"targetOcpus,omitempty"`
	TargetMemoryGB               int       `json:"targetMemoryGb,omitempty"`
	TargetBootVolumeGB           int       `json:"targetBootVolumeGb,omitempty"`
	TargetBootVolumeVPUsPerGB    int       `json:"targetBootVolumeVpusPerGb,omitempty"`
	CurrentBootVolumeGB          int       `json:"currentBootVolumeGb,omitempty"`
	CurrentBootVolumeVPUsPerGB   int       `json:"currentBootVolumeVpusPerGb,omitempty"`
	BootVolumeID                 string    `json:"bootVolumeId,omitempty"`
	BootVolumeExpanded           bool      `json:"bootVolumeExpanded"`
	BootVolumePerformanceChanged bool      `json:"bootVolumePerformanceChanged"`
	ErrorCode                    string    `json:"errorCode,omitempty"`
	ErrorMessage                 string    `json:"errorMessage,omitempty"`
	ExecutedAt                   time.Time `json:"executedAt"`
	WaitedForState               bool      `json:"waitedForState"`
	PreserveBootDisk             bool      `json:"preserveBootVolume"`
}

type InstanceReinstallExecutionRequest struct {
	InstanceID             string
	ImageID                string
	ImageName              string
	BootVolumeSizeGB       int
	BootVolumeVPUsPerGB    int
	PreserveOldBootVolume  bool
	CreateBootVolumeBackup bool
	JobID                  string
}

type InstanceReinstallExecutionResult struct {
	Verified                  bool      `json:"verified"`
	ExecutionMode             string    `json:"executionMode"`
	InstanceID                string    `json:"instanceId"`
	ImageID                   string    `json:"imageId"`
	ImageName                 string    `json:"imageName,omitempty"`
	RequestID                 string    `json:"requestId,omitempty"`
	WorkRequestID             string    `json:"workRequestId,omitempty"`
	InitialState              string    `json:"initialState,omitempty"`
	FinalState                string    `json:"finalState,omitempty"`
	BootVolumeSizeGB          int       `json:"bootVolumeSizeGb,omitempty"`
	BootVolumeVPUsPerGB       int       `json:"bootVolumeVpusPerGb,omitempty"`
	TargetBootVolumeGB        int       `json:"targetBootVolumeGb,omitempty"`
	TargetBootVolumeVPUsPerGB int       `json:"targetBootVolumeVpusPerGb,omitempty"`
	PreserveOldBootVolume     bool      `json:"preserveOldBootVolume"`
	ErrorCode                 string    `json:"errorCode,omitempty"`
	ErrorMessage              string    `json:"errorMessage,omitempty"`
	ExecutedAt                time.Time `json:"executedAt"`
	WaitedForState            bool      `json:"waitedForState"`
}

func ExecuteInstanceLifecycleAction(ctx context.Context, cfg ReadinessConfig, req InstanceActionExecutionRequest) InstanceActionExecutionResult {
	result := InstanceActionExecutionResult{
		ExecutionMode:             cfg.ExecutionMode,
		InstanceID:                req.InstanceID,
		Action:                    string(req.Action),
		TargetShape:               req.TargetShape,
		TargetOCPUs:               req.TargetOCPUs,
		TargetMemoryGB:            req.TargetMemoryGB,
		TargetBootVolumeGB:        req.TargetBootVolumeGB,
		TargetBootVolumeVPUsPerGB: req.TargetBootVolumeVPUsPerGB,
		ExecutedAt:                time.Now().UTC(),
		PreserveBootDisk:          req.PreserveBootVolume,
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

func ExecuteInstanceReinstall(ctx context.Context, cfg ReadinessConfig, req InstanceReinstallExecutionRequest) InstanceReinstallExecutionResult {
	result := InstanceReinstallExecutionResult{
		ExecutionMode:             cfg.ExecutionMode,
		InstanceID:                req.InstanceID,
		ImageID:                   req.ImageID,
		ImageName:                 req.ImageName,
		BootVolumeSizeGB:          req.BootVolumeSizeGB,
		BootVolumeVPUsPerGB:       req.BootVolumeVPUsPerGB,
		TargetBootVolumeGB:        req.BootVolumeSizeGB,
		TargetBootVolumeVPUsPerGB: req.BootVolumeVPUsPerGB,
		PreserveOldBootVolume:     req.PreserveOldBootVolume,
		ExecutedAt:                time.Now().UTC(),
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
	if strings.TrimSpace(req.ImageID) == "" {
		result.ErrorCode = "OCI_REINSTALL_IMAGE_REQUIRED"
		result.ErrorMessage = "imageId is required"
		return result
	}
	if req.CreateBootVolumeBackup {
		result.ErrorCode = "OCI_REINSTALL_BACKUP_NOT_IMPLEMENTED"
		result.ErrorMessage = "boot volume backup before reinstall is not implemented yet"
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

	source := core.UpdateInstanceSourceViaImageDetails{
		ImageId:                     common.String(strings.TrimSpace(req.ImageID)),
		IsPreserveBootVolumeEnabled: common.Bool(req.PreserveOldBootVolume),
	}
	if req.BootVolumeSizeGB > 0 {
		source.BootVolumeSizeInGBs = common.Int64(int64(req.BootVolumeSizeGB))
	}
	response, err := clients.Compute.UpdateInstance(ctx, core.UpdateInstanceRequest{
		InstanceId: common.String(req.InstanceID),
		UpdateInstanceDetails: core.UpdateInstanceDetails{
			SourceDetails:             source,
			UpdateOperationConstraint: core.UpdateInstanceDetailsUpdateOperationConstraintAllowDowntime,
		},
		OpcRetryToken: retryToken("reinstall", req.JobID),
		OpcRequestId:  requestID("codex-reinstall", req.JobID),
	})
	if response.OpcRequestId != nil {
		result.RequestID = *response.OpcRequestId
	}
	if response.OpcWorkRequestId != nil {
		result.WorkRequestID = *response.OpcWorkRequestId
	}
	if err != nil {
		result.ErrorCode = "OCI_REINSTALL_INSTANCE_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	result.WaitedForState = true
	state, err := waitInstanceState(ctx, clients, req.InstanceID, 15*time.Minute, core.InstanceLifecycleStateRunning, core.InstanceLifecycleStateStopped)
	result.FinalState = string(state)
	if err != nil {
		result.ErrorCode = "OCI_WAIT_REINSTALLED_INSTANCE_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	result.Verified = true

	if req.BootVolumeVPUsPerGB > 0 {
		actionReq := InstanceActionExecutionRequest{
			InstanceID:                req.InstanceID,
			TargetBootVolumeGB:        req.BootVolumeSizeGB,
			TargetBootVolumeVPUsPerGB: req.BootVolumeVPUsPerGB,
			ExpandBootVolume:          true,
			JobID:                     req.JobID,
		}
		actionResult := InstanceActionExecutionResult{
			Verified:                  true,
			ExecutionMode:             cfg.ExecutionMode,
			InstanceID:                req.InstanceID,
			RequestID:                 result.RequestID,
			WorkRequestID:             result.WorkRequestID,
			FinalState:                result.FinalState,
			TargetBootVolumeGB:        req.BootVolumeSizeGB,
			TargetBootVolumeVPUsPerGB: req.BootVolumeVPUsPerGB,
			ExecutedAt:                result.ExecutedAt,
			WaitedForState:            true,
			PreserveBootDisk:          req.PreserveOldBootVolume,
		}
		actionResult = executeBootVolumeExpansion(ctx, clients, actionReq, actionResult, current.Instance)
		result.RequestID = actionResult.RequestID
		result.TargetBootVolumeGB = actionResult.TargetBootVolumeGB
		result.TargetBootVolumeVPUsPerGB = actionResult.TargetBootVolumeVPUsPerGB
		if !actionResult.Verified {
			result.Verified = false
			result.ErrorCode = actionResult.ErrorCode
			result.ErrorMessage = actionResult.ErrorMessage
		}
	}
	return result
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
	targetIsFlexible := isFlexibleShape(req.TargetShape)
	if targetIsFlexible && (req.TargetOCPUs <= 0 || req.TargetMemoryGB <= 0) {
		result.ErrorCode = "OCI_RESIZE_SHAPE_CONFIG_REQUIRED"
		result.ErrorMessage = "targetOcpus and targetMemoryGb must be greater than zero for flexible shapes"
		return result
	}

	if !instanceResizeRequired(instance, req) {
		result.FinalState = string(instance.LifecycleState)
		result.Verified = true
		if req.ExpandBootVolume || req.TargetBootVolumeVPUsPerGB > 0 {
			return executeBootVolumeExpansion(ctx, clients, req, result, instance)
		}
		return result
	}

	updateDetails := core.UpdateInstanceDetails{
		Shape:                     common.String(req.TargetShape),
		UpdateOperationConstraint: core.UpdateInstanceDetailsUpdateOperationConstraintAllowDowntime,
	}
	if targetIsFlexible {
		updateDetails.ShapeConfig = &core.UpdateInstanceShapeConfigDetails{
			Ocpus:       common.Float32(float32(req.TargetOCPUs)),
			MemoryInGBs: common.Float32(float32(req.TargetMemoryGB)),
		}
	}
	response, err := clients.Compute.UpdateInstance(ctx, core.UpdateInstanceRequest{
		InstanceId:            common.String(req.InstanceID),
		UpdateInstanceDetails: updateDetails,
		OpcRetryToken:         retryToken("resize", req.JobID),
		OpcRequestId:          requestID("codex-resize", req.JobID),
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
	if req.ExpandBootVolume || req.TargetBootVolumeVPUsPerGB > 0 {
		return executeBootVolumeExpansion(ctx, clients, req, result, instance)
	}
	return result
}

func isFlexibleShape(shape string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(shape)), ".flex")
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
	bootVolumeID = firstAttachedBootVolumeID(attachments.Items)
	if bootVolumeID == "" {
		bootVolumeID, err = waitBootVolumeAttachment(ctx, clients, availabilityDomain, compartmentID, req.InstanceID, 10*time.Minute, requestID("codex-list-boot-volume", req.JobID))
		if err != nil {
			result.ErrorCode = "OCI_BOOT_VOLUME_ATTACHMENT_NOT_FOUND"
			result.ErrorMessage = err.Error()
			result.Verified = false
			return result
		}
	}
	result.BootVolumeID = bootVolumeID

	readyVolume, err := waitBootVolumeReady(ctx, clients, bootVolumeID, 20*time.Minute, requestID("codex-get-boot-volume", req.JobID))
	if err != nil {
		result.ErrorCode = "OCI_WAIT_BOOT_VOLUME_READY_FAILED"
		result.ErrorMessage = err.Error()
		result.Verified = false
		return result
	}
	if readyVolume.SizeInGBs != nil {
		result.CurrentBootVolumeGB = int(*readyVolume.SizeInGBs)
	}
	if readyVolume.VpusPerGB != nil {
		result.CurrentBootVolumeVPUsPerGB = int(*readyVolume.VpusPerGB)
	}
	targetBootVolumeGB := req.TargetBootVolumeGB
	if targetBootVolumeGB <= 0 {
		targetBootVolumeGB = result.CurrentBootVolumeGB
	}
	if result.CurrentBootVolumeGB > targetBootVolumeGB {
		result.ErrorCode = "OCI_BOOT_VOLUME_CANNOT_SHRINK"
		result.ErrorMessage = "boot volume expansion cannot decrease disk size"
		result.Verified = false
		return result
	}
	targetVPUsPerGB := req.TargetBootVolumeVPUsPerGB
	if targetVPUsPerGB <= 0 {
		targetVPUsPerGB = result.CurrentBootVolumeVPUsPerGB
	}
	if targetVPUsPerGB <= 0 {
		targetVPUsPerGB = 10
	}
	if targetVPUsPerGB < 10 || targetVPUsPerGB > 120 {
		result.ErrorCode = "OCI_BOOT_VOLUME_VPUS_INVALID"
		result.ErrorMessage = "boot volume VPUs/GB must be between 10 and 120"
		result.Verified = false
		return result
	}
	result.TargetBootVolumeVPUsPerGB = targetVPUsPerGB
	if result.CurrentBootVolumeGB == targetBootVolumeGB && result.CurrentBootVolumeVPUsPerGB == targetVPUsPerGB {
		result.TargetBootVolumeGB = result.CurrentBootVolumeGB
		return result
	}

	updateDetails := core.UpdateBootVolumeDetails{}
	if result.CurrentBootVolumeGB != targetBootVolumeGB {
		updateDetails.SizeInGBs = common.Int64(int64(targetBootVolumeGB))
	}
	if result.CurrentBootVolumeVPUsPerGB != targetVPUsPerGB {
		updateDetails.VpusPerGB = common.Int64(int64(targetVPUsPerGB))
	}
	update, err := clients.Blockstorage.UpdateBootVolume(ctx, core.UpdateBootVolumeRequest{
		BootVolumeId:            common.String(bootVolumeID),
		UpdateBootVolumeDetails: updateDetails,
		OpcRequestId:            requestID("codex-expand-boot-volume", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, update.OpcRequestId)
	if err != nil {
		result.ErrorCode = "OCI_UPDATE_BOOT_VOLUME_FAILED"
		result.ErrorMessage = err.Error()
		result.Verified = false
		return result
	}
	result.BootVolumeExpanded = targetBootVolumeGB > result.CurrentBootVolumeGB
	result.BootVolumePerformanceChanged = result.CurrentBootVolumeVPUsPerGB != targetVPUsPerGB
	result.TargetBootVolumeGB = targetBootVolumeGB

	if err := waitBootVolumeTarget(ctx, clients, bootVolumeID, targetBootVolumeGB, targetVPUsPerGB, 10*time.Minute); err != nil {
		result.ErrorCode = "OCI_WAIT_BOOT_VOLUME_SIZE_FAILED"
		result.ErrorMessage = err.Error()
		result.Verified = false
		return result
	}
	return result
}

func instanceResizeRequired(instance core.Instance, req InstanceActionExecutionRequest) bool {
	if strings.TrimSpace(stringValue(instance.Shape)) != strings.TrimSpace(req.TargetShape) {
		return true
	}
	if !isFlexibleShape(req.TargetShape) {
		return false
	}
	if instance.ShapeConfig == nil {
		return true
	}
	currentOCPUs := intFromFloat32Ptr(instance.ShapeConfig.Ocpus)
	currentMemoryGB := intFromFloat32Ptr(instance.ShapeConfig.MemoryInGBs)
	return currentOCPUs != req.TargetOCPUs || currentMemoryGB != req.TargetMemoryGB
}

func intFromFloat32Ptr(value *float32) int {
	if value == nil {
		return 0
	}
	return int(*value)
}

func bootVolumeReady(volume core.BootVolume) bool {
	hydrated := volume.IsHydrated == nil || *volume.IsHydrated
	return volume.LifecycleState == core.BootVolumeLifecycleStateAvailable && hydrated
}

func waitBootVolumeReady(ctx context.Context, clients Clients, bootVolumeID string, timeout time.Duration, opcRequestID *string) (core.BootVolume, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		response, err := clients.Blockstorage.GetBootVolume(ctx, core.GetBootVolumeRequest{
			BootVolumeId: common.String(bootVolumeID),
			OpcRequestId: opcRequestID,
		})
		if err != nil {
			lastErr = err
		} else {
			lastErr = nil
			if bootVolumeReady(response.BootVolume) {
				return response.BootVolume, nil
			}
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return core.BootVolume{}, fmt.Errorf("boot volume %s did not become available and hydrated before timeout; last error: %w", bootVolumeID, lastErr)
			}
			return core.BootVolume{}, fmt.Errorf("boot volume %s did not become available and hydrated before timeout", bootVolumeID)
		}
		select {
		case <-ctx.Done():
			return core.BootVolume{}, ctx.Err()
		case <-time.After(15 * time.Second):
		}
	}
}

func firstAttachedBootVolumeID(items []core.BootVolumeAttachment) string {
	for _, attachment := range items {
		if attachment.LifecycleState == core.BootVolumeAttachmentLifecycleStateDetached || attachment.LifecycleState == core.BootVolumeAttachmentLifecycleStateDetaching {
			continue
		}
		if attachment.BootVolumeId != nil && *attachment.BootVolumeId != "" {
			return *attachment.BootVolumeId
		}
	}
	return ""
}

func waitBootVolumeAttachment(ctx context.Context, clients Clients, availabilityDomain, compartmentID, instanceID string, timeout time.Duration, opcRequestID *string) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		attachments, err := clients.Compute.ListBootVolumeAttachments(ctx, core.ListBootVolumeAttachmentsRequest{
			AvailabilityDomain: common.String(availabilityDomain),
			CompartmentId:      common.String(compartmentID),
			InstanceId:         common.String(instanceID),
			Limit:              common.Int(25),
			OpcRequestId:       opcRequestID,
		})
		if err != nil {
			return "", err
		}
		if bootVolumeID := firstAttachedBootVolumeID(attachments.Items); bootVolumeID != "" {
			return bootVolumeID, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("no attached boot volume found for instance %s before timeout", instanceID)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

func waitBootVolumeSize(ctx context.Context, clients Clients, bootVolumeID string, targetGB int, timeout time.Duration) error {
	return waitBootVolumeTarget(ctx, clients, bootVolumeID, targetGB, 0, timeout)
}

func waitBootVolumeTarget(ctx context.Context, clients Clients, bootVolumeID string, targetGB int, targetVPUsPerGB int, timeout time.Duration) error {
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
		currentVPUsPerGB := 0
		if response.BootVolume.VpusPerGB != nil {
			currentVPUsPerGB = int(*response.BootVolume.VpusPerGB)
		}
		vpusReady := targetVPUsPerGB <= 0 || currentVPUsPerGB == targetVPUsPerGB
		if currentGB >= targetGB && vpusReady && bootVolumeReady(response.BootVolume) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("boot volume %s did not reach %d GB / %d VPUs per GB before timeout; current size is %d GB / %d VPUs per GB", bootVolumeID, targetGB, targetVPUsPerGB, currentGB, currentVPUsPerGB)
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
