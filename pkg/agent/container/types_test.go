package container

import (
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/worktree"
	"github.com/stretchr/testify/assert"
)

func TestAgentContainerHandle_Accessors(t *testing.T) {
	now := time.Now()

	// Create a mock container session
	containerSess := containersession.NewContainerSession(
		"session-123",
		"/workspace/path",
		map[string]string{"label": "value"},
		now,
	)
	containerSess.SetContainerID("container-abc123")
	containerSess.SetState(containersession.StateRunning)

	// Create a mock worktree
	wt := &worktree.AgentWorktree{}
	// Note: We can't directly set private fields, so we'll test what we can

	// Create handle
	handle := &AgentContainerHandle{
		agentID:         "test-agent-123",
		containerSess:   containerSess,
		worktree:        wt,
		credentialsPath: "/credentials/test-agent-123",
		createdAt:       now,
	}

	// Test accessors
	t.Run("AgentID", func(t *testing.T) {
		assert.Equal(t, "test-agent-123", handle.AgentID())
	})

	t.Run("ContainerID", func(t *testing.T) {
		assert.Equal(t, "container-abc123", handle.ContainerID())
	})

	t.Run("CredentialsPath", func(t *testing.T) {
		assert.Equal(t, "/credentials/test-agent-123", handle.CredentialsPath())
	})

	t.Run("State", func(t *testing.T) {
		assert.Equal(t, containersession.StateRunning, handle.State())
	})

	t.Run("CreatedAt", func(t *testing.T) {
		assert.Equal(t, now, handle.CreatedAt())
	})

	t.Run("ContainerSession", func(t *testing.T) {
		assert.Equal(t, containerSess, handle.ContainerSession())
	})

	t.Run("Worktree", func(t *testing.T) {
		assert.Equal(t, wt, handle.Worktree())
	})
}

func TestAgentContainerHandle_NilFields(t *testing.T) {
	// Test handle with nil fields (edge case)
	handle := &AgentContainerHandle{
		agentID:         "test-agent",
		containerSess:   nil,
		worktree:        nil,
		credentialsPath: "/creds",
		createdAt:       time.Now(),
	}

	t.Run("ContainerID_NilSession", func(t *testing.T) {
		// Should return empty string when containerSess is nil
		assert.Equal(t, "", handle.ContainerID())
	})

	t.Run("WorkspacePath_NilWorktree", func(t *testing.T) {
		// Should return empty string when worktree is nil
		assert.Equal(t, "", handle.WorkspacePath())
	})

	t.Run("BranchName_NilWorktree", func(t *testing.T) {
		// Should return empty string when worktree is nil
		assert.Equal(t, "", handle.BranchName())
	})

	t.Run("State_NilSession", func(t *testing.T) {
		// Should return StateFailed when containerSess is nil
		assert.Equal(t, containersession.StateFailed, handle.State())
	})
}

func TestSpawnConfig_Validation(t *testing.T) {
	// SpawnConfig is just a struct with no methods, but we can test
	// that it holds the expected data

	config := SpawnConfig{
		AgentID:     "coder-abc123",
		ImageName:   "ourocodus/agent:latest",
		Command:     []string{"/bin/bash"},
		GitSSHKey:   []byte("ssh-key-data"),
		GitHubToken: []byte("github-token-data"),
		Env:         []string{"FOO=bar", "BAZ=qux"},
	}

	assert.Equal(t, "coder-abc123", config.AgentID)
	assert.Equal(t, "ourocodus/agent:latest", config.ImageName)
	assert.Equal(t, []string{"/bin/bash"}, config.Command)
	assert.Equal(t, []byte("ssh-key-data"), config.GitSSHKey)
	assert.Equal(t, []byte("github-token-data"), config.GitHubToken)
	assert.Equal(t, []string{"FOO=bar", "BAZ=qux"}, config.Env)
}

func TestSpawnConfig_EmptyOptionalFields(t *testing.T) {
	// Test config with only required fields
	config := SpawnConfig{
		AgentID:   "coder-abc123",
		ImageName: "ourocodus/agent:latest",
		Command:   []string{"/bin/bash"},
		// Optional fields omitted
	}

	assert.Equal(t, "coder-abc123", config.AgentID)
	assert.Equal(t, "ourocodus/agent:latest", config.ImageName)
	assert.Equal(t, []string{"/bin/bash"}, config.Command)
	assert.Nil(t, config.GitSSHKey)
	assert.Nil(t, config.GitHubToken)
	assert.Nil(t, config.Env)
}

func TestCredentialFiles_EmptyFields(t *testing.T) {
	// Test CredentialFiles with empty optional fields
	files := &CredentialFiles{
		CredentialsDir: "/credentials/agent-123",
		// SSH key and token paths empty
	}

	assert.Equal(t, "/credentials/agent-123", files.CredentialsDir)
	assert.Empty(t, files.GitSSHKeyPath)
	assert.Empty(t, files.GitHubTokenPath)
}
