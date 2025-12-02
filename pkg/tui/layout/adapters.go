package layout

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Viewport wraps a bubbles viewport.Model to implement Resizable.
type Viewport struct {
	VP viewport.Model
}

// NewViewport creates a resizable viewport wrapper.
func NewViewport(vp viewport.Model) *Viewport {
	return &Viewport{VP: vp}
}

// Init implements tea.Model.
func (v *Viewport) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (v *Viewport) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	v.VP, cmd = v.VP.Update(msg)
	return v, cmd
}

// View implements tea.Model.
func (v *Viewport) View() string { return v.VP.View() }

// SetSize implements Resizable.
func (v *Viewport) SetSize(w, h int) {
	v.VP.Width = w
	v.VP.Height = h
}

// SetContent sets the viewport content.
func (v *Viewport) SetContent(s string) {
	v.VP.SetContent(s)
}

// GotoTop scrolls to the top.
func (v *Viewport) GotoTop() {
	v.VP.GotoTop()
}

// GotoBottom scrolls to the bottom.
func (v *Viewport) GotoBottom() {
	v.VP.GotoBottom()
}

// Table wraps a bubbles table.Model to implement Resizable.
type Table struct {
	T table.Model
}

// NewTable creates a resizable table wrapper.
func NewTable(t table.Model) *Table {
	return &Table{T: t}
}

// Init implements tea.Model.
func (t *Table) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (t *Table) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	t.T, cmd = t.T.Update(msg)
	return t, cmd
}

// View implements tea.Model.
func (t *Table) View() string { return t.T.View() }

// SetSize implements Resizable.
func (t *Table) SetSize(w, h int) {
	t.T.SetWidth(w)
	t.T.SetHeight(h)
}

// Cursor returns the current cursor position.
func (t *Table) Cursor() int {
	return t.T.Cursor()
}

// SelectedRow returns the selected row data.
func (t *Table) SelectedRow() table.Row {
	return t.T.SelectedRow()
}

// SetRows updates the table rows.
func (t *Table) SetRows(rows []table.Row) {
	t.T.SetRows(rows)
}
