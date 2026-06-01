package explore

import "github.com/charmbracelet/bubbles/key"

// keyMap collects every binding the explore TUI handles, so the help
// footer and the Update handlers can't drift. Action keys (edit,
// create, delete, download) are only shown when the current kind
// supports them.
type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Home     key.Binding
	End      key.Binding
	Filter   key.Binding
	Enter    key.Binding
	Esc      key.Binding
	Kinds    key.Binding
	Edit     key.Binding
	New      key.Binding
	Delete   key.Binding
	Download key.Binding
	Quit     key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Home:     key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g", "top")),
		End:      key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G", "bottom")),
		Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "open")),
		Esc:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Kinds:    key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "kinds")),
		Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		New:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		Delete:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		Download: key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "download")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}
