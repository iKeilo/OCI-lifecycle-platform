package oci

import (
	"strings"
	"testing"
)

func TestFirewallRuleDescriptionUsesUserNote(t *testing.T) {
	got := firewallRuleDescription("tcp", 80, 80, "Web service")
	if got != "Web service" {
		t.Fatalf("description = %q, want user note", got)
	}
}

func TestFirewallRuleDescriptionFallsBackWhenNoteEmpty(t *testing.T) {
	got := firewallRuleDescription("tcp", 80, 80, "   ")
	if got != "Managed by OCI lifecycle platform: allow tcp/80" {
		t.Fatalf("description = %q, want fallback", got)
	}
}

func TestFirewallRuleDescriptionNormalizesAndLimitsNote(t *testing.T) {
	got := firewallRuleDescription("udp", 1000, 1001, strings.Repeat("a", 260)+"\nignored")
	if len(got) != 255 {
		t.Fatalf("description length = %d, want 255", len(got))
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("description should not contain newlines: %q", got)
	}
}
