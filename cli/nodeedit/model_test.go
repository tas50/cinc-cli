package nodeedit

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	cinc "github.com/tas50/cinc-api"
)

// newModel builds a model and feeds the initial WindowSizeMsg the real
// program always sends first, so sizing matches production.
func newModel(in *cinc.Node) Model {
	m := New(in)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	return m
}

func key(m Model, t tea.KeyType) Model {
	m, _ = m.Update(tea.KeyMsg{Type: t})
	return m
}

func typeRunes(m Model, s string) Model {
	for _, r := range s {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestNewSeedsScalarFieldsAndBags(t *testing.T) {
	m := newModel(&cinc.Node{
		Name:    "web01",
		RunList: []string{"recipe[base]", "role[web]"},
		Normal:  cinc.Attributes{"role": "db"},
	})
	if got := m.inputs[0].Value(); got != "_default" {
		t.Errorf("chef_environment = %q, want _default for an empty environment", got)
	}
	if got := m.inputs[1].Value(); got != "recipe[base], role[web]" {
		t.Errorf("run_list field = %q, want the comma-joined run list", got)
	}
	if len(m.bags["normal"]) != 1 || len(m.bags["default"]) != 0 {
		t.Errorf("bags not seeded from the node: %+v", m.bags)
	}
}

func TestEditingScalarFieldUpdatesResult(t *testing.T) {
	m := newModel(&cinc.Node{Name: "web01", Environment: "prod"})
	// chef_environment is focused first; clear it and retype.
	m = typeRunes(key(m, tea.KeyCtrlU), "staging")
	out := m.Result()
	if out.Environment != "staging" {
		t.Errorf("environment = %q, want staging", out.Environment)
	}
}

func TestRunListFieldSplitsOnSave(t *testing.T) {
	m := newModel(&cinc.Node{Name: "web01"})
	m = key(m, tea.KeyDown) // move to run_list
	m = typeRunes(m, "recipe[a], role[b] ,, recipe[c]")
	out := m.Result()
	want := []string{"recipe[a]", "role[b]", "recipe[c]"}
	if len(out.RunList) != len(want) {
		t.Fatalf("run_list = %v, want %v", out.RunList, want)
	}
	for i := range want {
		if out.RunList[i] != want[i] {
			t.Errorf("run_list[%d] = %q, want %q", i, out.RunList[i], want[i])
		}
	}
}

func TestEnterOnAttributeRowOpensEditor(t *testing.T) {
	m := newModel(&cinc.Node{Name: "web01"})
	for range fieldCount {
		m = key(m, tea.KeyDown) // land on the first bag row (normal)
	}
	m = key(m, tea.KeyEnter)
	if m.screen != screenEditor {
		t.Fatalf("screen = %v, want editor after selecting an attribute bag", m.screen)
	}
	if m.editBag != "normal" {
		t.Errorf("editBag = %q, want normal", m.editBag)
	}
}

func TestEditingABagFoldsBackOnCommit(t *testing.T) {
	m := newModel(&cinc.Node{Name: "web01"})
	for range fieldCount {
		m = key(m, tea.KeyDown)
	}
	m = key(m, tea.KeyEnter) // open the normal bag editor (seed "{}")
	m = typeRunes(m, "a")    // 'a' adds a new member to the object
	m = key(m, tea.KeyCtrlD) // save the bag
	if m.screen != screenForm {
		t.Fatalf("screen = %v, want form after committing the bag", m.screen)
	}
	if _, ok := m.bags["normal"]["newKey"]; !ok {
		t.Errorf("committed bag not folded back: %+v", m.bags["normal"])
	}
}

func TestAbortingABagDiscardsChanges(t *testing.T) {
	m := newModel(&cinc.Node{Name: "web01", Normal: cinc.Attributes{"role": "db"}})
	for range fieldCount {
		m = key(m, tea.KeyDown)
	}
	m = key(m, tea.KeyEnter) // open normal
	m = typeRunes(m, "a")    // add a member, then change our mind
	m = key(m, tea.KeyEsc)   // abort the bag edit
	if m.screen != screenForm {
		t.Fatalf("screen = %v, want form after aborting the bag", m.screen)
	}
	if len(m.bags["normal"]) != 1 {
		t.Errorf("aborted bag should keep its original keys, got %+v", m.bags["normal"])
	}
}

func TestCtrlDOnFormSavesAndFinishes(t *testing.T) {
	m := newModel(&cinc.Node{Name: "web01"})
	m = key(m, tea.KeyCtrlD)
	if !m.Finished() || m.Aborted() {
		t.Fatalf("Ctrl-D should finish without aborting; finished=%v aborted=%v", m.Finished(), m.Aborted())
	}
}

func TestEscOnFormAborts(t *testing.T) {
	m := newModel(&cinc.Node{Name: "web01"})
	m = key(m, tea.KeyEsc)
	if !m.Finished() || !m.Aborted() {
		t.Fatalf("Esc should abort; finished=%v aborted=%v", m.Finished(), m.Aborted())
	}
}

func TestValidateAttributesRejectsNonObject(t *testing.T) {
	if err := validateAttributes([]byte(`[1,2,3]`)); err == nil {
		t.Error("an array is not a valid attribute bag")
	}
	if err := validateAttributes([]byte(`{"a":1}`)); err != nil {
		t.Errorf("an object is a valid attribute bag: %v", err)
	}
}

func TestViewShowsWarningAndBags(t *testing.T) {
	m := newModel(&cinc.Node{Name: "web01"})
	v := m.View()
	for _, want := range []string{"web01", "overwritten on the next cinc run", "normal", "automatic"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q", want)
		}
	}
}
