package oci

import "testing"

func TestCheckReadinessBlocksLocalMode(t *testing.T) {
	result := CheckReadiness(ReadinessConfig{ExecutionMode: "local"})
	if result.Ready {
		t.Fatal("local mode must not be ready for real OCI API execution")
	}
	if result.Message == "" {
		t.Fatal("expected readiness message")
	}
}

func TestCheckReadinessReportsMissingOCIConfig(t *testing.T) {
	result := CheckReadiness(ReadinessConfig{ExecutionMode: "oci", TenancyOCID: "tenancy"})
	if result.Ready {
		t.Fatal("incomplete OCI config must not be ready")
	}
	if len(result.Missing) == 0 {
		t.Fatal("expected missing fields")
	}
}

func TestCheckReadinessReadyWhenAllOCIConfigPresent(t *testing.T) {
	result := CheckReadiness(ReadinessConfig{
		ExecutionMode: "oci",
		TenancyOCID:   "tenancy",
		UserOCID:      "user",
		Fingerprint:   "fingerprint",
		PrivateKey:    "private-key",
		Region:        "region",
	})
	if !result.Ready {
		t.Fatalf("expected ready config, got %#v", result)
	}
}
