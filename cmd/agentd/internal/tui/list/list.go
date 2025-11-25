package list

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/render"
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
func Run(ctx context.Context, th *theme.RetroTheme, agents []render.AgentInfo, status string, opts RunOptions) error {
	if th == nil {
		th = theme.NewRetroTheme(theme.PaletteCGA)
	}

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
	th       *theme.RetroTheme
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
	Quit key.Binding
	Up   key.Binding
	Down key.Binding
	Top  key.Binding
	End  key.Binding
	Stop key.Binding
}

func newKeyMap(stopEnabled bool) keyMap {
	km := keyMap{
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Up:   key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down: key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Top:  key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("home", "top")),
		End:  key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("end", "bottom")),
		Stop: key.NewBinding(key.WithKeys("x", "delete"), key.WithHelp("x", "stop")),
	}
	if !stopEnabled {
		km.Stop.SetEnabled(false)
	}
	return km
}

func newModel(ctx context.Context, th *theme.RetroTheme, agents []render.AgentInfo, status string, loader loadAgentsFunc, stopper StopAgentFunc) model {
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
		Foreground(lipgloss.Color("#0A0A0A")). // dark text
		Background(th.Accent).                 // bright background
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
			m.statusMessage = fmt.Sprintf("Stopped %s", msg.agentID)
		}
		// Trigger immediate refresh to update the agent list
		cmd := m.refreshAgents()
		return m, cmd
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
	case key.Matches(msg, m.keys.Top):
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
	// allocate roughly 60% width to table, rest to detail
	left := int(float64(m.width) * 0.55)
	if left < 40 {
		left = m.width
	}
	right := m.width - left - 2
	if right < 20 {
		right = 20
	}
	// leave space for footer/help (2 lines)
	height := m.height - 3
	m.table.SetWidth(left)
	m.table.SetHeight(height)
	m.viewport.Width = right
	m.viewport.Height = height
}

// setAgents updates the table rows while trying to preserve cursor position.
func (m *model) setAgents(agents []render.AgentInfo) {
	var currentID string
	if len(m.agents) > 0 {
		row := m.table.Cursor()
		if row >= 0 && row < len(m.agents) {
			currentID = m.agents[row].AgentID
		}
	}

	m.agents = agents
	m.table.SetRows(m.buildRows())

	// Restore cursor to previous agent if it still exists
	if currentID != "" {
		for i, a := range m.agents {
			if a.AgentID == currentID {
				m.table.SetCursor(i)
				break
			}
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
			formatLastBeat(a.LastBeat),
			formatAge(a.CreatedAt),
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
		formatLastBeat(a.LastBeat),
		formatAge(a.CreatedAt),
		a.Workspace,
		a.ContainerID,
	)
	style := lipgloss.NewStyle().Foreground(m.th.Secondary)
	m.viewport.SetContent(style.Render(content))
}

func (m model) View() string {
	header := renderRainbowLogo(theme.GetLogo(theme.LogoSmall))

	var main string
	if m.width >= 70 {
		main = lipgloss.JoinHorizontal(lipgloss.Top, m.table.View(), m.viewport.View())
	} else {
		main = m.table.View() + "\n" + m.viewport.View()
	}

	bindings := []key.Binding{m.keys.Up, m.keys.Down, m.keys.Top, m.keys.End}
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

	return lipgloss.JoinVertical(lipgloss.Left, header, "", main, "", statusBar, footer)
}

func renderRainbowLogo(text string) string {
	lines := strings.Split(text, "\n")
	colors := []lipgloss.Color{
		lipgloss.Color("#FF5555"), // Red
		lipgloss.Color("#FFB86C"), // Orange
		lipgloss.Color("#F1FA8C"), // Yellow
		lipgloss.Color("#50FA7B"), // Green
		lipgloss.Color("#8BE9FD"), // Cyan
		lipgloss.Color("#6272A4"), // Blue
		lipgloss.Color("#BD93F9"), // Purple
	}

	colored := make([]string, 0, len(lines))
	for i, line := range lines {
		color := colors[i%len(colors)]
		colored = append(colored, lipgloss.NewStyle().Foreground(color).Render(line))
	}
	return strings.Join(colored, "\n")
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

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "–"
	}
	d := time.Since(t)
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

func formatLastBeat(t time.Time) string {
	if t.IsZero() {
		return "–"
	}
	d := time.Since(t)
	if d < time.Second {
		return "now"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
