package oci

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"a-series-oracle/backend/internal/domain"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

type LaunchOptionsRequest struct {
	ProfileID          string `json:"profileId"`
	Region             string `json:"region"`
	CompartmentID      string `json:"compartmentId"`
	AvailabilityDomain string `json:"availabilityDomain"`
	VCNID              string `json:"vcnId"`
	Shape              string `json:"shape"`
}

func DiscoverLaunchOptions(ctx context.Context, cfg ReadinessConfig, req LaunchOptionsRequest) domain.LaunchOptions {
	result := domain.LaunchOptions{
		ProfileID:     req.ProfileID,
		Region:        cfg.Region,
		CompartmentID: req.CompartmentID,
		LastSyncedAt:  time.Now().UTC(),
	}
	readiness := CheckReadiness(cfg)
	if !readiness.Ready {
		result.ErrorCode = "OCI_NOT_READY"
		result.ErrorMessage = readiness.Message
		return result
	}
	if strings.TrimSpace(result.CompartmentID) == "" {
		result.CompartmentID = cfg.TenancyOCID
	}

	clients, err := NewClients(cfg)
	if err != nil {
		result.ErrorCode = "OCI_CLIENT_INIT_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}

	if err := discoverRegions(ctx, clients, cfg, &result); err != nil {
		result.ErrorCode = "OCI_LIST_REGIONS_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	if err := discoverCompartments(ctx, clients, cfg, &result); err != nil {
		result.ErrorCode = "OCI_LIST_COMPARTMENTS_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	if err := discoverAvailabilityDomains(ctx, clients, cfg, &result); err != nil {
		result.ErrorCode = "OCI_LIST_ADS_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	discoverBootVolumeUsage(ctx, clients, &result)
	if err := discoverShapes(ctx, clients, req, &result); err != nil {
		result.ErrorCode = "OCI_LIST_SHAPES_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	if err := discoverImages(ctx, clients, req, &result); err != nil {
		result.ErrorCode = "OCI_LIST_IMAGES_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	if err := discoverVCNs(ctx, clients, &result); err != nil {
		result.ErrorCode = "OCI_LIST_VCNS_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	if err := discoverSubnets(ctx, clients, req, &result); err != nil {
		result.ErrorCode = "OCI_LIST_SUBNETS_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	if err := discoverReservedPublicIPs(ctx, clients, &result); err != nil {
		result.ErrorCode = "OCI_LIST_RESERVED_IPS_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}

	result.Verified = true
	return result
}

func discoverRegions(ctx context.Context, clients Clients, cfg ReadinessConfig, result *domain.LaunchOptions) error {
	resp, err := clients.Identity.ListRegionSubscriptions(ctx, identity.ListRegionSubscriptionsRequest{
		TenancyId: common.String(cfg.TenancyOCID),
	})
	appendRequestID(&result.RequestIDs, resp.OpcRequestId)
	if err != nil {
		return err
	}
	for _, item := range resp.Items {
		name := stringValue(item.RegionName)
		if name == "" {
			continue
		}
		result.Regions = append(result.Regions, domain.LaunchOption{
			ID:     name,
			Label:  name + " / " + string(item.Status),
			Region: name,
		})
	}
	sortLaunchOptions(result.Regions)
	return nil
}

func discoverCompartments(ctx context.Context, clients Clients, cfg ReadinessConfig, result *domain.LaunchOptions) error {
	result.Compartments = append(result.Compartments, domain.LaunchOption{
		ID:          cfg.TenancyOCID,
		Label:       "Root tenancy",
		Compartment: cfg.TenancyOCID,
	})
	page := ""
	for {
		req := identity.ListCompartmentsRequest{
			CompartmentId:          common.String(cfg.TenancyOCID),
			CompartmentIdInSubtree: common.Bool(true),
			AccessLevel:            identity.ListCompartmentsAccessLevelAccessible,
			LifecycleState:         identity.CompartmentLifecycleStateActive,
			Limit:                  common.Int(100),
		}
		if page != "" {
			req.Page = common.String(page)
		}
		resp, err := clients.Identity.ListCompartments(ctx, req)
		appendRequestID(&result.RequestIDs, resp.OpcRequestId)
		if err != nil {
			return err
		}
		for _, item := range resp.Items {
			id := stringValue(item.Id)
			if id == "" {
				continue
			}
			name := defaultString(stringValue(item.Name), id)
			result.Compartments = append(result.Compartments, domain.LaunchOption{
				ID:          id,
				Label:       name,
				Compartment: id,
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		page = *resp.OpcNextPage
	}
	sortLaunchOptions(result.Compartments)
	return nil
}

func discoverAvailabilityDomains(ctx context.Context, clients Clients, cfg ReadinessConfig, result *domain.LaunchOptions) error {
	resp, err := clients.Identity.ListAvailabilityDomains(ctx, identity.ListAvailabilityDomainsRequest{
		CompartmentId: common.String(cfg.TenancyOCID),
	})
	appendRequestID(&result.RequestIDs, resp.OpcRequestId)
	if err != nil {
		return err
	}
	for _, item := range resp.Items {
		name := stringValue(item.Name)
		if name == "" {
			continue
		}
		result.AvailabilityADs = append(result.AvailabilityADs, domain.LaunchOption{
			ID:     name,
			Label:  name,
			Region: cfg.Region,
		})
	}
	sortLaunchOptions(result.AvailabilityADs)
	return nil
}

func discoverShapes(ctx context.Context, clients Clients, req LaunchOptionsRequest, result *domain.LaunchOptions) error {
	page := ""
	for {
		listReq := core.ListShapesRequest{
			CompartmentId: common.String(result.CompartmentID),
			Limit:         common.Int(100),
		}
		if strings.TrimSpace(req.AvailabilityDomain) != "" {
			listReq.AvailabilityDomain = common.String(req.AvailabilityDomain)
		}
		if page != "" {
			listReq.Page = common.String(page)
		}
		resp, err := clients.Compute.ListShapes(ctx, listReq)
		appendRequestID(&result.RequestIDs, resp.OpcRequestId)
		if err != nil {
			return err
		}
		for _, item := range resp.Items {
			name := stringValue(item.Shape)
			if name == "" {
				continue
			}
			result.Shapes = append(result.Shapes, mapShapeOption(item))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		page = *resp.OpcNextPage
	}
	sort.Slice(result.Shapes, func(i, j int) bool { return result.Shapes[i].Name < result.Shapes[j].Name })
	return nil
}

func discoverImages(ctx context.Context, clients Clients, req LaunchOptionsRequest, result *domain.LaunchOptions) error {
	listReq := core.ListImagesRequest{
		CompartmentId:  common.String(result.CompartmentID),
		LifecycleState: core.ImageLifecycleStateAvailable,
		SortBy:         core.ListImagesSortByTimecreated,
		SortOrder:      core.ListImagesSortOrderDesc,
		Limit:          common.Int(50),
	}
	if strings.TrimSpace(req.Shape) != "" {
		listReq.Shape = common.String(req.Shape)
	}
	resp, err := clients.Compute.ListImages(ctx, listReq)
	appendRequestID(&result.RequestIDs, resp.OpcRequestId)
	if err != nil {
		return err
	}
	for _, item := range resp.Items {
		id := stringValue(item.Id)
		if id == "" {
			continue
		}
		label := defaultString(stringValue(item.DisplayName), id)
		osName := strings.TrimSpace(stringValue(item.OperatingSystem) + " " + stringValue(item.OperatingSystemVersion))
		if osName != "" {
			label = label + " / " + osName
		}
		result.Images = append(result.Images, domain.LaunchOption{
			ID:    id,
			Label: label,
		})
	}
	sortLaunchOptions(result.Images)
	return nil
}

func discoverVCNs(ctx context.Context, clients Clients, result *domain.LaunchOptions) error {
	resp, err := clients.VirtualNetwork.ListVcns(ctx, core.ListVcnsRequest{
		CompartmentId:  common.String(result.CompartmentID),
		LifecycleState: core.VcnLifecycleStateAvailable,
		Limit:          common.Int(100),
	})
	appendRequestID(&result.RequestIDs, resp.OpcRequestId)
	if err != nil {
		return err
	}
	for _, item := range resp.Items {
		id := stringValue(item.Id)
		if id == "" {
			continue
		}
		result.VCNs = append(result.VCNs, domain.LaunchOption{
			ID:          id,
			Label:       defaultString(stringValue(item.DisplayName), id),
			Compartment: result.CompartmentID,
		})
	}
	sortLaunchOptions(result.VCNs)
	return nil
}

func discoverSubnets(ctx context.Context, clients Clients, req LaunchOptionsRequest, result *domain.LaunchOptions) error {
	listReq := core.ListSubnetsRequest{
		CompartmentId:  common.String(result.CompartmentID),
		LifecycleState: core.SubnetLifecycleStateAvailable,
		Limit:          common.Int(100),
	}
	if strings.TrimSpace(req.VCNID) != "" {
		listReq.VcnId = common.String(req.VCNID)
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
		result.Subnets = append(result.Subnets, domain.LaunchOption{
			ID:          id,
			Label:       defaultString(stringValue(item.DisplayName), id),
			Compartment: stringValue(item.VcnId),
			Public:      public,
			IPv6Enabled: subnetIPv6Enabled(item),
		})
	}
	sortLaunchOptions(result.Subnets)
	return nil
}

func subnetIPv6Enabled(item core.Subnet) bool {
	return strings.TrimSpace(stringValue(item.Ipv6CidrBlock)) != "" || len(item.Ipv6CidrBlocks) > 0
}

func discoverReservedPublicIPs(ctx context.Context, clients Clients, result *domain.LaunchOptions) error {
	resp, err := clients.VirtualNetwork.ListPublicIps(ctx, core.ListPublicIpsRequest{
		CompartmentId: common.String(result.CompartmentID),
		Scope:         core.ListPublicIpsScopeRegion,
		Lifetime:      core.ListPublicIpsLifetimeReserved,
		Limit:         common.Int(100),
	})
	appendRequestID(&result.RequestIDs, resp.OpcRequestId)
	if err != nil {
		return err
	}
	for _, item := range resp.Items {
		id := stringValue(item.Id)
		if id == "" || stringValue(item.AssignedEntityId) != "" {
			continue
		}
		label := defaultString(stringValue(item.DisplayName), id)
		if stringValue(item.IpAddress) != "" {
			label = label + " / " + stringValue(item.IpAddress)
		}
		result.ReservedIPs = append(result.ReservedIPs, domain.LaunchOption{
			ID:          id,
			Label:       label,
			Compartment: result.CompartmentID,
			Public:      true,
		})
	}
	sortLaunchOptions(result.ReservedIPs)
	return nil
}

func discoverBootVolumeUsage(ctx context.Context, clients Clients, result *domain.LaunchOptions) {
	usage := domain.BootVolumeUsage{
		Region:       result.Region,
		LastSyncedAt: time.Now().UTC(),
	}
	compartmentIDs := uniqueLaunchOptionIDs(result.Compartments)
	if len(compartmentIDs) == 0 && strings.TrimSpace(result.CompartmentID) != "" {
		compartmentIDs = append(compartmentIDs, result.CompartmentID)
	}
	availabilityDomains := uniqueLaunchOptionIDs(result.AvailabilityADs)
	usage.CompartmentCount = len(compartmentIDs)
	usage.AvailabilityDomainCount = len(availabilityDomains)

	if len(compartmentIDs) == 0 || len(availabilityDomains) == 0 {
		usage.ErrorCode = "OCI_BOOT_VOLUME_SCOPE_EMPTY"
		usage.ErrorMessage = "boot volume usage requires at least one compartment and availability domain"
		result.BootVolumeUsage = usage
		return
	}

	for _, compartmentID := range compartmentIDs {
		for _, availabilityDomain := range availabilityDomains {
			page := ""
			for {
				req := core.ListBootVolumesRequest{
					AvailabilityDomain: common.String(availabilityDomain),
					CompartmentId:      common.String(compartmentID),
					Limit:              common.Int(100),
				}
				if page != "" {
					req.Page = common.String(page)
				}
				resp, err := clients.Blockstorage.ListBootVolumes(ctx, req)
				appendRequestID(&usage.RequestIDs, resp.OpcRequestId)
				appendRequestID(&result.RequestIDs, resp.OpcRequestId)
				if err != nil {
					usage.ErrorCode = "OCI_LIST_BOOT_VOLUMES_FAILED"
					usage.ErrorMessage = err.Error()
					result.BootVolumeUsage = usage
					return
				}
				for _, item := range resp.Items {
					if item.LifecycleState == core.BootVolumeLifecycleStateTerminated || item.LifecycleState == core.BootVolumeLifecycleStateTerminating {
						continue
					}
					if item.SizeInGBs != nil {
						usage.TotalGB += int(*item.SizeInGBs)
					}
					usage.BootVolumeCount++
				}
				if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
					break
				}
				page = *resp.OpcNextPage
			}
		}
	}
	usage.Verified = true
	result.BootVolumeUsage = usage
}

func uniqueLaunchOptionIDs(options []domain.LaunchOption) []string {
	seen := map[string]bool{}
	values := make([]string, 0, len(options))
	for _, option := range options {
		id := strings.TrimSpace(option.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		values = append(values, id)
	}
	return values
}

func mapShapeOption(item core.Shape) domain.ShapeOption {
	name := stringValue(item.Shape)
	minOCPUs := ceilFloat32(item.Ocpus)
	maxOCPUs := minOCPUs
	if item.OcpuOptions != nil {
		minOCPUs = ceilFloat32(item.OcpuOptions.Min)
		maxOCPUs = ceilFloat32(item.OcpuOptions.Max)
	}
	minMemory := ceilFloat32(item.MemoryInGBs)
	maxMemory := minMemory
	if item.MemoryOptions != nil {
		minMemory = ceilFloat32(item.MemoryOptions.MinInGBs)
		maxMemory = ceilFloat32(item.MemoryOptions.MaxInGBs)
	}
	return domain.ShapeOption{
		Name:        name,
		Arch:        defaultString(stringValue(item.ProcessorDescription), "unknown"),
		MinOCPUs:    minOCPUs,
		MaxOCPUs:    maxOCPUs,
		MinMemoryGB: minMemory,
		MaxMemoryGB: maxMemory,
	}
}

func ceilFloat32(value *float32) int {
	if value == nil {
		return 0
	}
	return int(math.Ceil(float64(*value)))
}

func sortLaunchOptions(options []domain.LaunchOption) {
	sort.Slice(options, func(i, j int) bool {
		if options[i].Label == options[j].Label {
			return options[i].ID < options[j].ID
		}
		return options[i].Label < options[j].Label
	})
}
