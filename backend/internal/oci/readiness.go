package oci

type ReadinessConfig struct {
	ExecutionMode  string
	TenancyOCID    string
	UserOCID       string
	Fingerprint    string
	PrivateKey     string
	PrivateKeyFile string
	Region         string
}

type Readiness struct {
	ExecutionMode string   `json:"executionMode"`
	Ready         bool     `json:"ready"`
	Missing       []string `json:"missing"`
	Message       string   `json:"message"`
}

func CheckReadiness(cfg ReadinessConfig) Readiness {
	missing := []string{}
	if cfg.ExecutionMode != "oci" {
		return Readiness{
			ExecutionMode: cfg.ExecutionMode,
			Ready:         false,
			Message:       "OCI execution mode is not enabled. Set OCI_EXECUTION_MODE=oci before real API execution.",
		}
	}
	if cfg.TenancyOCID == "" {
		missing = append(missing, "OCI_TENANCY_OCID")
	}
	if cfg.UserOCID == "" {
		missing = append(missing, "OCI_USER_OCID")
	}
	if cfg.Fingerprint == "" {
		missing = append(missing, "OCI_FINGERPRINT")
	}
	if cfg.PrivateKey == "" && cfg.PrivateKeyFile == "" {
		missing = append(missing, "OCI_PRIVATE_KEY or OCI_PRIVATE_KEY_FILE")
	}
	if cfg.Region == "" {
		missing = append(missing, "OCI_REGION")
	}
	if len(missing) > 0 {
		return Readiness{
			ExecutionMode: cfg.ExecutionMode,
			Ready:         false,
			Missing:       missing,
			Message:       "OCI credentials are incomplete. Real API calls are blocked until all required values are configured.",
		}
	}
	return Readiness{
		ExecutionMode: cfg.ExecutionMode,
		Ready:         true,
		Message:       "OCI configuration is present. Real API validation can proceed with OCI SDK calls.",
	}
}
