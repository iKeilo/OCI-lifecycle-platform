package profileconfig

import (
	"errors"
	"strings"

	"a-series-oracle/backend/internal/domain"
	"a-series-oracle/backend/internal/oci"
	"a-series-oracle/backend/internal/store"
)

var ErrProfileDisabled = errors.New("profile is disabled")

type Resolver struct {
	store    *store.Store
	fallback oci.ReadinessConfig
}

func NewResolver(store *store.Store, fallback oci.ReadinessConfig) *Resolver {
	return &Resolver{store: store, fallback: fallback}
}

func (r *Resolver) Resolve(profileID string, region string) (oci.ReadinessConfig, domain.Profile, error) {
	if r == nil || r.store == nil {
		return r.fallback, envProfile(r.fallback), nil
	}

	profile, ok := r.findProfile(profileID)
	if !ok {
		if strings.TrimSpace(profileID) == "" {
			profile = envProfile(r.fallback)
			if profile.ID == "" {
				return oci.ReadinessConfig{}, domain.Profile{}, store.ErrNotFound
			}
		} else {
			return oci.ReadinessConfig{}, domain.Profile{}, store.ErrNotFound
		}
	}
	if strings.EqualFold(profile.Status, "Disabled") {
		return oci.ReadinessConfig{}, profile, ErrProfileDisabled
	}

	cfg := oci.ReadinessConfig{
		ExecutionMode: r.fallback.ExecutionMode,
		TenancyOCID:   strings.TrimSpace(profile.TenancyOCID),
		UserOCID:      strings.TrimSpace(profile.UserOCID),
		Fingerprint:   strings.TrimSpace(profile.Fingerprint),
		Region:        firstNonEmpty(region, profile.DefaultRegion, r.fallback.Region),
	}

	if secret, err := r.store.GetProfileSecret(profile.ID); err == nil {
		cfg.PrivateKey = secret.PrivateKey
		cfg.PrivateKeyFile = secret.PrivateKeyFile
	} else if profileMatchesFallback(profile, r.fallback) {
		cfg.PrivateKey = r.fallback.PrivateKey
		cfg.PrivateKeyFile = r.fallback.PrivateKeyFile
	}

	return cfg, profile, nil
}

func (r *Resolver) findProfile(profileID string) (domain.Profile, bool) {
	profileID = strings.TrimSpace(profileID)
	if profileID != "" {
		return r.store.GetProfile(profileID)
	}

	profiles := r.store.ListProfiles()
	for _, profile := range profiles {
		if strings.EqualFold(profile.Name, "DEFAULT") && !strings.EqualFold(profile.Status, "Disabled") {
			return profile, true
		}
	}
	for _, profile := range profiles {
		if !strings.EqualFold(profile.Status, "Disabled") {
			return profile, true
		}
	}
	if len(profiles) > 0 {
		return profiles[0], true
	}
	return domain.Profile{}, false
}

func envProfile(cfg oci.ReadinessConfig) domain.Profile {
	if cfg.TenancyOCID == "" || cfg.UserOCID == "" || cfg.Fingerprint == "" {
		return domain.Profile{}
	}
	return domain.Profile{
		ID:            "env-default",
		Name:          "ENV DEFAULT",
		TenancyOCID:   cfg.TenancyOCID,
		UserOCID:      cfg.UserOCID,
		Fingerprint:   cfg.Fingerprint,
		DefaultRegion: cfg.Region,
		Status:        "Configured",
	}
}

func profileMatchesFallback(profile domain.Profile, cfg oci.ReadinessConfig) bool {
	if profile.ID == "env-default" {
		return true
	}
	return profile.TenancyOCID == cfg.TenancyOCID &&
		profile.UserOCID == cfg.UserOCID &&
		profile.Fingerprint == cfg.Fingerprint
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
