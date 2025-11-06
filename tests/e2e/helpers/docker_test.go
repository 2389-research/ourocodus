//go:build integration

package helpers_test

import (
	"context"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/tests/e2e/helpers"
)

func TestListAgentContainers(t *testing.T) {
	ctx := context.Background()

	// List containers (should work even if empty)
	containers, err := helpers.ListAgentContainers(ctx)
	if err != nil {
		t.Fatalf("ListAgentContainers failed: %v", err)
	}

	// Should return empty list initially (or existing containers from other tests)
	t.Logf("Found %d agent containers", len(containers))

	// Verify we got a valid slice (not nil)
	if containers == nil {
		t.Error("Expected non-nil containers slice")
	}
}

func TestDockerHelperCreation(t *testing.T) {
	// Test that we can create a Docker helper
	helper, err := helpers.NewDockerHelper()
	if err != nil {
		t.Fatalf("NewDockerHelper failed: %v", err)
	}
	defer helper.Close()

	// Verify helper is not nil
	if helper == nil {
		t.Fatal("Expected non-nil helper")
	}

	// Test Close
	if err := helper.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestVerifyContainerCleanup_NonExistent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test with a non-existent container ID
	fakeID := "nonexistent-container-id-12345"

	// Should return nil (success) since container doesn't exist
	err := helpers.VerifyContainerCleanup(ctx, fakeID)
	if err != nil {
		t.Errorf("VerifyContainerCleanup failed for non-existent container: %v", err)
	}
}

func TestGetContainerLogs_NonExistent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test with a non-existent container ID
	fakeID := "nonexistent-container-id-12345"

	// Should return error since container doesn't exist
	_, err := helpers.GetContainerLogs(ctx, fakeID)
	if err == nil {
		t.Error("Expected error for non-existent container, got nil")
	}
}

func TestInspectContainer_NonExistent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test with a non-existent container ID
	fakeID := "nonexistent-container-id-12345"

	// Should return error since container doesn't exist
	_, err := helpers.InspectContainer(ctx, fakeID)
	if err == nil {
		t.Error("Expected error for non-existent container, got nil")
	}
}
