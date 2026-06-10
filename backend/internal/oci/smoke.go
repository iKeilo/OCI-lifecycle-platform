package oci

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

const smokeDisplayNamePrefix = "codex-smoke-e2micro-"
const smokeShape = "VM.Standard.E2.1.Micro"
const e3LifecycleDisplayNamePrefix = "codex-e3flex-lifecycle-"
const e3LifecycleShape = "VM.Standard.E3.Flex"

type E2MicroSmokeRequest struct {
	CompartmentID string `json:"compartmentId"`
	SubnetID      string `json:"subnetId"`
	ImageID       string `json:"imageId"`
}

type E2MicroSmokeResult struct {
	Verified            bool      `json:"verified"`
	CompartmentID       string    `json:"compartmentId"`
	AvailabilityDomain  string    `json:"availabilityDomain,omitempty"`
	SubnetID            string    `json:"subnetId,omitempty"`
	ImageID             string    `json:"imageId,omitempty"`
	DisplayName         string    `json:"displayName,omitempty"`
	InstanceID          string    `json:"instanceId,omitempty"`
	LaunchRequestID     string    `json:"launchRequestId,omitempty"`
	LaunchWorkRequestID string    `json:"launchWorkRequestId,omitempty"`
	TerminateRequestID  string    `json:"terminateRequestId,omitempty"`
	FinalState          string    `json:"finalState,omitempty"`
	ErrorCode           string    `json:"errorCode,omitempty"`
	ErrorMessage        string    `json:"errorMessage,omitempty"`
	CleanupAttempted    bool      `json:"cleanupAttempted"`
	CleanupSucceeded    bool      `json:"cleanupSucceeded"`
	ValidatedAt         time.Time `json:"validatedAt"`
}

type SmokeCleanupRequest struct {
	CompartmentID string `json:"compartmentId"`
}

type SmokeCleanupResult struct {
	Verified      bool               `json:"verified"`
	CompartmentID string             `json:"compartmentId"`
	Items         []SmokeCleanupItem `json:"items"`
	ErrorCode     string             `json:"errorCode,omitempty"`
	ErrorMessage  string             `json:"errorMessage,omitempty"`
	ValidatedAt   time.Time          `json:"validatedAt"`
}

type SmokeCleanupItem struct {
	InstanceID         string `json:"instanceId"`
	DisplayName        string `json:"displayName"`
	InitialState       string `json:"initialState"`
	TerminateRequestID string `json:"terminateRequestId,omitempty"`
	FinalState         string `json:"finalState,omitempty"`
	CleanupSucceeded   bool   `json:"cleanupSucceeded"`
	ErrorMessage       string `json:"errorMessage,omitempty"`
}

type E3FlexLifecycleRequest struct {
	CompartmentID string `json:"compartmentId"`
	SubnetID      string `json:"subnetId"`
	ImageID       string `json:"imageId"`
	BootVolumeGB  int64  `json:"bootVolumeGb"`
}

type E3FlexLifecycleResult struct {
	Verified           bool                  `json:"verified"`
	CompartmentID      string                `json:"compartmentId"`
	AvailabilityDomain string                `json:"availabilityDomain,omitempty"`
	SubnetID           string                `json:"subnetId,omitempty"`
	ImageID            string                `json:"imageId,omitempty"`
	DisplayName        string                `json:"displayName,omitempty"`
	InstanceID         string                `json:"instanceId,omitempty"`
	Shape              string                `json:"shape"`
	BootVolumeGB       int64                 `json:"bootVolumeGb"`
	Steps              []E3FlexLifecycleStep `json:"steps"`
	FinalState         string                `json:"finalState,omitempty"`
	ErrorCode          string                `json:"errorCode,omitempty"`
	ErrorMessage       string                `json:"errorMessage,omitempty"`
	ValidatedAt        time.Time             `json:"validatedAt"`
}

type E3FlexLifecycleStep struct {
	Name          string `json:"name"`
	Operation     string `json:"operation"`
	RequestID     string `json:"requestId,omitempty"`
	WorkRequestID string `json:"workRequestId,omitempty"`
	State         string `json:"state,omitempty"`
	Verified      bool   `json:"verified"`
	ErrorCode     string `json:"errorCode,omitempty"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
}

type ReinstallInstanceSmokeRequest struct {
	InstanceID    string `json:"instanceId"`
	CompartmentID string `json:"compartmentId"`
	ImageID       string `json:"imageId"`
	BootVolumeGB  int64  `json:"bootVolumeGb"`
}

type ReinstallInstanceSmokeResult struct {
	Verified      bool                `json:"verified"`
	InstanceID    string              `json:"instanceId"`
	CompartmentID string              `json:"compartmentId"`
	ImageID       string              `json:"imageId,omitempty"`
	BootVolumeGB  int64               `json:"bootVolumeGb"`
	Step          E3FlexLifecycleStep `json:"step"`
	ErrorCode     string              `json:"errorCode,omitempty"`
	ErrorMessage  string              `json:"errorMessage,omitempty"`
	ValidatedAt   time.Time           `json:"validatedAt"`
}

func SmokeCreateDeleteE2Micro(ctx context.Context, cfg ReadinessConfig, req E2MicroSmokeRequest) E2MicroSmokeResult {
	result := E2MicroSmokeResult{
		CompartmentID: req.CompartmentID,
		SubnetID:      req.SubnetID,
		ImageID:       req.ImageID,
		ValidatedAt:   time.Now().UTC(),
	}
	readiness := CheckReadiness(cfg)
	if !readiness.Ready {
		result.ErrorCode = "OCI_NOT_READY"
		result.ErrorMessage = readiness.Message
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

	if result.AvailabilityDomain == "" {
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
		imageID, err := firstCompatibleImage(ctx, clients, result.CompartmentID)
		if err != nil {
			result.ErrorCode = "OCI_DISCOVER_IMAGE_FAILED"
			result.ErrorMessage = err.Error()
			return result
		}
		result.ImageID = imageID
	}

	displayName := smokeDisplayNamePrefix + time.Now().UTC().Format("20060102-150405")
	result.DisplayName = displayName
	launchResponse, err := clients.Compute.LaunchInstance(ctx, core.LaunchInstanceRequest{
		LaunchInstanceDetails: core.LaunchInstanceDetails{
			AvailabilityDomain: common.String(result.AvailabilityDomain),
			CompartmentId:      common.String(result.CompartmentID),
			DisplayName:        common.String(displayName),
			Shape:              common.String(smokeShape),
			CreateVnicDetails: &core.CreateVnicDetails{
				SubnetId:       common.String(result.SubnetID),
				AssignPublicIp: common.Bool(false),
			},
			Metadata: map[string]string{
				"ssh_authorized_keys": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJs3YT4JQAmNKtde3QopkkBGSfgHwYOZnQlfsdoxUN/q codex-smoke",
			},
			FreeformTags: map[string]string{
				"managedBy": "codex",
				"purpose":   "e2-micro-create-delete-smoke",
			},
			SourceDetails: core.InstanceSourceViaImageDetails{
				ImageId:             common.String(result.ImageID),
				BootVolumeSizeInGBs: common.Int64(50),
			},
		},
		OpcRetryToken: common.String("codex-e2micro-" + time.Now().UTC().Format("20060102150405")),
	})
	if launchResponse.OpcRequestId != nil {
		result.LaunchRequestID = *launchResponse.OpcRequestId
	}
	if launchResponse.OpcWorkRequestId != nil {
		result.LaunchWorkRequestID = *launchResponse.OpcWorkRequestId
	}
	if launchResponse.Instance.Id != nil {
		result.InstanceID = *launchResponse.Instance.Id
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

	state, waitErr := waitInstanceState(ctx, clients, result.InstanceID, 10*time.Minute, core.InstanceLifecycleStateRunning, core.InstanceLifecycleStateStopped)
	if waitErr != nil {
		result.ErrorCode = "OCI_WAIT_INSTANCE_FAILED"
		result.ErrorMessage = waitErr.Error()
	}
	result.FinalState = string(state)

	terminateID, cleanupErr := terminateSmokeInstance(ctx, clients, result.InstanceID)
	result.CleanupAttempted = true
	result.TerminateRequestID = terminateID
	if cleanupErr != nil {
		result.CleanupSucceeded = false
		if result.ErrorCode == "" {
			result.ErrorCode = "OCI_TERMINATE_INSTANCE_FAILED"
			result.ErrorMessage = cleanupErr.Error()
		} else {
			result.ErrorMessage = result.ErrorMessage + "; cleanup failed: " + cleanupErr.Error()
		}
		return result
	}
	result.CleanupSucceeded = true

	terminatedState, terminatedErr := waitInstanceState(ctx, clients, result.InstanceID, 10*time.Minute, core.InstanceLifecycleStateTerminated)
	if terminatedErr != nil {
		result.ErrorCode = "OCI_WAIT_TERMINATED_FAILED"
		result.ErrorMessage = terminatedErr.Error()
		return result
	}
	result.FinalState = string(terminatedState)

	if result.ErrorCode == "" {
		result.Verified = true
	}
	return result
}

func SmokeE3FlexLifecycle(ctx context.Context, cfg ReadinessConfig, req E3FlexLifecycleRequest) E3FlexLifecycleResult {
	result := E3FlexLifecycleResult{
		CompartmentID: req.CompartmentID,
		SubnetID:      req.SubnetID,
		ImageID:       req.ImageID,
		Shape:         e3LifecycleShape,
		BootVolumeGB:  req.BootVolumeGB,
		ValidatedAt:   time.Now().UTC(),
	}
	if result.BootVolumeGB == 0 {
		result.BootVolumeGB = 50
	}
	readiness := CheckReadiness(cfg)
	if !readiness.Ready {
		result.ErrorCode = "OCI_NOT_READY"
		result.ErrorMessage = readiness.Message
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

	ad, err := firstAvailabilityDomain(ctx, clients, result.CompartmentID)
	if err != nil {
		result.ErrorCode = "OCI_DISCOVER_AD_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	result.AvailabilityDomain = ad
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
		imageID, err := firstCompatibleImageForShape(ctx, clients, result.CompartmentID, e3LifecycleShape)
		if err != nil {
			result.ErrorCode = "OCI_DISCOVER_IMAGE_FAILED"
			result.ErrorMessage = err.Error()
			return result
		}
		result.ImageID = imageID
	}

	displayName := e3LifecycleDisplayNamePrefix + time.Now().UTC().Format("20060102-150405")
	result.DisplayName = displayName
	launchResponse, err := clients.Compute.LaunchInstance(ctx, core.LaunchInstanceRequest{
		LaunchInstanceDetails: core.LaunchInstanceDetails{
			AvailabilityDomain: common.String(result.AvailabilityDomain),
			CompartmentId:      common.String(result.CompartmentID),
			DisplayName:        common.String(displayName),
			Shape:              common.String(e3LifecycleShape),
			ShapeConfig: &core.LaunchInstanceShapeConfigDetails{
				Ocpus:       common.Float32(1),
				MemoryInGBs: common.Float32(1),
			},
			CreateVnicDetails: &core.CreateVnicDetails{
				SubnetId:       common.String(result.SubnetID),
				AssignPublicIp: common.Bool(false),
			},
			Metadata: map[string]string{
				"ssh_authorized_keys": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJs3YT4JQAmNKtde3QopkkBGSfgHwYOZnQlfsdoxUN/q codex-e3-lifecycle",
			},
			FreeformTags: map[string]string{
				"managedBy": "codex",
				"purpose":   "e3-flex-lifecycle-smoke",
			},
			SourceDetails: core.InstanceSourceViaImageDetails{
				ImageId:             common.String(result.ImageID),
				BootVolumeSizeInGBs: common.Int64(result.BootVolumeGB),
			},
		},
		OpcRetryToken: common.String("codex-e3flex-" + time.Now().UTC().Format("20060102150405")),
	})
	launchStep := E3FlexLifecycleStep{Name: "create", Operation: "LaunchInstance"}
	if launchResponse.OpcRequestId != nil {
		launchStep.RequestID = *launchResponse.OpcRequestId
	}
	if launchResponse.OpcWorkRequestId != nil {
		launchStep.WorkRequestID = *launchResponse.OpcWorkRequestId
	}
	if launchResponse.Instance.Id != nil {
		result.InstanceID = *launchResponse.Instance.Id
	}
	if err != nil {
		launchStep.ErrorCode = "OCI_LAUNCH_INSTANCE_FAILED"
		launchStep.ErrorMessage = err.Error()
		result.Steps = append(result.Steps, launchStep)
		result.ErrorCode = launchStep.ErrorCode
		result.ErrorMessage = launchStep.ErrorMessage
		return result
	}
	state, err := waitInstanceState(ctx, clients, result.InstanceID, 10*time.Minute, core.InstanceLifecycleStateRunning)
	launchStep.State = string(state)
	if err != nil {
		launchStep.ErrorCode = "OCI_WAIT_CREATED_INSTANCE_FAILED"
		launchStep.ErrorMessage = err.Error()
		result.Steps = append(result.Steps, launchStep)
		result.ErrorCode = launchStep.ErrorCode
		result.ErrorMessage = launchStep.ErrorMessage
		ensureStopped(ctx, clients, result.InstanceID, &result)
		return result
	}
	launchStep.Verified = true
	result.Steps = append(result.Steps, launchStep)

	steps := []struct {
		name      string
		operation string
		run       func() E3FlexLifecycleStep
	}{
		{name: "stop", run: func() E3FlexLifecycleStep {
			return e3PowerStep(ctx, clients, result.InstanceID, "stop", core.InstanceActionActionSoftstop, core.InstanceLifecycleStateStopped)
		}},
		{name: "start", run: func() E3FlexLifecycleStep {
			return e3PowerStep(ctx, clients, result.InstanceID, "start", core.InstanceActionActionStart, core.InstanceLifecycleStateRunning)
		}},
		{name: "reboot", run: func() E3FlexLifecycleStep {
			return e3PowerStep(ctx, clients, result.InstanceID, "reboot", core.InstanceActionActionSoftreset, core.InstanceLifecycleStateRunning)
		}},
		{name: "reinstall", run: func() E3FlexLifecycleStep {
			return e3ReinstallStep(ctx, clients, result.InstanceID, result.ImageID, result.BootVolumeGB)
		}},
		{name: "upgrade", run: func() E3FlexLifecycleStep {
			return e3ResizeStep(ctx, clients, result.InstanceID, "upgrade", 2, 2)
		}},
		{name: "downgrade", run: func() E3FlexLifecycleStep {
			return e3ResizeStep(ctx, clients, result.InstanceID, "downgrade", 1, 1)
		}},
		{name: "boot-volume-expand-plus-10g", run: func() E3FlexLifecycleStep {
			targetGB := result.BootVolumeGB + 10
			step := e3BootVolumeExpandStep(ctx, clients, result.InstanceID, result.CompartmentID, result.AvailabilityDomain, targetGB)
			if step.Verified {
				result.BootVolumeGB = targetGB
			}
			return step
		}},
		{name: "boot-volume-shrink-minus-10g-rejected", run: func() E3FlexLifecycleStep {
			return e3BootVolumeShrinkRejectedStep(ctx, clients, result.InstanceID, result.CompartmentID, result.AvailabilityDomain, result.BootVolumeGB-10)
		}},
	}
	for _, item := range steps {
		step := item.run()
		result.Steps = append(result.Steps, step)
		if !step.Verified {
			result.ErrorCode = step.ErrorCode
			result.ErrorMessage = step.ErrorMessage
			ensureStopped(ctx, clients, result.InstanceID, &result)
			return result
		}
	}

	finalStop := e3PowerStep(ctx, clients, result.InstanceID, "final-stop", core.InstanceActionActionSoftstop, core.InstanceLifecycleStateStopped)
	result.Steps = append(result.Steps, finalStop)
	result.FinalState = finalStop.State
	if !finalStop.Verified {
		result.ErrorCode = finalStop.ErrorCode
		result.ErrorMessage = finalStop.ErrorMessage
		return result
	}
	result.Verified = true
	return result
}

func CleanupSmokeInstances(ctx context.Context, cfg ReadinessConfig, req SmokeCleanupRequest) SmokeCleanupResult {
	result := SmokeCleanupResult{
		CompartmentID: req.CompartmentID,
		ValidatedAt:   time.Now().UTC(),
	}
	readiness := CheckReadiness(cfg)
	if !readiness.Ready {
		result.ErrorCode = "OCI_NOT_READY"
		result.ErrorMessage = readiness.Message
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

	limit := 100
	response, err := clients.Compute.ListInstances(ctx, core.ListInstancesRequest{
		CompartmentId: common.String(result.CompartmentID),
		Limit:         common.Int(limit),
	})
	if err != nil {
		result.ErrorCode = "OCI_LIST_SMOKE_INSTANCES_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}

	for _, instance := range response.Items {
		displayName := stringValue(instance.DisplayName)
		if !isSmokeInstanceDisplayName(displayName) || instance.Id == nil || *instance.Id == "" {
			continue
		}
		item := SmokeCleanupItem{
			InstanceID:   *instance.Id,
			DisplayName:  displayName,
			InitialState: string(instance.LifecycleState),
		}
		if instance.LifecycleState == core.InstanceLifecycleStateTerminated || instance.LifecycleState == core.InstanceLifecycleStateTerminating {
			item.FinalState = string(instance.LifecycleState)
			item.CleanupSucceeded = true
			result.Items = append(result.Items, item)
			continue
		}

		terminateID, cleanupErr := terminateSmokeInstance(ctx, clients, item.InstanceID)
		item.TerminateRequestID = terminateID
		if cleanupErr != nil {
			item.ErrorMessage = cleanupErr.Error()
			result.Items = append(result.Items, item)
			continue
		}

		terminatedState, waitErr := waitInstanceState(ctx, clients, item.InstanceID, 10*time.Minute, core.InstanceLifecycleStateTerminated)
		item.FinalState = string(terminatedState)
		if waitErr != nil {
			item.ErrorMessage = waitErr.Error()
			result.Items = append(result.Items, item)
			continue
		}
		item.CleanupSucceeded = true
		result.Items = append(result.Items, item)
	}

	result.Verified = true
	for _, item := range result.Items {
		if !item.CleanupSucceeded {
			result.Verified = false
			result.ErrorCode = "OCI_SMOKE_CLEANUP_INCOMPLETE"
			result.ErrorMessage = "one or more smoke instances could not be cleaned up"
			break
		}
	}
	return result
}

func SmokeReinstallInstance(ctx context.Context, cfg ReadinessConfig, req ReinstallInstanceSmokeRequest) ReinstallInstanceSmokeResult {
	result := ReinstallInstanceSmokeResult{
		InstanceID:    req.InstanceID,
		CompartmentID: req.CompartmentID,
		ImageID:       req.ImageID,
		BootVolumeGB:  req.BootVolumeGB,
		ValidatedAt:   time.Now().UTC(),
	}
	if result.BootVolumeGB == 0 {
		result.BootVolumeGB = 50
	}
	readiness := CheckReadiness(cfg)
	if !readiness.Ready {
		result.ErrorCode = "OCI_NOT_READY"
		result.ErrorMessage = readiness.Message
		return result
	}
	if strings.TrimSpace(result.InstanceID) == "" {
		result.ErrorCode = "OCI_INSTANCE_ID_REQUIRED"
		result.ErrorMessage = "instanceId is required"
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
	if result.ImageID == "" {
		imageID, err := firstCompatibleImageForShape(ctx, clients, result.CompartmentID, e3LifecycleShape)
		if err != nil {
			result.ErrorCode = "OCI_DISCOVER_IMAGE_FAILED"
			result.ErrorMessage = err.Error()
			return result
		}
		result.ImageID = imageID
	}

	step := e3ReinstallStep(ctx, clients, result.InstanceID, result.ImageID, result.BootVolumeGB)
	result.Step = step
	if !step.Verified {
		result.ErrorCode = step.ErrorCode
		result.ErrorMessage = step.ErrorMessage
		return result
	}
	result.Verified = true
	return result
}

func firstAvailabilityDomain(ctx context.Context, clients Clients, compartmentID string) (string, error) {
	response, err := clients.Identity.ListAvailabilityDomains(ctx, identity.ListAvailabilityDomainsRequest{
		CompartmentId: common.String(compartmentID),
	})
	if err != nil {
		return "", err
	}
	for _, item := range response.Items {
		if item.Name != nil && *item.Name != "" {
			return *item.Name, nil
		}
	}
	return "", fmt.Errorf("no availability domains found")
}

func firstAvailableSubnet(ctx context.Context, clients Clients, compartmentID string) (string, error) {
	limit := 50
	response, err := clients.VirtualNetwork.ListSubnets(ctx, core.ListSubnetsRequest{
		CompartmentId:  common.String(compartmentID),
		LifecycleState: core.SubnetLifecycleStateAvailable,
		Limit:          common.Int(limit),
	})
	if err != nil {
		return "", err
	}
	for _, item := range response.Items {
		if item.Id != nil && *item.Id != "" {
			return *item.Id, nil
		}
	}
	return "", fmt.Errorf("no available subnets found in compartment %s", compartmentID)
}

func firstCompatibleImage(ctx context.Context, clients Clients, compartmentID string) (string, error) {
	return firstCompatibleImageForShape(ctx, clients, compartmentID, smokeShape)
}

func firstCompatibleImageForShape(ctx context.Context, clients Clients, compartmentID string, shape string) (string, error) {
	limit := 20
	response, err := clients.Compute.ListImages(ctx, core.ListImagesRequest{
		CompartmentId:   common.String(compartmentID),
		Shape:           common.String(shape),
		OperatingSystem: common.String("Oracle Linux"),
		LifecycleState:  core.ImageLifecycleStateAvailable,
		SortBy:          core.ListImagesSortByTimecreated,
		SortOrder:       core.ListImagesSortOrderDesc,
		Limit:           common.Int(limit),
	})
	if err != nil {
		return "", err
	}
	for _, item := range response.Items {
		if item.Id != nil && *item.Id != "" {
			return *item.Id, nil
		}
	}
	return "", fmt.Errorf("no compatible Oracle Linux images found for %s", shape)
}

func waitInstanceState(ctx context.Context, clients Clients, instanceID string, timeout time.Duration, accepted ...core.InstanceLifecycleStateEnum) (core.InstanceLifecycleStateEnum, error) {
	deadline := time.Now().Add(timeout)
	for {
		response, err := clients.Compute.GetInstance(ctx, core.GetInstanceRequest{InstanceId: common.String(instanceID)})
		if err != nil {
			return "", err
		}
		for _, state := range accepted {
			if response.Instance.LifecycleState == state {
				return response.Instance.LifecycleState, nil
			}
		}
		if time.Now().After(deadline) {
			return response.Instance.LifecycleState, fmt.Errorf("timed out waiting for instance %s; last state=%s", instanceID, response.Instance.LifecycleState)
		}
		select {
		case <-ctx.Done():
			return response.Instance.LifecycleState, ctx.Err()
		case <-time.After(15 * time.Second):
		}
	}
}

func terminateSmokeInstance(ctx context.Context, clients Clients, instanceID string) (string, error) {
	response, err := clients.Compute.GetInstance(ctx, core.GetInstanceRequest{InstanceId: common.String(instanceID)})
	if err != nil {
		return "", err
	}
	if response.Instance.DisplayName == nil || !isSmokeInstanceDisplayName(*response.Instance.DisplayName) {
		return "", fmt.Errorf("refusing to terminate instance without smoke prefix: %s", stringValue(response.Instance.DisplayName))
	}
	terminateResponse, err := clients.Compute.TerminateInstance(ctx, core.TerminateInstanceRequest{
		InstanceId:         common.String(instanceID),
		PreserveBootVolume: common.Bool(false),
	})
	if terminateResponse.OpcRequestId != nil {
		return *terminateResponse.OpcRequestId, err
	}
	return "", err
}

func e3PowerStep(ctx context.Context, clients Clients, instanceID string, name string, action core.InstanceActionActionEnum, target core.InstanceLifecycleStateEnum) E3FlexLifecycleStep {
	step := E3FlexLifecycleStep{Name: name, Operation: "InstanceAction " + string(action)}
	response, err := clients.Compute.InstanceAction(ctx, core.InstanceActionRequest{
		InstanceId:    common.String(instanceID),
		Action:        action,
		OpcRetryToken: common.String("codex-e3-" + name + "-" + time.Now().UTC().Format("20060102150405")),
	})
	if response.OpcRequestId != nil {
		step.RequestID = *response.OpcRequestId
	}
	if err != nil {
		step.ErrorCode = "OCI_INSTANCE_ACTION_FAILED"
		step.ErrorMessage = err.Error()
		return step
	}
	state, err := waitInstanceState(ctx, clients, instanceID, 20*time.Minute, target)
	step.State = string(state)
	if err != nil {
		step.ErrorCode = "OCI_WAIT_INSTANCE_STATE_FAILED"
		step.ErrorMessage = err.Error()
		return step
	}
	step.Verified = true
	return step
}

func e3ResizeStep(ctx context.Context, clients Clients, instanceID string, name string, ocpus int, memoryGB int) E3FlexLifecycleStep {
	step := E3FlexLifecycleStep{Name: name, Operation: fmt.Sprintf("UpdateInstance ShapeConfig %dC/%dG", ocpus, memoryGB)}
	response, err := clients.Compute.UpdateInstance(ctx, core.UpdateInstanceRequest{
		InstanceId: common.String(instanceID),
		UpdateInstanceDetails: core.UpdateInstanceDetails{
			Shape: common.String(e3LifecycleShape),
			ShapeConfig: &core.UpdateInstanceShapeConfigDetails{
				Ocpus:       common.Float32(float32(ocpus)),
				MemoryInGBs: common.Float32(float32(memoryGB)),
			},
			UpdateOperationConstraint: core.UpdateInstanceDetailsUpdateOperationConstraintAllowDowntime,
		},
		OpcRetryToken: common.String("codex-e3-" + name + "-" + time.Now().UTC().Format("20060102150405")),
	})
	if response.OpcRequestId != nil {
		step.RequestID = *response.OpcRequestId
	}
	if response.OpcWorkRequestId != nil {
		step.WorkRequestID = *response.OpcWorkRequestId
	}
	if err != nil {
		step.ErrorCode = "OCI_UPDATE_INSTANCE_FAILED"
		step.ErrorMessage = err.Error()
		return step
	}
	state, err := waitInstanceState(ctx, clients, instanceID, 10*time.Minute, core.InstanceLifecycleStateRunning, core.InstanceLifecycleStateStopped)
	step.State = string(state)
	if err != nil {
		step.ErrorCode = "OCI_WAIT_INSTANCE_STATE_FAILED"
		step.ErrorMessage = err.Error()
		return step
	}
	step.Verified = true
	return step
}

func e3ReinstallStep(ctx context.Context, clients Clients, instanceID string, imageID string, bootVolumeGB int64) E3FlexLifecycleStep {
	step := E3FlexLifecycleStep{Name: "reinstall", Operation: "UpdateInstance SourceDetails image"}
	response, err := clients.Compute.UpdateInstance(ctx, core.UpdateInstanceRequest{
		InstanceId: common.String(instanceID),
		UpdateInstanceDetails: core.UpdateInstanceDetails{
			SourceDetails: core.UpdateInstanceSourceViaImageDetails{
				ImageId:                     common.String(imageID),
				IsPreserveBootVolumeEnabled: common.Bool(false),
				BootVolumeSizeInGBs:         common.Int64(bootVolumeGB),
			},
			UpdateOperationConstraint: core.UpdateInstanceDetailsUpdateOperationConstraintAllowDowntime,
		},
		OpcRetryToken: common.String("codex-e3-reinstall-" + time.Now().UTC().Format("20060102150405")),
	})
	if response.OpcRequestId != nil {
		step.RequestID = *response.OpcRequestId
	}
	if response.OpcWorkRequestId != nil {
		step.WorkRequestID = *response.OpcWorkRequestId
	}
	if err != nil {
		step.ErrorCode = "OCI_REINSTALL_INSTANCE_FAILED"
		step.ErrorMessage = err.Error()
		return step
	}
	state, err := waitInstanceState(ctx, clients, instanceID, 15*time.Minute, core.InstanceLifecycleStateRunning, core.InstanceLifecycleStateStopped)
	step.State = string(state)
	if err != nil {
		step.ErrorCode = "OCI_WAIT_REINSTALLED_INSTANCE_FAILED"
		step.ErrorMessage = err.Error()
		return step
	}
	step.Verified = true
	return step
}

func e3BootVolumeExpandStep(ctx context.Context, clients Clients, instanceID string, compartmentID string, availabilityDomain string, targetGB int64) E3FlexLifecycleStep {
	step := E3FlexLifecycleStep{Name: "boot-volume-expand-plus-10g", Operation: fmt.Sprintf("UpdateBootVolume sizeInGBs=%d", targetGB)}
	bootVolumeID, currentGB, err := e3BootVolumeForInstance(ctx, clients, instanceID, compartmentID, availabilityDomain)
	if err != nil {
		step.ErrorCode = "OCI_RESOLVE_BOOT_VOLUME_FAILED"
		step.ErrorMessage = err.Error()
		return step
	}
	if currentGB >= targetGB {
		step.State = fmt.Sprintf("%dGB", currentGB)
		step.Verified = true
		return step
	}
	response, err := clients.Blockstorage.UpdateBootVolume(ctx, core.UpdateBootVolumeRequest{
		BootVolumeId: common.String(bootVolumeID),
		UpdateBootVolumeDetails: core.UpdateBootVolumeDetails{
			SizeInGBs: common.Int64(targetGB),
		},
		OpcRequestId: requestID("codex-e3-boot-expand", instanceID),
	})
	if response.OpcRequestId != nil {
		step.RequestID = *response.OpcRequestId
	}
	if err != nil {
		step.ErrorCode = "OCI_UPDATE_BOOT_VOLUME_FAILED"
		step.ErrorMessage = err.Error()
		return step
	}
	if err := waitBootVolumeSize(ctx, clients, bootVolumeID, int(targetGB), 10*time.Minute); err != nil {
		step.ErrorCode = "OCI_WAIT_BOOT_VOLUME_SIZE_FAILED"
		step.ErrorMessage = err.Error()
		return step
	}
	step.State = fmt.Sprintf("%dGB", targetGB)
	step.Verified = true
	return step
}

func e3BootVolumeShrinkRejectedStep(ctx context.Context, clients Clients, instanceID string, compartmentID string, availabilityDomain string, targetGB int64) E3FlexLifecycleStep {
	step := E3FlexLifecycleStep{Name: "boot-volume-shrink-minus-10g-rejected", Operation: fmt.Sprintf("UpdateBootVolume sizeInGBs=%d expected rejection", targetGB)}
	bootVolumeID, currentGB, err := e3BootVolumeForInstance(ctx, clients, instanceID, compartmentID, availabilityDomain)
	if err != nil {
		step.ErrorCode = "OCI_RESOLVE_BOOT_VOLUME_FAILED"
		step.ErrorMessage = err.Error()
		return step
	}
	if targetGB >= currentGB {
		step.ErrorCode = "OCI_BOOT_VOLUME_SHRINK_TARGET_INVALID"
		step.ErrorMessage = fmt.Sprintf("target %dGB must be less than current %dGB for shrink rejection test", targetGB, currentGB)
		return step
	}
	response, err := clients.Blockstorage.UpdateBootVolume(ctx, core.UpdateBootVolumeRequest{
		BootVolumeId: common.String(bootVolumeID),
		UpdateBootVolumeDetails: core.UpdateBootVolumeDetails{
			SizeInGBs: common.Int64(targetGB),
		},
		OpcRequestId: requestID("codex-e3-boot-shrink", instanceID),
	})
	if response.OpcRequestId != nil {
		step.RequestID = *response.OpcRequestId
	}
	if err == nil {
		step.ErrorCode = "OCI_BOOT_VOLUME_SHRINK_UNEXPECTEDLY_ACCEPTED"
		step.ErrorMessage = "OCI accepted a boot volume size decrease; verify the volume manually before continuing"
		return step
	}
	step.State = fmt.Sprintf("expected rejection current=%dGB target=%dGB", currentGB, targetGB)
	step.Verified = true
	return step
}

func e3BootVolumeForInstance(ctx context.Context, clients Clients, instanceID string, compartmentID string, availabilityDomain string) (string, int64, error) {
	attachments, err := clients.Compute.ListBootVolumeAttachments(ctx, core.ListBootVolumeAttachmentsRequest{
		AvailabilityDomain: common.String(availabilityDomain),
		CompartmentId:      common.String(compartmentID),
		InstanceId:         common.String(instanceID),
		Limit:              common.Int(25),
	})
	if err != nil {
		return "", 0, err
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
		return "", 0, fmt.Errorf("no attached boot volume found for instance %s", instanceID)
	}
	bootVolume, err := clients.Blockstorage.GetBootVolume(ctx, core.GetBootVolumeRequest{BootVolumeId: common.String(bootVolumeID)})
	if err != nil {
		return "", 0, err
	}
	if bootVolume.BootVolume.SizeInGBs == nil {
		return bootVolumeID, 0, nil
	}
	return bootVolumeID, *bootVolume.BootVolume.SizeInGBs, nil
}

func ensureStopped(ctx context.Context, clients Clients, instanceID string, result *E3FlexLifecycleResult) {
	if strings.TrimSpace(instanceID) == "" {
		return
	}
	current, err := clients.Compute.GetInstance(ctx, core.GetInstanceRequest{InstanceId: common.String(instanceID)})
	if err != nil {
		result.Steps = append(result.Steps, E3FlexLifecycleStep{Name: "ensure-stopped", Operation: "GetInstance", ErrorCode: "OCI_GET_INSTANCE_FAILED", ErrorMessage: err.Error()})
		return
	}
	if current.Instance.LifecycleState == core.InstanceLifecycleStateStopped {
		result.FinalState = string(current.Instance.LifecycleState)
		return
	}
	if current.Instance.LifecycleState == core.InstanceLifecycleStateStopping {
		state, waitErr := waitInstanceState(ctx, clients, instanceID, 20*time.Minute, core.InstanceLifecycleStateStopped)
		result.FinalState = string(state)
		step := E3FlexLifecycleStep{
			Name:      "ensure-stopped",
			Operation: "WaitInstanceState STOPPED",
			State:     string(state),
			Verified:  waitErr == nil,
		}
		if waitErr != nil {
			step.ErrorCode = "OCI_WAIT_INSTANCE_STATE_FAILED"
			step.ErrorMessage = waitErr.Error()
		}
		result.Steps = append(result.Steps, step)
		return
	}
	if current.Instance.LifecycleState == core.InstanceLifecycleStateTerminated || current.Instance.LifecycleState == core.InstanceLifecycleStateTerminating {
		result.FinalState = string(current.Instance.LifecycleState)
		return
	}
	step := e3PowerStep(ctx, clients, instanceID, "ensure-stopped", core.InstanceActionActionSoftstop, core.InstanceLifecycleStateStopped)
	result.FinalState = step.State
	result.Steps = append(result.Steps, step)
}

func isSmokeInstanceDisplayName(displayName string) bool {
	return strings.HasPrefix(displayName, smokeDisplayNamePrefix) || strings.HasPrefix(displayName, e3LifecycleDisplayNamePrefix)
}
