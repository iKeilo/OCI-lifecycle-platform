package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"a-series-oracle/backend/internal/oci"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type result struct {
	Verified        bool   `json:"verified"`
	Action          string `json:"action"`
	CompartmentID   string `json:"compartmentId,omitempty"`
	DisplayName     string `json:"displayName,omitempty"`
	VCNID           string `json:"vcnId,omitempty"`
	SubnetID        string `json:"subnetId,omitempty"`
	IPv4CIDR        string `json:"ipv4Cidr,omitempty"`
	SubnetIPv4CIDR  string `json:"subnetIpv4Cidr,omitempty"`
	IPv6CIDR        string `json:"ipv6Cidr,omitempty"`
	SubnetIPv6CIDR  string `json:"subnetIpv6Cidr,omitempty"`
	ErrorCode       string `json:"errorCode,omitempty"`
	ErrorMessage    string `json:"errorMessage,omitempty"`
	CreateRequestID string `json:"createRequestId,omitempty"`
	DeleteRequestID string `json:"deleteRequestId,omitempty"`
}

func main() {
	log.SetOutput(os.Stderr)
	action := flag.String("action", "create", "create or delete")
	compartmentID := flag.String("compartment", "", "compartment OCID")
	vcnID := flag.String("vcn", "", "VCN OCID for delete")
	subnetID := flag.String("subnet", "", "subnet OCID for delete")
	flag.Parse()

	cfg := oci.ReadinessConfig{
		ExecutionMode:  "oci",
		TenancyOCID:    env("OCI_TENANCY_OCID"),
		UserOCID:       env("OCI_USER_OCID"),
		Fingerprint:    env("OCI_FINGERPRINT"),
		PrivateKey:     env("OCI_PRIVATE_KEY"),
		PrivateKeyFile: env("OCI_PRIVATE_KEY_FILE"),
		Region:         env("OCI_REGION"),
	}
	if *compartmentID == "" {
		*compartmentID = cfg.TenancyOCID
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	clients, err := oci.NewClients(cfg)
	if err != nil {
		write(result{Action: *action, ErrorCode: "OCI_CLIENT_INIT_FAILED", ErrorMessage: err.Error()})
		os.Exit(1)
	}

	switch strings.ToLower(strings.TrimSpace(*action)) {
	case "create":
		out := create(ctx, clients, *compartmentID)
		write(out)
		if !out.Verified {
			os.Exit(1)
		}
	case "delete":
		out := deleteResources(ctx, clients, *vcnID, *subnetID)
		write(out)
		if !out.Verified {
			os.Exit(1)
		}
	default:
		write(result{Action: *action, ErrorCode: "BAD_ACTION", ErrorMessage: "action must be create or delete"})
		os.Exit(1)
	}
}

func create(ctx context.Context, clients oci.Clients, compartmentID string) result {
	suffix := time.Now().UTC().Format("20060102-150405")
	hexPart := strconv.FormatInt(time.Now().UTC().UnixNano()&0xffff, 16)
	if len(hexPart) < 4 {
		hexPart = strings.Repeat("0", 4-len(hexPart)) + hexPart
	}
	ipv6CIDR := fmt.Sprintf("fd00:%s:%s:0000::/56", hexPart, suffix[2:6])
	subnetIPv6CIDR := fmt.Sprintf("fd00:%s:%s:0000::/64", hexPart, suffix[2:6])
	ipv4CIDR := "10.88.0.0/16"
	subnetIPv4CIDR := "10.88.1.0/24"
	displayName := "codex-ipv6-testnet-" + suffix

	out := result{
		Verified:       false,
		Action:         "create",
		CompartmentID:  compartmentID,
		DisplayName:    displayName,
		IPv4CIDR:       ipv4CIDR,
		SubnetIPv4CIDR: subnetIPv4CIDR,
		IPv6CIDR:       ipv6CIDR,
		SubnetIPv6CIDR: subnetIPv6CIDR,
	}

	vcnResp, err := clients.VirtualNetwork.CreateVcn(ctx, core.CreateVcnRequest{
		CreateVcnDetails: core.CreateVcnDetails{
			CompartmentId:                common.String(compartmentID),
			CidrBlocks:                   []string{ipv4CIDR},
			Ipv6PrivateCidrBlocks:        []string{ipv6CIDR},
			IsIpv6Enabled:                common.Bool(true),
			IsOracleGuaAllocationEnabled: common.Bool(false),
			DisplayName:                  common.String(displayName),
			DnsLabel:                     common.String("cv6" + hexPart),
			FreeformTags:                 map[string]string{"managedBy": "codex", "purpose": "ipv6-testnet"},
		},
	})
	if vcnResp.OpcRequestId != nil {
		out.CreateRequestID = *vcnResp.OpcRequestId
	}
	if vcnResp.Vcn.Id != nil {
		out.VCNID = *vcnResp.Vcn.Id
	}
	if err != nil {
		out.ErrorCode = "OCI_CREATE_VCN_FAILED"
		out.ErrorMessage = err.Error()
		return out
	}
	if err := waitVCN(ctx, clients, out.VCNID, core.VcnLifecycleStateAvailable); err != nil {
		out.ErrorCode = "OCI_WAIT_VCN_FAILED"
		out.ErrorMessage = err.Error()
		return out
	}

	subnetResp, err := clients.VirtualNetwork.CreateSubnet(ctx, core.CreateSubnetRequest{
		CreateSubnetDetails: core.CreateSubnetDetails{
			CompartmentId:           common.String(compartmentID),
			VcnId:                   common.String(out.VCNID),
			CidrBlock:               common.String(subnetIPv4CIDR),
			Ipv6CidrBlock:           common.String(subnetIPv6CIDR),
			DisplayName:             common.String(displayName + "-subnet"),
			DnsLabel:                common.String("s" + hexPart),
			ProhibitInternetIngress: common.Bool(true),
			ProhibitPublicIpOnVnic:  common.Bool(true),
			FreeformTags:            map[string]string{"managedBy": "codex", "purpose": "ipv6-testnet"},
		},
	})
	if subnetResp.OpcRequestId != nil && out.CreateRequestID == "" {
		out.CreateRequestID = *subnetResp.OpcRequestId
	}
	if subnetResp.Subnet.Id != nil {
		out.SubnetID = *subnetResp.Subnet.Id
	}
	if err != nil {
		out.ErrorCode = "OCI_CREATE_SUBNET_FAILED"
		out.ErrorMessage = err.Error()
		return out
	}
	if err := waitSubnet(ctx, clients, out.SubnetID, core.SubnetLifecycleStateAvailable); err != nil {
		out.ErrorCode = "OCI_WAIT_SUBNET_FAILED"
		out.ErrorMessage = err.Error()
		return out
	}
	out.Verified = true
	return out
}

func deleteResources(ctx context.Context, clients oci.Clients, vcnID string, subnetID string) result {
	out := result{Action: "delete", VCNID: vcnID, SubnetID: subnetID}
	if strings.TrimSpace(subnetID) != "" {
		resp, err := clients.VirtualNetwork.DeleteSubnet(ctx, core.DeleteSubnetRequest{SubnetId: common.String(subnetID)})
		if resp.OpcRequestId != nil {
			out.DeleteRequestID = *resp.OpcRequestId
		}
		if err != nil {
			out.ErrorCode = "OCI_DELETE_SUBNET_FAILED"
			out.ErrorMessage = err.Error()
			return out
		}
		if err := waitSubnet(ctx, clients, subnetID, core.SubnetLifecycleStateTerminated); err != nil {
			out.ErrorCode = "OCI_WAIT_SUBNET_DELETE_FAILED"
			out.ErrorMessage = err.Error()
			return out
		}
	}
	if strings.TrimSpace(vcnID) != "" {
		resp, err := clients.VirtualNetwork.DeleteVcn(ctx, core.DeleteVcnRequest{VcnId: common.String(vcnID)})
		if resp.OpcRequestId != nil && out.DeleteRequestID == "" {
			out.DeleteRequestID = *resp.OpcRequestId
		}
		if err != nil {
			out.ErrorCode = "OCI_DELETE_VCN_FAILED"
			out.ErrorMessage = err.Error()
			return out
		}
		if err := waitVCN(ctx, clients, vcnID, core.VcnLifecycleStateTerminated); err != nil {
			out.ErrorCode = "OCI_WAIT_VCN_DELETE_FAILED"
			out.ErrorMessage = err.Error()
			return out
		}
	}
	out.Verified = true
	return out
}

func waitVCN(ctx context.Context, clients oci.Clients, id string, target core.VcnLifecycleStateEnum) error {
	deadline := time.Now().Add(5 * time.Minute)
	for {
		resp, err := clients.VirtualNetwork.GetVcn(ctx, core.GetVcnRequest{VcnId: common.String(id)})
		if err != nil {
			if target == core.VcnLifecycleStateTerminated && strings.Contains(strings.ToLower(err.Error()), "notfound") {
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
	deadline := time.Now().Add(5 * time.Minute)
	for {
		resp, err := clients.VirtualNetwork.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: common.String(id)})
		if err != nil {
			if target == core.SubnetLifecycleStateTerminated && strings.Contains(strings.ToLower(err.Error()), "notfound") {
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

func write(out result) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		log.Fatal(err)
	}
}

func env(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
