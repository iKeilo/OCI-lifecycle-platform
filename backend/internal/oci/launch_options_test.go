package oci

import (
	"testing"

	"a-series-oracle/backend/internal/domain"
)

func TestShapeOptionsFingerprintIgnoresOrder(t *testing.T) {
	shapes := []domain.ShapeOption{
		{Name: "VM.Standard.E3.Flex", Arch: "x86", MinOCPUs: 1, MaxOCPUs: 64, MinMemoryGB: 1, MaxMemoryGB: 1024},
		{Name: "VM.Standard.A1.Flex", Arch: "arm", MinOCPUs: 1, MaxOCPUs: 4, MinMemoryGB: 1, MaxMemoryGB: 24},
	}
	reversed := []domain.ShapeOption{shapes[1], shapes[0]}

	if got, want := shapeOptionsFingerprint(shapes), shapeOptionsFingerprint(reversed); got == "" || got != want {
		t.Fatalf("fingerprint should be stable across ordering, got %q want %q", got, want)
	}
}

func TestImagesForShapeUsesBoundCatalog(t *testing.T) {
	shapeImages := map[string][]domain.LaunchOption{
		"VM.Standard.E3.Flex": {{ID: "image-e3", Label: "E3 image"}},
		"VM.Standard.A1.Flex": {{ID: "image-a1", Label: "A1 image"}},
	}
	images := imagesForShape("VM.Standard.A1.Flex", []domain.ShapeOption{{Name: "VM.Standard.E3.Flex"}, {Name: "VM.Standard.A1.Flex"}}, shapeImages)

	if len(images) != 1 || images[0].ID != "image-a1" {
		t.Fatalf("expected A1-bound image, got %#v", images)
	}
}

func TestLaunchCatalogKeyIncludesContext(t *testing.T) {
	req := LaunchOptionsRequest{
		ProfileID:          "profile-a",
		Region:             "ap-chuncheon-1",
		AvailabilityDomain: "ad-1",
	}
	first := launchCatalogKey(req, "compartment-a")
	second := launchCatalogKey(req, "compartment-b")

	if first == "" || second == "" || first == second {
		t.Fatalf("expected distinct non-empty catalog keys, got first=%q second=%q", first, second)
	}
}
