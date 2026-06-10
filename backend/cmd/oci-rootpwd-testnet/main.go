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

type result struct {
	Verified          bool   `json:"verified"`
	Action            string `json:"action"`
	DisplayName       string `json:"displayName,omitempty"`
	CompartmentID     string `json:"compartmentId,omitempty"`
	VCNID             string `json:"vcnId,omitempty"`
	SubnetID          string `json:"subnetId,omitempty"`
	RouteTableID      string `json:"routeTableId,omitempty"`
	SecurityListID    string `json:"securityListId,omitempty"`
	InternetGatewayID string `json:"internetGatewayId,omitempty"`
	ErrorCode         string `json:"errorCode,omitempty"`
	ErrorMessage      string `json:"errorMessage,omitempty"`
}

func main() {
	action := flag.String("action", "create", "create or delete")
	profileID := flag.String("profile", "profile-default-2", "profile ID or name")
	compartmentID := flag.String("compartment", "", "compartment OCID; defaults to tenancy")
	vcnID := flag.String("vcn", "", "VCN OCID for delete")
	subnetID := flag.String("subnet", "", "Subnet OCID for delete")
	routeTableID := flag.String("route-table", "", "Route table OCID for delete")
	securityListID := flag.String("security-list", "", "Security list OCID for delete")
	internetGatewayID := flag.String("internet-gateway", "", "Internet gateway OCID for delete")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	cfg, err := resolveConfig(*profileID)
	if err != nil {
		write(result{Action: *action, ErrorCode: "OCI_PROFILE_RESOLVE_FAILED", ErrorMessage: err.Error()})
		os.Exit(1)
	}
	if strings.TrimSpace(*compartmentID) == "" {
		*compartmentID = cfg.TenancyOCID
	}

	clients, err := oci.NewClients(cfg)
	if err != nil {
		write(result{Action: *action, ErrorCode: "OCI_CLIENT_INIT_FAILED", ErrorMessage: err.Error()})
		os.Exit(1)
	}

	var out result
	switch strings.ToLower(strings.TrimSpace(*action)) {
	case "create":
		out = createNetwork(ctx, clients, *compartmentID)
	case "delete":
		out = deleteNetwork(ctx, clients, *vcnID, *subnetID, *routeTableID, *securityListID, *internetGatewayID)
	default:
		out = result{Action: *action, ErrorCode: "BAD_ACTION", ErrorMessage: "action must be create or delete"}
	}
	write(out)
	if !out.Verified {
		os.Exit(1)
	}
}

func resolveConfig(profileID string) (oci.ReadinessConfig, error) {
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
	sink, err := fileprofile.New(loaded.Security.ProfileStoreFile)
	if err != nil {
		return readiness, err
	}
	if err := sink.SetProfileKeyEncryptionKey(loaded.Security.ProfileKeyEncryptionKey); err != nil {
		return readiness, err
	}
	profiles, err := sink.ListProfiles()
	if err != nil {
		return readiness, err
	}
	appStore.ReplaceProfiles(profiles)
	appStore.SetPersistenceSink(sink)
	resolved, _, err := profileconfig.NewResolver(appStore, readiness).Resolve(profileID, "")
	return resolved, err
}

func createNetwork(ctx context.Context, clients oci.Clients, compartmentID string) result {
	suffix := time.Now().UTC().Format("20060102-150405")
	hexPart := strconv.FormatInt(time.Now().UTC().UnixNano()&0xfffff, 16)
	ipv4Second := 120 + int(time.Now().UTC().UnixNano()%40)
	ipv4CIDR := fmt.Sprintf("10.%d.0.0/16", ipv4Second)
	subnetCIDR := fmt.Sprintf("10.%d.1.0/24", ipv4Second)
	displayName := "codex-rootpwd-smoke-" + suffix
	tags := map[string]string{"managedBy": "codex", "purpose": "root-password-smoke"}

	out := result{Action: "create", DisplayName: displayName, CompartmentID: compartmentID}
	vcnResp, err := clients.VirtualNetwork.CreateVcn(ctx, core.CreateVcnRequest{
		CreateVcnDetails: core.CreateVcnDetails{
			CompartmentId: common.String(compartmentID),
			CidrBlocks:    []string{ipv4CIDR},
			DisplayName:   common.String(displayName + "-vcn"),
			DnsLabel:      common.String("c" + safeDNSLabel(hexPart)),
			FreeformTags:  tags,
		},
	})
	out.VCNID = stringValue(vcnResp.Vcn.Id)
	if err != nil {
		return fail(out, "OCI_CREATE_VCN_FAILED", err)
	}
	if err := waitVCN(ctx, clients, out.VCNID, core.VcnLifecycleStateAvailable); err != nil {
		return fail(out, "OCI_WAIT_VCN_FAILED", err)
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
		return fail(out, "OCI_CREATE_IGW_FAILED", err)
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
			}},
			FreeformTags: tags,
		},
	})
	out.RouteTableID = stringValue(rtResp.RouteTable.Id)
	if err != nil {
		return fail(out, "OCI_CREATE_ROUTE_TABLE_FAILED", err)
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
			}},
			IngressSecurityRules: []core.IngressSecurityRule{{
				Source:     common.String("0.0.0.0/0"),
				SourceType: core.IngressSecurityRuleSourceTypeCidrBlock,
				Protocol:   common.String("6"),
				TcpOptions: &core.TcpOptions{
					DestinationPortRange: &core.PortRange{Min: common.Int(22), Max: common.Int(22)},
				},
			}},
			FreeformTags: tags,
		},
	})
	out.SecurityListID = stringValue(slResp.SecurityList.Id)
	if err != nil {
		return fail(out, "OCI_CREATE_SECURITY_LIST_FAILED", err)
	}

	subnetResp, err := clients.VirtualNetwork.CreateSubnet(ctx, core.CreateSubnetRequest{
		CreateSubnetDetails: core.CreateSubnetDetails{
			CompartmentId:          common.String(compartmentID),
			VcnId:                  common.String(out.VCNID),
			CidrBlock:              common.String(subnetCIDR),
			DisplayName:            common.String(displayName + "-subnet"),
			DnsLabel:               common.String("s" + safeDNSLabel(hexPart)),
			ProhibitPublicIpOnVnic: common.Bool(false),
			RouteTableId:           common.String(out.RouteTableID),
			SecurityListIds:        []string{out.SecurityListID},
			FreeformTags:           tags,
		},
	})
	out.SubnetID = stringValue(subnetResp.Subnet.Id)
	if err != nil {
		return fail(out, "OCI_CREATE_SUBNET_FAILED", err)
	}
	if err := waitSubnet(ctx, clients, out.SubnetID, core.SubnetLifecycleStateAvailable); err != nil {
		return fail(out, "OCI_WAIT_SUBNET_FAILED", err)
	}
	out.Verified = true
	return out
}

func deleteNetwork(ctx context.Context, clients oci.Clients, vcnID, subnetID, routeTableID, securityListID, internetGatewayID string) result {
	out := result{Action: "delete", VCNID: vcnID, SubnetID: subnetID, RouteTableID: routeTableID, SecurityListID: securityListID, InternetGatewayID: internetGatewayID}
	steps := []struct {
		name string
		id   string
		fn   func() error
	}{
		{"SUBNET", subnetID, func() error {
			_, err := clients.VirtualNetwork.DeleteSubnet(ctx, core.DeleteSubnetRequest{SubnetId: common.String(subnetID)})
			if err != nil {
				return err
			}
			return waitSubnet(ctx, clients, subnetID, core.SubnetLifecycleStateTerminated)
		}},
		{"ROUTE_TABLE", routeTableID, func() error {
			_, err := clients.VirtualNetwork.DeleteRouteTable(ctx, core.DeleteRouteTableRequest{RtId: common.String(routeTableID)})
			return err
		}},
		{"SECURITY_LIST", securityListID, func() error {
			_, err := clients.VirtualNetwork.DeleteSecurityList(ctx, core.DeleteSecurityListRequest{SecurityListId: common.String(securityListID)})
			return err
		}},
		{"INTERNET_GATEWAY", internetGatewayID, func() error {
			_, err := clients.VirtualNetwork.DeleteInternetGateway(ctx, core.DeleteInternetGatewayRequest{IgId: common.String(internetGatewayID)})
			return err
		}},
		{"VCN", vcnID, func() error {
			_, err := clients.VirtualNetwork.DeleteVcn(ctx, core.DeleteVcnRequest{VcnId: common.String(vcnID)})
			if err != nil {
				return err
			}
			return waitVCN(ctx, clients, vcnID, core.VcnLifecycleStateTerminated)
		}},
	}

	for _, step := range steps {
		if strings.TrimSpace(step.id) == "" {
			continue
		}
		var err error
		for attempt := 0; attempt < 36; attempt++ {
			err = step.fn()
			if err == nil || isNotFound(err) {
				err = nil
				break
			}
			select {
			case <-ctx.Done():
				return fail(out, "OCI_DELETE_"+step.name+"_FAILED", ctx.Err())
			case <-time.After(10 * time.Second):
			}
		}
		if err != nil {
			return fail(out, "OCI_DELETE_"+step.name+"_FAILED", err)
		}
	}
	out.Verified = true
	return out
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

func fail(out result, code string, err error) result {
	out.ErrorCode = code
	out.ErrorMessage = err.Error()
	return out
}

func write(out result) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(out)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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

var _ = domain.Profile{}
