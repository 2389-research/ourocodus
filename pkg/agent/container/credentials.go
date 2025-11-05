package container

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/mount"
)

// AgentCredentialMounter manages credential files for agent containers.
//
// Credentials are:
//   - Created in a secure directory with restricted permissions
//   - Mounted read-only into containers
//   - Cleaned up when containers are stopped
//
// Thread-safety: AgentCredentialMounter is safe for concurrent use.
type AgentCredentialMounter struct {
	baseCredentialsDir string
}

// NewAgentCredentialMounter creates a credential mounter.
//
// The baseCredentialsDir is where credential files will be stored.
// Each agent gets a subdirectory: {baseCredentialsDir}/agent-{agentID}/
//
// If baseCredentialsDir is empty, defaults to "./credentials".
func NewAgentCredentialMounter(baseCredentialsDir string) *AgentCredentialMounter {
	if baseCredentialsDir == "" {
		baseCredentialsDir = "./credentials"
	}

	return &AgentCredentialMounter{
		baseCredentialsDir: baseCredentialsDir,
	}
}

// CredentialFiles contains paths to credential files for mounting.
type CredentialFiles struct {
	// GitSSHKeyPath is the path to the SSH private key file (empty if not provided)
	GitSSHKeyPath string

	// GitHubTokenPath is the path to the GitHub token file (empty if not provided)
	GitHubTokenPath string

	// CredentialsDir is the directory containing all credential files
	CredentialsDir string
}

// Setup creates credential files for an agent and returns their paths.
//
// Parameters:
//   - ctx: Context for cancellation
//   - agentID: Unique identifier for the agent
//   - gitSSHKey: SSH private key data (optional, can be nil)
//   - githubToken: GitHub token data (optional, can be nil)
//
// Returns:
//   - CredentialFiles: Paths to created credential files
//   - error: Non-nil if creation fails
//
// The credentials directory is created with 0700 permissions (owner-only access).
// Credential files are created with 0600 permissions (owner read/write only).
func (m *AgentCredentialMounter) Setup(ctx context.Context, agentID string, gitSSHKey, githubToken []byte) (*CredentialFiles, error) {
	if agentID == "" {
		return nil, ErrInvalidAgentID
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Create credentials directory
	credDir := filepath.Join(m.baseCredentialsDir, fmt.Sprintf("agent-%s", agentID))
	if err := os.MkdirAll(credDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create credentials directory: %w", err)
	}

	files := &CredentialFiles{
		CredentialsDir: credDir,
	}

	// Write SSH key if provided
	if len(gitSSHKey) > 0 {
		sshKeyPath := filepath.Join(credDir, "id_ed25519")
		if err := os.WriteFile(sshKeyPath, gitSSHKey, 0o600); err != nil {
			return nil, fmt.Errorf("failed to write SSH key: %w", err)
		}
		files.GitSSHKeyPath = sshKeyPath
	}

	// Write GitHub token if provided
	if len(githubToken) > 0 {
		tokenPath := filepath.Join(credDir, "github-token")
		if err := os.WriteFile(tokenPath, githubToken, 0o600); err != nil {
			return nil, fmt.Errorf("failed to write GitHub token: %w", err)
		}
		files.GitHubTokenPath = tokenPath
	}

	return files, nil
}

// GetMounts returns Docker mount configurations for credential files.
//
// This creates read-only bind mounts for credentials in the container:
//   - SSH key: /root/.ssh/id_ed25519 (if provided)
//   - GitHub token: /root/.github-token (if provided)
//
// All mounts are read-only to prevent tampering from inside the container.
func (m *AgentCredentialMounter) GetMounts(files *CredentialFiles) []mount.Mount {
	if files == nil {
		return nil
	}

	var mounts []mount.Mount

	// Mount SSH key as read-only
	if files.GitSSHKeyPath != "" {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   files.GitSSHKeyPath,
			Target:   "/root/.ssh/id_ed25519",
			ReadOnly: true,
		})
	}

	// Mount GitHub token as read-only
	if files.GitHubTokenPath != "" {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   files.GitHubTokenPath,
			Target:   "/root/.github-token",
			ReadOnly: true,
		})
	}

	return mounts
}

// Cleanup removes credential files for an agent.
//
// This is idempotent - safe to call multiple times.
// Returns nil if credentials directory doesn't exist.
func (m *AgentCredentialMounter) Cleanup(ctx context.Context, agentID string) error {
	if agentID == "" {
		return ErrInvalidAgentID
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	credDir := filepath.Join(m.baseCredentialsDir, fmt.Sprintf("agent-%s", agentID))

	// Remove credentials directory and all contents
	if err := os.RemoveAll(credDir); err != nil {
		// Ignore if directory doesn't exist
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to cleanup credentials: %w", err)
	}

	return nil
}
