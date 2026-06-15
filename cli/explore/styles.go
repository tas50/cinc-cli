package explore

import "github.com/charmbracelet/lipgloss"

// styles bundles every lipgloss style the explore TUI uses, in one
// place so theming stays consistent. The palette matches the
// supermarket explore TUI.
type styles struct {
	Title      lipgloss.Style
	Crumb      lipgloss.Style
	ListItem   lipgloss.Style
	ListCursor lipgloss.Style
	Header     lipgloss.Style
	Body       lipgloss.Style
	Footer     lipgloss.Style
	HelpKey    lipgloss.Style
	HelpDesc   lipgloss.Style
	Error      lipgloss.Style
	Status     lipgloss.Style
	Warn       lipgloss.Style
	ServerInfo lipgloss.Style
}

func newStyles() styles {
	return styles{
		Title:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213")),
		Crumb:      lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		ListItem:   lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		ListCursor: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213")),
		Header:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("244")),
		Body:       lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Footer:     lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		HelpKey:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")),
		HelpDesc:   lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Error:      lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		Status:     lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
		Warn:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")),
		ServerInfo: lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("244")),
	}
}
