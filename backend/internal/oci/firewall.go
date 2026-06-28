package oci

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type FirewallExecutionRequest struct {
	InstanceID     string
	VNICID         string
	Action         string
	Protocol       string
	PortMin        int
	PortMax        int
	SourceCIDR     string
	TargetScope    string
	ContainerID    string
	ContainerType  string
	RuleID         string
	SnapshotBefore bool
	Note           string
	JobID          string
}

type FirewallExecutionResult struct {
	Verified            bool      `json:"verified"`
	ExecutionMode       string    `json:"executionMode"`
	InstanceID          string    `json:"instanceId"`
	VNICID              string    `json:"vnicId,omitempty"`
	SubnetID            string    `json:"subnetId,omitempty"`
	Action              string    `json:"action"`
	Protocol            string    `json:"protocol"`
	PortMin             int       `json:"portMin"`
	PortMax             int       `json:"portMax"`
	SourceCIDR          string    `json:"sourceCidr"`
	TargetScope         string    `json:"targetScope"`
	NSGIDs              []string  `json:"nsgIds,omitempty"`
	SecurityListIDs     []string  `json:"securityListIds,omitempty"`
	SnapshotBefore      bool      `json:"snapshotBefore"`
	RulesBefore         int       `json:"rulesBefore,omitempty"`
	AffectedContainers  int       `json:"affectedContainers,omitempty"`
	AddedRules          int       `json:"addedRules"`
	RemovedRules        int       `json:"removedRules"`
	Noop                bool      `json:"noop"`
	RequestID           string    `json:"requestId,omitempty"`
	ErrorCode           string    `json:"errorCode,omitempty"`
	ErrorMessage        string    `json:"errorMessage,omitempty"`
	ExecutedAt          time.Time `json:"executedAt"`
	UsedSecurityList    bool      `json:"usedSecurityList"`
	UsedNetworkSecurity bool      `json:"usedNetworkSecurity"`
}

type FirewallRulesInventory struct {
	Verified        bool           `json:"verified"`
	ExecutionMode   string         `json:"executionMode"`
	InstanceID      string         `json:"instanceId"`
	VNICID          string         `json:"vnicId,omitempty"`
	SubnetID        string         `json:"subnetId,omitempty"`
	NSGIDs          []string       `json:"nsgIds,omitempty"`
	SecurityListIDs []string       `json:"securityListIds,omitempty"`
	Rules           []FirewallRule `json:"rules"`
	RequestID       string         `json:"requestId,omitempty"`
	ErrorCode       string         `json:"errorCode,omitempty"`
	ErrorMessage    string         `json:"errorMessage,omitempty"`
	LoadedAt        time.Time      `json:"loadedAt"`
}

type FirewallRule struct {
	ID            string `json:"id"`
	ContainerID   string `json:"containerId"`
	ContainerType string `json:"containerType"`
	Protocol      string `json:"protocol"`
	PortMin       int    `json:"portMin,omitempty"`
	PortMax       int    `json:"portMax,omitempty"`
	PortLabel     string `json:"portLabel"`
	Direction     string `json:"direction"`
	Source        string `json:"source"`
	SourceType    string `json:"sourceType,omitempty"`
	Policy        string `json:"policy"`
	Status        string `json:"status"`
	Remark        string `json:"remark,omitempty"`
	Time          string `json:"time,omitempty"`
	IsBroadRule   bool   `json:"isBroadRule"`
	Editable      bool   `json:"editable"`
	DeleteMode    string `json:"deleteMode"`
}

func ListFirewallRules(ctx context.Context, cfg ReadinessConfig, instanceID string, requestedVNICID string) FirewallRulesInventory {
	result := FirewallRulesInventory{
		ExecutionMode: cfg.ExecutionMode,
		InstanceID:    strings.TrimSpace(instanceID),
		Rules:         []FirewallRule{},
		LoadedAt:      time.Now().UTC(),
	}
	readiness := CheckReadiness(cfg)
	if !readiness.Ready {
		result.ErrorCode = "OCI_NOT_READY"
		result.ErrorMessage = readiness.Message
		return result
	}
	if strings.TrimSpace(instanceID) == "" {
		result.ErrorCode = "OCI_INSTANCE_ID_REQUIRED"
		result.ErrorMessage = "OCI instance OCID is required"
		return result
	}
	clients, err := NewClients(cfg)
	if err != nil {
		result.ErrorCode = "OCI_CLIENT_INIT_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	instance, err := clients.Compute.GetInstance(ctx, core.GetInstanceRequest{
		InstanceId:   common.String(instanceID),
		OpcRequestId: requestID("codex-fw-inventory-instance", ""),
	})
	appendFirstRequestID(&result.RequestID, instance.OpcRequestId)
	if err != nil {
		result.ErrorCode = "OCI_GET_INSTANCE_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	compartmentID := defaultString(stringValue(instance.Instance.CompartmentId), cfg.TenancyOCID)
	vnic, err := resolveVNIC(ctx, clients, compartmentID, instanceID, requestedVNICID, &result.RequestID)
	if err != nil {
		result.ErrorCode = "OCI_RESOLVE_VNIC_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	result.VNICID = stringValue(vnic.Id)
	result.SubnetID = stringValue(vnic.SubnetId)
	result.NSGIDs = append(result.NSGIDs, vnic.NsgIds...)

	for _, nsgID := range result.NSGIDs {
		list, err := clients.VirtualNetwork.ListNetworkSecurityGroupSecurityRules(ctx, core.ListNetworkSecurityGroupSecurityRulesRequest{
			NetworkSecurityGroupId: common.String(nsgID),
			Limit:                  common.Int(200),
			OpcRequestId:           requestID("codex-fw-inventory-nsg", ""),
		})
		appendFirstRequestID(&result.RequestID, list.OpcRequestId)
		if err != nil {
			result.ErrorCode = "OCI_LIST_NSG_RULES_FAILED"
			result.ErrorMessage = err.Error()
			return result
		}
		for _, rule := range list.Items {
			if rule.Direction != core.SecurityRuleDirectionIngress {
				continue
			}
			result.Rules = append(result.Rules, mapNSGFirewallRule(nsgID, rule))
		}
	}

	if result.SubnetID != "" {
		subnet, err := clients.VirtualNetwork.GetSubnet(ctx, core.GetSubnetRequest{
			SubnetId:     common.String(result.SubnetID),
			OpcRequestId: requestID("codex-fw-inventory-subnet", ""),
		})
		appendFirstRequestID(&result.RequestID, subnet.OpcRequestId)
		if err != nil {
			result.ErrorCode = "OCI_GET_SUBNET_FAILED"
			result.ErrorMessage = err.Error()
			return result
		}
		result.SecurityListIDs = append(result.SecurityListIDs, subnet.Subnet.SecurityListIds...)
		for _, securityListID := range result.SecurityListIDs {
			response, err := clients.VirtualNetwork.GetSecurityList(ctx, core.GetSecurityListRequest{
				SecurityListId: common.String(securityListID),
				OpcRequestId:   requestID("codex-fw-inventory-seclist", ""),
			})
			appendFirstRequestID(&result.RequestID, response.OpcRequestId)
			if err != nil {
				result.ErrorCode = "OCI_GET_SECURITY_LIST_FAILED"
				result.ErrorMessage = err.Error()
				return result
			}
			for index, rule := range response.SecurityList.IngressSecurityRules {
				result.Rules = append(result.Rules, mapSecurityListFirewallRule(securityListID, index, rule))
			}
		}
	}
	result.Verified = true
	return result
}

func ExecuteFirewallTask(ctx context.Context, cfg ReadinessConfig, req FirewallExecutionRequest) FirewallExecutionResult {
	result := FirewallExecutionResult{
		ExecutionMode:  cfg.ExecutionMode,
		InstanceID:     req.InstanceID,
		Action:         normalizedFirewallAction(req.Action),
		Protocol:       normalizedFirewallProtocol(req.Protocol),
		PortMin:        req.PortMin,
		PortMax:        req.PortMax,
		SourceCIDR:     defaultString(req.SourceCIDR, "0.0.0.0/0"),
		TargetScope:    defaultString(strings.ToLower(strings.TrimSpace(req.TargetScope)), "auto"),
		SnapshotBefore: req.SnapshotBefore,
		ExecutedAt:     time.Now().UTC(),
	}
	if result.PortMax <= 0 {
		result.PortMax = result.PortMin
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
	if result.Action != "open" && result.Action != "close" && result.Action != "delete_broad" {
		result.ErrorCode = "OCI_FIREWALL_ACTION_INVALID"
		result.ErrorMessage = "firewall action must be open, close, or delete_broad"
		return result
	}
	clients, err := NewClients(cfg)
	if err != nil {
		result.ErrorCode = "OCI_CLIENT_INIT_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	if result.Action == "delete_broad" {
		if !executeDeleteBroadFirewallRule(ctx, clients, req, &result) {
			return result
		}
		result.Noop = result.RemovedRules == 0
		result.Verified = true
		return result
	}
	if result.Protocol != "tcp" && result.Protocol != "udp" {
		result.ErrorCode = "OCI_FIREWALL_PROTOCOL_INVALID"
		result.ErrorMessage = "firewall protocol must be tcp or udp"
		return result
	}
	if result.PortMin <= 0 || result.PortMin > 65535 || result.PortMax < result.PortMin || result.PortMax > 65535 {
		result.ErrorCode = "OCI_FIREWALL_PORT_INVALID"
		result.ErrorMessage = "firewall port range must be between 1 and 65535"
		return result
	}
	instance, err := clients.Compute.GetInstance(ctx, core.GetInstanceRequest{
		InstanceId:   common.String(req.InstanceID),
		OpcRequestId: requestID("codex-fw-get-instance", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, instance.OpcRequestId)
	if err != nil {
		result.ErrorCode = "OCI_GET_INSTANCE_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	compartmentID := defaultString(stringValue(instance.Instance.CompartmentId), cfg.TenancyOCID)
	vnic, err := resolveVNIC(ctx, clients, compartmentID, req.InstanceID, req.VNICID, &result.RequestID)
	if err != nil {
		result.ErrorCode = "OCI_RESOLVE_VNIC_FAILED"
		result.ErrorMessage = err.Error()
		return result
	}
	result.VNICID = stringValue(vnic.Id)
	result.SubnetID = stringValue(vnic.SubnetId)
	result.NSGIDs = append(result.NSGIDs, vnic.NsgIds...)

	scope := result.TargetScope
	useNSG := (scope == "auto" || scope == "nsg") && len(vnic.NsgIds) > 0
	if useNSG {
		for _, nsgID := range vnic.NsgIds {
			if !applyFirewallToNSG(ctx, clients, req, &result, strings.TrimSpace(nsgID)) {
				return result
			}
		}
		result.UsedNetworkSecurity = true
	} else if scope == "nsg" {
		result.ErrorCode = "OCI_FIREWALL_NSG_MISSING"
		result.ErrorMessage = "selected VNIC has no NSG; choose auto or security_list"
		return result
	} else {
		if result.SubnetID == "" {
			result.ErrorCode = "OCI_FIREWALL_SUBNET_MISSING"
			result.ErrorMessage = "selected VNIC does not include subnet id"
			return result
		}
		subnet, err := clients.VirtualNetwork.GetSubnet(ctx, core.GetSubnetRequest{
			SubnetId:     common.String(result.SubnetID),
			OpcRequestId: requestID("codex-fw-get-subnet", req.JobID),
		})
		appendFirstRequestID(&result.RequestID, subnet.OpcRequestId)
		if err != nil {
			result.ErrorCode = "OCI_GET_SUBNET_FAILED"
			result.ErrorMessage = err.Error()
			return result
		}
		for _, securityListID := range subnet.Subnet.SecurityListIds {
			if !applyFirewallToSecurityList(ctx, clients, req, &result, strings.TrimSpace(securityListID)) {
				return result
			}
		}
		result.UsedSecurityList = true
	}
	result.Noop = result.AddedRules == 0 && result.RemovedRules == 0
	result.Verified = true
	return result
}

func executeDeleteBroadFirewallRule(ctx context.Context, clients Clients, req FirewallExecutionRequest, result *FirewallExecutionResult) bool {
	containerID := strings.TrimSpace(req.ContainerID)
	containerType := strings.ToLower(strings.TrimSpace(req.ContainerType))
	if containerID == "" {
		result.ErrorCode = "OCI_FIREWALL_CONTAINER_REQUIRED"
		result.ErrorMessage = "containerId is required for delete_broad"
		return false
	}
	switch containerType {
	case "nsg":
		ruleID := strings.TrimSpace(req.RuleID)
		if ruleID == "" {
			result.ErrorCode = "OCI_FIREWALL_RULE_ID_REQUIRED"
			result.ErrorMessage = "ruleId is required for NSG delete_broad"
			return false
		}
		remove, err := clients.VirtualNetwork.RemoveNetworkSecurityGroupSecurityRules(ctx, core.RemoveNetworkSecurityGroupSecurityRulesRequest{
			NetworkSecurityGroupId: common.String(containerID),
			RemoveNetworkSecurityGroupSecurityRulesDetails: core.RemoveNetworkSecurityGroupSecurityRulesDetails{
				SecurityRuleIds: []string{ruleID},
			},
			OpcRequestId: requestID("codex-fw-remove-broad-nsg", req.JobID),
		})
		appendFirstRequestID(&result.RequestID, remove.OpcRequestId)
		if err != nil {
			result.ErrorCode = "OCI_REMOVE_BROAD_NSG_RULE_FAILED"
			result.ErrorMessage = err.Error()
			return false
		}
		result.NSGIDs = appendNonEmpty(result.NSGIDs, containerID)
		result.RemovedRules = 1
		result.UsedNetworkSecurity = true
		return true
	case "security_list":
		return deleteBroadSecurityListRule(ctx, clients, req, result, containerID)
	default:
		result.ErrorCode = "OCI_FIREWALL_CONTAINER_TYPE_INVALID"
		result.ErrorMessage = "containerType must be nsg or security_list"
		return false
	}
}

func deleteBroadSecurityListRule(ctx context.Context, clients Clients, req FirewallExecutionRequest, result *FirewallExecutionResult, securityListID string) bool {
	response, err := clients.VirtualNetwork.GetSecurityList(ctx, core.GetSecurityListRequest{
		SecurityListId: common.String(securityListID),
		OpcRequestId:   requestID("codex-fw-get-broad-seclist", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, response.OpcRequestId)
	if err != nil {
		result.ErrorCode = "OCI_GET_SECURITY_LIST_FAILED"
		result.ErrorMessage = err.Error()
		return false
	}
	if result.SnapshotBefore {
		result.RulesBefore += len(response.SecurityList.IngressSecurityRules)
		result.AffectedContainers++
	}
	next := make([]core.IngressSecurityRule, 0, len(response.SecurityList.IngressSecurityRules))
	removed := 0
	for _, rule := range response.SecurityList.IngressSecurityRules {
		if isBroadIngressSecurityListRule(rule) {
			removed++
			continue
		}
		next = append(next, rule)
	}
	if removed == 0 {
		return true
	}
	update, err := clients.VirtualNetwork.UpdateSecurityList(ctx, core.UpdateSecurityListRequest{
		SecurityListId: common.String(securityListID),
		UpdateSecurityListDetails: core.UpdateSecurityListDetails{
			DisplayName:          response.SecurityList.DisplayName,
			EgressSecurityRules:  response.SecurityList.EgressSecurityRules,
			IngressSecurityRules: next,
			DefinedTags:          response.SecurityList.DefinedTags,
			FreeformTags:         response.SecurityList.FreeformTags,
		},
		IfMatch:      response.Etag,
		OpcRequestId: requestID("codex-fw-delete-broad-seclist", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, update.OpcRequestId)
	if err != nil {
		result.ErrorCode = "OCI_UPDATE_SECURITY_LIST_FAILED"
		result.ErrorMessage = err.Error()
		return false
	}
	result.SecurityListIDs = appendNonEmpty(result.SecurityListIDs, securityListID)
	result.RemovedRules = removed
	result.UsedSecurityList = true
	return true
}

func mapNSGFirewallRule(nsgID string, rule core.SecurityRule) FirewallRule {
	protocol := protocolName(stringValue(rule.Protocol))
	portMin, portMax := firewallRulePorts(rule.TcpOptions, rule.UdpOptions, protocol)
	out := FirewallRule{
		ID:            defaultString(stringValue(rule.Id), firewallRuleSyntheticID(nsgID, protocol, portMin, portMax, stringValue(rule.Source))),
		ContainerID:   nsgID,
		ContainerType: "nsg",
		Protocol:      protocol,
		PortMin:       portMin,
		PortMax:       portMax,
		PortLabel:     firewallPortLabel(protocol, portMin, portMax),
		Direction:     "入站",
		Source:        defaultString(stringValue(rule.Source), "-"),
		SourceType:    string(rule.SourceType),
		Policy:        "放行",
		Status:        "正常",
		Remark:        stringValue(rule.Description),
		IsBroadRule:   isBroadFirewallRule(protocol, portMin, portMax, stringValue(rule.Source)),
		Editable:      protocol == "tcp" || protocol == "udp",
		DeleteMode:    "nsg_rule_id",
	}
	if out.IsBroadRule {
		out.Status = "宽规则"
		out.Editable = false
	}
	return out
}

func mapSecurityListFirewallRule(securityListID string, index int, rule core.IngressSecurityRule) FirewallRule {
	protocol := protocolName(stringValue(rule.Protocol))
	portMin, portMax := firewallRulePorts(rule.TcpOptions, rule.UdpOptions, protocol)
	out := FirewallRule{
		ID:            fmt.Sprintf("%s:%d", securityListID, index),
		ContainerID:   securityListID,
		ContainerType: "security_list",
		Protocol:      protocol,
		PortMin:       portMin,
		PortMax:       portMax,
		PortLabel:     firewallPortLabel(protocol, portMin, portMax),
		Direction:     "入站",
		Source:        defaultString(stringValue(rule.Source), "-"),
		SourceType:    string(rule.SourceType),
		Policy:        "放行",
		Status:        "正常",
		Remark:        stringValue(rule.Description),
		IsBroadRule:   isBroadFirewallRule(protocol, portMin, portMax, stringValue(rule.Source)),
		Editable:      protocol == "tcp" || protocol == "udp",
		DeleteMode:    "exact_match",
	}
	if out.IsBroadRule {
		out.Status = "宽规则"
		out.Editable = false
	}
	return out
}

func firewallRulePorts(tcp *core.TcpOptions, udp *core.UdpOptions, protocol string) (int, int) {
	if protocol == "udp" && udp != nil && udp.DestinationPortRange != nil {
		return intValue(udp.DestinationPortRange.Min), intValue(udp.DestinationPortRange.Max)
	}
	if protocol == "tcp" && tcp != nil && tcp.DestinationPortRange != nil {
		return intValue(tcp.DestinationPortRange.Min), intValue(tcp.DestinationPortRange.Max)
	}
	return 0, 0
}

func firewallPortLabel(protocol string, portMin int, portMax int) string {
	if protocol == "all" {
		return "全部"
	}
	if portMin <= 0 {
		return "-"
	}
	if portMax <= 0 || portMax == portMin {
		return fmt.Sprintf("%d", portMin)
	}
	return fmt.Sprintf("%d-%d", portMin, portMax)
}

func isBroadFirewallRule(protocol string, portMin int, portMax int, source string) bool {
	if protocol == "all" {
		return true
	}
	return (source == "0.0.0.0/0" || source == "::/0") && portMin == 0 && portMax == 0
}

func isBroadIngressSecurityListRule(rule core.IngressSecurityRule) bool {
	source := strings.TrimSpace(stringValue(rule.Source))
	protocol := strings.ToLower(strings.TrimSpace(stringValue(rule.Protocol)))
	return protocol == "all" && (source == "0.0.0.0/0" || source == "::/0")
}

func firewallRuleSyntheticID(containerID string, protocol string, portMin int, portMax int, source string) string {
	return fmt.Sprintf("%s:%s:%d:%d:%s", containerID, protocol, portMin, portMax, source)
}

func applyFirewallToNSG(ctx context.Context, clients Clients, req FirewallExecutionRequest, result *FirewallExecutionResult, nsgID string) bool {
	if nsgID == "" {
		return true
	}
	list, err := clients.VirtualNetwork.ListNetworkSecurityGroupSecurityRules(ctx, core.ListNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId: common.String(nsgID),
		Limit:                  common.Int(200),
		OpcRequestId:           requestID("codex-fw-list-nsg", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, list.OpcRequestId)
	if err != nil {
		result.ErrorCode = "OCI_LIST_NSG_RULES_FAILED"
		result.ErrorMessage = err.Error()
		return false
	}
	if result.SnapshotBefore {
		result.RulesBefore += len(list.Items)
		result.AffectedContainers++
	}
	if result.Action == "open" {
		if hasMatchingNSGFirewallRule(list.Items, result.Protocol, result.PortMin, result.PortMax, result.SourceCIDR) {
			return true
		}
		add, err := clients.VirtualNetwork.AddNetworkSecurityGroupSecurityRules(ctx, core.AddNetworkSecurityGroupSecurityRulesRequest{
			NetworkSecurityGroupId: common.String(nsgID),
			AddNetworkSecurityGroupSecurityRulesDetails: core.AddNetworkSecurityGroupSecurityRulesDetails{
				SecurityRules: []core.AddSecurityRuleDetails{nsgFirewallRule(result.Protocol, result.PortMin, result.PortMax, result.SourceCIDR, req.Note)},
			},
			OpcRequestId: requestID("codex-fw-add-nsg", req.JobID),
		})
		appendFirstRequestID(&result.RequestID, add.OpcRequestId)
		if err != nil {
			result.ErrorCode = "OCI_ADD_NSG_RULE_FAILED"
			result.ErrorMessage = err.Error()
			return false
		}
		result.AddedRules++
		return true
	}
	var ids []string
	for _, rule := range list.Items {
		if matchesNSGFirewallRule(rule, result.Protocol, result.PortMin, result.PortMax, result.SourceCIDR) && stringValue(rule.Id) != "" {
			ids = append(ids, stringValue(rule.Id))
		}
	}
	if len(ids) == 0 {
		return true
	}
	remove, err := clients.VirtualNetwork.RemoveNetworkSecurityGroupSecurityRules(ctx, core.RemoveNetworkSecurityGroupSecurityRulesRequest{
		NetworkSecurityGroupId: common.String(nsgID),
		RemoveNetworkSecurityGroupSecurityRulesDetails: core.RemoveNetworkSecurityGroupSecurityRulesDetails{
			SecurityRuleIds: ids,
		},
		OpcRequestId: requestID("codex-fw-remove-nsg", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, remove.OpcRequestId)
	if err != nil {
		result.ErrorCode = "OCI_REMOVE_NSG_RULE_FAILED"
		result.ErrorMessage = err.Error()
		return false
	}
	result.RemovedRules += len(ids)
	return true
}

func applyFirewallToSecurityList(ctx context.Context, clients Clients, req FirewallExecutionRequest, result *FirewallExecutionResult, securityListID string) bool {
	if securityListID == "" {
		return true
	}
	response, err := clients.VirtualNetwork.GetSecurityList(ctx, core.GetSecurityListRequest{
		SecurityListId: common.String(securityListID),
		OpcRequestId:   requestID("codex-fw-get-seclist", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, response.OpcRequestId)
	if err != nil {
		result.ErrorCode = "OCI_GET_SECURITY_LIST_FAILED"
		result.ErrorMessage = err.Error()
		return false
	}
	if result.SnapshotBefore {
		result.RulesBefore += len(response.SecurityList.IngressSecurityRules)
		result.AffectedContainers++
	}
	rules := append([]core.IngressSecurityRule{}, response.SecurityList.IngressSecurityRules...)
	changed := false
	if result.Action == "open" {
		if !hasMatchingIngressFirewallRule(rules, result.Protocol, result.PortMin, result.PortMax, result.SourceCIDR) {
			rules = append(rules, securityListFirewallRule(result.Protocol, result.PortMin, result.PortMax, result.SourceCIDR, req.Note))
			result.AddedRules++
			changed = true
		}
	} else {
		next := make([]core.IngressSecurityRule, 0, len(rules))
		for _, rule := range rules {
			if matchesIngressFirewallRule(rule, result.Protocol, result.PortMin, result.PortMax, result.SourceCIDR) {
				result.RemovedRules++
				changed = true
				continue
			}
			next = append(next, rule)
		}
		rules = next
	}
	if !changed {
		return true
	}
	update, err := clients.VirtualNetwork.UpdateSecurityList(ctx, core.UpdateSecurityListRequest{
		SecurityListId: common.String(securityListID),
		UpdateSecurityListDetails: core.UpdateSecurityListDetails{
			DisplayName:          response.SecurityList.DisplayName,
			EgressSecurityRules:  response.SecurityList.EgressSecurityRules,
			IngressSecurityRules: rules,
			DefinedTags:          response.SecurityList.DefinedTags,
			FreeformTags:         response.SecurityList.FreeformTags,
		},
		IfMatch:      response.Etag,
		OpcRequestId: requestID("codex-fw-update-seclist", req.JobID),
	})
	appendFirstRequestID(&result.RequestID, update.OpcRequestId)
	if err != nil {
		result.ErrorCode = "OCI_UPDATE_SECURITY_LIST_FAILED"
		result.ErrorMessage = err.Error()
		return false
	}
	result.SecurityListIDs = appendNonEmpty(result.SecurityListIDs, securityListID)
	return true
}

func nsgFirewallRule(protocol string, portMin int, portMax int, source string, note string) core.AddSecurityRuleDetails {
	rule := core.AddSecurityRuleDetails{
		Direction:   core.AddSecurityRuleDetailsDirectionIngress,
		Source:      common.String(source),
		SourceType:  core.AddSecurityRuleDetailsSourceTypeCidrBlock,
		Protocol:    common.String(protocolNumber(protocol)),
		Description: common.String(firewallRuleDescription(protocol, portMin, portMax, note)),
	}
	if protocol == "udp" {
		rule.UdpOptions = &core.UdpOptions{DestinationPortRange: &core.PortRange{Min: common.Int(portMin), Max: common.Int(portMax)}}
	} else {
		rule.TcpOptions = &core.TcpOptions{DestinationPortRange: &core.PortRange{Min: common.Int(portMin), Max: common.Int(portMax)}}
	}
	return rule
}

func securityListFirewallRule(protocol string, portMin int, portMax int, source string, note string) core.IngressSecurityRule {
	rule := core.IngressSecurityRule{
		Source:      common.String(source),
		SourceType:  core.IngressSecurityRuleSourceTypeCidrBlock,
		Protocol:    common.String(protocolNumber(protocol)),
		Description: common.String(firewallRuleDescription(protocol, portMin, portMax, note)),
	}
	if protocol == "udp" {
		rule.UdpOptions = &core.UdpOptions{DestinationPortRange: &core.PortRange{Min: common.Int(portMin), Max: common.Int(portMax)}}
	} else {
		rule.TcpOptions = &core.TcpOptions{DestinationPortRange: &core.PortRange{Min: common.Int(portMin), Max: common.Int(portMax)}}
	}
	return rule
}

func firewallRuleDescription(protocol string, portMin int, portMax int, note string) string {
	if cleaned := cleanFirewallRuleDescription(note); cleaned != "" {
		return cleaned
	}
	if portMin == portMax {
		return fmt.Sprintf("Managed by OCI lifecycle platform: allow %s/%d", protocol, portMin)
	}
	return fmt.Sprintf("Managed by OCI lifecycle platform: allow %s/%d-%d", protocol, portMin, portMax)
}

func cleanFirewallRuleDescription(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if len(value) <= 255 {
		return value
	}
	return value[:255]
}

func hasMatchingNSGFirewallRule(rules []core.SecurityRule, protocol string, portMin int, portMax int, source string) bool {
	for _, rule := range rules {
		if matchesNSGFirewallRule(rule, protocol, portMin, portMax, source) {
			return true
		}
	}
	return false
}

func matchesNSGFirewallRule(rule core.SecurityRule, protocol string, portMin int, portMax int, source string) bool {
	if rule.Direction != core.SecurityRuleDirectionIngress || stringValue(rule.Source) != source || stringValue(rule.Protocol) != protocolNumber(protocol) {
		return false
	}
	return firewallPortRangeMatches(rule.TcpOptions, rule.UdpOptions, protocol, portMin, portMax)
}

func hasMatchingIngressFirewallRule(rules []core.IngressSecurityRule, protocol string, portMin int, portMax int, source string) bool {
	for _, rule := range rules {
		if matchesIngressFirewallRule(rule, protocol, portMin, portMax, source) {
			return true
		}
	}
	return false
}

func matchesIngressFirewallRule(rule core.IngressSecurityRule, protocol string, portMin int, portMax int, source string) bool {
	if stringValue(rule.Source) != source || stringValue(rule.Protocol) != protocolNumber(protocol) {
		return false
	}
	return firewallPortRangeMatches(rule.TcpOptions, rule.UdpOptions, protocol, portMin, portMax)
}

func firewallPortRangeMatches(tcp *core.TcpOptions, udp *core.UdpOptions, protocol string, portMin int, portMax int) bool {
	if protocol == "udp" {
		if udp == nil || udp.DestinationPortRange == nil {
			return false
		}
		return intValue(udp.DestinationPortRange.Min) == portMin && intValue(udp.DestinationPortRange.Max) == portMax
	}
	if tcp == nil || tcp.DestinationPortRange == nil {
		return false
	}
	return intValue(tcp.DestinationPortRange.Min) == portMin && intValue(tcp.DestinationPortRange.Max) == portMax
}

func protocolNumber(protocol string) string {
	if strings.EqualFold(protocol, "udp") {
		return "17"
	}
	return "6"
}

func protocolName(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "6":
		return "tcp"
	case "17":
		return "udp"
	case "all":
		return "all"
	default:
		return strings.ToLower(strings.TrimSpace(protocol))
	}
}

func normalizedFirewallAction(action string) string {
	return strings.ToLower(strings.TrimSpace(action))
}

func normalizedFirewallProtocol(protocol string) string {
	return strings.ToLower(strings.TrimSpace(protocol))
}
