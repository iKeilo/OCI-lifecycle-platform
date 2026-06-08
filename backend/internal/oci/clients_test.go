package oci

import (
	"errors"
	"testing"
)

func TestNewClientsRequiresReadiness(t *testing.T) {
	_, err := NewClients(ReadinessConfig{ExecutionMode: "local"})
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("expected ErrNotReady, got %v", err)
	}
}
