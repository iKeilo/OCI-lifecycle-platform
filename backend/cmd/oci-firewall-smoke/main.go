package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"a-series-oracle/backend/internal/oci"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type inspectResult struct {
	Verified          bool              `json:"verified"`
	InstanceID        string            `json:"instanceId"`
	CompartmentID     string            `json:"compartmentId,omitempty"`
	VNICID            string            `json:"vnicId,omitempty"`
	SubnetID          string            `json:"subnetId,omitempty"`
	NSGIDs            []string          `json:"nsgIds,omitempty"`
	SecurityListIDs   []string          `json:"securityListIds,omitempty"`
	SubnetVNICs       []inspectVNIC     `json:"subnetVnics,omitempty"`
	NSGRules          []inspectRule     `json:"nsgRules,omitempty"`
	SecurityListRules []inspectRule     `json:"securityListRules,omitempty"`
	RequestIDs        []string          `json:"requestIds,omitempty"`
	ErrorCode         string            `json:"errorCode,omitempty"`
	ErrorMessage      string            `json:"errorMessage,omitempty"`
	Meta              map[string]string `json:"meta,omitempty"`
}

type inspectVNIC struct {
	InstanceID  string `json:"instanceId,omitempty"`
	VNICID      string `json:"vnicId,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	PrivateIP   string `json:"privateIp,omitempty"`
	PublicIP    string `json:"publicIp,omitempty"`
	State       string `json:"state,omitempty"`
}

type inspectRule struct {
	ContainerID string `json:"containerId"`
	Kind        string `json:"kind"`
	Direction   string `json:"direction,omitempty"`
	Protocol    string `json:"protocol"`
	Source      string `json:"source"`
	SourceType  string `json:"sourceType,omitempty"`
	TCPMin      int    `json:"tcpMin,omitempty"`
	TCPMax      int    `json:"tcpMax,omitempty"`
	UDPMin      int    `json:"udpMin,omitempty"`
	UDPMax      int    `json:"udpMax,omitempty"`
	Description string `json:"description,omitempty"`
}

func main() {
	instanceID := flag.String("instance", "", "OCI instance OCID")
	action := flag.String("action", "open", "open or close")
	protocol := flag.String("protocol", "tcp", "tcp or udp")
	port := flag.Int("port", 80, "single port")
	portMax := flag.Int("port-max", 0, "optional end port")
	source := flag.String("source", "0.0.0.0/0", "source CIDR")
	scope := flag.String("scope", "auto", "auto, nsg, or security_list")
	vnicID := flag.String("vnic", "primary", "primary or VNIC OCID")
	timeout := flag.Duration("timeout", 5*time.Minute, "overall timeout")
	flag.Parse()

	maxPort := *portMax
	if maxPort <= 0 {
		maxPort = *port
	}
	cfg := oci.ReadinessConfig{
		ExecutionMode:  "oci",
		TenancyOCID:    strings.TrimSpace(os.Getenv("OCI_TENANCY_OCID")),
		UserOCID:       strings.TrimSpace(os.Getenv("OCI_USER_OCID")),
		Fingerprint:    strings.TrimSpace(os.Getenv("OCI_FINGERPRINT")),
		PrivateKey:     os.Getenv("OCI_PRIVATE_KEY"),
		PrivateKeyFile: strings.TrimSpace(os.Getenv("OCI_PRIVATE_KEY_FILE")),
		Region:         strings.TrimSpace(os.Getenv("OCI_REGION")),
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if strings.EqualFold(strings.TrimSpace(*action), "inspect") {
		result := inspectFirewall(ctx, cfg, strings.TrimSpace(*instanceID))
		writeJSON(result)
		if !result.Verified {
			os.Exit(1)
		}
		return
	}
	if strings.EqualFold(strings.TrimSpace(*action), "narrow-http-test") {
		result := rewriteSingleInstanceBroadRule(ctx, cfg, strings.TrimSpace(*instanceID), false)
		writeJSON(result)
		if !result.Verified {
			os.Exit(1)
		}
		return
	}
	if strings.EqualFold(strings.TrimSpace(*action), "restore-all") {
		result := rewriteSingleInstanceBroadRule(ctx, cfg, strings.TrimSpace(*instanceID), true)
		writeJSON(result)
		if !result.Verified {
			os.Exit(1)
		}
		return
	}

	result := oci.ExecuteFirewallTask(ctx, cfg, oci.FirewallExecutionRequest{
		InstanceID:     strings.TrimSpace(*instanceID),
		VNICID:         strings.TrimSpace(*vnicID),
		Action:         strings.TrimSpace(*action),
		Protocol:       strings.TrimSpace(*protocol),
		PortMin:        *port,
		PortMax:        maxPort,
		SourceCIDR:     strings.TrimSpace(*source),
		TargetScope:    strings.TrimSpace(*scope),
		SnapshotBefore: true,
		JobID:          "oci-firewall-smoke",
	})
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode result: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(raw))
	if !result.Verified {
		os.Exit(1)
	}
}

func inspectFirewall(ctx context.Context, cfg oci.ReadinessConfig, instanceID string) inspectResult {
	out := inspectResult{InstanceID: instanceID, Meta: map[string]string{"action": "inspect"}}
	if strings.TrimSpace(instanceID) == "" {
		out.ErrorCode = "INSTANCE_REQUIRED"
		out.ErrorMessage = "instance OCID is required"
		return out
	}
	clients, err := oci.NewClients(cfg)
	if err != nil {
		out.ErrorCode = "OCI_CLIENT_INIT_FAILED"
		out.ErrorMessage = err.Error()
		return out
	}
	instance, err := clients.Compute.GetInstance(ctx, core.GetInstanceRequest{InstanceId: common.String(instanceID)})
	appendRequestID(&out.RequestIDs, instance.OpcRequestId)
	if err != nil {
		out.ErrorCode = "OCI_GET_INSTANCE_FAILED"
		out.ErrorMessage = err.Error()
		return out
	}
	compartmentID := value(instance.Instance.CompartmentId)
	out.CompartmentID = compartmentID
	attachments, err := clients.Compute.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
		CompartmentId: common.String(compartmentID),
		InstanceId:    common.String(instanceID),
		Limit:         common.Int(50),
	})
	appendRequestID(&out.RequestIDs, attachments.OpcRequestId)
	if err != nil {
		out.ErrorCode = "OCI_LIST_VNIC_ATTACHMENTS_FAILED"
		out.ErrorMessage = err.Error()
		return out
	}
	var vnicID string
	for _, attachment := range attachments.Items {
		if attachment.LifecycleState == core.VnicAttachmentLifecycleStateAttached {
			vnicID = value(attachment.VnicId)
			break
		}
	}
	if vnicID == "" && len(attachments.Items) > 0 {
		vnicID = value(attachments.Items[0].VnicId)
	}
	if vnicID == "" {
		out.ErrorCode = "OCI_VNIC_NOT_FOUND"
		out.ErrorMessage = "no VNIC attachment found"
		return out
	}
	vnic, err := clients.VirtualNetwork.GetVnic(ctx, core.GetVnicRequest{VnicId: common.String(vnicID)})
	appendRequestID(&out.RequestIDs, vnic.OpcRequestId)
	if err != nil {
		out.ErrorCode = "OCI_GET_VNIC_FAILED"
		out.ErrorMessage = err.Error()
		return out
	}
	out.VNICID = value(vnic.Vnic.Id)
	out.SubnetID = value(vnic.Vnic.SubnetId)
	out.NSGIDs = append(out.NSGIDs, vnic.Vnic.NsgIds...)
	for _, nsgID := range out.NSGIDs {
		rules, err := clients.VirtualNetwork.ListNetworkSecurityGroupSecurityRules(ctx, core.ListNetworkSecurityGroupSecurityRulesRequest{
			NetworkSecurityGroupId: common.String(nsgID),
			Limit:                  common.Int(200),
		})
		appendRequestID(&out.RequestIDs, rules.OpcRequestId)
		if err != nil {
			out.ErrorCode = "OCI_LIST_NSG_RULES_FAILED"
			out.ErrorMessage = err.Error()
			return out
		}
		for _, rule := range rules.Items {
			out.NSGRules = append(out.NSGRules, inspectRule{
				ContainerID: nsgID,
				Kind:        "nsg",
				Direction:   string(rule.Direction),
				Protocol:    value(rule.Protocol),
				Source:      value(rule.Source),
				SourceType:  string(rule.SourceType),
				TCPMin:      tcpMin(rule.TcpOptions),
				TCPMax:      tcpMax(rule.TcpOptions),
				UDPMin:      udpMin(rule.UdpOptions),
				UDPMax:      udpMax(rule.UdpOptions),
				Description: value(rule.Description),
			})
		}
	}
	if out.SubnetID != "" {
		subnet, err := clients.VirtualNetwork.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: common.String(out.SubnetID)})
		appendRequestID(&out.RequestIDs, subnet.OpcRequestId)
		if err != nil {
			out.ErrorCode = "OCI_GET_SUBNET_FAILED"
			out.ErrorMessage = err.Error()
			return out
		}
		out.SecurityListIDs = append(out.SecurityListIDs, subnet.Subnet.SecurityListIds...)
		members, err := listSubnetVNICs(ctx, clients, compartmentID, instanceID, out.SubnetID, &out.RequestIDs)
		if err != nil {
			out.ErrorCode = "OCI_LIST_SUBNET_VNICS_FAILED"
			out.ErrorMessage = err.Error()
			return out
		}
		out.SubnetVNICs = members
		for _, securityListID := range out.SecurityListIDs {
			list, err := clients.VirtualNetwork.GetSecurityList(ctx, core.GetSecurityListRequest{SecurityListId: common.String(securityListID)})
			appendRequestID(&out.RequestIDs, list.OpcRequestId)
			if err != nil {
				out.ErrorCode = "OCI_GET_SECURITY_LIST_FAILED"
				out.ErrorMessage = err.Error()
				return out
			}
			for _, rule := range list.SecurityList.IngressSecurityRules {
				out.SecurityListRules = append(out.SecurityListRules, inspectRule{
					ContainerID: securityListID,
					Kind:        "security_list",
					Protocol:    value(rule.Protocol),
					Source:      value(rule.Source),
					SourceType:  string(rule.SourceType),
					TCPMin:      tcpMin(rule.TcpOptions),
					TCPMax:      tcpMax(rule.TcpOptions),
					UDPMin:      udpMin(rule.UdpOptions),
					UDPMax:      udpMax(rule.UdpOptions),
					Description: value(rule.Description),
				})
			}
		}
	}
	out.Verified = true
	return out
}

func rewriteSingleInstanceBroadRule(ctx context.Context, cfg oci.ReadinessConfig, instanceID string, restoreAll bool) inspectResult {
	out := inspectFirewall(ctx, cfg, instanceID)
	if !out.Verified {
		return out
	}
	out.Verified = false
	if len(out.SubnetVNICs) != 1 || out.SubnetVNICs[0].InstanceID != instanceID {
		out.ErrorCode = "SUBNET_NOT_SINGLE_INSTANCE"
		out.ErrorMessage = "refusing to rewrite a shared subnet security list"
		return out
	}
	if len(out.SecurityListIDs) != 1 {
		out.ErrorCode = "SECURITY_LIST_COUNT_UNSUPPORTED"
		out.ErrorMessage = "expected exactly one security list for validation rewrite"
		return out
	}
	clients, err := oci.NewClients(cfg)
	if err != nil {
		out.ErrorCode = "OCI_CLIENT_INIT_FAILED"
		out.ErrorMessage = err.Error()
		return out
	}
	securityListID := out.SecurityListIDs[0]
	response, err := clients.VirtualNetwork.GetSecurityList(ctx, core.GetSecurityListRequest{SecurityListId: common.String(securityListID)})
	appendRequestID(&out.RequestIDs, response.OpcRequestId)
	if err != nil {
		out.ErrorCode = "OCI_GET_SECURITY_LIST_FAILED"
		out.ErrorMessage = err.Error()
		return out
	}
	var nextIngress []core.IngressSecurityRule
	if restoreAll {
		nextIngress = []core.IngressSecurityRule{{
			Source:      common.String("0.0.0.0/0"),
			SourceType:  core.IngressSecurityRuleSourceTypeCidrBlock,
			Protocol:    common.String("all"),
			Description: common.String("全部放行"),
		}}
		out.Meta["rewrite"] = "restore-all"
	} else {
		rules := response.SecurityList.IngressSecurityRules
		if len(rules) != 1 || value(rules[0].Source) != "0.0.0.0/0" || value(rules[0].Protocol) != "all" {
			out.ErrorCode = "BROAD_RULE_NOT_EXCLUSIVE"
			out.ErrorMessage = "expected exactly one broad all-protocol ingress rule before narrowing"
			return out
		}
		nextIngress = []core.IngressSecurityRule{
			tcpIngressRule(22, "temporary validation allow ssh"),
			tcpIngressRule(443, "temporary validation allow https"),
			tcpIngressRule(3389, "temporary validation allow rdp"),
		}
		out.Meta["rewrite"] = "narrow-without-80"
	}
	update, err := clients.VirtualNetwork.UpdateSecurityList(ctx, core.UpdateSecurityListRequest{
		SecurityListId: common.String(securityListID),
		UpdateSecurityListDetails: core.UpdateSecurityListDetails{
			DisplayName:          response.SecurityList.DisplayName,
			EgressSecurityRules:  response.SecurityList.EgressSecurityRules,
			IngressSecurityRules: nextIngress,
			DefinedTags:          response.SecurityList.DefinedTags,
			FreeformTags:         response.SecurityList.FreeformTags,
		},
		IfMatch: response.Etag,
	})
	appendRequestID(&out.RequestIDs, update.OpcRequestId)
	if err != nil {
		out.ErrorCode = "OCI_UPDATE_SECURITY_LIST_FAILED"
		out.ErrorMessage = err.Error()
		return out
	}
	out.SecurityListRules = nil
	for _, rule := range nextIngress {
		out.SecurityListRules = append(out.SecurityListRules, inspectRule{
			ContainerID: securityListID,
			Kind:        "security_list",
			Protocol:    value(rule.Protocol),
			Source:      value(rule.Source),
			SourceType:  string(rule.SourceType),
			TCPMin:      tcpMin(rule.TcpOptions),
			TCPMax:      tcpMax(rule.TcpOptions),
			Description: value(rule.Description),
		})
	}
	out.Verified = true
	out.ErrorCode = ""
	out.ErrorMessage = ""
	return out
}

func tcpIngressRule(port int, description string) core.IngressSecurityRule {
	return core.IngressSecurityRule{
		Source:      common.String("0.0.0.0/0"),
		SourceType:  core.IngressSecurityRuleSourceTypeCidrBlock,
		Protocol:    common.String("6"),
		TcpOptions:  &core.TcpOptions{DestinationPortRange: &core.PortRange{Min: common.Int(port), Max: common.Int(port)}},
		Description: common.String(description),
	}
}

func listSubnetVNICs(ctx context.Context, clients oci.Clients, compartmentID string, currentInstanceID string, subnetID string, requestIDs *[]string) ([]inspectVNIC, error) {
	attachments, err := clients.Compute.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
		CompartmentId: common.String(compartmentID),
		Limit:         common.Int(200),
	})
	appendRequestID(requestIDs, attachments.OpcRequestId)
	if err != nil {
		return nil, err
	}
	members := []inspectVNIC{}
	for _, attachment := range attachments.Items {
		if attachment.LifecycleState != core.VnicAttachmentLifecycleStateAttached || value(attachment.VnicId) == "" {
			continue
		}
		vnic, err := clients.VirtualNetwork.GetVnic(ctx, core.GetVnicRequest{VnicId: attachment.VnicId})
		appendRequestID(requestIDs, vnic.OpcRequestId)
		if err != nil {
			return nil, err
		}
		if value(vnic.Vnic.SubnetId) != subnetID {
			continue
		}
		members = append(members, inspectVNIC{
			InstanceID:  value(attachment.InstanceId),
			VNICID:      value(vnic.Vnic.Id),
			DisplayName: value(vnic.Vnic.DisplayName),
			PrivateIP:   value(vnic.Vnic.PrivateIp),
			PublicIP:    value(vnic.Vnic.PublicIp),
			State:       string(vnic.Vnic.LifecycleState),
		})
	}
	return members, nil
}

func writeJSON(value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode result: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(raw))
}

func value(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func intValue(ptr *int) int {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func tcpMin(options *core.TcpOptions) int {
	if options == nil || options.DestinationPortRange == nil {
		return 0
	}
	return intValue(options.DestinationPortRange.Min)
}

func tcpMax(options *core.TcpOptions) int {
	if options == nil || options.DestinationPortRange == nil {
		return 0
	}
	return intValue(options.DestinationPortRange.Max)
}

func udpMin(options *core.UdpOptions) int {
	if options == nil || options.DestinationPortRange == nil {
		return 0
	}
	return intValue(options.DestinationPortRange.Min)
}

func udpMax(options *core.UdpOptions) int {
	if options == nil || options.DestinationPortRange == nil {
		return 0
	}
	return intValue(options.DestinationPortRange.Max)
}

func appendRequestID(values *[]string, requestID *string) {
	if requestID != nil && strings.TrimSpace(*requestID) != "" {
		*values = append(*values, *requestID)
	}
}
