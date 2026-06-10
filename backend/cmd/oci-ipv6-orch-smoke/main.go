package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"a-series-oracle/backend/internal/config"
	"a-series-oracle/backend/internal/domain"
	"a-series-oracle/backend/internal/fileprofile"
	"a-series-oracle/backend/internal/oci"
	"a-series-oracle/backend/internal/profileconfig"
	"a-series-oracle/backend/internal/store"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

const (
	defaultShape    = "VM.Standard.E3.Flex"
	smokeNamePrefix = "codex-ipv6-orch-smoke-"
)

type smokeResult struct {
	Verified             bool                             `json:"verified"`
	CleanupVerified      bool                             `json:"cleanupVerified"`
	Phase                string                           `json:"phase,omitempty"`
	ProfileID            string                           `json:"profileId,omitempty"`
	ProfileName          string                           `json:"profileName,omitempty"`
	Region               string                           `json:"region,omitempty"`
	CompartmentID        string                           `json:"compartmentId,omitempty"`
	DisplayName          string                           `json:"displayName,omitempty"`
	Shape                string                           `json:"shape,omitempty"`
	OCPUs                int                              `json:"ocpus,omitempty"`
	MemoryGB             int                              `json:"memoryGb,omitempty"`
	BootVolumeGB         int                              `json:"bootVolumeGb,omitempty"`
	VCNID                string                           `json:"vcnId,omitempty"`
	SubnetID             string                           `json:"subnetId,omitempty"`
	RouteTableID         string                           `json:"routeTableId,omitempty"`
	SecurityListID       string                           `json:"securityListId,omitempty"`
	InternetGatewayID    string                           `json:"internetGatewayId,omitempty"`
	InstanceID           string                           `json:"instanceId,omitempty"`
	VNICID               string                           `json:"vnicId,omitempty"`
	NSGID                string                           `json:"nsgId,omitempty"`
	NSGBound             bool                             `json:"nsgBound"`
	NSGRulesChanged      bool                             `json:"nsgRulesChanged"`
	ReservedPublicIPID   string                           `json:"reservedPublicIpId,omitempty"`
	ReservedPublicIPv4   string                           `json:"reservedPublicIpv4,omitempty"`
	ReservedPublicIPUsed bool                             `json:"reservedPublicIpUsed"`
	InitialPublicIPv4    string                           `json:"initialPublicIpv4,omitempty"`
	FinalPublicIPv4      string                           `json:"finalPublicIpv4,omitempty"`
	IPv4Preserved        bool                             `json:"ipv4Preserved"`
	IPv4Changed          bool                             `json:"ipv4Changed"`
	IPv6Address          string                           `json:"ipv6Address,omitempty"`
	VCNIPv6CIDR          string                           `json:"vcnIpv6Cidr,omitempty"`
	SubnetIPv6CIDR       string                           `json:"subnetIpv6Cidr,omitempty"`
	Mode                 string                           `json:"mode,omitempty"`
	IPv6Result           *oci.IPManagementExecutionResult `json:"ipv6Result,omitempty"`
	Additive             *oci.IPManagementExecutionResult `json:"additive,omitempty"`
	Fallback             *oci.IPManagementExecutionResult `json:"fallback,omitempty"`
	Cleanup              []cleanupStep                    `json:"cleanup,omitempty"`
	Warnings             []string                         `json:"warnings,omitempty"`
	ErrorCode            string                           `json:"errorCode,omitempty"`
	ErrorMessage         string                           `json:"errorMessage,omitempty"`
	ExecutedAt           time.Time                        `json:"executedAt"`
}

type cleanupStep struct {
	Name         string `json:"name"`
	ResourceID   string `json:"resourceId,omitempty"`
	Verified     bool   `json:"verified"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type createdNetwork struct {
	VCNID             string
	SubnetID          string
	RouteTableID      string
	SecurityListID    string
	InternetGatewayID string
	IPv4CIDR          string
	SubnetIPv4CIDR    string
}

func main() {
	profileIDFlag := flag.String("profile", "", "profile ID or name; defaults to the first stored profile")
	regionFlag := flag.String("region", "", "OCI region override")
	compartmentFlag := flag.String("compartment", "", "compartment OCID; defaults to OCI_COMPARTMENT_OCID or tenancy")
	shapeFlag := flag.String("shape", defaultShape, "compute shape")
	ocpusFlag := flag.Int("ocpus", 1, "flex shape OCPUs")
	memoryFlag := flag.Int("memory-gb", 1, "flex shape memory in GB")
	bootFlag := flag.Int("boot-gb", 50, "boot volume size in GB")
	modeFlag := flag.String("mode", "auto", "IPv6 orchestration mode: auto, additive, clone_route_table, replace_public_path")
	nsgFlag := flag.Bool("nsg", false, "create a test NSG, attach it to the primary VNIC, and verify IPv6 NSG rules")
	reservedIPFlag := flag.Bool("reserved-public-ip", false, "create a test reserved public IP and use it for instance launch")
	cleanupFlag := flag.Bool("cleanup", true, "terminate and delete test resources before exit")
	fallbackFlag := flag.Bool("fallback", true, "try clone/replace network modes if additive mode fails")
	timeoutFlag := flag.Int("timeout-minutes", 75, "overall timeout in minutes")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutFlag)*time.Minute)
	defer cancel()

	result := smokeResult{
		ExecutedAt:   time.Now().UTC(),
		Shape:        strings.TrimSpace(*shapeFlag),
		OCPUs:        *ocpusFlag,
		MemoryGB:     *memoryFlag,
		BootVolumeGB: *bootFlag,
		Mode:         normalizeSmokeMode(*modeFlag),
	}
	defer func() {
		writeJSON(result)
		if !result.Verified || (*cleanupFlag && !result.CleanupVerified) {
			os.Exit(1)
		}
	}()

	cfg, profile, err := resolveConfig(*profileIDFlag, *regionFlag)
	if err != nil {
		fail(&result, "OCI_PROFILE_RESOLVE_FAILED", err.Error())
		return
	}
	result.ProfileID = profile.ID
	result.ProfileName = profile.Name
	result.Region = cfg.Region
	result.CompartmentID = firstNonEmpty(strings.TrimSpace(*compartmentFlag), os.Getenv("OCI_COMPARTMENT_OCID"), cfg.TenancyOCID)
	if result.CompartmentID == "" {
		fail(&result, "OCI_COMPARTMENT_REQUIRED", "compartment OCID is required")
		return
	}

	readiness := oci.CheckReadiness(cfg)
	if !readiness.Ready {
		fail(&result, "OCI_NOT_READY", readiness.Message)
		return
	}

	clients, err := oci.NewClients(cfg)
	if err != nil {
		fail(&result, "OCI_CLIENT_INIT_FAILED", err.Error())
		return
	}

	suffix := time.Now().UTC().Format("20060102-150405")
	result.DisplayName = smokeNamePrefix + suffix

	if err := createAndVerify(ctx, cfg, clients, &result, *fallbackFlag, *nsgFlag, *reservedIPFlag); err != nil && result.ErrorCode == "" {
		fail(&result, "OCI_IPV6_SMOKE_FAILED", err.Error())
	}

	if *cleanupFlag {
		cleanupResources(ctx, cfg, clients, &result)
	} else {
		result.CleanupVerified = true
		result.Warnings = append(result.Warnings, "cleanup disabled; test resources were left in OCI")
	}
}

func resolveConfig(profileID string, region string) (oci.ReadinessConfig, domain.Profile, error) {
	loaded := config.Load()
	readiness := oci.ReadinessConfig{
		ExecutionMode:  string(loaded.ExecutionMode),
		TenancyOCID:    loaded.OCI.TenancyOCID,
		UserOCID:       loaded.OCI.UserOCID,
		Fingerprint:    loaded.OCI.Fingerprint,
		PrivateKey:     loaded.OCI.PrivateKey,
		PrivateKeyFile: firstNonEmpty(loaded.OCI.PrivateKeyFile, os.Getenv("OCI_PRIVATE_KEY_PATH")),
		Region:         loaded.OCI.Region,
	}

	appStore := store.New()
	if path := strings.TrimSpace(loaded.Security.ProfileStoreFile); path != "" {
		sink, err := fileprofile.New(path)
		if err != nil {
			return oci.ReadinessConfig{}, domain.Profile{}, err
		}
		if err := sink.SetProfileKeyEncryptionKey(loaded.Security.ProfileKeyEncryptionKey); err != nil {
			return oci.ReadinessConfig{}, domain.Profile{}, err
		}
		profiles, err := sink.ListProfiles()
		if err != nil {
			return oci.ReadinessConfig{}, domain.Profile{}, err
		}
		appStore.ReplaceProfiles(profiles)
		appStore.SetPersistenceSink(sink)
		if strings.TrimSpace(profileID) == "" {
			for _, profile := range profiles {
				if !strings.EqualFold(profile.Status, "Disabled") {
					profileID = profile.ID
					break
				}
			}
		}
	}

	return profileconfig.NewResolver(appStore, readiness).Resolve(profileID, region)
}

func createAndVerify(ctx context.Context, cfg oci.ReadinessConfig, clients oci.Clients, result *smokeResult, allowFallback bool, useNSG bool, useReservedIP bool) error {
	result.Phase = "create_network"
	network, err := createIPv4OnlyNetwork(ctx, clients, result.CompartmentID, result.DisplayName)
	if err != nil {
		fail(result, "OCI_CREATE_TEST_NETWORK_FAILED", err.Error())
		return err
	}
	result.VCNID = network.VCNID
	result.SubnetID = network.SubnetID
	result.RouteTableID = network.RouteTableID
	result.SecurityListID = network.SecurityListID
	result.InternetGatewayID = network.InternetGatewayID

	if useReservedIP {
		result.Phase = "create_reserved_public_ip"
		publicIP, err := createReservedPublicIP(ctx, clients, result.CompartmentID, result.DisplayName)
		if err != nil {
			fail(result, "OCI_CREATE_RESERVED_PUBLIC_IP_FAILED", err.Error())
			return err
		}
		result.ReservedPublicIPID = stringValue(publicIP.Id)
		result.ReservedPublicIPv4 = stringValue(publicIP.IpAddress)
	}

	result.Phase = "launch_instance"
	launch := oci.LaunchInstanceFromRequest(ctx, cfg, domain.CreateInstanceRequest{
		Name:             result.DisplayName,
		CompartmentID:    result.CompartmentID,
		SubnetID:         result.SubnetID,
		Shape:            result.Shape,
		OCPUs:            result.OCPUs,
		MemoryGB:         result.MemoryGB,
		BootVolumeGB:     result.BootVolumeGB,
		AssignPublicIP:   true,
		ReservedPublicIP: result.ReservedPublicIPID,
		Tags: map[string]string{
			"managedBy": "codex",
			"purpose":   "ipv6-orchestration-smoke",
		},
	}, "ipv6-smoke-"+time.Now().UTC().Format("20060102150405"))
	result.InstanceID = launch.InstanceID
	if launch.ReservedPublicIPID != "" {
		result.ReservedPublicIPID = launch.ReservedPublicIPID
	}
	if launch.PublicIPv4 != "" {
		result.ReservedPublicIPv4 = launch.PublicIPv4
		result.ReservedPublicIPUsed = useReservedIP
	}
	if !launch.Verified {
		fail(result, firstNonEmpty(launch.ErrorCode, "OCI_LAUNCH_INSTANCE_FAILED"), launch.ErrorMessage)
		return fmt.Errorf("%s: %s", result.ErrorCode, result.ErrorMessage)
	}

	result.Phase = "capture_initial_ip"
	vnic, err := primaryVNIC(ctx, clients, result.CompartmentID, result.InstanceID)
	if err != nil {
		fail(result, "OCI_RESOLVE_PRIMARY_VNIC_FAILED", err.Error())
		return err
	}
	result.VNICID = stringValue(vnic.Id)
	result.InitialPublicIPv4 = stringValue(vnic.PublicIp)
	if useNSG {
		result.Phase = "bind_test_nsg"
		nsgID, err := createAndBindNSG(ctx, clients, result.CompartmentID, result.VCNID, result.VNICID, result.DisplayName)
		if err != nil {
			fail(result, "OCI_BIND_TEST_NSG_FAILED", err.Error())
			return err
		}
		result.NSGID = nsgID
		result.NSGBound = true
		vnic, err = primaryVNIC(ctx, clients, result.CompartmentID, result.InstanceID)
		if err != nil {
			fail(result, "OCI_REFRESH_NSG_VNIC_FAILED", err.Error())
			return err
		}
	}
	if result.InitialPublicIPv4 == "" {
		fail(result, "OCI_PUBLIC_IPV4_MISSING", "launched test instance did not receive an ephemeral public IPv4")
		return fmt.Errorf("%s", result.ErrorMessage)
	}

	active := executeIPv6SmokeMode(ctx, cfg, result, allowFallback)

	result.IPv6Address = active.IPv6Address
	result.VCNIPv6CIDR = active.VCNIPv6CIDR
	result.SubnetIPv6CIDR = active.SubnetIPv6CIDR
	result.NSGRulesChanged = active.NSGsChanged
	if !active.Verified {
		fail(result, firstNonEmpty(active.ErrorCode, "OCI_IPV6_ORCHESTRATION_FAILED"), active.ErrorMessage)
		return fmt.Errorf("%s: %s", result.ErrorCode, result.ErrorMessage)
	}

	result.Phase = "capture_final_ip"
	vnic, err = primaryVNIC(ctx, clients, result.CompartmentID, result.InstanceID)
	if err != nil {
		fail(result, "OCI_RESOLVE_FINAL_VNIC_FAILED", err.Error())
		return err
	}
	result.FinalPublicIPv4 = stringValue(vnic.PublicIp)
	result.IPv4Preserved = result.InitialPublicIPv4 != "" && result.InitialPublicIPv4 == result.FinalPublicIPv4
	result.IPv4Changed = result.InitialPublicIPv4 != "" && result.FinalPublicIPv4 != "" && result.InitialPublicIPv4 != result.FinalPublicIPv4
	if result.FinalPublicIPv4 == "" {
		fail(result, "OCI_FINAL_PUBLIC_IPV4_MISSING", "test instance no longer has a public IPv4 after IPv6 orchestration")
		return fmt.Errorf("%s", result.ErrorMessage)
	}
	result.Verified = true
	result.Phase = "verified"
	return nil
}

func executeIPv6SmokeMode(ctx context.Context, cfg oci.ReadinessConfig, result *smokeResult, allowFallback bool) oci.IPManagementExecutionResult {
	mode := normalizeSmokeMode(result.Mode)
	result.Mode = mode
	if mode != "auto" {
		result.Phase = "ipv6_" + mode
		active := executeSingleIPv6Mode(ctx, cfg, result, mode)
		result.IPv6Result = &active
		if mode == "additive" {
			result.Additive = &active
		} else {
			result.Fallback = &active
		}
		return active
	}

	result.Phase = "additive_ipv6"
	additive := executeSingleIPv6Mode(ctx, cfg, result, "additive")
	result.Additive = &additive
	result.IPv6Result = &additive
	active := additive

	if !active.Verified && allowFallback {
		result.Phase = "fallback_clone_route_table"
		fallback := executeSingleIPv6Mode(ctx, cfg, result, "clone_route_table")
		result.Fallback = &fallback
		result.IPv6Result = &fallback
		active = fallback
	}

	if !active.Verified && allowFallback {
		result.Phase = "fallback_replace_public_path"
		fallback := executeSingleIPv6Mode(ctx, cfg, result, "replace_public_path")
		result.Fallback = &fallback
		result.IPv6Result = &fallback
		active = fallback
	}
	return active
}

func executeSingleIPv6Mode(ctx context.Context, cfg oci.ReadinessConfig, result *smokeResult, mode string) oci.IPManagementExecutionResult {
	mode = normalizeSmokeMode(mode)
	if mode == "auto" {
		mode = "additive"
	}
	routeTableMode := "merge_existing"
	if mode == "clone_route_table" {
		routeTableMode = "clone"
	}
	return oci.ExecuteIPManagement(ctx, cfg, oci.IPManagementExecutionRequest{
		InstanceID:               result.InstanceID,
		VNICID:                   result.VNICID,
		EnableIPv6:               true,
		AutoConfigureIPv6:        true,
		IPv6Strategy:             mode,
		NetworkChangeMode:        mode,
		RouteTableMode:           routeTableMode,
		SecurityMode:             "append",
		AllowIrreversibleVCNIPv6: true,
		AllowPublicIPv4Change:    mode == "replace_public_path",
		OpenSSHIPv6:              true,
		JobID:                    "ipv6-smoke-" + strings.ReplaceAll(mode, "_", "-") + "-" + time.Now().UTC().Format("20060102150405"),
	})
}

func normalizeSmokeMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "auto":
		return "auto"
	case "additive", "replace_gateway":
		return "additive"
	case "clone", "clone_route_table":
		return "clone_route_table"
	case "replace", "replace_public_path":
		return "replace_public_path"
	default:
		return "auto"
	}
}

func createReservedPublicIP(ctx context.Context, clients oci.Clients, compartmentID string, displayName string) (core.PublicIp, error) {
	response, err := clients.VirtualNetwork.CreatePublicIp(ctx, core.CreatePublicIpRequest{
		CreatePublicIpDetails: core.CreatePublicIpDetails{
			CompartmentId: common.String(compartmentID),
			Lifetime:      core.CreatePublicIpDetailsLifetimeReserved,
			DisplayName:   common.String(displayName + "-reserved-ip"),
			FreeformTags: map[string]string{
				"managedBy": "codex",
				"purpose":   "ipv6-orchestration-smoke",
			},
		},
		OpcRetryToken: retryTokenLike("reserved-public-ip", displayName),
	})
	return response.PublicIp, err
}

func createAndBindNSG(ctx context.Context, clients oci.Clients, compartmentID string, vcnID string, vnicID string, displayName string) (string, error) {
	create, err := clients.VirtualNetwork.CreateNetworkSecurityGroup(ctx, core.CreateNetworkSecurityGroupRequest{
		CreateNetworkSecurityGroupDetails: core.CreateNetworkSecurityGroupDetails{
			CompartmentId: common.String(compartmentID),
			VcnId:         common.String(vcnID),
			DisplayName:   common.String(displayName + "-nsg"),
			FreeformTags: map[string]string{
				"managedBy": "codex",
				"purpose":   "ipv6-orchestration-smoke",
			},
		},
		OpcRetryToken: retryTokenLike("nsg", displayName),
	})
	if err != nil {
		return "", err
	}
	nsgID := stringValue(create.NetworkSecurityGroup.Id)
	if nsgID == "" {
		return "", fmt.Errorf("CreateNetworkSecurityGroup succeeded without an NSG OCID")
	}
	if err := waitNSG(ctx, clients, nsgID, core.NetworkSecurityGroupLifecycleStateAvailable); err != nil {
		return nsgID, err
	}

	current, err := clients.VirtualNetwork.GetVnic(ctx, core.GetVnicRequest{VnicId: common.String(vnicID)})
	if err != nil {
		return nsgID, err
	}
	nsgIDs := append([]string{}, current.Vnic.NsgIds...)
	if !containsString(nsgIDs, nsgID) {
		nsgIDs = append(nsgIDs, nsgID)
	}
	_, err = clients.VirtualNetwork.UpdateVnic(ctx, core.UpdateVnicRequest{
		VnicId: common.String(vnicID),
		UpdateVnicDetails: core.UpdateVnicDetails{
			NsgIds: nsgIDs,
		},
	})
	if err != nil {
		return nsgID, err
	}
	return nsgID, waitVNICNSG(ctx, clients, vnicID, nsgID)
}

func createIPv4OnlyNetwork(ctx context.Context, clients oci.Clients, compartmentID string, displayName string) (createdNetwork, error) {
	hexPart := strconv.FormatInt(time.Now().UTC().UnixNano()&0xfffffff, 16)
	ipv4Second := 80 + int(time.Now().UTC().UnixNano()%40)
	ipv4CIDR := fmt.Sprintf("10.%d.0.0/16", ipv4Second)
	subnetCIDR := fmt.Sprintf("10.%d.1.0/24", ipv4Second)
	tags := map[string]string{"managedBy": "codex", "purpose": "ipv6-orchestration-smoke"}
	dnsSuffix := safeDNSLabel(hexPart)

	vcnResp, err := clients.VirtualNetwork.CreateVcn(ctx, core.CreateVcnRequest{
		CreateVcnDetails: core.CreateVcnDetails{
			CompartmentId: common.String(compartmentID),
			CidrBlocks:    []string{ipv4CIDR},
			DisplayName:   common.String(displayName + "-vcn"),
			DnsLabel:      common.String("c" + dnsSuffix),
			FreeformTags:  tags,
		},
	})
	if err != nil {
		return createdNetwork{}, err
	}
	out := createdNetwork{VCNID: stringValue(vcnResp.Vcn.Id), IPv4CIDR: ipv4CIDR, SubnetIPv4CIDR: subnetCIDR}
	if err := waitVCN(ctx, clients, out.VCNID, core.VcnLifecycleStateAvailable); err != nil {
		return out, err
	}

	igwResp, err := clients.VirtualNetwork.CreateInternetGateway(ctx, core.CreateInternetGatewayRequest{
		CreateInternetGatewayDetails: core.CreateInternetGatewayDetails{
			CompartmentId: common.String(compartmentID),
			VcnId:         common.String(out.VCNID),
			IsEnabled:     common.Bool(true),
			DisplayName:   common.String(displayName + "-igw"),
			FreeformTags:  tags,
		},
	})
	out.InternetGatewayID = stringValue(igwResp.InternetGateway.Id)
	if err != nil {
		return out, err
	}

	rtResp, err := clients.VirtualNetwork.CreateRouteTable(ctx, core.CreateRouteTableRequest{
		CreateRouteTableDetails: core.CreateRouteTableDetails{
			CompartmentId: common.String(compartmentID),
			VcnId:         common.String(out.VCNID),
			DisplayName:   common.String(displayName + "-rt"),
			RouteRules: []core.RouteRule{{
				NetworkEntityId: common.String(out.InternetGatewayID),
				Destination:     common.String("0.0.0.0/0"),
				DestinationType: core.RouteRuleDestinationTypeCidrBlock,
				Description:     common.String("IPv4 default route for smoke test"),
			}},
			FreeformTags: tags,
		},
	})
	out.RouteTableID = stringValue(rtResp.RouteTable.Id)
	if err != nil {
		return out, err
	}

	slResp, err := clients.VirtualNetwork.CreateSecurityList(ctx, core.CreateSecurityListRequest{
		CreateSecurityListDetails: core.CreateSecurityListDetails{
			CompartmentId: common.String(compartmentID),
			VcnId:         common.String(out.VCNID),
			DisplayName:   common.String(displayName + "-sl"),
			EgressSecurityRules: []core.EgressSecurityRule{{
				Destination:     common.String("0.0.0.0/0"),
				DestinationType: core.EgressSecurityRuleDestinationTypeCidrBlock,
				Protocol:        common.String("all"),
				Description:     common.String("allow IPv4 egress for smoke test"),
			}},
			IngressSecurityRules: []core.IngressSecurityRule{{
				Source:      common.String("0.0.0.0/0"),
				SourceType:  core.IngressSecurityRuleSourceTypeCidrBlock,
				Protocol:    common.String("6"),
				Description: common.String("allow SSH ingress for smoke test"),
				TcpOptions: &core.TcpOptions{
					DestinationPortRange: &core.PortRange{Min: common.Int(22), Max: common.Int(22)},
				},
			}},
			FreeformTags: tags,
		},
	})
	out.SecurityListID = stringValue(slResp.SecurityList.Id)
	if err != nil {
		return out, err
	}

	subnetResp, err := clients.VirtualNetwork.CreateSubnet(ctx, core.CreateSubnetRequest{
		CreateSubnetDetails: core.CreateSubnetDetails{
			CompartmentId:          common.String(compartmentID),
			VcnId:                  common.String(out.VCNID),
			CidrBlock:              common.String(subnetCIDR),
			DisplayName:            common.String(displayName + "-subnet"),
			DnsLabel:               common.String("s" + dnsSuffix),
			ProhibitPublicIpOnVnic: common.Bool(false),
			RouteTableId:           common.String(out.RouteTableID),
			SecurityListIds:        []string{out.SecurityListID},
			FreeformTags:           tags,
		},
	})
	out.SubnetID = stringValue(subnetResp.Subnet.Id)
	if err != nil {
		return out, err
	}
	if err := waitSubnet(ctx, clients, out.SubnetID, core.SubnetLifecycleStateAvailable); err != nil {
		return out, err
	}
	return out, nil
}

func primaryVNIC(ctx context.Context, clients oci.Clients, compartmentID string, instanceID string) (core.Vnic, error) {
	resp, err := clients.Compute.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
		CompartmentId: common.String(compartmentID),
		InstanceId:    common.String(instanceID),
		Limit:         common.Int(50),
	})
	if err != nil {
		return core.Vnic{}, err
	}
	var fallback core.Vnic
	for _, attachment := range resp.Items {
		if attachment.VnicId == nil || *attachment.VnicId == "" {
			continue
		}
		if attachment.LifecycleState == core.VnicAttachmentLifecycleStateDetached || attachment.LifecycleState == core.VnicAttachmentLifecycleStateDetaching {
			continue
		}
		vnicResp, err := clients.VirtualNetwork.GetVnic(ctx, core.GetVnicRequest{VnicId: attachment.VnicId})
		if err != nil {
			return core.Vnic{}, err
		}
		if fallback.Id == nil {
			fallback = vnicResp.Vnic
		}
		if vnicResp.Vnic.IsPrimary == nil || *vnicResp.Vnic.IsPrimary {
			return vnicResp.Vnic, nil
		}
	}
	if fallback.Id != nil {
		return fallback, nil
	}
	return core.Vnic{}, fmt.Errorf("no VNIC found for instance %s", instanceID)
}

func cleanupResources(ctx context.Context, cfg oci.ReadinessConfig, clients oci.Clients, result *smokeResult) {
	ok := true
	if result.InstanceID != "" {
		step := cleanupStep{Name: "terminate_instance", ResourceID: result.InstanceID}
		action := oci.ExecuteInstanceLifecycleAction(ctx, cfg, oci.InstanceActionExecutionRequest{
			InstanceID:         result.InstanceID,
			Action:             domain.InstanceActionTerminate,
			PreserveBootVolume: false,
			TargetBootVolumeGB: 0,
			ExpandBootVolume:   false,
			Graceful:           false,
			TargetOCPUs:        0,
			TargetMemoryGB:     0,
			TargetShape:        "",
		})
		step.Verified = action.Verified
		step.ErrorCode = action.ErrorCode
		step.ErrorMessage = action.ErrorMessage
		result.Cleanup = append(result.Cleanup, step)
		ok = ok && step.Verified
	}

	steps := []struct {
		name string
		id   string
		fn   func(context.Context) error
	}{
		{"delete_reserved_public_ip", result.ReservedPublicIPID, func(ctx context.Context) error {
			_, err := clients.VirtualNetwork.DeletePublicIp(ctx, core.DeletePublicIpRequest{PublicIpId: common.String(result.ReservedPublicIPID)})
			return err
		}},
		{"delete_nsg", result.NSGID, func(ctx context.Context) error {
			_, err := clients.VirtualNetwork.DeleteNetworkSecurityGroup(ctx, core.DeleteNetworkSecurityGroupRequest{NetworkSecurityGroupId: common.String(result.NSGID)})
			if err != nil {
				return err
			}
			return waitNSG(ctx, clients, result.NSGID, core.NetworkSecurityGroupLifecycleStateTerminated)
		}},
		{"delete_subnet", result.SubnetID, func(ctx context.Context) error {
			_, err := clients.VirtualNetwork.DeleteSubnet(ctx, core.DeleteSubnetRequest{SubnetId: common.String(result.SubnetID)})
			if err != nil {
				return err
			}
			return waitSubnet(ctx, clients, result.SubnetID, core.SubnetLifecycleStateTerminated)
		}},
		{"delete_created_route_table", createdRouteTableID(result), func(ctx context.Context) error {
			_, err := clients.VirtualNetwork.DeleteRouteTable(ctx, core.DeleteRouteTableRequest{RtId: common.String(createdRouteTableID(result))})
			return err
		}},
		{"delete_route_table", result.RouteTableID, func(ctx context.Context) error {
			_, err := clients.VirtualNetwork.DeleteRouteTable(ctx, core.DeleteRouteTableRequest{RtId: common.String(result.RouteTableID)})
			return err
		}},
		{"delete_security_list", result.SecurityListID, func(ctx context.Context) error {
			_, err := clients.VirtualNetwork.DeleteSecurityList(ctx, core.DeleteSecurityListRequest{SecurityListId: common.String(result.SecurityListID)})
			return err
		}},
		{"delete_internet_gateway", result.InternetGatewayID, func(ctx context.Context) error {
			_, err := clients.VirtualNetwork.DeleteInternetGateway(ctx, core.DeleteInternetGatewayRequest{IgId: common.String(result.InternetGatewayID)})
			return err
		}},
		{"delete_vcn", result.VCNID, func(ctx context.Context) error {
			_, err := clients.VirtualNetwork.DeleteVcn(ctx, core.DeleteVcnRequest{VcnId: common.String(result.VCNID)})
			if err != nil {
				return err
			}
			return waitVCN(ctx, clients, result.VCNID, core.VcnLifecycleStateTerminated)
		}},
	}

	for _, item := range steps {
		if strings.TrimSpace(item.id) == "" {
			continue
		}
		step := cleanupStep{Name: item.name, ResourceID: item.id}
		err := retryCleanup(ctx, item.fn)
		if err != nil {
			step.ErrorCode = "OCI_CLEANUP_FAILED"
			step.ErrorMessage = err.Error()
			ok = false
		} else {
			step.Verified = true
		}
		result.Cleanup = append(result.Cleanup, step)
	}
	result.CleanupVerified = ok
}

func createdRouteTableID(result *smokeResult) string {
	if result.Additive != nil && strings.TrimSpace(result.Additive.CreatedRouteTableID) != "" {
		return result.Additive.CreatedRouteTableID
	}
	if result.Fallback != nil && strings.TrimSpace(result.Fallback.CreatedRouteTableID) != "" {
		return result.Fallback.CreatedRouteTableID
	}
	return ""
}

func retryCleanup(ctx context.Context, fn func(context.Context) error) error {
	var last error
	for attempt := 0; attempt < 24; attempt++ {
		if err := fn(ctx); err != nil {
			if isNotFound(err) {
				return nil
			}
			last = err
		} else {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
	return last
}

func waitVCN(ctx context.Context, clients oci.Clients, id string, target core.VcnLifecycleStateEnum) error {
	deadline := time.Now().Add(8 * time.Minute)
	for {
		resp, err := clients.VirtualNetwork.GetVcn(ctx, core.GetVcnRequest{VcnId: common.String(id)})
		if err != nil {
			if target == core.VcnLifecycleStateTerminated && isNotFound(err) {
				return nil
			}
			return err
		}
		if resp.Vcn.LifecycleState == target {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for VCN %s; last state=%s", id, resp.Vcn.LifecycleState)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func waitSubnet(ctx context.Context, clients oci.Clients, id string, target core.SubnetLifecycleStateEnum) error {
	deadline := time.Now().Add(8 * time.Minute)
	for {
		resp, err := clients.VirtualNetwork.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: common.String(id)})
		if err != nil {
			if target == core.SubnetLifecycleStateTerminated && isNotFound(err) {
				return nil
			}
			return err
		}
		if resp.Subnet.LifecycleState == target {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for subnet %s; last state=%s", id, resp.Subnet.LifecycleState)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func waitNSG(ctx context.Context, clients oci.Clients, id string, target core.NetworkSecurityGroupLifecycleStateEnum) error {
	deadline := time.Now().Add(8 * time.Minute)
	for {
		resp, err := clients.VirtualNetwork.GetNetworkSecurityGroup(ctx, core.GetNetworkSecurityGroupRequest{NetworkSecurityGroupId: common.String(id)})
		if err != nil {
			if target == core.NetworkSecurityGroupLifecycleStateTerminated && isNotFound(err) {
				return nil
			}
			return err
		}
		if resp.NetworkSecurityGroup.LifecycleState == target {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for NSG %s; last state=%s", id, resp.NetworkSecurityGroup.LifecycleState)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func waitVNICNSG(ctx context.Context, clients oci.Clients, vnicID string, nsgID string) error {
	deadline := time.Now().Add(5 * time.Minute)
	for {
		resp, err := clients.VirtualNetwork.GetVnic(ctx, core.GetVnicRequest{VnicId: common.String(vnicID)})
		if err != nil {
			return err
		}
		if containsString(resp.Vnic.NsgIds, nsgID) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for VNIC %s to attach NSG %s", vnicID, nsgID)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func fail(result *smokeResult, code string, message string) {
	result.ErrorCode = strings.TrimSpace(code)
	result.ErrorMessage = strings.TrimSpace(message)
}

func writeJSON(result smokeResult) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(result)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "notfound") || strings.Contains(text, "not authorized or not found") || strings.Contains(text, "404")
}

func safeDNSLabel(value string) string {
	value = strings.ToLower(value)
	var out strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
		}
	}
	label := out.String()
	if label == "" {
		label = "smoke"
	}
	if label[0] < 'a' || label[0] > 'z' {
		label = "a" + label
	}
	if len(label) > 12 {
		label = label[:12]
	}
	return label
}

func containsString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func retryTokenLike(prefix string, value string) *string {
	token := strings.ToLower(prefix + "-" + value)
	var out strings.Builder
	for _, r := range token {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			out.WriteRune(r)
		}
	}
	text := out.String()
	if len(text) > 64 {
		text = text[:64]
	}
	if text == "" {
		text = prefix + "-" + time.Now().UTC().Format("20060102150405")
	}
	return common.String(text)
}
