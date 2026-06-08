package fileprofile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"a-series-oracle/backend/internal/domain"
)

func TestSinkEncryptsAndReloadsInlinePrivateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	sink, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.SetProfileKeyEncryptionKey("12345678901234567890123456789012"); err != nil {
		t.Fatal(err)
	}

	profile := domain.Profile{
		ID:            "profile-default",
		Name:          "DEFAULT",
		TenancyOCID:   "ocid1.tenancy.oc1..example",
		UserOCID:      "ocid1.user.oc1..example",
		Fingerprint:   "01:02:03",
		DefaultRegion: "ap-chuncheon-1",
		Status:        "Pending",
		LastCheckedAt: time.Now().UTC(),
	}
	privateKey := "-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----"
	if err := sink.SaveProfile(profile, domain.ProfileSecret{PrivateKey: privateKey}); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), privateKey) || strings.Contains(string(raw), "secret") {
		t.Fatalf("profile store must not contain plaintext private key: %s", string(raw))
	}

	reloaded, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := reloaded.SetProfileKeyEncryptionKey("12345678901234567890123456789012"); err != nil {
		t.Fatal(err)
	}
	profiles, err := reloaded.ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].ID != profile.ID {
		t.Fatalf("unexpected profiles: %#v", profiles)
	}
	secret, err := reloaded.GetProfileSecret(profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if secret.PrivateKey != privateKey {
		t.Fatalf("decrypted key mismatch")
	}
}

func TestSinkPreservesSecretOnProfileStatusUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	sink, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.SetProfileKeyEncryptionKey("12345678901234567890123456789012"); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{ID: "profile-default", Name: "DEFAULT", TenancyOCID: "t", UserOCID: "u", Fingerprint: "f", DefaultRegion: "ap-chuncheon-1", Status: "Pending"}
	if err := sink.SaveProfile(profile, domain.ProfileSecret{PrivateKeyFile: `E:\keys\oci.pem`}); err != nil {
		t.Fatal(err)
	}
	profile.Status = "Healthy"
	if err := sink.SaveProfile(profile, domain.ProfileSecret{}); err != nil {
		t.Fatal(err)
	}
	secret, err := sink.GetProfileSecret(profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if secret.PrivateKeyFile != `E:\keys\oci.pem` {
		t.Fatalf("expected private key file to be preserved, got %q", secret.PrivateKeyFile)
	}
}

func TestSinkDeleteProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	sink, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{ID: "profile-default", Name: "DEFAULT", TenancyOCID: "t", UserOCID: "u", Fingerprint: "f", DefaultRegion: "ap-chuncheon-1", Status: "Pending"}
	if err := sink.SaveProfile(profile, domain.ProfileSecret{}); err != nil {
		t.Fatal(err)
	}
	if err := sink.DeleteProfile(profile.ID); err != nil {
		t.Fatal(err)
	}
	profiles, err := sink.ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 0 {
		t.Fatalf("expected no profiles, got %#v", profiles)
	}
}
