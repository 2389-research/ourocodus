package list

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/render"
	"github.com/2389-research/ourocodus/pkg/cli/format"
	"github.com/2389-research/ourocodus/pkg/tui/components/header"
	"github.com/2389-research/ourocodus/pkg/tui/keys"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type (
	refreshMsg    struct{}
	refreshErrMsg string
	stopResultMsg struct {
		agentID string
		err     error
	}
)

type loadAgentsFunc func(context.Context) ([]render.AgentInfo, error)

// StopAgentFunc defines the signature for stopping an agent.
// Returns an error if the stop operation fails.
type StopAgentFunc func(ctx context.Context, agentID string) error

// RunOptions configures the list dashboard.
type RunOptions struct {
	Loader  loadAgentsFunc // Required: function to load agents
	Stopper StopAgentFunc  // Optional: function to stop agents (if nil, stop is disabled)
}

// Run starts the Bubble Tea list dashboard for agentd.
// It renders a table of agents with a details pane and footer help.
// When a loader is provided, the table auto-refreshes on a fixed interval.
// If th is nil, the default theme is used.
func Run(ctx context.Context, th *theme.Theme, agents []render.AgentInfo, status string, opts RunOptions) error {
	th = theme.Ensure(th)

	// Suppress log output during TUI mode to prevent bleeding into the display
	originalLogOutput := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(originalLogOutput)

	m := newModel(ctx, th, agents, status, opts.Loader, opts.Stopper)
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

type model struct {
	agents   []render.AgentInfo
	table    table.Model
	viewport viewport.Model
	help     help.Model
	keys     keyMap
	th       *theme.Theme
	width    int
	height   int

	ctx             context.Context
	loader          loadAgentsFunc
	stopper         StopAgentFunc
	refreshInterval time.Duration
	statusMessage   string
	stopping        bool // true while a stop operation is in progress
}

type keyMap struct {
	keys.Navigation
	Stop key.Binding
}

func newKeyMap(stopEnabled bool) keyMap {
	km := keyMap{
		Navigation: keys.NewNavigation(),
		Stop:       key.NewBinding(key.WithKeys("x", "delete"), key.WithHelp("x", "stop")),
	}
	if !stopEnabled {
		km.Stop.SetEnabled(false)
	}
	return km
}

func newModel(ctx context.Context, th *theme.Theme, agents []render.AgentInfo, status string, loader loadAgentsFunc, stopper StopAgentFunc) model {
	cols := []table.Column{
		{Title: "AGENT", Width: 18},
		{Title: "STATUS", Width: 12},
		{Title: "SOURCE", Width: 10},
		{Title: "ATTACHED", Width: 14},
		{Title: "LAST BEAT", Width: 14},
		{Title: "CREATED", Width: 12},
	}

	t := table.New(table.WithColumns(cols), table.WithRows(nil), table.WithFocused(true))

	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(th.Primary).
		BorderBottom(true).
		Foreground(th.Primary).
		Bold(true)
	styles.Selected = styles.Selected.
		Foreground(th.HighlightForeground). // light text on dark background
		Background(th.Contrast).            // dark selection background
		Bold(true)
	styles.Cell = styles.Cell.
		Foreground(th.Secondary)
	t.SetStyles(styles)

	vp := viewport.New(0, 0)
	model := model{
		agents:          agents,
		table:           t,
		viewport:        vp,
		help:            help.New(),
		keys:            newKeyMap(stopper != nil),
		th:              th,
		ctx:             ctx,
		loader:          loader,
		stopper:         stopper,
		refreshInterval: 1 * time.Second,
		statusMessage:   status,
	}
	model.setAgents(agents)
	return model
}

func (m model) Init() tea.Cmd {
	if m.loader == nil {
		return nil
	}
	return tea.Tick(m.refreshInterval, func(time.Time) tea.Msg { return refreshMsg{} })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if newModel, cmd, handled := m.handleKeyMsg(msg); handled {
			return newModel, cmd
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
	case refreshMsg:
		cmd := m.refreshAgents()
		return m, tea.Batch(cmd, tea.Tick(m.refreshInterval, func(time.Time) tea.Msg { return refreshMsg{} }))
	case refreshErrMsg:
		m.statusMessage = string(msg)
	case stopResultMsg:
		m.stopping = false
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Failed to stop %s: %v", msg.agentID, msg.err)
		} else {
			// Clear status - the agent will disappear from the list on refresh
			m.statusMessage = ""
		}
		// Trigger immediate refresh and continue the refresh cycle
		cmd := m.refreshAgents()
		return m, tea.Batch(cmd, tea.Tick(m.refreshInterval, func(time.Time) tea.Msg { return refreshMsg{} }))
	}

	var cmds []tea.Cmd
	oldCursor := m.table.Cursor()
	m.table, _ = m.table.Update(msg)
	if m.table.Cursor() != oldCursor {
		m.refreshDetail()
	}
	m.viewport, _ = m.viewport.Update(msg)

	return m, tea.Batch(cmds...)
}

// handleKeyMsg processes keyboard input and returns (model, cmd, handled).
// If handled is true, the caller should return the model and cmd immediately.
func (m model) handleKeyMsg(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit, true
	case key.Matches(msg, m.keys.Home):
		m.table.SetCursor(0)
		m.refreshDetail()
		return m, nil, true
	case key.Matches(msg, m.keys.End):
		if n := len(m.table.Rows()); n > 0 {
			m.table.SetCursor(n - 1)
			m.refreshDetail()
		}
		return m, nil, true
	case key.Matches(msg, m.keys.Stop):
		return m.handleStopKey()
	}
	return m, nil, false
}

// handleStopKey handles the stop/delete key press to stop an agent.
func (m model) handleStopKey() (model, tea.Cmd, bool) {
	if m.stopper == nil || m.stopping || len(m.agents) == 0 {
		return m, nil, true
	}
	row := m.table.Cursor()
	if row < 0 || row >= len(m.agents) {
		return m, nil, true
	}
	agentID := m.agents[row].AgentID
	m.stopping = true
	m.statusMessage = fmt.Sprintf("Stopping %s...", agentID)
	return m, m.stopAgentCmd(agentID), true
}

func (m *model) resize() {
	if m.width == 0 || m.height == 0 {
		return
	}

	// Calculate vertical space needed for header, status bar, footer, and spacers
	headerStr := header.Render(m.th)
	headerLines := strings.Count(headerStr, "\n") + 1

	// Status bar and footer are typically 1 line each
	// Plus 2 spacer lines (empty strings in JoinVertical)
	reserved := headerLines + 1 + 1 + 2 // header + status + footer + spacers

	avail := m.height - reserved
	if avail < 5 {
		avail = 5
	}

	// Use same orientation cutoff as View()
	sideBySide := m.width >= 70

	if sideBySide {
		// allocate roughly 55% width to table, rest to detail
		left := int(float64(m.width) * 0.55)
		if left < 40 {
			left = m.width
		}
		right := m.width - left - 2
		if right < 20 {
			right = 20
		}
		m.table.SetWidth(left)
		m.table.SetHeight(avail)
		m.viewport.Width = right
		m.viewport.Height = avail
	} else {
		// Stacked: both components take full width, split height between them
		m.table.SetWidth(m.width)
		m.viewport.Width = m.width

		top := avail / 2
		bottom := avail - top - 1 // -1 for the newline between them
		if bottom < 1 {
			bottom = 1
		}
		if top < 1 {
			top = 1
		}
		m.table.SetHeight(top)
		m.viewport.Height = bottom
	}
}

// setAgents updates the table rows while trying to preserve cursor position.
func (m *model) setAgents(agents []render.AgentInfo) {
	// Save current selection by agent ID (not row index)
	var currentID string
	cursorBefore := m.table.Cursor()
	if len(m.agents) > 0 && cursorBefore >= 0 && cursorBefore < len(m.agents) {
		currentID = m.agents[cursorBefore].AgentID
	}

	// Update the agents list
	m.agents = agents
	m.table.SetRows(m.buildRows())

	// Restore cursor to the same agent if it still exists
	if currentID != "" {
		for i, a := range m.agents {
			if a.AgentID == currentID {
				m.table.SetCursor(i)
				m.refreshDetail()
				return
			}
		}
	}

	// Agent not found or no previous selection - ensure cursor is valid
	if len(m.agents) > 0 {
		// Clamp cursor to valid range if needed
		if cursorBefore >= len(m.agents) {
			m.table.SetCursor(len(m.agents) - 1)
		}
	}
	m.refreshDetail()
}

func (m *model) buildRows() []table.Row {
	rows := make([]table.Row, 0, len(m.agents))
	for _, a := range m.agents {
		rows = append(rows, table.Row{
			a.AgentID,
			formatStatus(a.Status),
			a.SpawnSource,
			formatAttached(a.AttachedTo),
			format.FormatLastBeat(a.LastBeat),
			format.FormatAge(a.CreatedAt),
		})
	}
	return rows
}

func (m *model) refreshAgents() tea.Cmd {
	if m.loader == nil {
		return nil
	}
	if m.ctx != nil && m.ctx.Err() != nil {
		return tea.Quit
	}

	agents, err := m.loader(m.ctx)
	if err != nil {
		return func() tea.Msg { return refreshErrMsg(fmt.Sprintf("refresh failed: %v", err)) }
	}
	m.setAgents(agents)
	m.statusMessage = ""
	return nil
}

// stopAgentCmd returns a command that stops an agent asynchronously
func (m *model) stopAgentCmd(agentID string) tea.Cmd {
	return func() tea.Msg {
		err := m.stopper(m.ctx, agentID)
		return stopResultMsg{agentID: agentID, err: err}
	}
}

func (m *model) refreshDetail() {
	if len(m.agents) == 0 {
		m.viewport.SetContent("No agents running")
		return
	}
	row := m.table.Cursor()
	if row >= len(m.agents) {
		row = len(m.agents) - 1
	}
	a := m.agents[row]
	content := fmt.Sprintf(
		"Agent: %s\nStatus: %s\nSource: %s\nAttached: %s\nLast beat: %s\nCreated: %s ago\nWorkspace: %s\nContainer: %s",
		a.AgentID,
		a.Status,
		a.SpawnSource,
		formatAttached(a.AttachedTo),
		format.FormatLastBeat(a.LastBeat),
		format.FormatAge(a.CreatedAt),
		a.Workspace,
		a.ContainerID,
	)
	m.viewport.SetContent(m.th.SecondaryText.Render(content))
}

func (m model) View() string {
	logoHeader := header.Render(m.th)

	var main string
	if m.width >= 70 {
		main = lipgloss.JoinHorizontal(lipgloss.Top, m.table.View(), m.viewport.View())
	} else {
		main = m.table.View() + "\n" + m.viewport.View()
	}

	bindings := []key.Binding{m.keys.Up, m.keys.Down, m.keys.Home, m.keys.End}
	if m.keys.Stop.Enabled() {
		bindings = append(bindings, m.keys.Stop)
	}
	bindings = append(bindings, m.keys.Quit)
	footer := m.help.ShortHelpView(bindings)

	status := m.statusMessage
	if status == "" {
		status = fmt.Sprintf("Refreshing every %ds", int(m.refreshInterval.Seconds()))
	} else {
		status = fmt.Sprintf("%s • refresh %ds", status, int(m.refreshInterval.Seconds()))
	}
	statusBar := m.th.StatusBar.Render(status)

	return lipgloss.JoinVertical(lipgloss.Left, logoHeader, "", main, "", statusBar, footer)
}

func formatStatus(s string) string { return s }

func formatAttached(a string) string {
	if a == "" {
		return "–"
	}
	if len(a) > 12 {
		return a[:12] + "…"
	}
	return a
}
