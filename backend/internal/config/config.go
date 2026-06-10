package config

import (
	"os"
	"strconv"
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
	Email         EmailConfig
	Webhook       WebhookConfig
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

type EmailConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       []string
	UseTLS   bool
	StartTLS bool
}

type WebhookConfig struct {
	Enabled bool
	URL     string
	Secret  string
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
		Email: EmailConfig{
			Enabled:  envBool("SMTP_ENABLED", false),
			Host:     env("SMTP_HOST", ""),
			Port:     envInt("SMTP_PORT", 587),
			Username: env("SMTP_USERNAME", ""),
			Password: env("SMTP_PASSWORD", ""),
			From:     env("SMTP_FROM", ""),
			To:       envList("SMTP_TO"),
			UseTLS:   envBool("SMTP_USE_TLS", false),
			StartTLS: envBool("SMTP_STARTTLS", true),
		},
		Webhook: WebhookConfig{
			Enabled: envBool("WEBHOOK_ENABLED", false),
			URL:     env("WEBHOOK_URL", ""),
			Secret:  env("WEBHOOK_SECRET", ""),
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

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envList(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
