package render

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/detect"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// AgentInfo represents an agent for rendering
type AgentInfo struct {
	AgentID     string
	ContainerID string
	Status      string
	Workspace   string
	SpawnSource string
	AttachedTo  string
	CreatedAt   time.Time
}

// RenderAgentList renders a list of agents in the specified output mode
func RenderAgentList(w io.Writer, agents []AgentInfo, mode output.Mode, th *theme.RetroTheme) error {
	if len(agents) == 0 {
		return renderEmptyList(w, mode, th)
	}

	switch {
	case mode.IsJSON():
		return renderJSON(w, agents)
	case mode.IsPlain():
		return renderPlainTable(w, agents)
	case mode.IsRich():
		if th == nil {
			th = theme.NewRetroTheme(theme.PaletteCGA)
		}
		return renderRichTable(w, agents, th)
	default:
		return renderPlainTable(w, agents)
	}
}

func renderEmptyList(w io.Writer, mode output.Mode, th *theme.RetroTheme) error {
	if mode.IsJSON() {
		return json.NewEncoder(w).Encode([]AgentInfo{})
	}

	msg := "✨ No agents running."
	if mode.IsRich() && th != nil {
		mutedStyle := lipgloss.NewStyle().Foreground(th.Muted)
		msg = mutedStyle.Render(msg)
	}

	_, err := fmt.Fprintln(w, msg)
	return err
}

func renderJSON(w io.Writer, agents []AgentInfo) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(agents)
}

func renderPlainTable(w io.Writer, agents []AgentInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	// Header
	_, _ = fmt.Fprintln(tw)
	_, _ = fmt.Fprintf(tw, "AGENT\tSTATUS\tSOURCE\tATTACHED TO\tWORKSPACE\tCREATED\n")

	// Rows
	for _, agent := range agents {
		attachedTo := "-"
		if agent.AttachedTo != "" {
			attachedTo = formatShortID(agent.AttachedTo, 9)
		}

		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			agent.AgentID,
			agent.Status,
			agent.SpawnSource,
			attachedTo,
			formatWorkspace(agent.Workspace),
			formatDuration(time.Since(agent.CreatedAt)),
		)
	}

	_, _ = fmt.Fprintln(tw)
	return tw.Flush()
}

func renderRichTable(w io.Writer, agents []AgentInfo, th *theme.RetroTheme) error {
	// Detect unicode support
	supportsUnicode := detect.SupportsUnicode()

	// Header with theme styling
	header := th.Header.Render(theme.GetLogo(theme.LogoSmall))
	_, _ = fmt.Fprintln(w, header)
	_, _ = fmt.Fprintln(w)

	// Define table columns with proper widths
	columns := []table.Column{
		{Title: "AGENT", Width: 20},
		{Title: "STATUS", Width: 15},
		{Title: "SOURCE", Width: 12},
		{Title: "ATTACHED TO", Width: 15},
		{Title: "CREATED", Width: 12},
	}

	// Build table rows
	rows := make([]table.Row, 0, len(agents))
	for _, agent := range agents {
		statusIcon := getStatusIcon(agent.Status, supportsUnicode)
		statusColor := getStatusColor(agent.Status, th)

		// Format attached session
		attachedTo := "─"
		if agent.AttachedTo != "" {
			attachedTo = formatShortID(agent.AttachedTo, 9)
		}

		// Apply styling to cell content
		sourceStyle := getSourceStyle(agent.SpawnSource, th)
		mutedStyle := lipgloss.NewStyle().Foreground(th.Muted)

		rows = append(rows, table.Row{
			agent.AgentID,
			statusIcon + " " + statusColor.Render(agent.Status),
			sourceStyle.Render(agent.SpawnSource),
			attachedTo,
			mutedStyle.Render(formatDuration(time.Since(agent.CreatedAt))),
		})
	}

	// Create and configure the table
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(false),
	)

	// Apply theme styling to table
	tableStyle := table.DefaultStyles()
	tableStyle.Header = tableStyle.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(th.Primary).
		BorderBottom(true).
		Bold(true).
		Foreground(th.Primary)
	tableStyle.Selected = tableStyle.Selected.
		Foreground(th.Accent).
		Bold(false)
	tableStyle.Cell = tableStyle.Cell.
		Foreground(th.Accent)

	t.SetStyles(tableStyle)

	// Render the table
	_, _ = fmt.Fprintln(w, t.View())
	_, _ = fmt.Fprintln(w)

	// Footer with summary
	summaryStyle := lipgloss.NewStyle().Foreground(th.Secondary)
	summary := fmt.Sprintf("Total: %d agents", len(agents))
	_, _ = fmt.Fprintln(w, summaryStyle.Render(summary))

	return nil
}

func getStatusIcon(status string, unicode bool) string {
	switch status {
	case "running":
		return theme.GetAgentStatusIcon(theme.StatusRunning, unicode)
	case "paused":
		return theme.GetAgentStatusIcon(theme.StatusPaused, unicode)
	case "exited", "stopped":
		return theme.GetAgentStatusIcon(theme.StatusStopped, unicode)
	default:
		return theme.GetAgentStatusIcon(theme.StatusIdle, unicode)
	}
}

func getStatusColor(status string, th *theme.RetroTheme) lipgloss.Style {
	switch status {
	case "running":
		return lipgloss.NewStyle().Foreground(th.Success)
	case "paused":
		return lipgloss.NewStyle().Foreground(th.Warning)
	case "exited", "stopped":
		return lipgloss.NewStyle().Foreground(th.Error)
	default:
		return lipgloss.NewStyle().Foreground(th.Muted)
	}
}

func getSourceStyle(source string, th *theme.RetroTheme) lipgloss.Style {
	switch source {
	case "cli":
		return lipgloss.NewStyle().Foreground(th.Primary)
	case "relay":
		return lipgloss.NewStyle().Foreground(th.Secondary)
	default:
		return lipgloss.NewStyle().Foreground(th.Muted)
	}
}

func formatShortID(id string, maxLen int) string {
	if len(id) <= maxLen {
		return id
	}
	return id[:maxLen] + "..."
}

func formatWorkspace(path string) string {
	if len(path) > 60 {
		return "..." + path[len(path)-57:]
	}
	return path
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}
