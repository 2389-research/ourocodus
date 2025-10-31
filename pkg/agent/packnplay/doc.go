// Package packnplay provides a Packnplay-based implementation of the agent.AgentLauncher interface.
//
// This package will implement AgentLauncher using Packnplay's library for Docker containerization,
// git worktree management, and credential mounting.
//
// The implementation is tracked in issue #83.
package packnplay

// Verify Packnplay packages are accessible.
// These imports will be used in the actual implementation.
import (
	_ "github.com/obra/packnplay/pkg/docker"
	_ "github.com/obra/packnplay/pkg/git"
	_ "github.com/obra/packnplay/pkg/runner"
)
