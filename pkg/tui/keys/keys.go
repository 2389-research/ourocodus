// Package keys provides common key bindings for TUI components.
package keys

import "github.com/charmbracelet/bubbles/key"

// Common contains key bindings shared across TUI components.
type Common struct {
	Quit   key.Binding
	Help   key.Binding
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Escape key.Binding
}

// NewCommon creates the default common key bindings.
func NewCommon() Common {
	return Common{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
	}
}

// ShortHelp returns the short help for common keys.
func (k Common) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns the full help for common keys.
func (k Common) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter},
		{k.Help, k.Quit, k.Escape},
	}
}

// Navigation contains key bindings for navigation.
type Navigation struct {
	Common
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
}

// NewNavigation creates navigation key bindings.
func NewNavigation() Navigation {
	return Navigation{
		Common: NewCommon(),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home", "go to top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end", "go to bottom"),
		),
	}
}

// ShortHelp returns the short help for navigation keys.
func (k Navigation) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Help, k.Quit}
}

// FullHelp returns the full help for navigation keys.
func (k Navigation) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown},
		{k.Home, k.End, k.Enter, k.Escape},
		{k.Help, k.Quit},
	}
}
