package config

import (
	"os"
	"strings"
)

type ExecutionMode string

const (
	ExecutionModeLocal ExecutionMode = "local"
	ExecutionModeOCI   ExecutionMode = "oci"
)

type Config struct {
	Port          string
	ExecutionMode ExecutionMode
	OCI           OCIConfig
	Database      DatabaseConfig
	Security      SecurityConfig
	StaticDir     string
}

type OCIConfig struct {
	TenancyOCID    string
	UserOCID       string
	Fingerprint    string
	PrivateKey     string
	PrivateKeyFile string
	Region         string
}

type DatabaseConfig struct {
	URL string
}

type SecurityConfig struct {
	ProfileKeyEncryptionKey string
	ProfileStoreFile        string
	PanelPasswordHash       string
	PanelPassword           string
	PanelSessionSecret      string
	PanelAuthDisabled       bool
}

func Load() Config {
	return Config{
		Port:          env("PORT", "8080"),
		ExecutionMode: executionMode(env("OCI_EXECUTION_MODE", string(ExecutionModeLocal))),
		StaticDir:     env("STATIC_DIR", ""),
		OCI: OCIConfig{
			TenancyOCID:    env("OCI_TENANCY_OCID", ""),
			UserOCID:       env("OCI_USER_OCID", ""),
			Fingerprint:    env("OCI_FINGERPRINT", ""),
			PrivateKey:     env("OCI_PRIVATE_KEY", ""),
			PrivateKeyFile: env("OCI_PRIVATE_KEY_FILE", ""),
			Region:         env("OCI_REGION", ""),
		},
		Database: DatabaseConfig{
			URL: env("DATABASE_URL", ""),
		},
		Security: SecurityConfig{
			ProfileKeyEncryptionKey: env("PROFILE_KEY_ENCRYPTION_KEY", ""),
			ProfileStoreFile:        env("PROFILE_STORE_FILE", ""),
			PanelPasswordHash:       env("PANEL_PASSWORD_HASH", ""),
			PanelPassword:           env("PANEL_PASSWORD", ""),
			PanelSessionSecret:      env("PANEL_SESSION_SECRET", ""),
			PanelAuthDisabled:       envBool("PANEL_AUTH_DISABLED", false),
		},
	}
}

func executionMode(value string) ExecutionMode {
	if value == string(ExecutionModeOCI) {
		return ExecutionModeOCI
	}
	return ExecutionModeLocal
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
