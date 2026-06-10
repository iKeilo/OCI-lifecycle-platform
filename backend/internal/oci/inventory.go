package oci

import (
	"context"
	"fmt"
	"math"
	"time"

	"a-series-oracle/backend/internal/domain"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type InstanceInventoryRequest struct {
	CompartmentID  string `json:"compartmentId"`
	Status         string `json:"status"`
	IncludeNetwork bool   `json:"includeNetwork"`
}

type InstanceInventoryResult struct {
	Verified       bool              `json:"verified"`
	ExecutionMode  string            `json:"executionMode"`
	Region         string            `json:"region"`
	CompartmentID  string            `json:"compartmentId"`
	RequestIDs     []string          `json:"requestIds"`
	Items          []domain.Instance `json:"items"`
	ErrorCode      string            `json:"errorCode,omitempty"`
	ErrorMessage   string            `json:"errorMessage,omitempty"`
	LastSyncedAt   time.Time         `json:"lastSyncedAt"`
	IncludeNetwork bool              `json:"includeNetwork"`
}

func ListInstanceInventory(ctx context.Context, cfg ReadinessConfig, req InstanceInventoryRequest) InstanceInventoryResult {
	result := InstanceInventoryResult{
		ExecutionMode:  cfg.ExecutionMode,
		Region:         cfg.Region,
		CompartmentID:  req.CompartmentID,
		LastSyncedAt:   time.Now().UTC(),
		IncludeNetwork: req.IncludeNetwork,
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

	page := ""
	for {
		limit := 100
		listReq := core.ListInstancesRequest{
			CompartmentId: common.String(result.CompartmentID),
			Limit:         common.Int(limit),
			SortBy:        core.ListInstancesSortByTimecreated,
			SortOrder:     core.ListInstancesSortOrderDesc,
		}
		if page != "" {
			listReq.Page = common.String(page)
		}

		response, err := clients.Compute.ListInstances(ctx, listReq)
		appendRequestID(&result.RequestIDs, response.OpcRequestId)
		if err != nil {
			result.ErrorCode = "OCI_LIST_INSTANCES_FAILED"
			result.ErrorMessage = err.Error()
			return result
		}

		for _, item := range response.Items {
			mapped := mapOCIInstance(cfg, result.CompartmentID, item, result.LastSyncedAt)
			if req.Status != "" && req.Status != "All" && string(mapped.Status) != req.Status {
				continue
			}
			if req.IncludeNetwork && mapped.Status != domain.InstanceTerminated {
				if err := fillPrimaryVNIC(ctx, clients, result.CompartmentID, &mapped, &result.RequestIDs); err != nil {
					result.ErrorCode = "OCI_LIST_INSTANCE_NETWORK_FAILED"
					result.ErrorMessage = err.Error()
					return result
				}
			}
			result.Items = append(result.Items, mapped)
		}

		if response.OpcNextPage == nil || *response.OpcNextPage == "" {
			break
		}
		page = *response.OpcNextPage
	}

	result.Verified = true
	return result
}

func mapOCIInstance(cfg ReadinessConfig, compartmentID string, item core.Instance, syncedAt time.Time) domain.Instance {
	created := ""
	if item.TimeCreated != nil {
		created = item.TimeCreated.Time.Format(time.RFC3339)
	}

	ocpus, memoryGB := shapeConfigValues(item.ShapeConfig)
	instanceID := stringValue(item.Id)
	return domain.Instance{
		ID:            instanceID,
		Name:          stringValue(item.DisplayName),
		Created:       created,
		Shape:         stringValue(item.Shape),
		Region:        defaultString(stringValue(item.Region), cfg.Region),
		Compartment:   compartmentID,
		OCPUs:         ocpus,
		MemoryGB:      memoryGB,
		Status:        mapInstanceStatus(item.LifecycleState),
		Protected:     false,
		OCIInstanceID: instanceID,
		ProfileID:     "DEFAULT",
		CompartmentID: compartmentID,
		LastSyncedAt:  syncedAt,
	}
}

func fillPrimaryVNIC(ctx context.Context, clients Clients, compartmentID string, instance *domain.Instance, requestIDs *[]string) error {
	response, err := clients.Compute.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
		CompartmentId: common.String(compartmentID),
		InstanceId:    common.String(instance.OCIInstanceID),
		Limit:         common.Int(50),
	})
	appendRequestID(requestIDs, response.OpcRequestId)
	if err != nil {
		return err
	}

	for _, attachment := range response.Items {
		if attachment.VnicId == nil || *attachment.VnicId == "" {
			continue
		}
		vnic, err := clients.VirtualNetwork.GetVnic(ctx, core.GetVnicRequest{
			VnicId: attachment.VnicId,
		})
		appendRequestID(requestIDs, vnic.OpcRequestId)
		if err != nil {
			return err
		}
		if vnic.Vnic.IsPrimary == nil || *vnic.Vnic.IsPrimary {
			instance.PrivateIP = stringValue(vnic.Vnic.PrivateIp)
			instance.PrimaryIP = stringValue(vnic.Vnic.PublicIp)
			return nil
		}
	}

	return nil
}

func shapeConfigValues(config *core.InstanceShapeConfig) (int, int) {
	if config == nil {
		return 0, 0
	}
	ocpus := 0
	if config.Ocpus != nil {
		ocpus = int(math.Ceil(float64(*config.Ocpus)))
	}
	memoryGB := 0
	if config.MemoryInGBs != nil {
		memoryGB = int(math.Ceil(float64(*config.MemoryInGBs)))
	}
	return ocpus, memoryGB
}

func mapInstanceStatus(state core.InstanceLifecycleStateEnum) domain.InstanceStatus {
	switch state {
	case core.InstanceLifecycleStateRunning:
		return domain.InstanceRunning
	case core.InstanceLifecycleStateStopped:
		return domain.InstanceStopped
	case core.InstanceLifecycleStateTerminating:
		return domain.InstanceTerminating
	case core.InstanceLifecycleStateTerminated:
		return domain.InstanceTerminated
	case core.InstanceLifecycleStateProvisioning, core.InstanceLifecycleStateStarting, core.InstanceLifecycleStateStopping, core.InstanceLifecycleStateMoving:
		return domain.InstanceProvisioning
	default:
		return domain.InstanceProvisioning
	}
}

func appendRequestID(requestIDs *[]string, requestID *string) {
	if requestID != nil && *requestID != "" {
		*requestIDs = append(*requestIDs, *requestID)
	}
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func inventoryErrorResponse(code string, err error) InstanceInventoryResult {
	return InstanceInventoryResult{
		ErrorCode:    code,
		ErrorMessage: fmt.Sprint(err),
		LastSyncedAt: time.Now().UTC(),
	}
}
