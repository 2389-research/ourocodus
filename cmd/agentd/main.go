package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	// Banner colors
	bannerColor  = color.New(color.FgCyan, color.Bold)
	taglineColor = color.New(color.FgHiBlack)
)

var rootCmd = &cobra.Command{
	Use:   "agentd",
	Short: "✨ agentd - Multi-agent isolation orchestrator",
	Long: bannerColor.Sprint("agentd") + " demonstrates Ourocodus's three-layer isolation architecture:\n\n" +
		"  🌳 Git worktrees isolate code\n" +
		"  📦 Docker containers isolate processes\n" +
		"  🔑 Credential volumes isolate access\n\n" +
		taglineColor.Sprint("Multiple agents work concurrently without conflicts."),
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
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(sendCmd)
	rootCmd.AddCommand(replCmd)
}
