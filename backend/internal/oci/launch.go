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
	Verified            bool      `json:"verified"`
	ExecutionMode       string    `json:"executionMode"`
	CompartmentID       string    `json:"compartmentId"`
	AvailabilityDomain  string    `json:"availabilityDomain,omitempty"`
	SubnetID            string    `json:"subnetId,omitempty"`
	ImageID             string    `json:"imageId,omitempty"`
	DisplayName         string    `json:"displayName,omitempty"`
	InstanceID          string    `json:"instanceId,omitempty"`
	Shape               string    `json:"shape"`
	OCPUs               int       `json:"ocpus"`
	MemoryGB            int       `json:"memoryGb"`
	BootVolumeGB        int       `json:"bootVolumeGb"`
	BootVolumeVPUsPerGB int       `json:"bootVolumeVpusPerGb"`
	AssignPublicIP      bool      `json:"assignPublicIp"`
	EnableIPv6          bool      `json:"enableIpv6"`
	ReservedPublicIP    string    `json:"reservedPublicIp,omitempty"`
	ReservedPublicIPID  string    `json:"reservedPublicIpId,omitempty"`
	PrimaryVNICID       string    `json:"primaryVnicId,omitempty"`
	PrimaryPrivateIPID  string    `json:"primaryPrivateIpId,omitempty"`
	PublicIPv4          string    `json:"publicIpv4,omitempty"`
	RequestID           string    `json:"requestId,omitempty"`
	WorkRequestID       string    `json:"workRequestId,omitempty"`
	FinalState          string    `json:"finalState,omitempty"`
	ErrorCode           string    `json:"errorCode,omitempty"`
	ErrorMessage        string    `json:"errorMessage,omitempty"`
	ExecutedAt          time.Time `json:"executedAt"`
}

func LaunchInstanceFromRequest(ctx context.Context, cfg ReadinessConfig, req domain.CreateInstanceRequest, jobID string) LaunchExecutionResult {
	result := LaunchExecutionResult{
		ExecutionMode:       cfg.ExecutionMode,
		CompartmentID:       req.CompartmentID,
		SubnetID:            req.SubnetID,
		ImageID:             req.ImageID,
		DisplayName:         req.Name,
		Shape:               req.Shape,
		OCPUs:               req.OCPUs,
		MemoryGB:            req.MemoryGB,
		BootVolumeGB:        req.BootVolumeGB,
		BootVolumeVPUsPerGB: req.BootVolumeVPUsPerGB,
		AssignPublicIP:      req.AssignPublicIP || strings.TrimSpace(req.ReservedPublicIP) != "",
		EnableIPv6:          req.EnableIPv6,
		ReservedPublicIP:    strings.TrimSpace(req.ReservedPublicIP),
		ExecutedAt:          time.Now().UTC(),
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
	if result.BootVolumeVPUsPerGB <= 0 {
		result.BootVolumeVPUsPerGB = 10
	}
	if result.BootVolumeVPUsPerGB < 10 || result.BootVolumeVPUsPerGB > 120 {
		result.ErrorCode = "OCI_LAUNCH_BOOT_VOLUME_VPUS_INVALID"
		result.ErrorMessage = "bootVolumeVpusPerGb must be between 10 and 120"
		return result
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

	assignEphemeralPublicIP := result.AssignPublicIP && result.ReservedPublicIP == ""
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
				AssignPublicIp: common.Bool(assignEphemeralPublicIP),
				AssignIpv6Ip:   common.Bool(result.EnableIPv6),
			},
			Metadata:     metadata,
			FreeformTags: mapStringAnyToString(req.Tags),
			SourceDetails: core.InstanceSourceViaImageDetails{
				ImageId:             common.String(result.ImageID),
				BootVolumeSizeInGBs: common.Int64(int64(result.BootVolumeGB)),
				BootVolumeVpusPerGB: common.Int64(int64(result.BootVolumeVPUsPerGB)),
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
	if result.ReservedPublicIP != "" {
		assignment, err := assignReservedPublicIP(ctx, clients, result.CompartmentID, result.InstanceID, result.ReservedPublicIP, jobID)
		if err != nil {
			result.ErrorCode = "OCI_ASSIGN_RESERVED_PUBLIC_IP_FAILED"
			result.ErrorMessage = err.Error()
			result.Verified = false
			return result
		}
		result.ReservedPublicIPID = assignment.PublicIPID
		result.PrimaryVNICID = assignment.VNICID
		result.PrimaryPrivateIPID = assignment.PrivateIPID
		result.PublicIPv4 = assignment.PublicIPAddress
		appendFirstString(&result.RequestID, assignment.RequestID)
	} else if result.AssignPublicIP {
		if vnic, err := resolveVNIC(ctx, clients, result.CompartmentID, result.InstanceID, "primary", &result.RequestID); err == nil {
			result.PrimaryVNICID = stringValue(vnic.Id)
			result.PublicIPv4 = stringValue(vnic.PublicIp)
		}
	}
	result.Verified = true
	return result
}

type reservedPublicIPAssignment struct {
	PublicIPID      string
	PublicIPAddress string
	VNICID          string
	PrivateIPID     string
	RequestID       string
}

func assignReservedPublicIP(ctx context.Context, clients Clients, compartmentID string, instanceID string, reservedPublicIP string, jobID string) (reservedPublicIPAssignment, error) {
	vnic, err := resolveVNIC(ctx, clients, compartmentID, instanceID, "primary", nil)
	if err != nil {
		return reservedPublicIPAssignment{}, err
	}
	vnicID := stringValue(vnic.Id)
	if vnicID == "" {
		return reservedPublicIPAssignment{}, fmt.Errorf("primary VNIC ID is empty")
	}
	privateIP, err := primaryPrivateIP(ctx, clients, vnicID)
	if err != nil {
		return reservedPublicIPAssignment{}, err
	}
	privateIPID := stringValue(privateIP.Id)
	if privateIPID == "" {
		return reservedPublicIPAssignment{}, fmt.Errorf("primary private IP ID is empty")
	}

	publicIP, requestIDValue, err := resolveReservedPublicIP(ctx, clients, compartmentID, reservedPublicIP)
	if err != nil {
		return reservedPublicIPAssignment{}, err
	}
	publicIPID := stringValue(publicIP.Id)
	if publicIPID == "" {
		return reservedPublicIPAssignment{}, fmt.Errorf("reserved public IP ID is empty")
	}
	if publicIP.Lifetime != core.PublicIpLifetimeReserved {
		return reservedPublicIPAssignment{}, fmt.Errorf("public IP %s is not RESERVED", publicIPID)
	}
	assignedEntityID := strings.TrimSpace(stringValue(publicIP.AssignedEntityId))
	privateIDOnPublicIP := strings.TrimSpace(stringValue(publicIP.PrivateIpId))
	if assignedEntityID != "" && assignedEntityID != privateIPID {
		return reservedPublicIPAssignment{}, fmt.Errorf("reserved public IP %s is already assigned to another entity", publicIPID)
	}
	if privateIDOnPublicIP != "" && privateIDOnPublicIP != privateIPID {
		return reservedPublicIPAssignment{}, fmt.Errorf("reserved public IP %s is already assigned to another private IP", publicIPID)
	}

	if assignedEntityID != privateIPID && privateIDOnPublicIP != privateIPID {
		update, err := clients.VirtualNetwork.UpdatePublicIp(ctx, core.UpdatePublicIpRequest{
			PublicIpId: common.String(publicIPID),
			UpdatePublicIpDetails: core.UpdatePublicIpDetails{
				PrivateIpId: common.String(privateIPID),
			},
			OpcRequestId: requestID("codex-assign-reserved-ip", jobID),
		})
		requestIDValue = firstNonEmpty(requestIDValue, stringValue(update.OpcRequestId))
		if err != nil {
			return reservedPublicIPAssignment{}, err
		}
		publicIP = update.PublicIp
	}

	publicIP, waitRequestID, err := waitPublicIPAssigned(ctx, clients, publicIPID, privateIPID, 5*time.Minute)
	requestIDValue = firstNonEmpty(requestIDValue, waitRequestID)
	if err != nil {
		return reservedPublicIPAssignment{}, err
	}
	return reservedPublicIPAssignment{
		PublicIPID:      publicIPID,
		PublicIPAddress: stringValue(publicIP.IpAddress),
		VNICID:          vnicID,
		PrivateIPID:     privateIPID,
		RequestID:       requestIDValue,
	}, nil
}

func primaryPrivateIP(ctx context.Context, clients Clients, vnicID string) (core.PrivateIp, error) {
	response, err := clients.VirtualNetwork.ListPrivateIps(ctx, core.ListPrivateIpsRequest{
		VnicId: common.String(vnicID),
		Limit:  common.Int(50),
	})
	if err != nil {
		return core.PrivateIp{}, err
	}
	var fallback core.PrivateIp
	for _, item := range response.Items {
		if item.Id == nil || stringValue(item.Id) == "" {
			continue
		}
		if fallback.Id == nil {
			fallback = item
		}
		if item.IsPrimary == nil || *item.IsPrimary {
			return item, nil
		}
	}
	if fallback.Id != nil {
		return fallback, nil
	}
	return core.PrivateIp{}, fmt.Errorf("no private IP found for VNIC %s", vnicID)
}

func resolveReservedPublicIP(ctx context.Context, clients Clients, compartmentID string, value string) (core.PublicIp, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return core.PublicIp{}, "", fmt.Errorf("reserved public IP is required")
	}
	if strings.HasPrefix(value, "ocid1.publicip.") {
		response, err := clients.VirtualNetwork.GetPublicIp(ctx, core.GetPublicIpRequest{
			PublicIpId: common.String(value),
		})
		return response.PublicIp, stringValue(response.OpcRequestId), err
	}

	response, err := clients.VirtualNetwork.ListPublicIps(ctx, core.ListPublicIpsRequest{
		CompartmentId: common.String(compartmentID),
		Scope:         core.ListPublicIpsScopeRegion,
		Lifetime:      core.ListPublicIpsLifetimeReserved,
		Limit:         common.Int(100),
	})
	if err != nil {
		return core.PublicIp{}, stringValue(response.OpcRequestId), err
	}
	for _, item := range response.Items {
		if strings.EqualFold(stringValue(item.IpAddress), value) || strings.EqualFold(stringValue(item.DisplayName), value) || strings.EqualFold(stringValue(item.Id), value) {
			return item, stringValue(response.OpcRequestId), nil
		}
	}
	return core.PublicIp{}, stringValue(response.OpcRequestId), fmt.Errorf("reserved public IP %q was not found in compartment %s", value, compartmentID)
}

func waitPublicIPAssigned(ctx context.Context, clients Clients, publicIPID string, privateIPID string, timeout time.Duration) (core.PublicIp, string, error) {
	deadline := time.Now().Add(timeout)
	requestIDValue := ""
	for {
		response, err := clients.VirtualNetwork.GetPublicIp(ctx, core.GetPublicIpRequest{
			PublicIpId: common.String(publicIPID),
		})
		appendFirstString(&requestIDValue, stringValue(response.OpcRequestId))
		if err != nil {
			return response.PublicIp, requestIDValue, err
		}
		if strings.TrimSpace(stringValue(response.PublicIp.AssignedEntityId)) == privateIPID || strings.TrimSpace(stringValue(response.PublicIp.PrivateIpId)) == privateIPID {
			return response.PublicIp, requestIDValue, nil
		}
		if time.Now().After(deadline) {
			return response.PublicIp, requestIDValue, fmt.Errorf("timed out waiting for reserved public IP %s to assign to private IP %s; last state=%s", publicIPID, privateIPID, response.PublicIp.LifecycleState)
		}
		select {
		case <-ctx.Done():
			return response.PublicIp, requestIDValue, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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
		Name:                stringFromAny(input["name"]),
		ProfileID:           stringFromAny(input["profileId"]),
		Region:              stringFromAny(input["region"]),
		Compartment:         stringFromAny(input["compartment"]),
		CompartmentID:       stringFromAny(input["compartmentId"]),
		AvailabilityAD:      stringFromAny(input["availabilityAd"]),
		TemplateID:          stringFromAny(input["templateId"]),
		ImageID:             stringFromAny(input["imageId"]),
		Shape:               stringFromAny(input["shape"]),
		OCPUs:               intFromAny(input["ocpus"]),
		MemoryGB:            intFromAny(input["memoryGb"]),
		BootVolumeGB:        intFromAny(input["bootVolumeGb"]),
		BootVolumeVPUsPerGB: intFromAny(input["bootVolumeVpusPerGb"]),
		AssignPublicIP:      boolFromAny(input["assignPublicIp"]),
		EnableIPv6:          boolFromAny(input["enableIpv6"]),
		ReservedPublicIP:    stringFromAny(input["reservedPublicIp"]),
		VCNID:               stringFromAny(input["vcnId"]),
		SubnetID:            stringFromAny(input["subnetId"]),
		SSHKey:              stringFromAny(input["sshKey"]),
		CloudInit:           stringFromAny(input["cloudInit"]),
		RequireApproval:     boolFromAny(input["requireApproval"]),
		SnapshotBefore:      boolFromAny(input["snapshotBefore"]),
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
