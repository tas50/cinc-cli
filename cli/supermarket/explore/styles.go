package explore

import "github.com/charmbracelet/lipgloss"

// styles bundles every lipgloss style the TUI uses. Keeping them in one
// place makes it cheap to retheme and easy to spot inconsistencies.
type styles struct {
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	SortBar     lipgloss.Style
	SortActive  lipgloss.Style
	SortDim     lipgloss.Style
	SearchLabel lipgloss.Style
	SearchInput lipgloss.Style
	ListItem    lipgloss.Style
	ListCursor  lipgloss.Style
	Maintainer  lipgloss.Style
	PreviewKey  lipgloss.Style
	PreviewBody lipgloss.Style
	Footer      lipgloss.Style
	HelpKey     lipgloss.Style
	HelpDesc    lipgloss.Style
	Error       lipgloss.Style
	Status      lipgloss.Style
	Deprecated  lipgloss.Style
	Divider     lipgloss.Style
}

func newStyles() styles {
	return styles{
		Title:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213")),
		Subtitle:    lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		SortBar:     lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		SortActive:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")),
		SortDim:     lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		SearchLabel: lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
		SearchInput: lipgloss.NewStyle().Foreground(lipgloss.Color("231")),
		ListItem:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		ListCursor:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")),
		Maintainer:  lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		PreviewKey:  lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		PreviewBody: lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Footer:      lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		HelpKey:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")),
		HelpDesc:    lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Error:       lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		Status:      lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Deprecated:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")),
		Divider:     lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
	}
}
