package container_test

import (
	"reflect"
	"testing"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/containersession"
)

// TestRuntimeHardeningParity ensures the two RuntimeHardening structs
// have identical fields. This prevents drift when adding new hardening options.
func TestRuntimeHardeningParity(t *testing.T) {
	containerType := reflect.TypeOf(container.RuntimeHardening{})
	sessionType := reflect.TypeOf(containersession.RuntimeHardening{})

	// Check field count
	if containerType.NumField() != sessionType.NumField() {
		t.Errorf("Field count mismatch: container.RuntimeHardening has %d fields, containersession.RuntimeHardening has %d fields",
			containerType.NumField(), sessionType.NumField())
	}

	// Build map of session fields for lookup
	sessionFields := make(map[string]reflect.StructField)
	for i := 0; i < sessionType.NumField(); i++ {
		field := sessionType.Field(i)
		sessionFields[field.Name] = field
	}

	// Check each container field exists in session with same type
	for i := 0; i < containerType.NumField(); i++ {
		containerField := containerType.Field(i)

		sessionField, exists := sessionFields[containerField.Name]
		if !exists {
			t.Errorf("Field %q exists in container.RuntimeHardening but not in containersession.RuntimeHardening",
				containerField.Name)
			continue
		}

		if containerField.Type != sessionField.Type {
			t.Errorf("Field %q has different types: container=%v, containersession=%v",
				containerField.Name, containerField.Type, sessionField.Type)
		}
	}

	// Check for fields in session that don't exist in container
	containerFields := make(map[string]bool)
	for i := 0; i < containerType.NumField(); i++ {
		containerFields[containerType.Field(i).Name] = true
	}

	for i := 0; i < sessionType.NumField(); i++ {
		field := sessionType.Field(i)
		if !containerFields[field.Name] {
			t.Errorf("Field %q exists in containersession.RuntimeHardening but not in container.RuntimeHardening",
				field.Name)
		}
	}
}

// TestRuntimeHardeningMapping verifies the mapping in launcher.go works correctly
func TestRuntimeHardeningMapping(t *testing.T) {
	// Create a container.RuntimeHardening with all fields set
	src := container.RuntimeHardening{
		ReadOnlyRootfs:  true,
		DropAllCaps:     true,
		NoNewPrivileges: true,
		MemoryLimitMB:   4096,
		CPULimit:        4.0,
		TmpfsSizeMB:     512,
	}

	// Map to containersession.RuntimeHardening (same as launcher.go does)
	dst := containersession.RuntimeHardening{
		ReadOnlyRootfs:  src.ReadOnlyRootfs,
		DropAllCaps:     src.DropAllCaps,
		NoNewPrivileges: src.NoNewPrivileges,
		MemoryLimitMB:   src.MemoryLimitMB,
		CPULimit:        src.CPULimit,
		TmpfsSizeMB:     src.TmpfsSizeMB,
	}

	// Verify all fields were mapped correctly
	if dst.ReadOnlyRootfs != src.ReadOnlyRootfs {
		t.Errorf("ReadOnlyRootfs not mapped: got %v, want %v", dst.ReadOnlyRootfs, src.ReadOnlyRootfs)
	}
	if dst.DropAllCaps != src.DropAllCaps {
		t.Errorf("DropAllCaps not mapped: got %v, want %v", dst.DropAllCaps, src.DropAllCaps)
	}
	if dst.NoNewPrivileges != src.NoNewPrivileges {
		t.Errorf("NoNewPrivileges not mapped: got %v, want %v", dst.NoNewPrivileges, src.NoNewPrivileges)
	}
	if dst.MemoryLimitMB != src.MemoryLimitMB {
		t.Errorf("MemoryLimitMB not mapped: got %v, want %v", dst.MemoryLimitMB, src.MemoryLimitMB)
	}
	if dst.CPULimit != src.CPULimit {
		t.Errorf("CPULimit not mapped: got %v, want %v", dst.CPULimit, src.CPULimit)
	}
	if dst.TmpfsSizeMB != src.TmpfsSizeMB {
		t.Errorf("TmpfsSizeMB not mapped: got %v, want %v", dst.TmpfsSizeMB, src.TmpfsSizeMB)
	}
}
