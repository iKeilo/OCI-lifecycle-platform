package oci

import (
	"context"
	"strings"
	"time"

	"a-series-oracle/backend/internal/domain"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

func DiscoverNetworkInventory(ctx context.Context, cfg ReadinessConfig, req domain.NetworkInventoryRequest) domain.NetworkInventory {
	result := domain.NetworkInventory{
		ExecutionMode: cfg.ExecutionMode,
		ProfileID:     req.ProfileID,
		Region:        cfg.Region,
		CompartmentID: strings.TrimSpace(req.CompartmentID),
		LastSyncedAt:  time.Now().UTC(),
		PublicIPs:     []domain.PublicIPResource{},
		PrivateIPs:    []domain.PrivateIPResource{},
		IPv6s:         []domain.IPv6Resource{},
		VCNs:          []domain.VCNResource{},
		Subnets:       []domain.SubnetResource{},
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
	if err := discoverNetworkVCNs(ctx, clients, req, &result); err != nil {
		result.ErrorCode = "OCI_LIST_VCNS_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	if err := discoverNetworkSubnets(ctx, clients, req, &result); err != nil {
		result.ErrorCode = "OCI_LIST_SUBNETS_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	if err := discoverNetworkPublicIPs(ctx, clients, &result); err != nil {
		result.ErrorCode = "OCI_LIST_PUBLIC_IPS_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	if err := discoverNetworkPrivateAndIPv6(ctx, clients, &result); err != nil {
		result.ErrorCode = "OCI_LIST_PRIVATE_OR_IPV6_IPS_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	result.Verified = true
	return result
}

func discoverNetworkVCNs(ctx context.Context, clients Clients, req domain.NetworkInventoryRequest, result *domain.NetworkInventory) error {
	page := ""
	for {
		listReq := core.ListVcnsRequest{
			CompartmentId:  common.String(result.CompartmentID),
			LifecycleState: core.VcnLifecycleStateAvailable,
			Limit:          common.Int(100),
		}
		if page != "" {
			listReq.Page = common.String(page)
		}
		resp, err := clients.VirtualNetwork.ListVcns(ctx, listReq)
		appendRequestID(&result.RequestIDs, resp.OpcRequestId)
		if err != nil {
			return err
		}
		for _, item := range resp.Items {
			id := stringValue(item.Id)
			if id == "" {
				continue
			}
			if strings.TrimSpace(req.VCNID) != "" && strings.TrimSpace(req.VCNID) != id {
				continue
			}
			result.VCNs = append(result.VCNs, domain.VCNResource{
				ID:             id,
				DisplayName:    defaultString(stringValue(item.DisplayName), id),
				CIDRBlock:      stringValue(item.CidrBlock),
				IPv6CIDRBlocks: append([]string(nil), item.Ipv6CidrBlocks...),
				LifecycleState: string(item.LifecycleState),
				CompartmentID:  stringValue(item.CompartmentId),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		page = *resp.OpcNextPage
	}
	return nil
}

func discoverNetworkSubnets(ctx context.Context, clients Clients, req domain.NetworkInventoryRequest, result *domain.NetworkInventory) error {
	page := ""
	for {
		listReq := core.ListSubnetsRequest{
			CompartmentId:  common.String(result.CompartmentID),
			LifecycleState: core.SubnetLifecycleStateAvailable,
			Limit:          common.Int(100),
		}
		if strings.TrimSpace(req.VCNID) != "" {
			listReq.VcnId = common.String(strings.TrimSpace(req.VCNID))
		}
		if page != "" {
			listReq.Page = common.String(page)
		}
		resp, err := clients.VirtualNetwork.ListSubnets(ctx, listReq)
		appendRequestID(&result.RequestIDs, resp.OpcRequestId)
		if err != nil {
			return err
		}
		for _, item := range resp.Items {
			id := stringValue(item.Id)
			if id == "" {
				continue
			}
			public := true
			if item.ProhibitPublicIpOnVnic != nil {
				public = !*item.ProhibitPublicIpOnVnic
			}
			ipv6CIDRs := append([]string(nil), item.Ipv6CidrBlocks...)
			if stringValue(item.Ipv6CidrBlock) != "" && len(ipv6CIDRs) == 0 {
				ipv6CIDRs = append(ipv6CIDRs, stringValue(item.Ipv6CidrBlock))
			}
			result.Subnets = append(result.Subnets, domain.SubnetResource{
				ID:             id,
				DisplayName:    defaultString(stringValue(item.DisplayName), id),
				VCNID:          stringValue(item.VcnId),
				CIDRBlock:      stringValue(item.CidrBlock),
				IPv6CIDRBlocks: ipv6CIDRs,
				Public:         public,
				CompartmentID:  stringValue(item.CompartmentId),
				LifecycleState: string(item.LifecycleState),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		page = *resp.OpcNextPage
	}
	return nil
}

func discoverNetworkPublicIPs(ctx context.Context, clients Clients, result *domain.NetworkInventory) error {
	page := ""
	for {
		req := core.ListPublicIpsRequest{
			CompartmentId: common.String(result.CompartmentID),
			Scope:         core.ListPublicIpsScopeRegion,
			Limit:         common.Int(100),
		}
		if page != "" {
			req.Page = common.String(page)
		}
		resp, err := clients.VirtualNetwork.ListPublicIps(ctx, req)
		appendRequestID(&result.RequestIDs, resp.OpcRequestId)
		if err != nil {
			return err
		}
		for _, item := range resp.Items {
			id := stringValue(item.Id)
			if id == "" {
				continue
			}
			result.PublicIPs = append(result.PublicIPs, domain.PublicIPResource{
				ID:               id,
				DisplayName:      defaultString(stringValue(item.DisplayName), id),
				IPAddress:        stringValue(item.IpAddress),
				Lifetime:         string(item.Lifetime),
				Scope:            string(item.Scope),
				LifecycleState:   string(item.LifecycleState),
				AssignedEntityID: stringValue(item.AssignedEntityId),
				CompartmentID:    stringValue(item.CompartmentId),
				Region:           result.Region,
				TimeCreated:      timeValue(item.TimeCreated),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		page = *resp.OpcNextPage
	}
	return nil
}

func discoverNetworkPrivateAndIPv6(ctx context.Context, clients Clients, result *domain.NetworkInventory) error {
	for _, subnet := range result.Subnets {
		if err := discoverNetworkPrivateIPsForSubnet(ctx, clients, subnet.ID, result); err != nil {
			return err
		}
		if err := discoverNetworkIPv6ForSubnet(ctx, clients, subnet.ID, result); err != nil {
			return err
		}
	}
	return nil
}

func discoverNetworkPrivateIPsForSubnet(ctx context.Context, clients Clients, subnetID string, result *domain.NetworkInventory) error {
	page := ""
	for {
		req := core.ListPrivateIpsRequest{
			SubnetId: common.String(subnetID),
			Limit:    common.Int(100),
		}
		if page != "" {
			req.Page = common.String(page)
		}
		resp, err := clients.VirtualNetwork.ListPrivateIps(ctx, req)
		appendRequestID(&result.RequestIDs, resp.OpcRequestId)
		if err != nil {
			return err
		}
		for _, item := range resp.Items {
			id := stringValue(item.Id)
			if id == "" {
				continue
			}
			result.PrivateIPs = append(result.PrivateIPs, domain.PrivateIPResource{
				ID:             id,
				DisplayName:    defaultString(stringValue(item.DisplayName), id),
				IPAddress:      stringValue(item.IpAddress),
				HostnameLabel:  stringValue(item.HostnameLabel),
				VNICID:         stringValue(item.VnicId),
				SubnetID:       stringValue(item.SubnetId),
				CompartmentID:  stringValue(item.CompartmentId),
				LifecycleState: "ASSIGNED",
				TimeCreated:    timeValue(item.TimeCreated),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		page = *resp.OpcNextPage
	}
	return nil
}

func discoverNetworkIPv6ForSubnet(ctx context.Context, clients Clients, subnetID string, result *domain.NetworkInventory) error {
	page := ""
	for {
		req := core.ListIpv6sRequest{
			SubnetId: common.String(subnetID),
			Limit:    common.Int(100),
		}
		if page != "" {
			req.Page = common.String(page)
		}
		resp, err := clients.VirtualNetwork.ListIpv6s(ctx, req)
		appendRequestID(&result.RequestIDs, resp.OpcRequestId)
		if err != nil {
			return err
		}
		for _, item := range resp.Items {
			id := stringValue(item.Id)
			if id == "" {
				continue
			}
			result.IPv6s = append(result.IPv6s, domain.IPv6Resource{
				ID:             id,
				DisplayName:    defaultString(stringValue(item.DisplayName), id),
				IPAddress:      stringValue(item.IpAddress),
				VNICID:         stringValue(item.VnicId),
				SubnetID:       stringValue(item.SubnetId),
				CompartmentID:  stringValue(item.CompartmentId),
				LifecycleState: string(item.LifecycleState),
				TimeCreated:    timeValue(item.TimeCreated),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		page = *resp.OpcNextPage
	}
	return nil
}

func timeValue(value *common.SDKTime) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.Time
}
