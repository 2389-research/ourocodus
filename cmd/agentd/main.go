package main

import (
	"os"

	"github.com/2389-research/ourocodus/pkg/cli"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "agentd",
	Short: "✨ agentd - Multi-agent isolation orchestrator",
	Long: func() string {
		th := theme.NewRetroTheme(theme.PaletteCGA)
		bannerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Primary))).Bold(true)
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Muted)))
		return bannerStyle.Render("agentd") + " demonstrates Ourocodus's three-layer isolation architecture:\n\n" +
			"  🌳 Git worktrees isolate code\n" +
			"  📦 Docker containers isolate processes\n" +
			"  🔑 Credential volumes isolate access\n\n" +
			mutedStyle.Render("Multiple agents work concurrently without conflicts.")
	}(),
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
