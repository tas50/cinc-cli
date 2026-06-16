package explore

import "github.com/charmbracelet/bubbles/key"

// keyMap collects every binding the explore TUI handles, in one place
// so the help footer and the actual Update handlers can't drift.
type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	PageUp     key.Binding
	PageDown   key.Binding
	Home       key.Binding
	End        key.Binding
	Search     key.Binding
	Enter      key.Binding
	Esc        key.Binding
	SortDown   key.Binding
	SortUpdate key.Binding
	SortAlpha  key.Binding
	Open       key.Binding
	Install    key.Binding
	Help       key.Binding
	Quit       key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
		Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
		PageUp:     key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "jump up")),
		PageDown:   key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "jump down")),
		Home:       key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g", "top")),
		End:        key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G", "bottom")),
		Search:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Enter:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "open")),
		Esc:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		SortDown:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "downloads")),
		SortUpdate: key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "updated")),
		SortAlpha:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "alpha")),
		Open:       key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open in browser")),
		Install:    key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "install to server")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}
