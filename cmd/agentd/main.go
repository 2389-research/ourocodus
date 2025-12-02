package main

import (
	"fmt"
	"os"

	"github.com/2389-research/ourocodus/pkg/cli"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "agentd",
	Short: "✨ agentd - Multi-agent isolation orchestrator",
	Long: `agentd demonstrates Ourocodus's three-layer isolation architecture:

  🌳 Git worktrees isolate code
  📦 Docker containers isolate processes
  🔑 Credential volumes isolate access

Multiple agents work concurrently without conflicts.`,
	Version: Version,
}

func main() {
	app := cli.NewApp(rootCmd)
	os.Exit(app.Execute())
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(spawnCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(executeCmd)
	rootCmd.AddCommand(replCmd)
}

// printError is a shared helper for consistent error output formatting.
// Used by legacy (non-TUI) code paths that haven't been migrated to Output interface yet.
func printError(msg string, th *theme.Theme) {
	fmt.Print(th.ErrorText.Render("× "))
	fmt.Println(msg)
}
