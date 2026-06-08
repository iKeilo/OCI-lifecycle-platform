package oci

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"a-series-oracle/backend/internal/domain"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type LaunchExecutionResult struct {
	Verified           bool      `json:"verified"`
	ExecutionMode      string    `json:"executionMode"`
	CompartmentID      string    `json:"compartmentId"`
	AvailabilityDomain string    `json:"availabilityDomain,omitempty"`
	SubnetID           string    `json:"subnetId,omitempty"`
	ImageID            string    `json:"imageId,omitempty"`
	DisplayName        string    `json:"displayName,omitempty"`
	InstanceID         string    `json:"instanceId,omitempty"`
	Shape              string    `json:"shape"`
	OCPUs              int       `json:"ocpus"`
	MemoryGB           int       `json:"memoryGb"`
	BootVolumeGB       int       `json:"bootVolumeGb"`
	AssignPublicIP     bool      `json:"assignPublicIp"`
	EnableIPv6         bool      `json:"enableIpv6"`
	RequestID          string    `json:"requestId,omitempty"`
	WorkRequestID      string    `json:"workRequestId,omitempty"`
	FinalState         string    `json:"finalState,omitempty"`
	ErrorCode          string    `json:"errorCode,omitempty"`
	ErrorMessage       string    `json:"errorMessage,omitempty"`
	ExecutedAt         time.Time `json:"executedAt"`
}

func LaunchInstanceFromRequest(ctx context.Context, cfg ReadinessConfig, req domain.CreateInstanceRequest, jobID string) LaunchExecutionResult {
	result := LaunchExecutionResult{
		ExecutionMode:  cfg.ExecutionMode,
		CompartmentID:  req.CompartmentID,
		SubnetID:       req.SubnetID,
		ImageID:        req.ImageID,
		DisplayName:    req.Name,
		Shape:          req.Shape,
		OCPUs:          req.OCPUs,
		MemoryGB:       req.MemoryGB,
		BootVolumeGB:   req.BootVolumeGB,
		AssignPublicIP: req.AssignPublicIP,
		EnableIPv6:     req.EnableIPv6,
		ExecutedAt:     time.Now().UTC(),
	}
	readiness := CheckReadiness(cfg)
	if !readiness.Ready {
		result.ErrorCode = "OCI_NOT_READY"
		result.ErrorMessage = readiness.Message
		return result
	}
	if strings.TrimSpace(result.DisplayName) == "" {
		result.ErrorCode = "OCI_LAUNCH_NAME_REQUIRED"
		result.ErrorMessage = "name is required"
		return result
	}
	if strings.TrimSpace(result.Shape) == "" {
		result.ErrorCode = "OCI_LAUNCH_SHAPE_REQUIRED"
		result.ErrorMessage = "shape is required"
		return result
	}
	if result.OCPUs <= 0 || result.MemoryGB <= 0 {
		result.ErrorCode = "OCI_LAUNCH_SHAPE_CONFIG_REQUIRED"
		result.ErrorMessage = "ocpus and memoryGb must be greater than zero"
		return result
	}
	if result.BootVolumeGB <= 0 {
		result.BootVolumeGB = 50
	}
	if result.CompartmentID == "" {
		result.CompartmentID = cfg.TenancyOCID
	}

	clients, err := NewClients(cfg)
	if err != nil {
		result.ErrorCode = "OCI_CLIENT_INIT_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	if req.AvailabilityAD != "" {
		result.AvailabilityDomain = req.AvailabilityAD
	} else {
		ad, err := firstAvailabilityDomain(ctx, clients, result.CompartmentID)
		if err != nil {
			result.ErrorCode = "OCI_DISCOVER_AD_FAILED"
			result.ErrorMessage = err.Error()
			return result
		}
		result.AvailabilityDomain = ad
	}
	if result.SubnetID == "" {
		subnetID, err := firstAvailableSubnet(ctx, clients, result.CompartmentID)
		if err != nil {
			result.ErrorCode = "OCI_DISCOVER_SUBNET_FAILED"
			result.ErrorMessage = err.Error()
			return result
		}
		result.SubnetID = subnetID
	}
	if result.ImageID == "" {
		imageID, err := firstCompatibleImageForShape(ctx, clients, result.CompartmentID, result.Shape)
		if err != nil {
			result.ErrorCode = "OCI_DISCOVER_IMAGE_FAILED"
			result.ErrorMessage = err.Error()
			return result
		}
		result.ImageID = imageID
	}

	metadata := map[string]string{}
	if strings.TrimSpace(req.SSHKey) != "" {
		metadata["ssh_authorized_keys"] = req.SSHKey
	}
	if strings.TrimSpace(req.CloudInit) != "" {
		metadata["user_data"] = base64.StdEncoding.EncodeToString([]byte(req.CloudInit))
	}

	response, err := clients.Compute.LaunchInstance(ctx, core.LaunchInstanceRequest{
		LaunchInstanceDetails: core.LaunchInstanceDetails{
			AvailabilityDomain: common.String(result.AvailabilityDomain),
			CompartmentId:      common.String(result.CompartmentID),
			DisplayName:        common.String(result.DisplayName),
			Shape:              common.String(result.Shape),
			ShapeConfig: &core.LaunchInstanceShapeConfigDetails{
				Ocpus:       common.Float32(float32(result.OCPUs)),
				MemoryInGBs: common.Float32(float32(result.MemoryGB)),
			},
			CreateVnicDetails: &core.CreateVnicDetails{
				SubnetId:       common.String(result.SubnetID),
				AssignPublicIp: common.Bool(result.AssignPublicIP),
				AssignIpv6Ip:   common.Bool(result.EnableIPv6),
			},
			Metadata:     metadata,
			FreeformTags: mapStringAnyToString(req.Tags),
			SourceDetails: core.InstanceSourceViaImageDetails{
				ImageId:             common.String(result.ImageID),
				BootVolumeSizeInGBs: common.Int64(int64(result.BootVolumeGB)),
			},
		},
		OpcRetryToken: retryToken("launch", jobID),
		OpcRequestId:  requestID("codex-launch", jobID),
	})
	if response.OpcRequestId != nil {
		result.RequestID = *response.OpcRequestId
	}
	if response.OpcWorkRequestId != nil {
		result.WorkRequestID = *response.OpcWorkRequestId
	}
	if response.Instance.Id != nil {
		result.InstanceID = *response.Instance.Id
	}
	if err != nil {
		result.ErrorCode = "OCI_LAUNCH_INSTANCE_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	if result.InstanceID == "" {
		result.ErrorCode = "OCI_LAUNCH_INSTANCE_EMPTY_ID"
		result.ErrorMessage = "LaunchInstance succeeded without an instance OCID"
		return result
	}

	state, err := waitInstanceState(ctx, clients, result.InstanceID, 15*time.Minute, core.InstanceLifecycleStateRunning, core.InstanceLifecycleStateStopped)
	result.FinalState = string(state)
	if err != nil {
		result.ErrorCode = "OCI_WAIT_INSTANCE_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	result.Verified = true
	return result
}

func mapStringAnyToString(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func CreateRequestFromJobInput(input map[string]any) (domain.CreateInstanceRequest, error) {
	req := domain.CreateInstanceRequest{
		Name:             stringFromAny(input["name"]),
		ProfileID:        stringFromAny(input["profileId"]),
		Region:           stringFromAny(input["region"]),
		Compartment:      stringFromAny(input["compartment"]),
		CompartmentID:    stringFromAny(input["compartmentId"]),
		AvailabilityAD:   stringFromAny(input["availabilityAd"]),
		TemplateID:       stringFromAny(input["templateId"]),
		ImageID:          stringFromAny(input["imageId"]),
		Shape:            stringFromAny(input["shape"]),
		OCPUs:            intFromAny(input["ocpus"]),
		MemoryGB:         intFromAny(input["memoryGb"]),
		BootVolumeGB:     intFromAny(input["bootVolumeGb"]),
		AssignPublicIP:   boolFromAny(input["assignPublicIp"]),
		EnableIPv6:       boolFromAny(input["enableIpv6"]),
		ReservedPublicIP: stringFromAny(input["reservedPublicIp"]),
		VCNID:            stringFromAny(input["vcnId"]),
		SubnetID:         stringFromAny(input["subnetId"]),
		SSHKey:           stringFromAny(input["sshKey"]),
		CloudInit:        stringFromAny(input["cloudInit"]),
		RequireApproval:  boolFromAny(input["requireApproval"]),
		SnapshotBefore:   boolFromAny(input["snapshotBefore"]),
	}
	if tags, ok := input["tags"].(map[string]string); ok {
		req.Tags = tags
	} else if raw, ok := input["tags"].(map[string]any); ok {
		req.Tags = make(map[string]string, len(raw))
		for key, value := range raw {
			req.Tags[key] = fmt.Sprint(value)
		}
	}
	return req, nil
}

func stringFromAny(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func boolFromAny(value any) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	return false
}

func intFromAny(value any) int {
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
