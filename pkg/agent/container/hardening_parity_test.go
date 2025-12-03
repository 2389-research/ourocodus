package container_test

import (
	"testing"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/containersession"
)

// TestRuntimeHardeningAlias verifies that container.RuntimeHardening
// is an alias to containersession.RuntimeHardening, ensuring type compatibility.
func TestRuntimeHardeningAlias(t *testing.T) {
	// Create a container.RuntimeHardening with all fields set
	src := container.RuntimeHardening{
		ReadOnlyRootfs:  true,
		DropAllCaps:     true,
		NoNewPrivileges: true,
		MemoryLimitMB:   4096,
		CPULimit:        4.0,
		TmpfsSizeMB:     512,
	}

	// Since RuntimeHardening is a type alias, we can assign directly
	// without conversion. This line will fail to compile if the alias breaks.
	var dst containersession.RuntimeHardening = src

	// Verify the values are identical (this also proves the types are compatible)
	if dst.ReadOnlyRootfs != true {
		t.Error("ReadOnlyRootfs should be true")
	}
	if dst.DropAllCaps != true {
		t.Error("DropAllCaps should be true")
	}
	if dst.NoNewPrivileges != true {
		t.Error("NoNewPrivileges should be true")
	}
	if dst.MemoryLimitMB != 4096 {
		t.Errorf("MemoryLimitMB should be 4096, got %d", dst.MemoryLimitMB)
	}
	if dst.CPULimit != 4.0 {
		t.Errorf("CPULimit should be 4.0, got %f", dst.CPULimit)
	}
	if dst.TmpfsSizeMB != 512 {
		t.Errorf("TmpfsSizeMB should be 512, got %d", dst.TmpfsSizeMB)
	}
}

// TestRuntimeHardeningBidirectional verifies assignment works both directions
func TestRuntimeHardeningBidirectional(t *testing.T) {
	// containersession -> container (via alias)
	session := containersession.RuntimeHardening{
		ReadOnlyRootfs: true,
		MemoryLimitMB:  2048,
	}
	var containerType container.RuntimeHardening = session
	if containerType.MemoryLimitMB != 2048 {
		t.Error("Assignment from containersession to container failed")
	}

	// container -> containersession (via alias)
	containerSrc := container.RuntimeHardening{
		CPULimit: 2.0,
	}
	var sessionDst containersession.RuntimeHardening = containerSrc
	if sessionDst.CPULimit != 2.0 {
		t.Error("Assignment from container to containersession failed")
	}
}
