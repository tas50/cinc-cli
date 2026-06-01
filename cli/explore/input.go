package explore

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// handleKey routes a keypress to the active screen's handler. Each
// handler owns its own quit semantics so text-entry screens (filter,
// name, editor) can treat ordinary keys as input.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenEditor:
		return m.updateEditorKey(msg)
	case screenConfirm:
		return m.updateConfirmKey(msg)
	case screenName:
		return m.updateNameKey(msg)
	case screenResult:
		return m.updateResultKey(msg)
	case screenProfiles:
		return m.updateProfilesKey(msg)
	case screenKinds:
		return m.updateKindsKey(msg)
	case screenDetail:
		return m.updateDetailKey(msg)
	default:
		return m.updateListKey(msg)
	}
}

// ----- profile picker --------------------------------------------------

func (m model) updateProfilesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Up):
		if m.profileCursor > 0 {
			m.profileCursor--
		}
	case key.Matches(msg, m.keys.Down):
		if m.profileCursor < len(m.profileNames)-1 {
			m.profileCursor++
		}
	case key.Matches(msg, m.keys.Enter):
		name := m.profileNames[m.profileCursor]
		m.listErr = ""
		return m, buildClientCmd(m.opts, name, m.opts.Profiles[name])
	}
	return m, nil
}

// ----- kind menu -------------------------------------------------------

func (m model) updateKindsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Up):
		if m.kindCursor > 0 {
			m.kindCursor--
		}
	case key.Matches(msg, m.keys.Down):
		if m.kindCursor < len(m.kinds)-1 {
			m.kindCursor++
		}
	case key.Matches(msg, m.keys.Esc):
		// Back to the profile picker only when there's a choice to make.
		if len(m.profileNames) > 1 {
			m.screen = screenProfiles
		}
	case key.Matches(msg, m.keys.Enter):
		m.stack = nil
		cmd := m.openList(m.kinds[m.kindCursor])
		return m, cmd
	}
	return m, nil
}

// ----- list ------------------------------------------------------------

func (m model) updateListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		return m.updateFilterKey(msg)
	}
	prev := m.cursor
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Kinds):
		m.screen = screenKinds
	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(m.filteredRows())-1 {
			m.cursor++
		}
	case key.Matches(msg, m.keys.Home):
		m.cursor = 0
	case key.Matches(msg, m.keys.End):
		m.cursor = max(0, len(m.filteredRows())-1)
	case key.Matches(msg, m.keys.Filter):
		m.filtering = true
		m.status = ""
		cmd := m.filter.Focus()
		return m, cmd
	case key.Matches(msg, m.keys.Esc):
		return m.goBack()
	case key.Matches(msg, m.keys.Enter):
		return m.openSelected()
	case key.Matches(msg, m.keys.Edit):
		if row, ok := m.selectedRow(); ok {
			return m.startEdit(row.Name)
		}
	case key.Matches(msg, m.keys.New):
		return m.startCreate()
	case key.Matches(msg, m.keys.Delete):
		if row, ok := m.selectedRow(); ok {
			return m.startDelete(row.Name)
		}
	case key.Matches(msg, m.keys.Download):
		if row, ok := m.selectedRow(); ok {
			return m.startDownload(row.Name)
		}
	}
	// A cursor move re-arms the summary panel for the newly selected row.
	if m.cursor != prev {
		return m, m.scheduleSummary()
	}
	return m, nil
}

func (m model) updateFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.filtering = false
		m.filter.Blur()
		m.cursor = 0
		return m, m.scheduleSummary()
	case tea.KeyEsc:
		m.filtering = false
		m.filter.Blur()
		m.filter.SetValue("")
		m.cursor = 0
		return m, m.scheduleSummary()
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	if m.cursor >= len(m.filteredRows()) {
		m.cursor = max(0, len(m.filteredRows())-1)
	}
	// Narrowing the filter changes which row is selected, so refresh the
	// panel for it (debounced).
	return m, tea.Batch(cmd, m.scheduleSummary())
}

// goBack pops one drill-down level, or returns to the kind menu at the
// top level.
func (m model) goBack() (tea.Model, tea.Cmd) {
	if len(m.stack) == 0 {
		m.screen = screenKinds
		return m, nil
	}
	top := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	m.cur = top.kind
	m.rows = top.rows
	m.cursor = top.cursor
	m.listErr = ""
	m.status = ""
	m.filtering = false
	m.filter.SetValue("")
	// Drilling in cleared the parent's cache, so reload the panel for the
	// row we're returning to.
	m.summaryCache = map[string]summaryView{}
	m.summaryErr = ""
	m.screen = screenList
	return m, m.scheduleSummary()
}

// openSelected drills into the highlighted row when the kind has
// children, otherwise opens its detail view.
func (m model) openSelected() (tea.Model, tea.Cmd) {
	row, ok := m.selectedRow()
	if !ok {
		return m, nil
	}
	if dd, ok := m.cur.(DrillDown); ok {
		m.stack = append(m.stack, navFrame{kind: m.cur, rows: m.rows, cursor: m.cursor})
		cmd := m.openList(dd.Child(row.Name))
		return m, cmd
	}
	if v, ok := m.cur.(Viewable); ok {
		m.detailErr = ""
		m.detailName = row.Name
		m.status = ""
		m.screen = screenDetail
		return m, describeCmd(m.ctx, m.client, v, row.Name, m.nextReqID())
	}
	return m, nil
}

// ----- detail ----------------------------------------------------------

func (m model) updateDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Esc):
		m.screen = screenList
		return m, nil
	case key.Matches(msg, m.keys.Edit):
		return m.startEdit(m.detailName)
	case key.Matches(msg, m.keys.Delete):
		return m.startDelete(m.detailName)
	case key.Matches(msg, m.keys.Download):
		return m.startDownload(m.detailName)
	}
	var cmd tea.Cmd
	m.detail, cmd = m.detail.Update(msg)
	return m, cmd
}

// ----- action initiators ----------------------------------------------

func (m model) startEdit(name string) (tea.Model, tea.Cmd) {
	e, ok := m.cur.(Editable)
	if !ok || name == "" {
		return m, nil
	}
	m.returnTo = m.screen
	return m, editSeedCmd(m.ctx, m.client, e, name, m.nextReqID())
}

func (m model) startCreate() (tea.Model, tea.Cmd) {
	m.returnTo = m.screen
	if cr, ok := m.cur.(Creatable); ok {
		m.openEditor(actionCreate, "", cr.NewTemplate())
		return m, m.editor.Init()
	}
	if nc, ok := m.cur.(NamedCreatable); ok {
		label := "New " + strings.TrimSuffix(m.cur.Title(), "s") + " name"
		m.openName(label, "", func(name string) tea.Cmd {
			if name == "" {
				return nil
			}
			return createNamedCmd(m.ctx, m.client, nc, name)
		})
		cmd := m.nameInput.Focus()
		return m, cmd
	}
	return m, nil
}

func (m model) startDelete(name string) (tea.Model, tea.Cmd) {
	d, ok := m.cur.(Deletable)
	if !ok || name == "" {
		return m, nil
	}
	m.returnTo = m.screen
	m.confirmPrompt = "Delete " + name + "? (y/N)"
	m.pending = func() tea.Cmd { return deleteCmd(m.ctx, m.client, d, name) }
	m.screen = screenConfirm
	return m, nil
}

func (m model) startDownload(name string) (tea.Model, tea.Cmd) {
	d, ok := m.cur.(Downloadable)
	if !ok || name == "" {
		return m, nil
	}
	m.returnTo = m.screen
	m.openName("Download "+name+" to directory", ".", func(dir string) tea.Cmd {
		if dir == "" {
			dir = "."
		}
		return downloadCmd(m.ctx, m.client, d, name, dir)
	})
	cmd := m.nameInput.Focus()
	return m, cmd
}

// ----- modals: editor, confirm, name, result ---------------------------

func (m model) updateEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	if !m.editor.Finished() {
		return m, cmd
	}
	if m.editor.Aborted() {
		m.screen = m.returnTo
		return m, nil
	}
	committed := m.editor.Committed()
	switch m.editKind {
	case actionEdit:
		if e, ok := m.cur.(Editable); ok {
			m.screen = screenList
			m.status = "Saving…"
			return m, saveCmd(m.ctx, m.client, e, m.editName, committed)
		}
	case actionCreate:
		if cr, ok := m.cur.(Creatable); ok {
			m.screen = screenList
			m.status = "Creating…"
			return m, createCmd(m.ctx, m.client, cr, committed)
		}
	}
	m.screen = screenList
	return m, nil
}

func (m model) updateConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		run := m.pending
		m.pending = nil
		m.screen = screenList
		m.status = "Working…"
		if run != nil {
			return m, run()
		}
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	default:
		// Anything else (n, N, esc, q, …) cancels.
		m.pending = nil
		m.screen = m.returnTo
		return m, nil
	}
}

func (m model) updateNameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		action := m.nameAction
		val := strings.TrimSpace(m.nameInput.Value())
		m.nameInput.Blur()
		m.screen = screenList
		if action != nil {
			return m, action(val)
		}
		return m, nil
	case tea.KeyEsc:
		m.nameInput.Blur()
		m.screen = m.returnTo
		return m, nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

func (m model) updateResultKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	m.screen = screenList
	return m, nil
}

// openName configures and shows the name-prompt modal.
func (m *model) openName(prompt, initial string, action func(string) tea.Cmd) {
	m.namePrompt = prompt
	m.nameInput.SetValue(initial)
	m.nameInput.CursorEnd()
	m.nameAction = action
	m.screen = screenName
}
