package oci

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type IPManagementExecutionRequest struct {
	InstanceID               string
	VNICID                   string
	EnableIPv6               bool
	DisableIPv6              bool
	AutoConfigureIPv6        bool
	IPv6Strategy             string
	NetworkChangeMode        string
	RouteTableMode           string
	SecurityMode             string
	AllowIrreversibleVCNIPv6 bool
	AllowPublicIPv4Change    bool
	OpenSSHIPv6              bool
	OpenHTTPIPv6             bool
	OpenHTTPSIPv6            bool
	JobID                    string
}

type IPManagementExecutionResult struct {
	Verified               bool              `json:"verified"`
	ExecutionMode          string            `json:"executionMode"`
	InstanceID             string            `json:"instanceId"`
	VNICID                 string            `json:"vnicId,omitempty"`
	VCNID                  string            `json:"vcnId,omitempty"`
	SubnetID               string            `json:"subnetId,omitempty"`
	VCNIPv6CIDR            string            `json:"vcnIpv6Cidr,omitempty"`
	SubnetIPv6CIDR         string            `json:"subnetIpv6Cidr,omitempty"`
	ExistingIPv6           []string          `json:"existingIpv6,omitempty"`
	AutoConfigureIPv6      bool              `json:"autoConfigureIpv6"`
	IPv6Strategy           string            `json:"ipv6Strategy,omitempty"`
	NetworkChangeMode      string            `json:"networkChangeMode,omitempty"`
	RouteTableMode         string            `json:"routeTableMode,omitempty"`
	SecurityMode           string            `json:"securityMode,omitempty"`
	InternetGatewayID      string            `json:"internetGatewayId,omitempty"`
	CreatedInternetGateway bool              `json:"createdInternetGateway"`
	RouteTableID           string            `json:"routeTableId,omitempty"`
	CreatedRouteTableID    string            `json:"createdRouteTableId,omitempty"`
	RouteTableChanged      bool              `json:"routeTableChanged"`
	SecurityListIDs        []string          `json:"securityListIds,omitempty"`
	NSGIDs                 []string          `json:"nsgIds,omitempty"`
	SecurityListsChanged   bool              `json:"securityListsChanged"`
	NSGsChanged            bool              `json:"nsgsChanged"`
	PublicIPv4Changed      bool              `json:"publicIpv4Changed"`
	Warnings               []string          `json:"warnings,omitempty"`
	IrreversibleChanges    []string          `json:"irreversibleChanges,omitempty"`
	NetworkSteps           []IPv6NetworkStep `json:"networkSteps,omitempty"`
	IPv6ID                 string            `json:"ipv6Id,omitempty"`
	IPv6Address            string            `json:"ipv6Address,omitempty"`
	IPv6State              string            `json:"ipv6State,omitempty"`
	DeletedIPv6IDs         []string          `json:"deletedIpv6Ids,omitempty"`
	DeletedIPv6Addresses   []string          `json:"deletedIpv6Addresses,omitempty"`
	RequestID              string            `json:"requestId,omitempty"`
	WorkRequestID          string            `json:"workRequestId,omitempty"`
	WorkRequestIDs         []string          `json:"workRequestIds,omitempty"`
	Noop                   bool              `json:"noop"`
	ErrorCode              string            `json:"errorCode,omitempty"`
	ErrorMessage           string            `json:"errorMessage,omitempty"`
	ExecutedAt             time.Time         `json:"executedAt"`
	WaitedForAddress       bool              `json:"waitedForAddress"`
}

func ExecuteIPManagement(ctx context.Context, cfg ReadinessConfig, req IPManagementExecutionRequest) IPManagementExecutionResult {
	result := IPManagementExecutionResult{
		ExecutionMode:     cfg.ExecutionMode,
		InstanceID:        req.InstanceID,
		VNICID:            req.VNICID,
		AutoConfigureIPv6: req.AutoConfigureIPv6,
		IPv6Strategy:      defaultString(req.IPv6Strategy, "assign_only"),
		NetworkChangeMode: normalizedIPv6NetworkMode(req),
		RouteTableMode:    defaultString(req.RouteTableMode, routeTableModeFromNetworkMode(normalizedIPv6NetworkMode(req))),
		SecurityMode:      defaultString(req.SecurityMode, "append"),
		ExecutedAt:        time.Now().UTC(),
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
	if !req.EnableIPv6 && !req.DisableIPv6 {
		result.ErrorCode = "OCI_IP_OPERATION_UNSUPPORTED"
		result.ErrorMessage = "only IPv6 assignment and deletion are implemented for real OCI IP management"
		return result
	}

	clients, err := NewClients(cfg)
	if err != nil {
		result.ErrorCode = "OCI_CLIENT_INIT_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}

	instance, err := clients.Compute.GetInstance(ctx, core.GetInstanceRequest{
		InstanceId: common.String(req.InstanceID),
	})
	appendFirstRequestID(&result.RequestID, instance.OpcRequestId)
	if err != nil {
		result.ErrorCode = "OCI_GET_INSTANCE_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	compartmentID := stringValue(instance.Instance.CompartmentId)
	if compartmentID == "" {
		compartmentID = cfg.TenancyOCID
	}

	vnic, err := resolveVNIC(ctx, clients, compartmentID, req.InstanceID, req.VNICID, &result.RequestID)
	if err != nil {
		result.ErrorCode = "OCI_RESOLVE_VNIC_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	result.VNICID = stringValue(vnic.Id)
	result.SubnetID = stringValue(vnic.SubnetId)
	result.ExistingIPv6 = append(result.ExistingIPv6, vnic.Ipv6Addresses...)
	if req.DisableIPv6 {
		return deleteIPv6ForVNIC(ctx, clients, req, result)
	}
	if len(result.ExistingIPv6) > 0 {
		result.Noop = true
		result.Verified = true
		result.IPv6Address = result.ExistingIPv6[0]
		return result
	}
	if result.SubnetID == "" {
		result.ErrorCode = "OCI_VNIC_SUBNET_MISSING"
		result.ErrorMessage = "selected VNIC does not belong to a subnet"
		return result
	}

	subnet, err := clients.VirtualNetwork.GetSubnet(ctx, core.GetSubnetRequest{
		SubnetId: common.String(result.SubnetID),
	})
	appendFirstRequestID(&result.RequestID, subnet.OpcRequestId)
	if err != nil {
		result.ErrorCode = "OCI_GET_SUBNET_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	result.VCNID = stringValue(subnet.Subnet.VcnId)
	result.SubnetIPv6CIDR = firstSubnetIPv6CIDR(subnet.Subnet)
	if result.SubnetIPv6CIDR == "" {
		if !req.AutoConfigureIPv6 && normalizedIPv6NetworkMode(req) == ipv6ModeAssignOnly {
			result.ErrorCode = "OCI_IPV6_SUBNET_NOT_ENABLED"
			result.ErrorMessage = fmt.Sprintf("subnet %s has no IPv6 CIDR block; enable IPv6 on the VCN/subnet or choose automatic IPv6 network orchestration", result.SubnetID)
			return result
		}
	}
	if req.AutoConfigureIPv6 || normalizedIPv6NetworkMode(req) != ipv6ModeAssignOnly {
		updatedSubnet, ok := ensureIPv6Network(ctx, clients, cfg, req, &result, instance.Instance, vnic, subnet.Subnet)
		if !ok {
			return result
		}
		subnet.Subnet = updatedSubnet
		result.SubnetIPv6CIDR = firstSubnetIPv6CIDR(subnet.Subnet)
	}
	if result.SubnetIPv6CIDR == "" {
		result.ErrorCode = "OCI_IPV6_SUBNET_NOT_ENABLED"
		result.ErrorMessage = fmt.Sprintf("subnet %s has no IPv6 CIDR block after orchestration", result.SubnetID)
		return result
	}

	response, err := clients.VirtualNetwork.CreateIpv6(ctx, core.CreateIpv6Request{
		CreateIpv6Details: core.CreateIpv6Details{
			DisplayName:    common.String("codex-ipv6-" + time.Now().UTC().Format("20060102-150405")),
			VnicId:         common.String(result.VNICID),
			SubnetId:       common.String(result.SubnetID),
			Ipv6SubnetCidr: common.String(result.SubnetIPv6CIDR),
			Lifetime:       core.CreateIpv6DetailsLifetimeEphemeral,
			FreeformTags: map[string]string{
				"managedBy": "codex",
				"purpose":   "ip-management-ipv6",
			},
		},
		OpcRetryToken: retryToken("ipv6", req.JobID),
		OpcRequestId:  requestID("codex-ipv6", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, response.OpcRequestId)
	if response.Ipv6.Id != nil {
		result.IPv6ID = *response.Ipv6.Id
	}
	if response.Ipv6.IpAddress != nil {
		result.IPv6Address = *response.Ipv6.IpAddress
	}
	result.IPv6State = string(response.Ipv6.LifecycleState)
	if err != nil {
		result.ErrorCode = "OCI_CREATE_IPV6_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	if result.IPv6ID == "" {
		result.ErrorCode = "OCI_CREATE_IPV6_EMPTY_ID"
		result.ErrorMessage = "CreateIpv6 succeeded without an IPv6 OCID"
		return result
	}

	ipv6, err := waitIPv6State(ctx, clients, result.IPv6ID, 5*time.Minute, core.Ipv6LifecycleStateAvailable)
	result.WaitedForAddress = true
	result.IPv6State = string(ipv6.LifecycleState)
	result.IPv6Address = defaultString(stringValue(ipv6.IpAddress), result.IPv6Address)
	if err != nil {
		result.ErrorCode = "OCI_WAIT_IPV6_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}

	result.Verified = true
	return result
}

func deleteIPv6ForVNIC(ctx context.Context, clients Clients, req IPManagementExecutionRequest, result IPManagementExecutionResult) IPManagementExecutionResult {
	if result.VNICID == "" {
		result.ErrorCode = "OCI_VNIC_ID_REQUIRED"
		result.ErrorMessage = "selected VNIC is required to delete IPv6 addresses"
		return result
	}
	page := ""
	for {
		listResp, err := clients.VirtualNetwork.ListIpv6s(ctx, core.ListIpv6sRequest{
			VnicId:       common.String(result.VNICID),
			Limit:        common.Int(100),
			Page:         optionalString(page),
			OpcRequestId: requestID("codex-list-ipv6", req.JobID),
		})
		appendFirstRequestID(&result.RequestID, listResp.OpcRequestId)
		if err != nil {
			result.ErrorCode = "OCI_LIST_IPV6_FAILED"
			result.ErrorMessage = err.Error()
			return result
		}
		for _, item := range listResp.Items {
			ipv6ID := stringValue(item.Id)
			if ipv6ID == "" {
				continue
			}
			deleteResp, err := clients.VirtualNetwork.DeleteIpv6(ctx, core.DeleteIpv6Request{
				Ipv6Id:       common.String(ipv6ID),
				OpcRequestId: requestID("codex-delete-ipv6", req.JobID),
			})
			appendFirstRequestID(&result.RequestID, deleteResp.OpcRequestId)
			if err != nil {
				result.ErrorCode = "OCI_DELETE_IPV6_FAILED"
				result.ErrorMessage = err.Error()
				return result
			}
			result.DeletedIPv6IDs = append(result.DeletedIPv6IDs, ipv6ID)
			if address := stringValue(item.IpAddress); address != "" {
				result.DeletedIPv6Addresses = append(result.DeletedIPv6Addresses, address)
			}
		}
		if listResp.OpcNextPage == nil || *listResp.OpcNextPage == "" {
			break
		}
		page = *listResp.OpcNextPage
	}
	result.Noop = len(result.DeletedIPv6IDs) == 0
	result.Verified = true
	return result
}

func resolveVNIC(ctx context.Context, clients Clients, compartmentID string, instanceID string, requestedVNICID string, requestID *string) (core.Vnic, error) {
	requestedVNICID = strings.TrimSpace(requestedVNICID)
	if strings.HasPrefix(requestedVNICID, "ocid1.vnic.") {
		response, err := clients.VirtualNetwork.GetVnic(ctx, core.GetVnicRequest{
			VnicId: common.String(requestedVNICID),
		})
		appendFirstRequestID(requestID, response.OpcRequestId)
		return response.Vnic, err
	}

	response, err := clients.Compute.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
		CompartmentId: common.String(compartmentID),
		InstanceId:    common.String(instanceID),
		Limit:         common.Int(50),
	})
	appendFirstRequestID(requestID, response.OpcRequestId)
	if err != nil {
		return core.Vnic{}, err
	}
	var fallback core.Vnic
	for _, attachment := range response.Items {
		if attachment.VnicId == nil || *attachment.VnicId == "" {
			continue
		}
		if attachment.LifecycleState == core.VnicAttachmentLifecycleStateDetached || attachment.LifecycleState == core.VnicAttachmentLifecycleStateDetaching {
			continue
		}
		vnicResponse, err := clients.VirtualNetwork.GetVnic(ctx, core.GetVnicRequest{
			VnicId: attachment.VnicId,
		})
		appendFirstRequestID(requestID, vnicResponse.OpcRequestId)
		if err != nil {
			return core.Vnic{}, err
		}
		if fallback.Id == nil {
			fallback = vnicResponse.Vnic
		}
		if requestedVNICID == "" || strings.EqualFold(requestedVNICID, "primary") {
			if vnicResponse.Vnic.IsPrimary == nil || *vnicResponse.Vnic.IsPrimary {
				return vnicResponse.Vnic, nil
			}
			continue
		}
		if strings.EqualFold(requestedVNICID, stringValue(vnicResponse.Vnic.DisplayName)) || strings.EqualFold(requestedVNICID, stringValue(vnicResponse.Vnic.Id)) {
			return vnicResponse.Vnic, nil
		}
	}
	if fallback.Id != nil {
		return fallback, nil
	}
	return core.Vnic{}, fmt.Errorf("no attached VNIC found for instance %s", instanceID)
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return common.String(value)
}

func firstSubnetIPv6CIDR(subnet core.Subnet) string {
	if strings.TrimSpace(stringValue(subnet.Ipv6CidrBlock)) != "" {
		return strings.TrimSpace(stringValue(subnet.Ipv6CidrBlock))
	}
	for _, cidr := range subnet.Ipv6CidrBlocks {
		if strings.TrimSpace(cidr) != "" {
			return strings.TrimSpace(cidr)
		}
	}
	return ""
}

func waitIPv6State(ctx context.Context, clients Clients, ipv6ID string, timeout time.Duration, accepted ...core.Ipv6LifecycleStateEnum) (core.Ipv6, error) {
	deadline := time.Now().Add(timeout)
	for {
		response, err := clients.VirtualNetwork.GetIpv6(ctx, core.GetIpv6Request{Ipv6Id: common.String(ipv6ID)})
		if err != nil {
			return response.Ipv6, err
		}
		for _, state := range accepted {
			if response.Ipv6.LifecycleState == state {
				return response.Ipv6, nil
			}
		}
		if time.Now().After(deadline) {
			return response.Ipv6, fmt.Errorf("timed out waiting for IPv6 %s; last state=%s", ipv6ID, response.Ipv6.LifecycleState)
		}
		select {
		case <-ctx.Done():
			return response.Ipv6, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func appendFirstRequestID(target *string, value *string) {
	if target == nil || *target != "" || value == nil || *value == "" {
		return
	}
	*target = *value
}
