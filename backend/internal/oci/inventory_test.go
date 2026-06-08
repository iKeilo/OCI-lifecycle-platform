package oci

import (
	"testing"

	"a-series-oracle/backend/internal/domain"

	"github.com/oracle/oci-go-sdk/v65/core"
)

func TestMapInstanceStatus(t *testing.T) {
	cases := []struct {
		state core.InstanceLifecycleStateEnum
		want  domain.InstanceStatus
	}{
		{state: core.InstanceLifecycleStateRunning, want: domain.InstanceRunning},
		{state: core.InstanceLifecycleStateStopped, want: domain.InstanceStopped},
		{state: core.InstanceLifecycleStateTerminated, want: domain.InstanceTerminated},
		{state: core.InstanceLifecycleStateStarting, want: domain.InstanceProvisioning},
	}

	for _, tc := range cases {
		if got := mapInstanceStatus(tc.state); got != tc.want {
			t.Fatalf("state %s mapped to %s, want %s", tc.state, got, tc.want)
		}
	}
}

func TestShapeConfigValues(t *testing.T) {
	ocpus := float32(1.5)
	memory := float32(6.2)
	gotOCPUs, gotMemory := shapeConfigValues(&core.InstanceShapeConfig{
		Ocpus:       &ocpus,
		MemoryInGBs: &memory,
	})

	if gotOCPUs != 2 || gotMemory != 7 {
		t.Fatalf("shape values = %d OCPU / %d GB, want 2 OCPU / 7 GB", gotOCPUs, gotMemory)
	}
}
