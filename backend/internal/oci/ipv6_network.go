package oci

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

const (
	ipv6ModeAssignOnly        = "assign_only"
	ipv6ModeAdditive          = "additive"
	ipv6ModeCloneRouteTable   = "clone_route_table"
	ipv6ModeReplacePublicPath = "replace_public_path"
)

type IPv6NetworkStep struct {
	Name          string `json:"name"`
	Status        string `json:"status"`
	ResourceID    string `json:"resourceId,omitempty"`
	RequestID     string `json:"requestId,omitempty"`
	WorkRequestID string `json:"workRequestId,omitempty"`
	Message       string `json:"message,omitempty"`
}

func ensureIPv6Network(ctx context.Context, clients Clients, cfg ReadinessConfig, req IPManagementExecutionRequest, result *IPManagementExecutionResult, instance core.Instance, vnic core.Vnic, subnet core.Subnet) (core.Subnet, bool) {
	mode := normalizedIPv6NetworkMode(req)
	result.NetworkChangeMode = mode
	result.RouteTableMode = defaultString(req.RouteTableMode, routeTableModeFromNetworkMode(mode))
	result.SecurityMode = defaultString(req.SecurityMode, "append")
	result.PublicIPv4Changed = false

	if mode == ipv6ModeAssignOnly {
		return subnet, true
	}
	if mode == ipv6ModeReplacePublicPath && !req.AllowPublicIPv4Change {
		failIPv6Network(result, "OCI_IPV6_PUBLIC_IPV4_CHANGE_CONFIRMATION_REQUIRED", "replace_public_path may change the current ephemeral IPv4 public IP; confirm allowPublicIpv4Change or choose additive mode")
		return subnet, false
	}

	vcnID := strings.TrimSpace(stringValue(subnet.VcnId))
	if vcnID == "" {
		failIPv6Network(result, "OCI_IPV6_VCN_ID_MISSING", "subnet does not include a VCN OCID")
		return subnet, false
	}
	result.VCNID = vcnID

	compartmentID := strings.TrimSpace(stringValue(subnet.CompartmentId))
	if compartmentID == "" {
		compartmentID = strings.TrimSpace(stringValue(instance.CompartmentId))
	}
	if compartmentID == "" {
		compartmentID = cfg.TenancyOCID
	}

	vcnResponse, err := clients.VirtualNetwork.GetVcn(ctx, core.GetVcnRequest{
		VcnId:        common.String(vcnID),
		OpcRequestId: requestID("codex-ipv6-get-vcn", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, vcnResponse.OpcRequestId)
	if err != nil {
		failIPv6Network(result, "OCI_GET_VCN_FAILED", err.Error())
		return subnet, false
	}

	vcnIPv6CIDR := firstString(vcnResponse.Vcn.Ipv6CidrBlocks)
	if vcnIPv6CIDR == "" {
		if !req.AllowIrreversibleVCNIPv6 {
			failIPv6Network(result, "OCI_IPV6_VCN_CIDR_CONFIRMATION_REQUIRED", "adding an Oracle GUA IPv6 CIDR to a VCN is irreversible; confirm allowIrreversibleVcnIpv6 before applying")
			return subnet, false
		}
		addStep(result, IPv6NetworkStep{Name: "ADD_VCN_IPV6_CIDR", Status: "RUNNING", ResourceID: vcnID, Message: "requesting Oracle-assigned GUA IPv6 /56 for VCN"})
		response, err := clients.VirtualNetwork.AddIpv6VcnCidr(ctx, core.AddIpv6VcnCidrRequest{
			VcnId: common.String(vcnID),
			AddVcnIpv6CidrDetails: core.AddVcnIpv6CidrDetails{
				IsOracleGuaAllocationEnabled: common.Bool(true),
			},
			OpcRetryToken: retryToken("vcn-ipv6", req.JobID),
			OpcRequestId:  requestID("codex-add-vcn-ipv6", req.JobID),
		})
		appendFirstRequestID(&result.RequestID, response.OpcRequestId)
		appendFirstString(&result.WorkRequestID, stringValue(response.OpcWorkRequestId))
		result.WorkRequestIDs = appendNonEmpty(result.WorkRequestIDs, stringValue(response.OpcWorkRequestId))
		if err != nil {
			failIPv6Network(result, "OCI_ADD_VCN_IPV6_CIDR_FAILED", err.Error())
			return subnet, false
		}
		result.IrreversibleChanges = append(result.IrreversibleChanges, "VCN Oracle GUA IPv6 CIDR added; OCI treats this as non-removable/non-modifiable for this flow")
		markStepDone(result, "ADD_VCN_IPV6_CIDR", stringValue(response.OpcRequestId), stringValue(response.OpcWorkRequestId), "VCN IPv6 CIDR request accepted")

		vcnResponse.Vcn, err = waitVCNIPv6CIDR(ctx, clients, vcnID, 12*time.Minute)
		if err != nil {
			failIPv6Network(result, "OCI_WAIT_VCN_IPV6_CIDR_FAILED", err.Error())
			return subnet, false
		}
		vcnIPv6CIDR = firstString(vcnResponse.Vcn.Ipv6CidrBlocks)
	}
	result.VCNIPv6CIDR = vcnIPv6CIDR

	subnetIPv6CIDR := firstSubnetIPv6CIDR(subnet)
	if subnetIPv6CIDR == "" {
		candidate, err := selectSubnetIPv6CIDR(ctx, clients, compartmentID, vcnID, vcnIPv6CIDR)
		if err != nil {
			failIPv6Network(result, "OCI_SELECT_SUBNET_IPV6_CIDR_FAILED", err.Error())
			return subnet, false
		}
		addStep(result, IPv6NetworkStep{Name: "ADD_SUBNET_IPV6_CIDR", Status: "RUNNING", ResourceID: stringValue(subnet.Id), Message: "adding IPv6 /64 to subnet"})
		response, err := clients.VirtualNetwork.AddIpv6SubnetCidr(ctx, core.AddIpv6SubnetCidrRequest{
			SubnetId: common.String(stringValue(subnet.Id)),
			AddSubnetIpv6CidrDetails: core.AddSubnetIpv6CidrDetails{
				Ipv6CidrBlock: common.String(candidate),
			},
			OpcRetryToken: retryToken("subnet-ipv6", req.JobID),
			OpcRequestId:  requestID("codex-add-subnet-ipv6", req.JobID),
		})
		appendFirstRequestID(&result.RequestID, response.OpcRequestId)
		appendFirstString(&result.WorkRequestID, stringValue(response.OpcWorkRequestId))
		result.WorkRequestIDs = appendNonEmpty(result.WorkRequestIDs, stringValue(response.OpcWorkRequestId))
		if err != nil {
			failIPv6Network(result, "OCI_ADD_SUBNET_IPV6_CIDR_FAILED", err.Error())
			return subnet, false
		}
		markStepDone(result, "ADD_SUBNET_IPV6_CIDR", stringValue(response.OpcRequestId), stringValue(response.OpcWorkRequestId), "Subnet IPv6 CIDR request accepted")

		subnet, err = waitSubnetIPv6CIDR(ctx, clients, stringValue(subnet.Id), 12*time.Minute)
		if err != nil {
			failIPv6Network(result, "OCI_WAIT_SUBNET_IPV6_CIDR_FAILED", err.Error())
			return subnet, false
		}
		subnetIPv6CIDR = firstSubnetIPv6CIDR(subnet)
	}
	result.SubnetIPv6CIDR = subnetIPv6CIDR
	if subnet.ProhibitInternetIngress != nil && *subnet.ProhibitInternetIngress {
		result.Warnings = append(result.Warnings, "当前 Subnet 禁止互联网入站；IPv6 地址可分配，但公网入站连通性可能不可用")
	}

	igw, created, ok := ensureInternetGateway(ctx, clients, req, result, compartmentID, vcnID)
	if !ok {
		return subnet, false
	}
	result.InternetGatewayID = stringValue(igw.Id)
	result.CreatedInternetGateway = created

	routeTableID := strings.TrimSpace(stringValue(vnic.RouteTableId))
	if routeTableID == "" {
		routeTableID = strings.TrimSpace(stringValue(subnet.RouteTableId))
	}
	if routeTableID == "" {
		failIPv6Network(result, "OCI_ROUTE_TABLE_ID_MISSING", "subnet/VNIC does not include a route table OCID")
		return subnet, false
	}

	subnet, ok = ensureIPv6Route(ctx, clients, req, result, subnet, compartmentID, vcnID, routeTableID, result.InternetGatewayID)
	if !ok {
		return subnet, false
	}

	if !strings.EqualFold(result.SecurityMode, "none") {
		if ok := ensureIPv6Security(ctx, clients, req, result, subnet, vnic); !ok {
			return subnet, false
		}
	}
	return subnet, true
}

func normalizedIPv6NetworkMode(req IPManagementExecutionRequest) string {
	mode := strings.TrimSpace(req.NetworkChangeMode)
	if mode == "" {
		mode = strings.TrimSpace(req.IPv6Strategy)
	}
	switch mode {
	case "", ipv6ModeAssignOnly:
		if req.AutoConfigureIPv6 {
			return ipv6ModeAdditive
		}
		return ipv6ModeAssignOnly
	case "replace_gateway":
		return ipv6ModeAdditive
	case ipv6ModeAdditive, ipv6ModeCloneRouteTable, ipv6ModeReplacePublicPath:
		return mode
	default:
		return ipv6ModeAdditive
	}
}

func routeTableModeFromNetworkMode(mode string) string {
	if mode == ipv6ModeCloneRouteTable {
		return "clone"
	}
	return "merge_existing"
}

func ensureInternetGateway(ctx context.Context, clients Clients, req IPManagementExecutionRequest, result *IPManagementExecutionResult, compartmentID, vcnID string) (core.InternetGateway, bool, bool) {
	response, err := clients.VirtualNetwork.ListInternetGateways(ctx, core.ListInternetGatewaysRequest{
		CompartmentId:  common.String(compartmentID),
		VcnId:          common.String(vcnID),
		LifecycleState: core.InternetGatewayLifecycleStateAvailable,
		Limit:          common.Int(100),
		OpcRequestId:   requestID("codex-list-igw", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, response.OpcRequestId)
	if err != nil {
		failIPv6Network(result, "OCI_LIST_INTERNET_GATEWAYS_FAILED", err.Error())
		return core.InternetGateway{}, false, false
	}
	for _, gateway := range response.Items {
		if gateway.Id == nil || stringValue(gateway.Id) == "" {
			continue
		}
		if gateway.IsEnabled == nil || *gateway.IsEnabled {
			addStep(result, IPv6NetworkStep{Name: "ENSURE_INTERNET_GATEWAY", Status: "SUCCESS", ResourceID: stringValue(gateway.Id), Message: "reused enabled internet gateway"})
			return gateway, false, true
		}
		update, err := clients.VirtualNetwork.UpdateInternetGateway(ctx, core.UpdateInternetGatewayRequest{
			IgId: common.String(stringValue(gateway.Id)),
			UpdateInternetGatewayDetails: core.UpdateInternetGatewayDetails{
				IsEnabled: common.Bool(true),
			},
			OpcRequestId: requestID("codex-enable-igw", req.JobID),
		})
		appendFirstRequestID(&result.RequestID, update.OpcRequestId)
		if err != nil {
			failIPv6Network(result, "OCI_ENABLE_INTERNET_GATEWAY_FAILED", err.Error())
			return core.InternetGateway{}, false, false
		}
		addStep(result, IPv6NetworkStep{Name: "ENSURE_INTERNET_GATEWAY", Status: "SUCCESS", ResourceID: stringValue(update.InternetGateway.Id), RequestID: stringValue(update.OpcRequestId), Message: "enabled existing internet gateway"})
		return update.InternetGateway, false, true
	}

	addStep(result, IPv6NetworkStep{Name: "ENSURE_INTERNET_GATEWAY", Status: "RUNNING", ResourceID: vcnID, Message: "creating enabled internet gateway"})
	create, err := clients.VirtualNetwork.CreateInternetGateway(ctx, core.CreateInternetGatewayRequest{
		CreateInternetGatewayDetails: core.CreateInternetGatewayDetails{
			CompartmentId: common.String(compartmentID),
			IsEnabled:     common.Bool(true),
			VcnId:         common.String(vcnID),
			DisplayName:   common.String("codex-ipv6-igw-" + time.Now().UTC().Format("20060102-150405")),
			FreeformTags: map[string]string{
				"managedBy": "codex",
				"purpose":   "ipv6-network-orchestration",
			},
		},
		OpcRetryToken: retryToken("igw", req.JobID),
		OpcRequestId:  requestID("codex-create-igw", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, create.OpcRequestId)
	if err != nil {
		failIPv6Network(result, "OCI_CREATE_INTERNET_GATEWAY_FAILED", err.Error())
		return core.InternetGateway{}, false, false
	}
	markStepDone(result, "ENSURE_INTERNET_GATEWAY", stringValue(create.OpcRequestId), "", "created enabled internet gateway")
	return create.InternetGateway, true, true
}

func ensureIPv6Route(ctx context.Context, clients Clients, req IPManagementExecutionRequest, result *IPManagementExecutionResult, subnet core.Subnet, compartmentID, vcnID, routeTableID, igwID string) (core.Subnet, bool) {
	routeResponse, err := clients.VirtualNetwork.GetRouteTable(ctx, core.GetRouteTableRequest{
		RtId:         common.String(routeTableID),
		OpcRequestId: requestID("codex-get-route-table", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, routeResponse.OpcRequestId)
	if err != nil {
		failIPv6Network(result, "OCI_GET_ROUTE_TABLE_FAILED", err.Error())
		return subnet, false
	}
	merged, changed := appendIPv6DefaultRoute(routeResponse.RouteTable.RouteRules, igwID)
	if !changed {
		result.RouteTableID = routeTableID
		addStep(result, IPv6NetworkStep{Name: "ENSURE_IPV6_ROUTE", Status: "SUCCESS", ResourceID: routeTableID, Message: "route table already has ::/0 IPv6 default route"})
		return subnet, true
	}

	if strings.EqualFold(result.RouteTableMode, "clone") {
		create, err := clients.VirtualNetwork.CreateRouteTable(ctx, core.CreateRouteTableRequest{
			CreateRouteTableDetails: core.CreateRouteTableDetails{
				CompartmentId: common.String(compartmentID),
				VcnId:         common.String(vcnID),
				DisplayName:   common.String("codex-ipv6-rt-" + time.Now().UTC().Format("20060102-150405")),
				RouteRules:    merged,
				FreeformTags: map[string]string{
					"managedBy": "codex",
					"purpose":   "ipv6-network-orchestration",
				},
			},
			OpcRetryToken: retryToken("rt-clone", req.JobID),
			OpcRequestId:  requestID("codex-create-ipv6-route-table", req.JobID),
		})
		appendFirstRequestID(&result.RequestID, create.OpcRequestId)
		if err != nil {
			failIPv6Network(result, "OCI_CREATE_ROUTE_TABLE_FAILED", err.Error())
			return subnet, false
		}
		newRouteTableID := stringValue(create.RouteTable.Id)
		update, err := clients.VirtualNetwork.UpdateSubnet(ctx, core.UpdateSubnetRequest{
			SubnetId: common.String(stringValue(subnet.Id)),
			UpdateSubnetDetails: core.UpdateSubnetDetails{
				RouteTableId: common.String(newRouteTableID),
			},
			OpcRequestId: requestID("codex-update-subnet-route-table", req.JobID),
		})
		appendFirstRequestID(&result.RequestID, update.OpcRequestId)
		if err != nil {
			failIPv6Network(result, "OCI_UPDATE_SUBNET_ROUTE_TABLE_FAILED", err.Error())
			return subnet, false
		}
		result.RouteTableID = newRouteTableID
		result.CreatedRouteTableID = newRouteTableID
		result.RouteTableChanged = true
		addStep(result, IPv6NetworkStep{Name: "ENSURE_IPV6_ROUTE", Status: "SUCCESS", ResourceID: newRouteTableID, RequestID: stringValue(create.OpcRequestId), Message: "created cloned route table with ::/0 and attached it to subnet"})
		return update.Subnet, true
	}

	update, err := clients.VirtualNetwork.UpdateRouteTable(ctx, core.UpdateRouteTableRequest{
		RtId: common.String(routeTableID),
		UpdateRouteTableDetails: core.UpdateRouteTableDetails{
			RouteRules: merged,
		},
		IfMatch:      routeResponse.Etag,
		OpcRequestId: requestID("codex-update-route-table", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, update.OpcRequestId)
	if err != nil {
		failIPv6Network(result, "OCI_UPDATE_ROUTE_TABLE_FAILED", err.Error())
		return subnet, false
	}
	result.RouteTableID = routeTableID
	result.RouteTableChanged = true
	addStep(result, IPv6NetworkStep{Name: "ENSURE_IPV6_ROUTE", Status: "SUCCESS", ResourceID: routeTableID, RequestID: stringValue(update.OpcRequestId), Message: "merged ::/0 IPv6 default route into existing route table"})
	return subnet, true
}

func ensureIPv6Security(ctx context.Context, clients Clients, req IPManagementExecutionRequest, result *IPManagementExecutionResult, subnet core.Subnet, vnic core.Vnic) bool {
	for _, securityListID := range subnet.SecurityListIds {
		securityListID = strings.TrimSpace(securityListID)
		if securityListID == "" {
			continue
		}
		response, err := clients.VirtualNetwork.GetSecurityList(ctx, core.GetSecurityListRequest{
			SecurityListId: common.String(securityListID),
			OpcRequestId:   requestID("codex-get-security-list", req.JobID),
		})
		appendFirstRequestID(&result.RequestID, response.OpcRequestId)
		if err != nil {
			failIPv6Network(result, "OCI_GET_SECURITY_LIST_FAILED", err.Error())
			return false
		}
		egress, egressChanged := appendIPv6EgressRules(response.SecurityList.EgressSecurityRules)
		ingress, ingressChanged := appendIPv6IngressRules(response.SecurityList.IngressSecurityRules, req)
		if !egressChanged && !ingressChanged {
			continue
		}
		update, err := clients.VirtualNetwork.UpdateSecurityList(ctx, core.UpdateSecurityListRequest{
			SecurityListId: common.String(securityListID),
			UpdateSecurityListDetails: core.UpdateSecurityListDetails{
				EgressSecurityRules:  egress,
				IngressSecurityRules: ingress,
			},
			IfMatch:      response.Etag,
			OpcRequestId: requestID("codex-update-security-list", req.JobID),
		})
		appendFirstRequestID(&result.RequestID, update.OpcRequestId)
		if err != nil {
			failIPv6Network(result, "OCI_UPDATE_SECURITY_LIST_FAILED", err.Error())
			return false
		}
		result.SecurityListIDs = appendNonEmpty(result.SecurityListIDs, securityListID)
		result.SecurityListsChanged = true
		addStep(result, IPv6NetworkStep{Name: "ENSURE_SECURITY_LIST_IPV6", Status: "SUCCESS", ResourceID: securityListID, RequestID: stringValue(update.OpcRequestId), Message: "appended IPv6 security list rules"})
	}

	for _, nsgID := range vnic.NsgIds {
		nsgID = strings.TrimSpace(nsgID)
		if nsgID == "" {
			continue
		}
		if ok := ensureNSGIPv6Security(ctx, clients, req, result, nsgID); !ok {
			return false
		}
	}
	return true
}

func ensureNSGIPv6Security(ctx context.Context, clients Clients, req IPManagementExecutionRequest, result *IPManagementExecutionResult, nsgID string) bool {
	response, err := clients.VirtualNetwork.ListNetworkSecurityGroupSecurityRules(ctx, core.ListNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId: common.String(nsgID),
		Limit:                  common.Int(200),
		OpcRequestId:           requestID("codex-list-nsg-rules", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, response.OpcRequestId)
	if err != nil {
		failIPv6Network(result, "OCI_LIST_NSG_RULES_FAILED", err.Error())
		return false
	}
	additions := missingNSGIPv6Rules(response.Items, req)
	if len(additions) == 0 {
		return true
	}
	add, err := clients.VirtualNetwork.AddNetworkSecurityGroupSecurityRules(ctx, core.AddNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId: common.String(nsgID),
		AddNetworkSecurityGroupSecurityRulesDetails: core.AddNetworkSecurityGroupSecurityRulesDetails{
			SecurityRules: additions,
		},
		OpcRequestId: requestID("codex-add-nsg-ipv6-rules", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, add.OpcRequestId)
	if err != nil {
		failIPv6Network(result, "OCI_ADD_NSG_RULES_FAILED", err.Error())
		return false
	}
	result.NSGIDs = appendNonEmpty(result.NSGIDs, nsgID)
	result.NSGsChanged = true
	addStep(result, IPv6NetworkStep{Name: "ENSURE_NSG_IPV6", Status: "SUCCESS", ResourceID: nsgID, RequestID: stringValue(add.OpcRequestId), Message: "appended IPv6 NSG rules"})
	return true
}

func appendIPv6DefaultRoute(rules []core.RouteRule, igwID string) ([]core.RouteRule, bool) {
	for _, rule := range rules {
		if strings.TrimSpace(stringValue(rule.NetworkEntityId)) == igwID && strings.TrimSpace(stringValue(rule.Destination)) == "::/0" {
			return rules, false
		}
	}
	next := append([]core.RouteRule{}, rules...)
	next = append(next, core.RouteRule{
		Destination:     common.String("::/0"),
		DestinationType: core.RouteRuleDestinationTypeCidrBlock,
		NetworkEntityId: common.String(igwID),
		Description:     common.String("Managed by OCI lifecycle platform: IPv6 default route to internet gateway"),
	})
	return next, true
}

func appendIPv6EgressRules(rules []core.EgressSecurityRule) ([]core.EgressSecurityRule, bool) {
	for _, rule := range rules {
		if strings.TrimSpace(stringValue(rule.Destination)) == "::/0" && strings.TrimSpace(stringValue(rule.Protocol)) == "all" {
			return rules, false
		}
	}
	next := append([]core.EgressSecurityRule{}, rules...)
	next = append(next, core.EgressSecurityRule{
		Destination:     common.String("::/0"),
		DestinationType: core.EgressSecurityRuleDestinationTypeCidrBlock,
		Protocol:        common.String("all"),
		Description:     common.String("Managed by OCI lifecycle platform: allow IPv6 egress"),
	})
	return next, true
}

func appendIPv6IngressRules(rules []core.IngressSecurityRule, req IPManagementExecutionRequest) ([]core.IngressSecurityRule, bool) {
	next := append([]core.IngressSecurityRule{}, rules...)
	changed := false
	if !hasIngressICMPv6PacketTooBig(next) {
		typeValue := 2
		codeValue := 0
		next = append(next, core.IngressSecurityRule{
			Source:      common.String("::/0"),
			SourceType:  core.IngressSecurityRuleSourceTypeCidrBlock,
			Protocol:    common.String("58"),
			IcmpOptions: &core.IcmpOptions{Type: &typeValue, Code: &codeValue},
			Description: common.String("Managed by OCI lifecycle platform: allow ICMPv6 Packet Too Big"),
		})
		changed = true
	}
	if req.OpenSSHIPv6 && !hasIngressTCPPort(next, 22) {
		next = append(next, ipv6IngressTCPPortRule(22, "Managed by OCI lifecycle platform: allow IPv6 SSH"))
		changed = true
	}
	if req.OpenHTTPIPv6 && !hasIngressTCPPort(next, 80) {
		next = append(next, ipv6IngressTCPPortRule(80, "Managed by OCI lifecycle platform: allow IPv6 HTTP"))
		changed = true
	}
	if req.OpenHTTPSIPv6 && !hasIngressTCPPort(next, 443) {
		next = append(next, ipv6IngressTCPPortRule(443, "Managed by OCI lifecycle platform: allow IPv6 HTTPS"))
		changed = true
	}
	return next, changed
}

func ipv6IngressTCPPortRule(port int, description string) core.IngressSecurityRule {
	return core.IngressSecurityRule{
		Source:     common.String("::/0"),
		SourceType: core.IngressSecurityRuleSourceTypeCidrBlock,
		Protocol:   common.String("6"),
		TcpOptions: &core.TcpOptions{
			DestinationPortRange: &core.PortRange{Min: common.Int(port), Max: common.Int(port)},
		},
		Description: common.String(description),
	}
}

func missingNSGIPv6Rules(existing []core.SecurityRule, req IPManagementExecutionRequest) []core.AddSecurityRuleDetails {
	var additions []core.AddSecurityRuleDetails
	if !hasNSGEgressAllIPv6(existing) {
		additions = append(additions, core.AddSecurityRuleDetails{
			Direction:       core.AddSecurityRuleDetailsDirectionEgress,
			Destination:     common.String("::/0"),
			DestinationType: core.AddSecurityRuleDetailsDestinationTypeCidrBlock,
			Protocol:        common.String("all"),
			Description:     common.String("Managed by OCI lifecycle platform: allow IPv6 egress"),
		})
	}
	if !hasNSGIngressICMPv6PacketTooBig(existing) {
		typeValue := 2
		codeValue := 0
		additions = append(additions, core.AddSecurityRuleDetails{
			Direction:   core.AddSecurityRuleDetailsDirectionIngress,
			Source:      common.String("::/0"),
			SourceType:  core.AddSecurityRuleDetailsSourceTypeCidrBlock,
			Protocol:    common.String("58"),
			IcmpOptions: &core.IcmpOptions{Type: &typeValue, Code: &codeValue},
			Description: common.String("Managed by OCI lifecycle platform: allow ICMPv6 Packet Too Big"),
		})
	}
	if req.OpenSSHIPv6 && !hasNSGIngressTCPPort(existing, 22) {
		additions = append(additions, nsgIPv6IngressTCPPortRule(22, "Managed by OCI lifecycle platform: allow IPv6 SSH"))
	}
	if req.OpenHTTPIPv6 && !hasNSGIngressTCPPort(existing, 80) {
		additions = append(additions, nsgIPv6IngressTCPPortRule(80, "Managed by OCI lifecycle platform: allow IPv6 HTTP"))
	}
	if req.OpenHTTPSIPv6 && !hasNSGIngressTCPPort(existing, 443) {
		additions = append(additions, nsgIPv6IngressTCPPortRule(443, "Managed by OCI lifecycle platform: allow IPv6 HTTPS"))
	}
	return additions
}

func nsgIPv6IngressTCPPortRule(port int, description string) core.AddSecurityRuleDetails {
	return core.AddSecurityRuleDetails{
		Direction:  core.AddSecurityRuleDetailsDirectionIngress,
		Source:     common.String("::/0"),
		SourceType: core.AddSecurityRuleDetailsSourceTypeCidrBlock,
		Protocol:   common.String("6"),
		TcpOptions: &core.TcpOptions{
			DestinationPortRange: &core.PortRange{Min: common.Int(port), Max: common.Int(port)},
		},
		Description: common.String(description),
	}
}

func hasIngressICMPv6PacketTooBig(rules []core.IngressSecurityRule) bool {
	for _, rule := range rules {
		if strings.TrimSpace(stringValue(rule.Source)) != "::/0" || strings.TrimSpace(stringValue(rule.Protocol)) != "58" || rule.IcmpOptions == nil || rule.IcmpOptions.Type == nil {
			continue
		}
		if *rule.IcmpOptions.Type == 2 {
			return true
		}
	}
	return false
}

func hasIngressTCPPort(rules []core.IngressSecurityRule, port int) bool {
	for _, rule := range rules {
		if strings.TrimSpace(stringValue(rule.Source)) != "::/0" || strings.TrimSpace(stringValue(rule.Protocol)) != "6" || rule.TcpOptions == nil || rule.TcpOptions.DestinationPortRange == nil {
			continue
		}
		if intValue(rule.TcpOptions.DestinationPortRange.Min) <= port && intValue(rule.TcpOptions.DestinationPortRange.Max) >= port {
			return true
		}
	}
	return false
}

func hasNSGEgressAllIPv6(rules []core.SecurityRule) bool {
	for _, rule := range rules {
		if rule.Direction == core.SecurityRuleDirectionEgress && strings.TrimSpace(stringValue(rule.Destination)) == "::/0" && strings.TrimSpace(stringValue(rule.Protocol)) == "all" {
			return true
		}
	}
	return false
}

func hasNSGIngressICMPv6PacketTooBig(rules []core.SecurityRule) bool {
	for _, rule := range rules {
		if rule.Direction != core.SecurityRuleDirectionIngress || strings.TrimSpace(stringValue(rule.Source)) != "::/0" || strings.TrimSpace(stringValue(rule.Protocol)) != "58" || rule.IcmpOptions == nil || rule.IcmpOptions.Type == nil {
			continue
		}
		if *rule.IcmpOptions.Type == 2 {
			return true
		}
	}
	return false
}

func hasNSGIngressTCPPort(rules []core.SecurityRule, port int) bool {
	for _, rule := range rules {
		if rule.Direction != core.SecurityRuleDirectionIngress || strings.TrimSpace(stringValue(rule.Source)) != "::/0" || strings.TrimSpace(stringValue(rule.Protocol)) != "6" || rule.TcpOptions == nil || rule.TcpOptions.DestinationPortRange == nil {
			continue
		}
		if intValue(rule.TcpOptions.DestinationPortRange.Min) <= port && intValue(rule.TcpOptions.DestinationPortRange.Max) >= port {
			return true
		}
	}
	return false
}

func selectSubnetIPv6CIDR(ctx context.Context, clients Clients, compartmentID, vcnID, vcnIPv6CIDR string) (string, error) {
	used := map[string]bool{}
	page := (*string)(nil)
	for {
		response, err := clients.VirtualNetwork.ListSubnets(ctx, core.ListSubnetsRequest{
			CompartmentId:  common.String(compartmentID),
			VcnId:          common.String(vcnID),
			LifecycleState: core.SubnetLifecycleStateAvailable,
			Limit:          common.Int(100),
			Page:           page,
		})
		if err != nil {
			return "", err
		}
		for _, subnet := range response.Items {
			for _, cidr := range subnet.Ipv6CidrBlocks {
				if prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr)); err == nil {
					used[prefix.Masked().String()] = true
				}
			}
			if cidr := strings.TrimSpace(stringValue(subnet.Ipv6CidrBlock)); cidr != "" {
				if prefix, err := netip.ParsePrefix(cidr); err == nil {
					used[prefix.Masked().String()] = true
				}
			}
		}
		if response.OpcNextPage == nil || *response.OpcNextPage == "" {
			break
		}
		page = response.OpcNextPage
	}
	return firstAvailableIPv6SubnetPrefix(vcnIPv6CIDR, used)
}

func firstAvailableIPv6SubnetPrefix(vcnCIDR string, used map[string]bool) (string, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(vcnCIDR))
	if err != nil {
		return "", err
	}
	prefix = prefix.Masked()
	if !prefix.Addr().Is6() {
		return "", fmt.Errorf("VCN IPv6 CIDR must be IPv6: %s", vcnCIDR)
	}
	if prefix.Bits() > 64 {
		return "", fmt.Errorf("VCN IPv6 prefix %s is narrower than /64 and cannot be split into subnet /64s", vcnCIDR)
	}
	if prefix.Bits() < 48 {
		return "", fmt.Errorf("refusing to scan IPv6 prefix %s because it is broader than /48", vcnCIDR)
	}

	addr := prefix.Addr().As16()
	baseHigh := binary.BigEndian.Uint64(addr[0:8])
	mask := uint64(0)
	if prefix.Bits() > 0 {
		mask = ^uint64(0) << uint(64-prefix.Bits())
	}
	network := baseHigh & mask
	count := uint64(1) << uint(64-prefix.Bits())
	for i := uint64(0); i < count; i++ {
		var candidateBytes [16]byte
		binary.BigEndian.PutUint64(candidateBytes[0:8], network|i)
		candidate := netip.PrefixFrom(netip.AddrFrom16(candidateBytes), 64).Masked()
		if !used[candidate.String()] {
			return candidate.String(), nil
		}
	}
	return "", fmt.Errorf("no available /64 remains in VCN IPv6 CIDR %s", vcnCIDR)
}

func waitVCNIPv6CIDR(ctx context.Context, clients Clients, vcnID string, timeout time.Duration) (core.Vcn, error) {
	deadline := time.Now().Add(timeout)
	for {
		response, err := clients.VirtualNetwork.GetVcn(ctx, core.GetVcnRequest{VcnId: common.String(vcnID)})
		if err != nil {
			return response.Vcn, err
		}
		if len(response.Vcn.Ipv6CidrBlocks) > 0 && response.Vcn.LifecycleState == core.VcnLifecycleStateAvailable {
			return response.Vcn, nil
		}
		if time.Now().After(deadline) {
			return response.Vcn, fmt.Errorf("timed out waiting for VCN %s IPv6 CIDR; state=%s cidrs=%v", vcnID, response.Vcn.LifecycleState, response.Vcn.Ipv6CidrBlocks)
		}
		select {
		case <-ctx.Done():
			return response.Vcn, ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

func waitSubnetIPv6CIDR(ctx context.Context, clients Clients, subnetID string, timeout time.Duration) (core.Subnet, error) {
	deadline := time.Now().Add(timeout)
	for {
		response, err := clients.VirtualNetwork.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: common.String(subnetID)})
		if err != nil {
			return response.Subnet, err
		}
		if firstSubnetIPv6CIDR(response.Subnet) != "" && response.Subnet.LifecycleState == core.SubnetLifecycleStateAvailable {
			return response.Subnet, nil
		}
		if time.Now().After(deadline) {
			return response.Subnet, fmt.Errorf("timed out waiting for subnet %s IPv6 CIDR; state=%s cidrs=%v", subnetID, response.Subnet.LifecycleState, response.Subnet.Ipv6CidrBlocks)
		}
		select {
		case <-ctx.Done():
			return response.Subnet, ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

func failIPv6Network(result *IPManagementExecutionResult, code, message string) {
	result.ErrorCode = code
	result.ErrorMessage = message
	result.Verified = false
}

func addStep(result *IPManagementExecutionResult, step IPv6NetworkStep) {
	result.NetworkSteps = append(result.NetworkSteps, step)
}

func markStepDone(result *IPManagementExecutionResult, name, requestIDValue, workRequestIDValue, message string) {
	for i := len(result.NetworkSteps) - 1; i >= 0; i-- {
		if result.NetworkSteps[i].Name == name {
			result.NetworkSteps[i].Status = "SUCCESS"
			result.NetworkSteps[i].RequestID = requestIDValue
			result.NetworkSteps[i].WorkRequestID = workRequestIDValue
			result.NetworkSteps[i].Message = message
			return
		}
	}
	addStep(result, IPv6NetworkStep{Name: name, Status: "SUCCESS", RequestID: requestIDValue, WorkRequestID: workRequestIDValue, Message: message})
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func appendFirstString(target *string, value string) {
	if target == nil || *target != "" || strings.TrimSpace(value) == "" {
		return
	}
	*target = strings.TrimSpace(value)
}

func appendNonEmpty(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
