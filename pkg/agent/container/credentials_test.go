package container

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgentCredentialMounter(t *testing.T) {
	t.Run("WithBaseDir", func(t *testing.T) {
		mounter := NewAgentCredentialMounter("/custom/path")
		assert.NotNil(t, mounter)
		assert.Equal(t, "/custom/path", mounter.baseCredentialsDir)
	})

	t.Run("EmptyBaseDir", func(t *testing.T) {
		mounter := NewAgentCredentialMounter("")
		assert.NotNil(t, mounter)
		assert.Equal(t, "./credentials", mounter.baseCredentialsDir)
	})
}

func TestAgentCredentialMounter_Setup(t *testing.T) {
	t.Run("Success_WithBothCredentials", func(t *testing.T) {
		tmpDir := t.TempDir()
		mounter := NewAgentCredentialMounter(tmpDir)

		ctx := context.Background()
		sshKey := []byte("test-ssh-key-data")
		githubToken := []byte("test-github-token")

		files, err := mounter.Setup(ctx, "test-agent", sshKey, githubToken)
		require.NoError(t, err)
		require.NotNil(t, files)

		// Verify credentials directory was created
		assert.Contains(t, files.CredentialsDir, "agent-test-agent")
		_, err = os.Stat(files.CredentialsDir)
		assert.NoError(t, err, "Credentials directory should exist")

		// Verify SSH key file was created
		assert.NotEmpty(t, files.GitSSHKeyPath)
		assert.Contains(t, files.GitSSHKeyPath, "id_ed25519")
		content, err := os.ReadFile(files.GitSSHKeyPath)
		require.NoError(t, err)
		assert.Equal(t, sshKey, content)

		// Verify GitHub token file was created
		assert.NotEmpty(t, files.GitHubTokenPath)
		assert.Contains(t, files.GitHubTokenPath, "github-token")
		content, err = os.ReadFile(files.GitHubTokenPath)
		require.NoError(t, err)
		assert.Equal(t, githubToken, content)

		// Verify file permissions
		info, err := os.Stat(files.GitSSHKeyPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "SSH key should have 0600 permissions")

		info, err = os.Stat(files.GitHubTokenPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "Token should have 0600 permissions")

		// Verify directory permissions
		info, err = os.Stat(files.CredentialsDir)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), info.Mode().Perm(), "Credentials dir should have 0700 permissions")
	})

	t.Run("Success_OnlySSHKey", func(t *testing.T) {
		tmpDir := t.TempDir()
		mounter := NewAgentCredentialMounter(tmpDir)

		ctx := context.Background()
		sshKey := []byte("test-ssh-key-data")

		files, err := mounter.Setup(ctx, "test-agent", sshKey, nil)
		require.NoError(t, err)
		require.NotNil(t, files)

		assert.NotEmpty(t, files.GitSSHKeyPath)
		assert.Empty(t, files.GitHubTokenPath)

		// Verify SSH key exists
		_, err = os.Stat(files.GitSSHKeyPath)
		assert.NoError(t, err)
	})

	t.Run("Success_OnlyGitHubToken", func(t *testing.T) {
		tmpDir := t.TempDir()
		mounter := NewAgentCredentialMounter(tmpDir)

		ctx := context.Background()
		githubToken := []byte("test-github-token")

		files, err := mounter.Setup(ctx, "test-agent", nil, githubToken)
		require.NoError(t, err)
		require.NotNil(t, files)

		assert.Empty(t, files.GitSSHKeyPath)
		assert.NotEmpty(t, files.GitHubTokenPath)

		// Verify token exists
		_, err = os.Stat(files.GitHubTokenPath)
		assert.NoError(t, err)
	})

	t.Run("Success_NoCredentials", func(t *testing.T) {
		tmpDir := t.TempDir()
		mounter := NewAgentCredentialMounter(tmpDir)

		ctx := context.Background()

		files, err := mounter.Setup(ctx, "test-agent", nil, nil)
		require.NoError(t, err)
		require.NotNil(t, files)

		// Should create directory but no credential files
		assert.Empty(t, files.GitSSHKeyPath)
		assert.Empty(t, files.GitHubTokenPath)
		assert.NotEmpty(t, files.CredentialsDir)

		_, err = os.Stat(files.CredentialsDir)
		assert.NoError(t, err, "Directory should still be created")
	})

	t.Run("EmptyAgentID", func(t *testing.T) {
		tmpDir := t.TempDir()
		mounter := NewAgentCredentialMounter(tmpDir)

		ctx := context.Background()
		files, err := mounter.Setup(ctx, "", []byte("key"), []byte("token"))
		assert.Error(t, err)
		assert.Nil(t, files)
		assert.Equal(t, ErrInvalidAgentID, err)
	})

	t.Run("ContextCanceled", func(t *testing.T) {
		tmpDir := t.TempDir()
		mounter := NewAgentCredentialMounter(tmpDir)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		files, err := mounter.Setup(ctx, "test-agent", []byte("key"), []byte("token"))
		assert.Error(t, err)
		assert.Nil(t, files)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestAgentCredentialMounter_GetMounts(t *testing.T) {
	mounter := NewAgentCredentialMounter("./credentials")

	t.Run("WithBothCredentials", func(t *testing.T) {
		files := &CredentialFiles{
			GitSSHKeyPath:   "/path/to/ssh/key",
			GitHubTokenPath: "/path/to/token",
			CredentialsDir:  "/path/to/creds",
		}

		mounts := mounter.GetMounts(files)
		require.Len(t, mounts, 2)

		// Verify SSH key mount
		sshMount := mounts[0]
		assert.Equal(t, "/path/to/ssh/key", sshMount.Source)
		assert.Equal(t, "/root/.ssh/id_ed25519", sshMount.Target)
		assert.True(t, sshMount.ReadOnly, "SSH key should be read-only")

		// Verify token mount
		tokenMount := mounts[1]
		assert.Equal(t, "/path/to/token", tokenMount.Source)
		assert.Equal(t, "/root/.github-token", tokenMount.Target)
		assert.True(t, tokenMount.ReadOnly, "Token should be read-only")
	})

	t.Run("OnlySSHKey", func(t *testing.T) {
		files := &CredentialFiles{
			GitSSHKeyPath:  "/path/to/ssh/key",
			CredentialsDir: "/path/to/creds",
		}

		mounts := mounter.GetMounts(files)
		require.Len(t, mounts, 1)
		assert.Equal(t, "/root/.ssh/id_ed25519", mounts[0].Target)
	})

	t.Run("OnlyToken", func(t *testing.T) {
		files := &CredentialFiles{
			GitHubTokenPath: "/path/to/token",
			CredentialsDir:  "/path/to/creds",
		}

		mounts := mounter.GetMounts(files)
		require.Len(t, mounts, 1)
		assert.Equal(t, "/root/.github-token", mounts[0].Target)
	})

	t.Run("NoCredentials", func(t *testing.T) {
		files := &CredentialFiles{
			CredentialsDir: "/path/to/creds",
		}

		mounts := mounter.GetMounts(files)
		assert.Empty(t, mounts)
	})

	t.Run("NilFiles", func(t *testing.T) {
		mounts := mounter.GetMounts(nil)
		assert.Nil(t, mounts)
	})
}

func TestAgentCredentialMounter_Cleanup(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		mounter := NewAgentCredentialMounter(tmpDir)

		ctx := context.Background()

		// Setup credentials first
		sshKey := []byte("test-ssh-key")
		githubToken := []byte("test-token")
		files, err := mounter.Setup(ctx, "test-agent", sshKey, githubToken)
		require.NoError(t, err)

		// Verify credentials directory exists
		_, err = os.Stat(files.CredentialsDir)
		require.NoError(t, err)

		// Cleanup
		err = mounter.Cleanup(ctx, "test-agent")
		require.NoError(t, err)

		// Verify credentials directory is removed
		_, err = os.Stat(files.CredentialsDir)
		assert.True(t, os.IsNotExist(err), "Credentials directory should be removed")
	})

	t.Run("NonexistentDirectory", func(t *testing.T) {
		tmpDir := t.TempDir()
		mounter := NewAgentCredentialMounter(tmpDir)

		ctx := context.Background()

		// Cleanup non-existent credentials (should not error - idempotent)
		err := mounter.Cleanup(ctx, "nonexistent-agent")
		assert.NoError(t, err, "Cleanup should be idempotent")
	})

	t.Run("EmptyAgentID", func(t *testing.T) {
		tmpDir := t.TempDir()
		mounter := NewAgentCredentialMounter(tmpDir)

		ctx := context.Background()
		err := mounter.Cleanup(ctx, "")
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidAgentID, err)
	})

	t.Run("ContextCanceled", func(t *testing.T) {
		tmpDir := t.TempDir()
		mounter := NewAgentCredentialMounter(tmpDir)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := mounter.Cleanup(ctx, "test-agent")
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("Multiple_Cleanup_Idempotent", func(t *testing.T) {
		tmpDir := t.TempDir()
		mounter := NewAgentCredentialMounter(tmpDir)

		ctx := context.Background()

		// Setup
		files, err := mounter.Setup(ctx, "test-agent", []byte("key"), nil)
		require.NoError(t, err)

		// First cleanup
		err = mounter.Cleanup(ctx, "test-agent")
		require.NoError(t, err)

		// Second cleanup (should not error)
		err = mounter.Cleanup(ctx, "test-agent")
		assert.NoError(t, err, "Multiple cleanups should be idempotent")

		// Third cleanup (should still not error)
		err = mounter.Cleanup(ctx, "test-agent")
		assert.NoError(t, err)

		// Verify still doesn't exist
		_, err = os.Stat(files.CredentialsDir)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestAgentCredentialMounter_Isolation(t *testing.T) {
	// Test that multiple agents don't interfere with each other
	tmpDir := t.TempDir()
	mounter := NewAgentCredentialMounter(tmpDir)

	ctx := context.Background()

	// Setup credentials for agent 1
	files1, err := mounter.Setup(ctx, "agent-1", []byte("key-1"), []byte("token-1"))
	require.NoError(t, err)

	// Setup credentials for agent 2
	files2, err := mounter.Setup(ctx, "agent-2", []byte("key-2"), []byte("token-2"))
	require.NoError(t, err)

	// Verify both exist and are different
	assert.NotEqual(t, files1.CredentialsDir, files2.CredentialsDir)
	assert.NotEqual(t, files1.GitSSHKeyPath, files2.GitSSHKeyPath)

	// Verify agent 1 credentials
	content, err := os.ReadFile(files1.GitSSHKeyPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("key-1"), content)

	// Verify agent 2 credentials
	content, err = os.ReadFile(files2.GitSSHKeyPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("key-2"), content)

	// Cleanup agent 1
	err = mounter.Cleanup(ctx, "agent-1")
	require.NoError(t, err)

	// Verify agent 1 is gone but agent 2 still exists
	_, err = os.Stat(files1.CredentialsDir)
	assert.True(t, os.IsNotExist(err), "Agent 1 should be removed")

	_, err = os.Stat(files2.CredentialsDir)
	assert.NoError(t, err, "Agent 2 should still exist")

	// Cleanup agent 2
	err = mounter.Cleanup(ctx, "agent-2")
	require.NoError(t, err)

	_, err = os.Stat(files2.CredentialsDir)
	assert.True(t, os.IsNotExist(err), "Agent 2 should be removed")
}
