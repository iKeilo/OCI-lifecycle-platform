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
	JobID              string
}

type InstanceActionExecutionResult struct {
	Verified         bool      `json:"verified"`
	ExecutionMode    string    `json:"executionMode"`
	InstanceID       string    `json:"instanceId"`
	Action           string    `json:"action"`
	RequestID        string    `json:"requestId,omitempty"`
	WorkRequestID    string    `json:"workRequestId,omitempty"`
	InitialState     string    `json:"initialState,omitempty"`
	FinalState       string    `json:"finalState,omitempty"`
	TargetShape      string    `json:"targetShape,omitempty"`
	TargetOCPUs      int       `json:"targetOcpus,omitempty"`
	TargetMemoryGB   int       `json:"targetMemoryGb,omitempty"`
	ErrorCode        string    `json:"errorCode,omitempty"`
	ErrorMessage     string    `json:"errorMessage,omitempty"`
	ExecutedAt       time.Time `json:"executedAt"`
	WaitedForState   bool      `json:"waitedForState"`
	PreserveBootDisk bool      `json:"preserveBootVolume"`
}

func ExecuteInstanceLifecycleAction(ctx context.Context, cfg ReadinessConfig, req InstanceActionExecutionRequest) InstanceActionExecutionResult {
	result := InstanceActionExecutionResult{
		ExecutionMode:    cfg.ExecutionMode,
		InstanceID:       req.InstanceID,
		Action:           string(req.Action),
		TargetShape:      req.TargetShape,
		TargetOCPUs:      req.TargetOCPUs,
		TargetMemoryGB:   req.TargetMemoryGB,
		ExecutedAt:       time.Now().UTC(),
		PreserveBootDisk: req.PreserveBootVolume,
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
		return executeResize(ctx, clients, req, result)
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

func executeResize(ctx context.Context, clients Clients, req InstanceActionExecutionRequest, result InstanceActionExecutionResult) InstanceActionExecutionResult {
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
	return waitForActionState(ctx, clients, req.InstanceID, result, core.InstanceLifecycleStateRunning, core.InstanceLifecycleStateStopped)
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
