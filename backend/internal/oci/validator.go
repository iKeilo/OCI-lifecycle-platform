package oci

import (
	"context"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

type ReadOnlyValidationRequest struct {
	ProfileID     string `json:"profileId"`
	Region        string `json:"region"`
	CompartmentID string `json:"compartmentId"`
}

type RegionValidationItem struct {
	RegionName string `json:"regionName"`
	Status     string `json:"status"`
}

type InstanceValidationItem struct {
	ID             string `json:"id"`
	DisplayName    string `json:"displayName"`
	LifecycleState string `json:"lifecycleState"`
	Shape          string `json:"shape"`
}

type ReadOnlyValidationResult struct {
	Verified           bool                     `json:"verified"`
	ExecutionMode      string                   `json:"executionMode"`
	Region             string                   `json:"region"`
	TenancyOCID        string                   `json:"tenancyOcid"`
	CompartmentID      string                   `json:"compartmentId"`
	RegionRequestID    string                   `json:"regionRequestId,omitempty"`
	InstancesRequestID string                   `json:"instancesRequestId,omitempty"`
	Regions            []RegionValidationItem   `json:"regions"`
	Instances          []InstanceValidationItem `json:"instances"`
	ErrorCode          string                   `json:"errorCode,omitempty"`
	ErrorMessage       string                   `json:"errorMessage,omitempty"`
	ValidatedAt        time.Time                `json:"validatedAt"`
}

func ValidateReadOnly(ctx context.Context, cfg ReadinessConfig, req ReadOnlyValidationRequest) ReadOnlyValidationResult {
	result := ReadOnlyValidationResult{
		ExecutionMode: cfg.ExecutionMode,
		Region:        cfg.Region,
		TenancyOCID:   cfg.TenancyOCID,
		CompartmentID: req.CompartmentID,
		ValidatedAt:   time.Now().UTC(),
	}
	readiness := CheckReadiness(cfg)
	if !readiness.Ready {
		result.ErrorCode = "OCI_NOT_READY"
		result.ErrorMessage = readiness.Message
		return result
	}
	if req.CompartmentID == "" {
		req.CompartmentID = cfg.TenancyOCID
		result.CompartmentID = req.CompartmentID
	}

	clients, err := NewClients(cfg)
	if err != nil {
		result.ErrorCode = "OCI_CLIENT_INIT_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}

	regions, err := clients.Identity.ListRegionSubscriptions(ctx, identity.ListRegionSubscriptionsRequest{
		TenancyId: common.String(cfg.TenancyOCID),
	})
	if regions.OpcRequestId != nil {
		result.RegionRequestID = *regions.OpcRequestId
	}
	if err != nil {
		result.ErrorCode = "OCI_LIST_REGIONS_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	for _, item := range regions.Items {
		result.Regions = append(result.Regions, RegionValidationItem{
			RegionName: stringValue(item.RegionName),
			Status:     string(item.Status),
		})
	}

	limit := 5
	instances, err := clients.Compute.ListInstances(ctx, core.ListInstancesRequest{
		CompartmentId: common.String(req.CompartmentID),
		Limit:         common.Int(limit),
	})
	if instances.OpcRequestId != nil {
		result.InstancesRequestID = *instances.OpcRequestId
	}
	if err != nil {
		result.ErrorCode = "OCI_LIST_INSTANCES_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	for _, item := range instances.Items {
		result.Instances = append(result.Instances, InstanceValidationItem{
			ID:             stringValue(item.Id),
			DisplayName:    stringValue(item.DisplayName),
			LifecycleState: string(item.LifecycleState),
			Shape:          stringValue(item.Shape),
		})
	}

	result.Verified = true
	return result
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
