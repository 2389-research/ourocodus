package main

const (
	// LabelNamespace identifies agentd-managed containers
	LabelNamespace = "org.ourocodus.agentd"

	// LabelAgentID stores the agent identifier
	LabelAgentID = "agentd.id"

	// LabelRepoHash stores the repository hash
	LabelRepoHash = "agentd.repo"

	// LabelWorktreePath stores the worktree path
	LabelWorktreePath = "agentd.worktree"

	// LabelVersion stores the agentd version
	LabelVersion = "agentd.version"

	// Version is the current agentd version
	Version = "0.1.0"
)

// BuildLabels creates the label map for a container
func BuildLabels(agentID, repoHash, worktreePath string) map[string]string {
	return map[string]string{
		LabelNamespace:    "true",
		LabelAgentID:      agentID,
		LabelRepoHash:     repoHash,
		LabelWorktreePath: worktreePath,
		LabelVersion:      Version,
	}
}
