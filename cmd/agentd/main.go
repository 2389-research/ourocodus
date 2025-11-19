package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "agentd",
	Short: "agentd - Multi-agent isolation orchestrator",
	Long: `agentd demonstrates Ourocodus's three-layer isolation architecture:
  - Git worktrees isolate code
  - Docker containers isolate processes
  - Credential volumes isolate access

Multiple agents work concurrently without conflicts.`,
	Version: Version,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(spawnCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(logsCmd)
}
