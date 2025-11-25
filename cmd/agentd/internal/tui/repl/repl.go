package repl

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
	"github.com/2389-research/ourocodus/pkg/acp"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Run starts a Bubble Tea REPL against an agent ACP stream.
// writer sends JSON-RPC requests to the agent, stdout/stderr deliver responses/logs.
func Run(ctx context.Context, th *theme.RetroTheme, agentID string, writer io.Writer, stdout io.Reader, stderr io.Reader) error {
	if th == nil {
		th = theme.NewRetroTheme(theme.PaletteCGA)
	}

	respCh := make(chan tea.Msg)

	// Stream stdout -> responseMsg
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			respCh <- responseMsg(line)
		}
		if err := scanner.Err(); err != nil {
			respCh <- errorMsg(fmt.Sprintf("read stdout: %v", err))
		}
		close(respCh)
	}()

	// Stream stderr -> logMsg
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			respCh <- logMsg(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			respCh <- errorMsg(fmt.Sprintf("read stderr: %v", err))
		}
	}()

	m := newModel(ctx, th, agentID, writer, respCh)
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

type (
	responseMsg string
	logMsg      string
	errorMsg    string
)

type keyMap struct {
	Send key.Binding
	Quit key.Binding
	Raw  key.Binding
}

type model struct {
	ctx     context.Context
	th      *theme.RetroTheme
	agentID string

	main  viewport.Model
	raw   viewport.Model
	input textarea.Model
	help  help.Model
	keys  keyMap

	writer io.Writer
	msgCh  <-chan tea.Msg
	nextID int

	status  string
	showRaw bool
}

func newModel(ctx context.Context, th *theme.RetroTheme, agentID string, writer io.Writer, msgCh <-chan tea.Msg) model {
	ti := textarea.New()
	ti.Placeholder = "Type message and hit Enter…"
	ti.Focus()
	ti.SetHeight(3)
	ti.SetWidth(80)
	ti.Prompt = "› "
	ti.CharLimit = 2000
	ti.ShowLineNumbers = false
	ti.SetCursor(0)

	main := viewport.New(0, 0)
	main.SetContent("")
	raw := viewport.New(0, 0)
	raw.SetContent("")

	keys := keyMap{
		Send: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send")),
		Quit: key.NewBinding(key.WithKeys("ctrl+c", "esc"), key.WithHelp("ctrl+c", "quit")),
		Raw:  key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "toggle raw")),
	}

	return model{
		ctx:     ctx,
		th:      th,
		agentID: agentID,
		main:    main,
		raw:     raw,
		input:   ti,
		help:    help.New(),
		keys:    keys,
		writer:  writer,
		msgCh:   msgCh,
		nextID:  1,
		status:  "Connected",
		showRaw: true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen, m.waitForMsg())
}

func (m model) waitForMsg() tea.Cmd {
	return func() tea.Msg {
		if m.msgCh == nil {
			return nil
		}
		msg, ok := <-m.msgCh
		if !ok {
			return errorMsg("connection closed")
		}
		return msg
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Send):
			content := strings.TrimSpace(m.input.Value())
			if content != "" {
				_ = m.send(content)
				m.append(fmt.Sprintf("You: %s", content), m.th.Primary)
				m.input.SetValue("")
			}
			return m, nil
		case key.Matches(msg, m.keys.Raw):
			m.showRaw = !m.showRaw
			return m, nil
		}
	case responseMsg:
		m.handleLine(string(msg))
		return m, m.waitForMsg()
	case logMsg:
		m.appendRaw("log: "+string(msg), m.th.Warning)
		return m, m.waitForMsg()
	case errorMsg:
		m.appendBoth("error: "+string(msg), m.th.Error)
		return m, nil
	case tea.WindowSizeMsg:
		// reserve 4 lines for input+status+help
		availH := msg.Height - m.input.Height() - 4
		if availH < 6 {
			availH = 6
		}
		if m.showRaw && msg.Width >= 90 {
			// split side by side
			left := msg.Width / 2
			right := msg.Width - left - 1
			m.main.Width = left
			m.raw.Width = right
			m.main.Height = availH
			m.raw.Height = availH
		} else {
			m.main.Width = msg.Width
			m.raw.Width = msg.Width
			m.main.Height = availH
			m.raw.Height = availH / 2
		}
		m.input.SetWidth(msg.Width - 2)
		return m, nil
	}

	// Update input for other keypresses
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) append(line string, color lipgloss.Color) {
	style := lipgloss.NewStyle().Foreground(color)
	m.main.SetContent(joinLines(m.main.View(), style.Render(line)))
	m.main.GotoBottom()
}

func (m *model) appendRaw(line string, color lipgloss.Color) {
	style := lipgloss.NewStyle().Foreground(color)
	m.raw.SetContent(joinLines(m.raw.View(), style.Render(line)))
	m.raw.GotoBottom()
}

func (m *model) appendBoth(line string, color lipgloss.Color) {
	m.append(line, color)
	m.appendRaw(line, color)
}

func joinLines(existing, line string) string {
	content := strings.TrimSpace(existing)
	if content == "" {
		return line
	}
	return content + "\n" + line
}

func (m *model) send(text string) error {
	req := acp.Request{
		ID:      m.nextID,
		JSONRPC: "2.0",
		Method:  acp.MethodSendMessage,
		Params: acp.SendMessageParams{
			Content: text,
		},
	}
	m.nextID++
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = m.writer.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func (m model) View() string {
	header := m.th.Header.Render(fmt.Sprintf("REPL → %s", m.agentID))
	body := m.main.View()
	if m.showRaw {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.main.View(), m.raw.View())
	}
	footer := m.help.ShortHelpView([]key.Binding{m.keys.Send, m.keys.Raw, m.keys.Quit})
	status := m.th.StatusBar.Render(fmt.Sprintf("%s • id=%d", m.status, m.nextID))
	return lipgloss.JoinVertical(lipgloss.Left, header, body, "", m.input.View(), status, footer)
}

func (m *model) handleLine(line string) {
	m.appendRaw(line, m.th.Muted)

	var resp acp.Response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		m.append(fmt.Sprintf("raw: %s", line), m.th.Secondary)
		return
	}

	if resp.Error != nil {
		m.append(fmt.Sprintf("error: %s", resp.Error.Message), m.th.Error)
		return
	}

	// Try to interpret result as AgentMessage
	data, _ := json.Marshal(resp.Result)
	var msg acp.AgentMessage
	if err := json.Unmarshal(data, &msg); err == nil && msg.Content != "" {
		color := m.th.Secondary
		if msg.Type == "toolCall" {
			color = m.th.Accent
		}
		m.append(fmt.Sprintf("Agent: %s", msg.Content), color)
		return
	}

	m.append(fmt.Sprintf("resp: %s", line), m.th.Secondary)
}
