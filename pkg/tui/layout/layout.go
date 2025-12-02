// Package layout provides a standardized page layout for Bubble Tea TUIs.
//
// The Page model eliminates magic number height calculations by measuring
// header/status/footer heights at runtime and computing content size automatically.
//
// Example usage:
//
//	page := layout.NewPage(
//	    layout.NewViewport(viewport.New(0, 0)),
//	    layout.WithHeader(func(w int) string { return header.Render(th) }),
//	    layout.WithFooter(func(w int) string { return help.View(keys) }),
//	)
//	p := tea.NewProgram(page, tea.WithAltScreen())
package layout

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Resizable is implemented by content models that can be resized.
// The Page calls SetSize when the terminal size changes.
type Resizable interface {
	SetSize(width, height int)
}

// ViewFunc renders a section at a given width.
// Used for header, status bar, and footer rendering.
type ViewFunc func(width int) string

// Option configures a Page.
type Option func(*Page)

// Page is a Bubble Tea model that provides a standardized layout with
// header, content, status bar, and footer sections.
//
// It automatically measures the height of each section and sizes the
// content area to fill the remaining space. This eliminates magic number
// height calculations that cause layout bugs.
type Page struct {
	content   tea.Model
	resizable Resizable // nil if content doesn't implement Resizable

	headerFn ViewFunc
	statusFn ViewFunc
	footerFn ViewFunc

	width, height      int
	contentW, contentH int
	ready              bool
}

// NewPage creates a new Page with the given content model.
// If content implements Resizable, it will be resized automatically.
func NewPage(content tea.Model, opts ...Option) *Page {
	p := &Page{content: content}
	if r, ok := content.(Resizable); ok {
		p.resizable = r
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// WithHeader sets the header render function.
func WithHeader(fn ViewFunc) Option {
	return func(p *Page) { p.headerFn = fn }
}

// WithStatusBar sets the status bar render function.
func WithStatusBar(fn ViewFunc) Option {
	return func(p *Page) { p.statusFn = fn }
}

// WithFooter sets the footer render function.
func WithFooter(fn ViewFunc) Option {
	return func(p *Page) { p.footerFn = fn }
}

// Init initializes the Page and its content.
func (p *Page) Init() tea.Cmd {
	return p.content.Init()
}

// Update handles messages for the Page.
func (p *Page) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height
		p.ready = true
		p.reflow()
		// Fall through to forward to content
	}

	// Forward all messages to content
	var cmd tea.Cmd
	p.content, cmd = p.content.Update(msg)

	// Reflow after content update in case header/status/footer changed
	if p.ready {
		p.reflow()
	}

	return p, cmd
}

// View renders the Page.
func (p *Page) View() string {
	if !p.ready {
		return "Initializing..."
	}

	w := p.width

	var header, status, footer string
	if p.headerFn != nil {
		header = p.headerFn(w)
	}
	if p.statusFn != nil {
		status = p.statusFn(w)
	}
	if p.footerFn != nil {
		footer = p.footerFn(w)
	}

	body := p.content.View()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, status, footer)
}

// reflow measures section heights and resizes content.
func (p *Page) reflow() {
	if p.width == 0 || p.height == 0 {
		return
	}

	w := p.width

	// Measure heights by rendering
	headerH := 0
	statusH := 0
	footerH := 0

	if p.headerFn != nil {
		headerH = lipgloss.Height(p.headerFn(w))
	}
	if p.statusFn != nil {
		statusH = lipgloss.Height(p.statusFn(w))
	}
	if p.footerFn != nil {
		footerH = lipgloss.Height(p.footerFn(w))
	}

	// Compute content size
	contentH := p.height - headerH - statusH - footerH
	if contentH < 1 {
		contentH = 1
	}
	contentW := w

	// Resize content if dimensions changed
	if p.resizable != nil && (contentW != p.contentW || contentH != p.contentH) {
		p.resizable.SetSize(contentW, contentH)
		p.contentW = contentW
		p.contentH = contentH
	}
}

// Content returns the underlying content model.
// Useful for accessing content-specific state.
func (p *Page) Content() tea.Model {
	return p.content
}

// Ready returns true if the Page has received a WindowSizeMsg.
func (p *Page) Ready() bool {
	return p.ready
}

// MeasureHeight returns the height in terminal rows of the given string.
// This is useful for calculating layout dimensions without magic numbers.
func MeasureHeight(s string) int {
	return lipgloss.Height(s)
}

// ContentHeight calculates the available height for content given total height
// and rendered header/footer strings. This eliminates magic number calculations.
//
// Example:
//
//	headerStr := header.RenderWithContent(th, status)
//	footerStr := help.View(keys)
//	contentH := layout.ContentHeight(windowHeight, headerStr, footerStr)
//	viewport.Height = contentH
func ContentHeight(totalHeight int, sections ...string) int {
	used := 0
	for _, s := range sections {
		used += lipgloss.Height(s)
	}
	h := totalHeight - used
	if h < 1 {
		h = 1
	}
	return h
}
