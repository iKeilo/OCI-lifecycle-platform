package oci

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type PublicIPBatchExecutionRequest struct {
	Action        string
	CompartmentID string
	Count         int
	DisplayPrefix string
	PublicIPIDs   []string
	JobID         string
}

type PublicIPBatchExecutionResult struct {
	Verified     bool                         `json:"verified"`
	Action       string                       `json:"action"`
	RequestID    string                       `json:"requestId,omitempty"`
	Items        []PublicIPBatchExecutionItem `json:"items"`
	ErrorCode    string                       `json:"errorCode,omitempty"`
	ErrorMessage string                       `json:"errorMessage,omitempty"`
}

type PublicIPBatchExecutionItem struct {
	ID           string `json:"id,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	IPAddress    string `json:"ipAddress,omitempty"`
	RequestID    string `json:"requestId,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

func ExecutePublicIPBatch(ctx context.Context, cfg ReadinessConfig, req PublicIPBatchExecutionRequest) PublicIPBatchExecutionResult {
	result := PublicIPBatchExecutionResult{
		Action: strings.ToLower(strings.TrimSpace(req.Action)),
		Items:  []PublicIPBatchExecutionItem{},
	}
	clients, err := NewClients(cfg)
	if err != nil {
		result.ErrorCode = "OCI_CLIENT_INIT_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	switch result.Action {
	case "create":
		return executeCreateReservedPublicIPs(ctx, clients, req, result)
	case "delete":
		return executeDeleteReservedPublicIPs(ctx, clients, req, result)
	default:
		result.ErrorCode = "BAD_ACTION"
		result.ErrorMessage = "action must be create or delete"
		return result
	}
}

func executeCreateReservedPublicIPs(ctx context.Context, clients Clients, req PublicIPBatchExecutionRequest, result PublicIPBatchExecutionResult) PublicIPBatchExecutionResult {
	if strings.TrimSpace(req.CompartmentID) == "" {
		result.ErrorCode = "COMPARTMENT_REQUIRED"
		result.ErrorMessage = "compartmentId is required"
		return result
	}
	if req.Count <= 0 {
		result.ErrorCode = "COUNT_REQUIRED"
		result.ErrorMessage = "count must be greater than zero"
		return result
	}
	if req.Count > 50 {
		result.ErrorCode = "COUNT_TOO_LARGE"
		result.ErrorMessage = "count cannot exceed 50 per batch"
		return result
	}
	prefix := strings.TrimSpace(req.DisplayPrefix)
	if prefix == "" {
		prefix = "reserved-public-ip"
	}
	for i := 1; i <= req.Count; i++ {
		displayName := fmt.Sprintf("%s-%02d", prefix, i)
		resp, err := clients.VirtualNetwork.CreatePublicIp(ctx, core.CreatePublicIpRequest{
			CreatePublicIpDetails: core.CreatePublicIpDetails{
				CompartmentId: common.String(req.CompartmentID),
				Lifetime:      core.CreatePublicIpDetailsLifetimeReserved,
				DisplayName:   common.String(displayName),
				FreeformTags: map[string]string{
					"managedBy": "oci-lifecycle-platform",
					"jobId":     req.JobID,
					"createdAt": time.Now().UTC().Format(time.RFC3339),
				},
			},
		})
		item := PublicIPBatchExecutionItem{DisplayName: displayName}
		if resp.OpcRequestId != nil {
			item.RequestID = *resp.OpcRequestId
			if result.RequestID == "" {
				result.RequestID = *resp.OpcRequestId
			}
		}
		if err != nil {
			item.ErrorCode = "OCI_CREATE_PUBLIC_IP_FAILED"
			item.ErrorMessage = err.Error()
			result.Items = append(result.Items, item)
			continue
		}
		item.ID = stringValue(resp.PublicIp.Id)
		item.IPAddress = stringValue(resp.PublicIp.IpAddress)
		result.Items = append(result.Items, item)
	}
	return finishPublicIPBatchResult(result)
}

func executeDeleteReservedPublicIPs(ctx context.Context, clients Clients, req PublicIPBatchExecutionRequest, result PublicIPBatchExecutionResult) PublicIPBatchExecutionResult {
	if len(req.PublicIPIDs) == 0 {
		result.ErrorCode = "PUBLIC_IP_IDS_REQUIRED"
		result.ErrorMessage = "publicIpIds is required"
		return result
	}
	for _, id := range req.PublicIPIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		resp, err := clients.VirtualNetwork.DeletePublicIp(ctx, core.DeletePublicIpRequest{
			PublicIpId: common.String(id),
		})
		item := PublicIPBatchExecutionItem{ID: id}
		if resp.OpcRequestId != nil {
			item.RequestID = *resp.OpcRequestId
			if result.RequestID == "" {
				result.RequestID = *resp.OpcRequestId
			}
		}
		if err != nil {
			item.ErrorCode = "OCI_DELETE_PUBLIC_IP_FAILED"
			item.ErrorMessage = err.Error()
		}
		result.Items = append(result.Items, item)
	}
	return finishPublicIPBatchResult(result)
}

func finishPublicIPBatchResult(result PublicIPBatchExecutionResult) PublicIPBatchExecutionResult {
	failures := 0
	for _, item := range result.Items {
		if item.ErrorCode != "" || item.ErrorMessage != "" {
			failures++
		}
	}
	if failures > 0 {
		result.ErrorCode = "OCI_PUBLIC_IP_BATCH_PARTIAL_FAILED"
		result.ErrorMessage = fmt.Sprintf("%d of %d public IP operations failed", failures, len(result.Items))
		return result
	}
	result.Verified = true
	return result
}
