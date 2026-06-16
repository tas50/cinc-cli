package nodeedit

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/jsoneditor"
)

// screen is the active surface: the scalar form or the embedded JSON
// editor for one attribute bag.
type screen uint8

const (
	screenForm screen = iota
	screenEditor
)

// The form has four directly-editable scalar fields, then four attribute
// bags. The scalar fields are edited in place with no JSON; selecting a
// bag jumps into the full-screen JSON editor.
const fieldCount = 4

// bagNames are the four Chef attribute precedence levels, in the order
// they appear on the form. Selecting any of them opens the JSON editor.
var bagNames = []string{"normal", "default", "override", "automatic"}

// rowCount is every navigable row: the scalar fields plus the bags.
const rowCount = fieldCount + 4

// dash is the placeholder shown for an empty scalar value.
const dash = "—"

var (
	styHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	styLabel  = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	styValue  = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	styDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styCursor = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
	styWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

// selectedMarker is the cursor glyph drawn against the highlighted row,
// matching the rest of the cinc TUIs. It is one cell wide so the blank
// prefix on unselected rows keeps every row aligned.
const selectedMarker = "▶ "

// Model is the node editor: a non-JSON form for the scalar node fields
// (chef_environment, run_list, policy_name, policy_group) plus four
// attribute-bag rows (normal, default, override, automatic) that each open
// the shared JSON editor. It is an embeddable bubbletea component — call
// New, forward Update/View, and check Finished after each Update.
type Model struct {
	name string // pinned by the command arg; shown read-only

	// inputs holds the scalar fields in row order: chef_environment,
	// run_list, policy_name, policy_group.
	inputs [fieldCount]textinput.Model

	// bags holds the four attribute precedence levels, keyed by bagNames.
	bags map[string]cinc.Attributes

	cursor int
	screen screen

	editor  jsoneditor.Model
	editBag string

	committed bool
	aborted   bool
	finished  bool

	width, height int
}

// New returns a node editor seeded from in. The scalar fields become
// editable text inputs and each attribute bag is carried so it can be
// opened in the JSON editor. The node's name is fixed (the command arg
// owns it) and shown read-only.
func New(in *cinc.Node) Model {
	mk := func(val string) textinput.Model {
		ti := textinput.New()
		ti.Prompt = ""
		ti.SetValue(val)
		return ti
	}
	env := in.Environment
	if env == "" {
		env = "_default"
	}
	m := Model{
		name: in.Name,
		bags: map[string]cinc.Attributes{
			"normal":    in.Normal,
			"default":   in.Default,
			"override":  in.Override,
			"automatic": in.Automatic,
		},
	}
	m.inputs[0] = mk(env)
	m.inputs[1] = mk(strings.Join(in.RunList, ", "))
	m.inputs[2] = mk(in.PolicyName)
	m.inputs[3] = mk(in.PolicyGroup)
	m.syncFocus()
	return m
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

// SetSize fits the form to the terminal. The embedded editor sizes itself
// from the WindowSizeMsg it receives while it is the active screen.
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	if width > 24 {
		for i := range m.inputs {
			m.inputs[i].Width = width - 24
		}
	}
}

// Update advances the editor. It returns the concrete Model so embedding
// code can call Finished/Aborted/Result without a type assertion.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.SetSize(sz.Width, sz.Height)
	}
	if m.screen == screenEditor {
		return m.updateEditor(msg)
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		return m.updateForm(k)
	}
	// Forward non-key messages (e.g. cursor blink) to the focused field.
	if m.cursor < fieldCount {
		var cmd tea.Cmd
		m.inputs[m.cursor], cmd = m.inputs[m.cursor].Update(msg)
		return m, cmd
	}
	return m, nil
}

// updateForm handles keys on the scalar form. Arrow keys and Tab move
// between rows; Enter on a bag row opens the JSON editor; Ctrl-D saves and
// Esc aborts. Any other key edits the focused scalar field.
func (m Model) updateForm(k tea.KeyMsg) (Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.aborted = true
		m.finished = true
		return m, nil
	case tea.KeyCtrlD:
		m.committed = true
		m.finished = true
		return m, nil
	case tea.KeyUp, tea.KeyShiftTab:
		m.moveCursor(-1)
		return m, nil
	case tea.KeyDown, tea.KeyTab:
		m.moveCursor(1)
		return m, nil
	case tea.KeyEnter:
		if m.cursor >= fieldCount {
			return m.openBag(bagNames[m.cursor-fieldCount])
		}
		m.moveCursor(1)
		return m, nil
	}
	if m.cursor < fieldCount {
		var cmd tea.Cmd
		m.inputs[m.cursor], cmd = m.inputs[m.cursor].Update(k)
		return m, cmd
	}
	return m, nil
}

// updateEditor forwards messages to the embedded JSON editor. When the
// editor finishes, a committed bag is folded back into the node and an
// aborted one is discarded; either way we return to the form.
func (m Model) updateEditor(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	if m.editor.Finished() {
		if !m.editor.Aborted() {
			var attrs cinc.Attributes
			if err := json.Unmarshal(m.editor.Committed(), &attrs); err == nil {
				m.bags[m.editBag] = attrs
			}
		}
		m.screen = screenForm
		m.syncFocus()
		return m, nil
	}
	return m, cmd
}

// openBag seeds and shows the JSON editor for one attribute bag.
func (m Model) openBag(name string) (Model, tea.Cmd) {
	seed := []byte("{}")
	if len(m.bags[name]) > 0 {
		if b, err := json.MarshalIndent(m.bags[name], "", "  "); err == nil {
			seed = b
		}
	}
	m.editBag = name
	m.editor = jsoneditor.New(seed, validateAttributes)
	m.editor.SetSize(m.width, m.height)
	m.screen = screenEditor
	return m, m.editor.Init()
}

func (m *Model) moveCursor(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > rowCount-1 {
		m.cursor = rowCount - 1
	}
	m.syncFocus()
}

// syncFocus focuses the scalar field under the cursor and blurs the rest,
// so typing only ever lands in the selected field.
func (m *Model) syncFocus() {
	for i := range m.inputs {
		if i == m.cursor {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

// Finished reports whether the user has saved or aborted.
func (m Model) Finished() bool { return m.finished }

// Aborted reports whether the user abandoned the edit.
func (m Model) Aborted() bool { return m.aborted }

// Result assembles the edited node from the form. The caller should only
// rely on it once the edit was committed (not aborted).
func (m Model) Result() *cinc.Node {
	out := cinc.Node{
		Name:        m.name,
		Environment: strings.TrimSpace(m.inputs[0].Value()),
		RunList:     splitRunList(m.inputs[1].Value()),
		PolicyName:  strings.TrimSpace(m.inputs[2].Value()),
		PolicyGroup: strings.TrimSpace(m.inputs[3].Value()),
		Normal:      m.bags["normal"],
		Default:     m.bags["default"],
		Override:    m.bags["override"],
		Automatic:   m.bags["automatic"],
	}
	if out.RunList == nil {
		out.RunList = []string{}
	}
	return &out
}

// validateAttributes rejects a bag edited into anything but a JSON object —
// an attribute bag is always a map.
func validateAttributes(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if _, ok := v.(map[string]any); !ok {
		return errors.New("an attribute bag must be a JSON object, e.g. {}")
	}
	return nil
}

// splitRunList parses a comma-separated run-list field into items, trimming
// whitespace and dropping blanks — matching `cinc node create --run-list`.
func splitRunList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ----- view ------------------------------------------------------------

func (m Model) View() string {
	if m.screen == screenEditor {
		return m.editor.View()
	}

	var b strings.Builder
	b.WriteString(styHeader.Render("Editing node "+m.name) + "\n\n")

	labels := []string{"chef_environment", "run_list", "policy_name", "policy_group"}
	width := 0
	for _, l := range append(append([]string{}, labels...), bagNames...) {
		if len(l) > width {
			width = len(l)
		}
	}

	for i, label := range labels {
		b.WriteString(m.renderRow(i, label, m.fieldValue(i), width) + "\n")
	}

	b.WriteString("\n" + styWarn.Render("⚠ Changes to attributes may be overwritten on the next cinc run.") + "\n")
	for i, name := range bagNames {
		b.WriteString(m.renderRow(fieldCount+i, name, m.bagValue(name), width) + "\n")
	}

	b.WriteString("\n" + styDim.Render("↑/↓ move · ↵ edit attributes · Ctrl-D save · Esc abort"))
	return b.String()
}

// renderRow draws one label/value line, marking the cursor row. A focused
// scalar field shows its live text input; everything else shows a value.
func (m Model) renderRow(row int, label, value string, width int) string {
	marker := "  "
	lblStyle := styLabel
	if row == m.cursor {
		marker = styCursor.Render(selectedMarker)
		lblStyle = styCursor
	}
	paddedLabel := lblStyle.Render(fmt.Sprintf("%-*s", width, label))
	return marker + paddedLabel + "  " + value
}

// fieldValue is the displayed value for scalar field i: its live input when
// focused, otherwise the styled text (or the em-dash placeholder).
func (m Model) fieldValue(i int) string {
	if i == m.cursor {
		return m.inputs[i].View()
	}
	v := strings.TrimSpace(m.inputs[i].Value())
	if v == "" {
		return styDim.Render(dash)
	}
	return styValue.Render(v)
}

// bagValue summarizes an attribute bag: how many top-level keys it holds,
// with a hint that selecting it opens the editor.
func (m Model) bagValue(name string) string {
	n := len(m.bags[name])
	if n == 0 {
		return styDim.Render("empty · ↵ to edit")
	}
	return styValue.Render(fmt.Sprintf("%d keys", n)) + styDim.Render(" · ↵ to edit")
}
