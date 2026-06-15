package nodeedit

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/jsoneditor"
)

// focus identifies which part of the form has the keyboard.
type focus int

const (
	focusEnv focus = iota
	focusPolicyGroup
	focusPolicyName
	focusRunList
	focusAttrs
	focusCount
)

var (
	titleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
	ruleStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	labelStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	focusLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
	hintStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// Model is the node-edit form. The node name is a read-only heading;
// environment, policy, and run list are plain fields; and the attribute
// bags are edited in the embedded JSON editor. The form owns the lifecycle
// (Ctrl-D saves the whole node, Ctrl-C cancels) and forwards other keys to
// the focused field.
type Model struct {
	name string
	orig *cinc.Node

	env         textinput.Model
	policyGroup textinput.Model
	policyName  textinput.Model
	runList     textarea.Model
	attrs       jsoneditor.Model

	focus focus
	dived bool // editing inside the focused run-list / attributes section

	errMsg   string
	result   *cinc.Node
	changed  bool
	finished bool
	aborted  bool

	width, height int
}

// New builds a node-edit form seeded from node.
func New(node *cinc.Node) (Model, error) {
	seed, err := attributesSeed(node)
	if err != nil {
		return Model{}, err
	}

	env := textinput.New()
	env.Prompt = ""
	env.SetValue(node.Environment)
	env.Placeholder = "_default"

	pg := textinput.New()
	pg.Prompt = ""
	pg.SetValue(node.PolicyGroup)
	pn := textinput.New()
	pn.Prompt = ""
	pn.SetValue(node.PolicyName)

	rl := textarea.New()
	rl.ShowLineNumbers = false
	rl.Prompt = ""
	rl.SetValue(runListText(node))

	m := Model{
		name:        node.Name,
		orig:        node,
		env:         env,
		policyGroup: pg,
		policyName:  pn,
		runList:     rl,
		attrs:       jsoneditor.New(seed, func([]byte) error { return nil }),
	}
	m.setFocus(focusEnv)
	return m, nil
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

// Update advances the form. It returns the concrete Model so the standalone
// driver and tests can read Finished/Result without a type assertion.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.setSize(sz.Width, sz.Height)
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m.forward(msg)
	}
	switch k.Type {
	case tea.KeyCtrlC:
		m.aborted = true
		m.finished = true
		return m, nil
	case tea.KeyCtrlD:
		return m.save(), nil
	}
	if m.dived {
		return m.updateDived(k)
	}
	return m.updateNavigating(k)
}

// updateNavigating handles the top-level field cursor: up/down move between
// fields, Enter dives into the run-list or attributes editor, Esc cancels
// the whole edit. Text fields are edited inline while focused; the run-list
// and attributes sections ignore typing until dived into.
func (m Model) updateNavigating(k tea.KeyMsg) (Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyUp:
		m.setFocus(m.focus - 1)
		return m, nil
	case tea.KeyDown:
		m.setFocus(m.focus + 1)
		return m, nil
	case tea.KeyEsc:
		m.aborted = true
		m.finished = true
		return m, nil
	case tea.KeyEnter:
		if m.focus == focusRunList || m.focus == focusAttrs {
			m.enterDive()
		}
		return m, nil
	}
	if m.focus == focusRunList || m.focus == focusAttrs {
		return m, nil // not dived: nothing to type into
	}
	return m.forward(k)
}

// updateDived forwards keys to the dived-into section. Esc cancels an open
// attribute sub-edit if there is one, otherwise it backs out to the field
// cursor.
func (m Model) updateDived(k tea.KeyMsg) (Model, tea.Cmd) {
	if k.Type == tea.KeyEsc {
		if m.focus == focusAttrs && m.attrs.Editing() {
			return m.forward(k)
		}
		m.exitDive()
		return m, nil
	}
	return m.forward(k)
}

// forward hands the message to the focused field. Ctrl-D and Ctrl-C are
// consumed in Update before this, so the embedded JSON editor never finishes
// on its own.
func (m Model) forward(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focus {
	case focusEnv:
		m.env, cmd = m.env.Update(msg)
	case focusPolicyGroup:
		m.policyGroup, cmd = m.policyGroup.Update(msg)
	case focusPolicyName:
		m.policyName, cmd = m.policyName.Update(msg)
	case focusRunList:
		m.runList, cmd = m.runList.Update(msg)
	case focusAttrs:
		m.attrs, cmd = m.attrs.Update(msg)
	}
	return m, cmd
}

// save assembles the node from every field and finishes the form. On a
// validation error (e.g. an unknown attribute bag) it shows the message and
// moves focus to the attributes so the user can fix it.
func (m Model) save() Model {
	node, err := buildNode(
		m.orig,
		m.env.Value(),
		m.policyName.Value(),
		m.policyGroup.Value(),
		parseRunList(m.runList.Value()),
		m.attrs.Value(),
	)
	if err != nil {
		m.errMsg = err.Error()
		m.setFocus(focusAttrs)
		return m
	}
	m.errMsg = ""
	m.result = node
	m.changed = !nodeUnchanged(m.orig, node)
	m.finished = true
	return m
}

func (m *Model) setFocus(f focus) {
	switch {
	case f < 0:
		f = 0
	case f >= focusCount:
		f = focusCount - 1
	}
	m.focus = f
	m.dived = false
	m.env.Blur()
	m.policyGroup.Blur()
	m.policyName.Blur()
	m.runList.Blur()
	switch m.focus {
	case focusEnv:
		m.env.Focus()
	case focusPolicyGroup:
		m.policyGroup.Focus()
	case focusPolicyName:
		m.policyName.Focus()
	}
}

// enterDive activates editing inside the focused run-list or attributes
// section.
func (m *Model) enterDive() {
	m.dived = true
	if m.focus == focusRunList {
		m.runList.Focus()
	}
}

// exitDive returns to the field cursor.
func (m *Model) exitDive() {
	m.dived = false
	m.runList.Blur()
}

func (m *Model) setSize(w, h int) {
	m.width, m.height = w, h
	fieldWidth := max(w-16, 10)
	m.env.Width = fieldWidth
	m.policyGroup.Width = fieldWidth
	m.policyName.Width = fieldWidth
	m.runList.SetWidth(w)
	m.runList.SetHeight(4)
	// The attributes editor takes whatever vertical space the fixed chrome
	// (heading, fields, run list, labels, footer) leaves.
	m.attrs.SetSize(w, max(h-14, 3))
}

// Finished reports whether the user saved or cancelled.
func (m Model) Finished() bool { return m.finished }

// Aborted reports whether the user cancelled.
func (m Model) Aborted() bool { return m.aborted }

// Changed reports whether the saved node differs from the original.
func (m Model) Changed() bool { return m.changed }

// Result is the assembled node after a save, or nil if cancelled or still
// editing.
func (m Model) Result() *cinc.Node { return m.result }

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Node: "+m.name) + "\n")
	b.WriteString(ruleStyle.Render(strings.Repeat("─", 48)) + "\n")
	b.WriteString(m.field("Environment", focusEnv, m.env.View()))
	b.WriteString(m.field("Policy group", focusPolicyGroup, m.policyGroup.View()))
	b.WriteString(m.field("Policy name", focusPolicyName, m.policyName.View()))
	b.WriteString("\n" + m.fieldLabel("Run list", focusRunList) + "\n")
	b.WriteString(m.runList.View() + "\n")
	b.WriteString("\n" + m.fieldLabel("Attributes:", focusAttrs) + "\n")
	b.WriteString(m.attrs.ContentView() + "\n")
	b.WriteString("\n" + m.footer())
	return b.String()
}

// field renders a single-line "label  value" row.
func (m Model) field(label string, f focus, view string) string {
	return m.fieldLabel(label, f) + "  " + view + "\n"
}

// fieldLabel renders a focus-aware, fixed-width label with a cursor marker.
func (m Model) fieldLabel(label string, f focus) string {
	style, marker := labelStyle, "  "
	if m.focus == f {
		style, marker = focusLabelStyle, "▶ "
	}
	return marker + style.Render(fmt.Sprintf("%-13s", label))
}

func (m Model) footer() string {
	var hint string
	switch {
	case m.dived && m.focus == focusAttrs:
		hint = "↑/↓ move · Enter edit · a add · d delete · Esc back · Ctrl-D save"
	case m.dived && m.focus == focusRunList:
		hint = "one entry per line · Esc back · Ctrl-D save"
	case m.focus == focusRunList || m.focus == focusAttrs:
		hint = "↑/↓ move · Enter edit · Ctrl-D save · Esc cancel"
	default:
		hint = "↑/↓ move · type to edit · Ctrl-D save · Esc cancel"
	}
	if m.errMsg != "" {
		return errStyle.Render("Error: "+m.errMsg) + "\n" + hintStyle.Render(hint)
	}
	return hintStyle.Render(hint)
}
