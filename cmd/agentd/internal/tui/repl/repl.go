package repl

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/2389-research/ourocodus/pkg/acp"
	"github.com/2389-research/ourocodus/pkg/cli/format"
	"github.com/2389-research/ourocodus/pkg/tui/components/header"
	"github.com/2389-research/ourocodus/pkg/tui/layout"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Run starts a Bubble Tea REPL against an agent ACP stream.
// writer sends JSON-RPC requests to the agent, stdout/stderr deliver responses/logs.
// If th is nil, the default theme is used.
func Run(ctx context.Context, th *theme.Theme, agentID string, writer io.Writer, stdout io.Reader, stderr io.Reader) error {
	th = theme.Ensure(th)

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
	Up   key.Binding
	Down key.Binding
}

// message represents a single conversation entry with both display and raw JSON forms.
type message struct {
	display string         // Formatted display text (e.g., "You: hello")
	json    string         // Raw JSON (empty for non-JSON messages like errors)
	color   lipgloss.Color // Color for display
}

type model struct {
	ctx     context.Context
	th      *theme.Theme
	agentID string

	main  viewport.Model
	raw   viewport.Model
	input textarea.Model
	help  help.Model
	keys  keyMap

	writer io.Writer
	msgCh  <-chan tea.Msg
	nextID int

	status   string
	showRaw  bool
	messages []message // All messages with display and JSON forms
	selected int       // Currently selected message index (-1 = none/input focused)
	width    int       // Terminal width for truncation
}

func newModel(ctx context.Context, th *theme.Theme, agentID string, writer io.Writer, msgCh <-chan tea.Msg) model {
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
		Up:   key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "prev msg")),
		Down: key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "next msg")),
	}

	return model{
		ctx:      ctx,
		th:       th,
		agentID:  agentID,
		main:     main,
		raw:      raw,
		input:    ti,
		help:     help.New(),
		keys:     keys,
		writer:   writer,
		msgCh:    msgCh,
		nextID:   1,
		status:   "Connected",
		showRaw:  true,
		selected: -1, // Start with input focused
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
		// Handle global keys first
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Raw):
			m.showRaw = !m.showRaw
			return m, nil
		case key.Matches(msg, m.keys.Up):
			m.navigateUp()
			m.refreshViewports()
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.navigateDown()
			m.refreshViewports()
			return m, nil
		case key.Matches(msg, m.keys.Send):
			if m.selected == -1 {
				// Send message if input is focused
				content := strings.TrimSpace(m.input.Value())
				if content != "" {
					jsonData := m.send(content)
					m.addMessage(fmt.Sprintf("You: %s", content), jsonData, m.th.Primary)
					m.input.SetValue("")
					m.refreshViewports()
				}
			} else {
				// Return to input if message is selected
				m.selected = -1
				m.input.Focus()
				m.refreshViewports()
			}
			return m, nil
		}

		// Pass other keys to textarea when input is focused
		if m.selected == -1 {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		return m, nil

	case responseMsg:
		m.handleLine(string(msg))
		return m, m.waitForMsg()
	case logMsg:
		m.addMessage("log: "+string(msg), "", m.th.Warning)
		m.refreshViewports()
		return m, m.waitForMsg()
	case errorMsg:
		m.addMessage("error: "+string(msg), "", m.th.Error)
		m.refreshViewports()
		return m, nil
	case tea.WindowSizeMsg:
		m.handleResize(msg)
		return m, nil
	}

	// Pass other messages to textarea when input is focused
	if m.selected == -1 {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

// navigateUp moves selection up through messages.
func (m *model) navigateUp() {
	if m.selected == -1 && len(m.messages) > 0 {
		// From input, go to last message
		m.selected = len(m.messages) - 1
		m.input.Blur()
	} else if m.selected > 0 {
		m.selected--
	}
}

// navigateDown moves selection down through messages.
func (m *model) navigateDown() {
	if m.selected >= 0 && m.selected < len(m.messages)-1 {
		m.selected++
	} else if m.selected == len(m.messages)-1 {
		// From last message, go back to input
		m.selected = -1
		m.input.Focus()
	}
}

// handleResize processes window resize events.
func (m *model) handleResize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	// Measure actual rendered heights instead of using magic numbers
	statusContent := m.th.Title.Render(fmt.Sprintf("REPL → %s", m.agentID))
	headerStr := header.RenderWithContent(m.th, statusContent)
	footerStr := m.help.ShortHelpView([]key.Binding{m.keys.Send, m.keys.Raw, m.keys.Quit, m.keys.Up, m.keys.Down})
	statusStr := m.th.StatusBar.Render(fmt.Sprintf("%s • id=%d", m.status, m.nextID))
	inputHeight := m.input.Height()
	// Calculate available height for content viewports
	availH := layout.ContentHeight(msg.Height, headerStr, footerStr, statusStr) - inputHeight - 1 // -1 for spacing
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
	m.refreshViewports()
}

// addMessage adds a new message to the list.
func (m *model) addMessage(display, jsonData string, color lipgloss.Color) {
	m.messages = append(m.messages, message{
		display: display,
		json:    jsonData,
		color:   color,
	})
}

// refreshViewports rebuilds both viewport contents based on current messages and selection.
func (m *model) refreshViewports() {
	var mainLines []string
	var rawLines []string

	rawWidth := m.raw.Width
	if rawWidth <= 0 {
		rawWidth = 40
	}

	// Create a subtle divider line
	divider := m.th.MutedText.Render(strings.Repeat("─", 40))
	rawDivider := m.th.MutedText.Render(strings.Repeat("─", min(rawWidth, 40)))

	for i, msg := range m.messages {
		isSelected := i == m.selected

		// Add divider before user messages (except first) to separate exchanges
		if i > 0 && strings.HasPrefix(msg.display, "You:") {
			mainLines = append(mainLines, divider)
			rawLines = append(rawLines, rawDivider)
		}

		// Left panel: display text with selection highlight
		displayStyle := lipgloss.NewStyle().Foreground(msg.color)
		if isSelected {
			// Highlight selected message with background
			displayStyle = displayStyle.Background(m.th.Muted).Bold(true)
		}
		mainLines = append(mainLines, displayStyle.Render(msg.display))

		// Right panel: JSON (compact or expanded based on selection)
		if msg.json != "" {
			if isSelected {
				// Pretty-print and highlight selected JSON
				rawLines = append(rawLines, m.formatJSONExpanded(msg.json))
			} else {
				// Compact single-line, truncated
				rawLines = append(rawLines, m.formatJSONCompact(msg.json, rawWidth))
			}
		} else {
			// Non-JSON message (logs, errors)
			style := lipgloss.NewStyle().Foreground(msg.color)
			line := msg.display
			if len(line) > rawWidth-3 {
				line = line[:rawWidth-3] + "..."
			}
			rawLines = append(rawLines, style.Render(line))
		}
	}

	m.main.SetContent(strings.Join(mainLines, "\n"))
	m.raw.SetContent(strings.Join(rawLines, "\n"))

	// Scroll to keep selection visible
	if m.selected >= 0 {
		// Approximate: scroll so selected line is visible
		m.main.SetYOffset(max(0, m.selected-m.main.Height/2))
		m.raw.SetYOffset(m.calculateRawOffset())
	} else {
		m.main.GotoBottom()
		m.raw.GotoBottom()
	}
}

// calculateRawOffset calculates the scroll offset for the raw panel
// accounting for expanded JSON taking multiple lines.
func (m *model) calculateRawOffset() int {
	if m.selected < 0 {
		return 0
	}

	lineCount := 0
	for i := 0; i < m.selected; i++ {
		if m.messages[i].json != "" {
			lineCount++ // Compact JSON is 1 line
		} else {
			lineCount++ // Non-JSON is 1 line
		}
	}
	return max(0, lineCount-m.raw.Height/2)
}

// formatJSONExpanded returns pretty-printed, syntax-highlighted JSON.
func (m *model) formatJSONExpanded(jsonStr string) string {
	var data any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return jsonStr
	}
	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return jsonStr
	}
	return format.HighlightJSON(string(pretty), m.jsonColors())
}

// formatJSONCompact returns single-line, syntax-highlighted JSON, truncated to width.
func (m *model) formatJSONCompact(jsonStr string, width int) string {
	// Compact the JSON (remove whitespace)
	var data any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return truncate(jsonStr, width)
	}
	compact, err := json.Marshal(data)
	if err != nil {
		return truncate(jsonStr, width)
	}

	line := string(compact)
	// Highlight BEFORE truncating - truncated JSON isn't valid JSON
	// and won't be recognized by the highlighter
	highlighted := format.HighlightJSONCompact(line, m.jsonColors())
	return truncateWithANSI(highlighted, width)
}

// truncate shortens a string to fit within width, adding ellipsis if needed.
func truncate(s string, width int) string {
	if width <= 3 {
		return s
	}
	if len(s) > width-3 {
		return s[:width-3] + "..."
	}
	return s
}

// truncateWithANSI truncates a string containing ANSI escape codes to fit within width.
// It counts only visible characters (not escape sequences) toward the width limit.
func truncateWithANSI(s string, width int) string {
	if width <= 3 {
		return s
	}

	var result strings.Builder
	visibleLen := 0
	inEscape := false
	targetWidth := width - 3 // Leave room for "..."

	for i := 0; i < len(s); i++ {
		c := s[i]

		// Detect start of ANSI escape sequence
		if c == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			inEscape = true
			result.WriteByte(c)
			continue
		}

		if inEscape {
			result.WriteByte(c)
			// End of ANSI sequence (typically ends with a letter)
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				inEscape = false
			}
			continue
		}

		// Visible character
		if visibleLen >= targetWidth {
			result.WriteString("...")
			// Add reset escape sequence to ensure colors don't leak
			result.WriteString("\x1b[0m")
			return result.String()
		}

		result.WriteByte(c)
		visibleLen++
	}

	return result.String()
}

// send sends a message and returns the JSON that was sent.
func (m *model) send(text string) string {
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
		return ""
	}

	jsonStr := string(data)
	data = append(data, '\n')
	_, _ = m.writer.Write(data)
	return jsonStr
}

// jsonColors returns the JSON syntax highlighting colors for the current theme.
func (m *model) jsonColors() *format.JSONHighlightColors {
	return &format.JSONHighlightColors{
		Key:     m.th.Primary,
		String:  m.th.Success,
		Number:  m.th.Accent,
		Bool:    m.th.Warning,
		Null:    m.th.Error,
		Bracket: m.th.Muted,
	}
}

func (m model) View() string {
	// Build status content for header
	statusContent := m.th.Title.Render(fmt.Sprintf("REPL → %s", m.agentID))

	logoHeader := header.RenderWithContent(m.th, statusContent)

	body := m.main.View()
	if m.showRaw {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.main.View(), m.raw.View())
	}
	footer := m.help.ShortHelpView([]key.Binding{m.keys.Send, m.keys.Raw, m.keys.Quit})
	status := m.th.StatusBar.Render(fmt.Sprintf("%s • id=%d", m.status, m.nextID))
	return lipgloss.JoinVertical(lipgloss.Left, logoHeader, body, "", m.input.View(), status, footer)
}

func (m *model) handleLine(line string) {
	jsonData := ""
	if format.IsJSON(line) {
		jsonData = line
	}

	var resp acp.Response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		m.addMessage(fmt.Sprintf("raw: %s", line), jsonData, m.th.Secondary)
		m.refreshViewports()
		return
	}

	if resp.Error != nil {
		m.addMessage(fmt.Sprintf("error: %s", resp.Error.Message), jsonData, m.th.Error)
		m.refreshViewports()
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
		m.addMessage(fmt.Sprintf("Agent: %s", msg.Content), jsonData, color)
		m.refreshViewports()
		return
	}

	m.addMessage(fmt.Sprintf("resp: %s", line), jsonData, m.th.Secondary)
	m.refreshViewports()
}
