package nodeedit

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/jsoneditor"
)

func keyType(m Model, t tea.KeyType) Model { m, _ = m.Update(tea.KeyMsg{Type: t}); return m }
func ctrlD(m Model) Model                  { return keyType(m, tea.KeyCtrlD) }
func ctrlC(m Model) Model                  { return keyType(m, tea.KeyCtrlC) }
func tab(m Model) Model                    { return keyType(m, tea.KeyTab) }

func typeRunes(m Model, s string) Model {
	for _, r := range s {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func newForm(t *testing.T, n *cinc.Node) Model {
	t.Helper()
	m, err := New(n)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

func TestFormSavesEditedEnvironment(t *testing.T) {
	m := newForm(t, &cinc.Node{Name: "db01", Environment: "prod"})
	m.env.SetValue("staging")
	m = ctrlD(m)
	if !m.Finished() || m.Aborted() {
		t.Fatalf("expected a saved edit, finished=%v aborted=%v", m.Finished(), m.Aborted())
	}
	if !m.Changed() {
		t.Error("an environment change should be reported as changed")
	}
	got := m.Result()
	if got.Environment != "staging" {
		t.Errorf("environment = %q, want staging", got.Environment)
	}
	if got.Name != "db01" {
		t.Errorf("name = %q, want db01 (carried from original)", got.Name)
	}
}

func TestFormUnchangedWhenNothingEdited(t *testing.T) {
	node := &cinc.Node{
		Name:        "db01",
		Environment: "prod",
		RunList:     []string{"recipe[nginx]"},
		Normal:      cinc.Attributes{"role": "db"},
	}
	m := ctrlD(newForm(t, node))
	if !m.Finished() {
		t.Fatal("Ctrl-D should finish the form")
	}
	if m.Changed() {
		t.Error("no edits should report unchanged")
	}
}

func TestFormAbortsOnCtrlC(t *testing.T) {
	m := ctrlC(newForm(t, &cinc.Node{Name: "db01"}))
	if !m.Finished() || !m.Aborted() {
		t.Fatalf("Ctrl-C should abort: finished=%v aborted=%v", m.Finished(), m.Aborted())
	}
	if m.Result() != nil {
		t.Error("an aborted edit must not produce a result")
	}
}

func TestFormTabCyclesFocus(t *testing.T) {
	m := newForm(t, &cinc.Node{Name: "db01"})
	if m.focus != focusEnv {
		t.Fatalf("form should start focused on environment, got %v", m.focus)
	}
	want := []focus{focusPolicyGroup, focusPolicyName, focusRunList, focusAttrs, focusEnv}
	for i, w := range want {
		m = tab(m)
		if m.focus != w {
			t.Errorf("after %d tabs focus=%v, want %v", i+1, m.focus, w)
		}
	}
}

func TestFormEditsRunListByTyping(t *testing.T) {
	m := newForm(t, &cinc.Node{Name: "db01"})
	for m.focus != focusRunList {
		m = tab(m)
	}
	m = typeRunes(m, "recipe[apache]")
	m = ctrlD(m)
	rl := m.Result().RunList
	if len(rl) != 1 || rl[0] != "recipe[apache]" {
		t.Errorf("run_list = %v, want [recipe[apache]]", rl)
	}
}

func TestFormSavesAttributeEdits(t *testing.T) {
	m := newForm(t, &cinc.Node{Name: "db01"})
	// Stand in a fresh attributes editor carrying an edit, then save.
	m.attrs = jsoneditor.New(
		[]byte(`{"normal":{"role":"db"},"default":{},"override":{},"automatic":{}}`),
		func([]byte) error { return nil },
	)
	m = ctrlD(m)
	if m.Result().Normal["role"] != "db" {
		t.Errorf("attribute edit not saved: %v", m.Result().Normal)
	}
	if !m.Changed() {
		t.Error("adding an attribute should be a change")
	}
}

func TestFormRejectsInvalidAttributeBag(t *testing.T) {
	m := newForm(t, &cinc.Node{Name: "db01"})
	m.attrs = jsoneditor.New(
		[]byte(`{"normal":{},"default":{},"override":{},"automatic":{},"bogus":{}}`),
		func([]byte) error { return nil },
	)
	m = ctrlD(m)
	if m.Finished() {
		t.Fatal("an unknown attribute bag should keep the form open")
	}
	if m.errMsg == "" {
		t.Error("expected an inline error for the unknown bag")
	}
}
