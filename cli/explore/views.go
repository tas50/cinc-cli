package explore

import (
	"fmt"
	"strings"
)

// ----- shared chrome ---------------------------------------------------

// header renders the title bar: the profile name and the current
// breadcrumb.
func (m model) header(title string) string {
	left := m.styles.Title.Render(title)
	if m.profileName != "" {
		left += m.styles.Crumb.Render("  [" + m.profileName + "]")
	}
	return left + "\n"
}

// crumb is the drill-down breadcrumb, e.g. "Data Bags › creds".
func (m model) crumb() string {
	parts := make([]string, 0, len(m.stack)+1)
	for _, f := range m.stack {
		parts = append(parts, f.kind.Title())
	}
	if m.cur != nil {
		parts = append(parts, m.cur.Title())
	}
	return strings.Join(parts, " › ")
}

// footer renders the status/error line and a contextual key hint line.
func (m model) footer(hints []string) string {
	var b strings.Builder
	b.WriteString("\n")
	switch {
	case m.listErr != "":
		b.WriteString(m.styles.Error.Render("✗ " + m.listErr))
	case m.detailErr != "":
		b.WriteString(m.styles.Error.Render("✗ " + m.detailErr))
	case m.status != "":
		b.WriteString(m.styles.Status.Render("✓ " + m.status))
	}
	b.WriteString("\n")
	b.WriteString(m.styles.Footer.Render(strings.Join(hints, "   ")))
	return b.String()
}

// actionHints lists the capability-gated action keys the current kind
// supports, in a stable order.
func (m model) actionHints() []string {
	var hints []string
	if _, ok := m.cur.(Editable); ok {
		hints = append(hints, m.hint("e", "edit"))
	}
	if _, ok := m.cur.(Creatable); ok {
		hints = append(hints, m.hint("n", "new"))
	} else if _, ok := m.cur.(NamedCreatable); ok {
		hints = append(hints, m.hint("n", "new"))
	}
	if _, ok := m.cur.(Deletable); ok {
		hints = append(hints, m.hint("d", "delete"))
	}
	if _, ok := m.cur.(Downloadable); ok {
		hints = append(hints, m.hint("D", "download"))
	}
	return hints
}

func (m model) hint(k, desc string) string {
	return m.styles.HelpKey.Render(k) + " " + m.styles.HelpDesc.Render(desc)
}

// ----- profiles --------------------------------------------------------

func (m model) viewProfiles() string {
	var b strings.Builder
	b.WriteString(m.header("cinc explore"))
	b.WriteString(m.styles.Header.Render("Select a profile") + "\n\n")
	for i, name := range m.profileNames {
		b.WriteString(m.renderChoice(name, i == m.profileCursor) + "\n")
	}
	b.WriteString(m.footer([]string{
		m.hint("↑/↓", "move"), m.hint("↵", "select"), m.hint("q", "quit"),
	}))
	return b.String()
}

// ----- kind menu -------------------------------------------------------

func (m model) viewKinds() string {
	var b strings.Builder
	b.WriteString(m.header("cinc explore"))
	b.WriteString(m.styles.Header.Render("Object types") + "\n\n")
	for i, k := range m.kinds {
		b.WriteString(m.renderChoice(k.Title(), i == m.kindCursor) + "\n")
	}
	hints := []string{m.hint("↑/↓", "move"), m.hint("↵", "open")}
	if len(m.profileNames) > 1 {
		hints = append(hints, m.hint("esc", "profiles"))
	}
	hints = append(hints, m.hint("q", "quit"))
	b.WriteString(m.footer(hints))
	return b.String()
}

func (m model) renderChoice(label string, selected bool) string {
	if selected {
		return m.styles.ListCursor.Render("❯ " + label)
	}
	return m.styles.ListItem.Render("  " + label)
}

// ----- list ------------------------------------------------------------

func (m model) viewList() string {
	var b strings.Builder
	b.WriteString(m.header(m.crumb()))

	rows := m.filteredRows()
	if m.filtering || m.filter.Value() != "" {
		b.WriteString(m.filter.View() + "\n")
	}

	switch {
	case m.loading:
		b.WriteString(m.styles.Body.Render("Loading…") + "\n")
	case len(rows) == 0 && m.listErr == "":
		b.WriteString(m.styles.Body.Render("No "+strings.ToLower(m.crumb())+" found.") + "\n")
	default:
		b.WriteString(m.renderTable(rows))
	}

	hints := []string{m.hint("↵", m.enterVerb()), m.hint("/", "filter"), m.hint(":", "kinds")}
	hints = append(hints, m.actionHints()...)
	hints = append(hints, m.hint("esc", "back"), m.hint("q", "quit"))
	b.WriteString(m.footer(hints))
	return b.String()
}

// enterVerb labels the Enter key by what it does for the current kind.
func (m model) enterVerb() string {
	if _, ok := m.cur.(DrillDown); ok {
		return "open"
	}
	return "view"
}

// renderTable lays out the columned, cursor-highlighted rows, windowed
// to the available height.
func (m model) renderTable(rows []Row) string {
	cols := m.cur.Columns()
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = len(c)
	}
	for _, r := range rows {
		for i, cell := range r.Cells {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var b strings.Builder
	b.WriteString(m.styles.Header.Render("  "+padCells(cols, widths)) + "\n")

	start, end := m.window(len(rows))
	for i := start; i < end; i++ {
		line := padCells(rows[i].Cells, widths)
		if i == m.cursor {
			b.WriteString(m.styles.ListCursor.Render("❯ "+line) + "\n")
		} else {
			b.WriteString(m.styles.ListItem.Render("  "+line) + "\n")
		}
	}
	return b.String()
}

// window returns the visible [start,end) slice of rows so the cursor
// stays on screen.
func (m model) window(n int) (int, int) {
	height := max(1, m.height-6) // header, filter, footer chrome
	if n <= height {
		return 0, n
	}
	start := max(0, m.cursor-height/2)
	if start+height > n {
		start = n - height
	}
	return start, start + height
}

func padCells(cells []string, widths []int) string {
	parts := make([]string, len(cells))
	for i, c := range cells {
		if i < len(widths) {
			parts[i] = fmt.Sprintf("%-*s", widths[i], c)
		} else {
			parts[i] = c
		}
	}
	return strings.Join(parts, "  ")
}

// ----- detail ----------------------------------------------------------

func (m model) viewDetail() string {
	var b strings.Builder
	b.WriteString(m.header(m.crumb() + " › " + m.detailName))
	if m.detailErr != "" {
		b.WriteString(m.styles.Error.Render("✗ "+m.detailErr) + "\n")
	} else {
		b.WriteString(m.detail.View() + "\n")
	}
	hints := []string{m.hint("↑/↓", "scroll")}
	hints = append(hints, m.actionHints()...)
	hints = append(hints, m.hint("esc", "back"), m.hint("q", "quit"))
	b.WriteString(m.footer(hints))
	return b.String()
}

// ----- modals ----------------------------------------------------------

func (m model) viewConfirm() string {
	var b strings.Builder
	b.WriteString(m.header(m.crumb()))
	b.WriteString("\n" + m.styles.Warn.Render(m.confirmPrompt) + "\n")
	b.WriteString(m.footer([]string{m.hint("y", "yes"), m.hint("n", "no")}))
	return b.String()
}

func (m model) viewName() string {
	var b strings.Builder
	b.WriteString(m.header(m.crumb()))
	b.WriteString("\n" + m.styles.Header.Render(m.namePrompt) + "\n\n")
	b.WriteString(m.nameInput.View() + "\n")
	b.WriteString(m.footer([]string{m.hint("↵", "confirm"), m.hint("esc", "cancel")}))
	return b.String()
}

func (m model) viewResult() string {
	var b strings.Builder
	b.WriteString(m.header(m.resultTitle))
	b.WriteString("\n" + m.styles.Body.Render(m.resultBody) + "\n")
	b.WriteString(m.footer([]string{m.hint("any key", "dismiss")}))
	return b.String()
}
