package explore

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cinc-project/cinc-cli/cli/jsoneditor"
)

// subEditor is the modal editor the explorer drives while editing or
// creating an object. It owns the screen while open, advances on each
// message, and reports when the user commits (Committed returns the JSON
// document to save or create) or aborts. The generic JSON editor and the
// typed node form both plug in through this interface, so the edit/create
// flow in input.go is the same regardless of which one is on screen.
type subEditor interface {
	Init() tea.Cmd
	Update(tea.Msg) (subEditor, tea.Cmd)
	View() string
	SetSize(w, h int)
	Finished() bool
	Aborted() bool
	Committed() []byte
}

// CustomForm kinds supply their own modal editor (a typed form) instead of
// the generic JSON editor. NewForm returns nil to fall back to the JSON
// editor — so a kind can offer a form for some actions and not others.
type CustomForm interface {
	NewForm(action editAction, seed []byte) (subEditor, error)
}

// jsonEditorAdapter wraps the JSON editor as a subEditor. It is the default
// when a kind has no custom form.
type jsonEditorAdapter struct{ m jsoneditor.Model }

// newJSONEditor builds the default JSON editor seeded with the given bytes.
func newJSONEditor(seed []byte) *jsonEditorAdapter {
	return &jsonEditorAdapter{m: jsoneditor.New(seed, jsonSyntaxOnly)}
}

// jsonSyntaxOnly accepts any well-formed JSON; per-kind validation happens
// server-side on save. The editor still pretty-prints and previews before
// committing.
func jsonSyntaxOnly([]byte) error { return nil }

func (a *jsonEditorAdapter) Init() tea.Cmd { return a.m.Init() }

func (a *jsonEditorAdapter) Update(msg tea.Msg) (subEditor, tea.Cmd) {
	var cmd tea.Cmd
	a.m, cmd = a.m.Update(msg)
	return a, cmd
}

func (a *jsonEditorAdapter) View() string      { return a.m.View() }
func (a *jsonEditorAdapter) SetSize(w, h int)  { a.m.SetSize(w, h) }
func (a *jsonEditorAdapter) Finished() bool    { return a.m.Finished() }
func (a *jsonEditorAdapter) Aborted() bool     { return a.m.Aborted() }
func (a *jsonEditorAdapter) Committed() []byte { return a.m.Committed() }
