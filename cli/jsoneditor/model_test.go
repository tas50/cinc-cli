package jsoneditor

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func press(m Model, t tea.KeyType) Model { m, _ = m.Update(tea.KeyMsg{Type: t}); return m }
func down(m Model) Model                 { return press(m, tea.KeyDown) }
func tab(m Model) Model                  { return press(m, tea.KeyTab) }
func runeKey(m Model, r rune) Model {
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return m
}

// commit saves a structural edit. Structural mode commits immediately on
// Ctrl-D — no preview to confirm.
func commit(m Model) Model { return ctrlD(m) }

func TestStructuralEditScalarValue(t *testing.T) {
	m := New([]byte(`{"name":"web01"}`), func([]byte) error { return nil })
	m = down(m)  // key[0] -> scalar[0]
	m = enter(m) // begin inline edit of the value
	m.input.SetValue(`"web99"`)
	m = enter(m) // apply
	m = commit(m)
	if got := string(m.Committed()); !strings.Contains(got, `"name": "web99"`) {
		t.Errorf("scalar edit not committed, got %q", got)
	}
}

func TestStructuralEditKey(t *testing.T) {
	m := New([]byte(`{"name":"web01"}`), func([]byte) error { return nil })
	m = enter(m) // begin inline edit of the key
	m.input.SetValue("host")
	m = enter(m) // apply
	m = commit(m)
	if got := string(m.Committed()); !strings.Contains(got, `"host": "web01"`) {
		t.Errorf("key rename not committed, got %q", got)
	}
}

func TestStructuralRejectsInvalidScalar(t *testing.T) {
	m := New([]byte(`{"port":80}`), func([]byte) error { return nil })
	m = down(m)
	m = enter(m)
	m.input.SetValue(`not json`)
	m = enter(m) // apply -> should fail and keep editing
	if m.state != stInlineEdit {
		t.Fatalf("invalid scalar should keep editing, state=%v", m.state)
	}
	if m.errMsg == "" {
		t.Error("expected an inline error for invalid JSON")
	}
}

func TestStructuralAddMember(t *testing.T) {
	m := New([]byte(`{"a":1}`), func([]byte) error { return nil })
	m = runeKey(m, 'a') // add sibling member after "a"
	m = commit(m)
	got := string(m.Committed())
	if !strings.Contains(got, `"a": 1`) || !strings.Contains(got, `"newKey": null`) {
		t.Errorf("add member not committed, got %q", got)
	}
}

func TestStructuralDeleteMember(t *testing.T) {
	m := New([]byte(`{"a":1,"b":2}`), func([]byte) error { return nil })
	m = runeKey(m, 'd') // delete "a"
	m = commit(m)
	got := string(m.Committed())
	if strings.Contains(got, `"a"`) {
		t.Errorf("deleted key still present, got %q", got)
	}
	if !strings.Contains(got, `"b": 2`) {
		t.Errorf("surviving key missing, got %q", got)
	}
}

func TestStructuralEditWholeObjectBlock(t *testing.T) {
	m := New([]byte(`{"o":{"x":1}}`), func([]byte) error { return nil })
	m = down(m)  // key[0] -> block[0] (the object)
	m = enter(m) // objects still open as a whole-block edit
	if m.state != stBlockEdit {
		t.Fatalf("expected block edit, state=%v", m.state)
	}
	m.textarea.SetValue(`{"y":2,"z":3}`)
	m = ctrlD(m) // apply the block
	m = commit(m)
	got := string(m.Committed())
	if !strings.Contains(got, `"y": 2`) || !strings.Contains(got, `"z": 3`) {
		t.Errorf("object block edit not committed, got %q", got)
	}
}

func TestStructuralEnterArrayDrillsIntoEntries(t *testing.T) {
	m := New([]byte(`{"l":["a","b"]}`), func([]byte) error { return nil })
	m = down(m)  // key[0] -> block[0] (the array)
	m = enter(m) // arrays drill into their entries, not a raw block edit
	if m.state == stBlockEdit {
		t.Fatal("entering an array should not open a raw block edit")
	}
	u := m.selected()
	if u.typ != uScalar || !pathEq(u.path, []int{0, 0}) {
		t.Errorf("expected the first array entry selected, got %+v", u)
	}
}

func TestStructuralEnterEmptyArrayAddsEntry(t *testing.T) {
	m := New([]byte(`{"l":[]}`), func([]byte) error { return nil })
	m = down(m)  // key[0] -> block[0] (empty array)
	m = enter(m) // opening an empty array gives it a first entry to edit
	if m.state == stBlockEdit {
		t.Fatal("entering an array should not open a raw block edit")
	}
	u := m.selected()
	if u.typ != uScalar || !pathEq(u.path, []int{0, 0}) {
		t.Errorf("expected a new first entry selected, got %+v", u)
	}
	m = commit(m)
	if !strings.Contains(string(m.Committed()), "null") {
		t.Errorf("new entry not committed, got %q", m.Committed())
	}
}

func TestRawModeToggleRoundTrips(t *testing.T) {
	m := New([]byte(`{"a":1}`), func([]byte) error { return nil })
	m = tab(m) // structural -> raw
	if m.mode != modeRaw {
		t.Fatalf("Tab should switch to raw mode, mode=%v", m.mode)
	}
	m.textarea.SetValue(`{"a":2}`)
	m = tab(m) // raw -> structural (reparses)
	if m.mode != modeStructural {
		t.Fatalf("Tab should switch back to structural, mode=%v", m.mode)
	}
	m = commit(m)
	if got := string(m.Committed()); !strings.Contains(got, `"a": 2`) {
		t.Errorf("raw edit not carried back into the tree, got %q", got)
	}
}

func TestValueReflectsLiveEditsWithoutCommit(t *testing.T) {
	m := New([]byte(`{"a":1}`), func([]byte) error { return nil })
	if got := string(m.Value()); got != "{\n  \"a\": 1\n}" {
		t.Errorf("initial Value = %q", got)
	}
	m = down(m)  // key[0] -> scalar[0]
	m = enter(m) // inline edit
	m.input.SetValue("2")
	m = enter(m) // apply (no commit)
	if m.Finished() {
		t.Fatal("editing a value must not finish the editor")
	}
	if got := string(m.Value()); !strings.Contains(got, `"a": 2`) {
		t.Errorf("Value after edit = %q, want it to reflect the change", got)
	}
}

func TestStructuralSaveIsImmediate(t *testing.T) {
	m := New([]byte(`{"a":1}`), func([]byte) error { return nil })
	m = ctrlD(m) // structural: commits without a preview step
	if !m.Finished() {
		t.Fatal("structural Ctrl-D should commit immediately")
	}
	if !strings.Contains(string(m.Committed()), `"a": 1`) {
		t.Errorf("unexpected committed content %q", m.Committed())
	}
}

func TestRawModeSaveStillPreviews(t *testing.T) {
	m := New([]byte(`{"a":1}`), func([]byte) error { return nil })
	m = tab(m)   // structural -> raw
	m = ctrlD(m) // raw save shows a preview to confirm
	if m.Finished() {
		t.Fatal("raw-mode save should preview before committing")
	}
	if m.state != stPreview {
		t.Fatalf("expected preview state, got %v", m.state)
	}
	m = enter(m) // confirm the preview
	if !m.Finished() {
		t.Fatal("Enter should confirm the raw-mode preview")
	}
}

func TestRawModeInvalidJSONBlocksToggle(t *testing.T) {
	m := New([]byte(`{"a":1}`), func([]byte) error { return nil })
	m = tab(m)
	m.textarea.SetValue(`{"a":`)
	m = tab(m) // should refuse to leave raw mode
	if m.mode != modeRaw {
		t.Errorf("invalid raw JSON should keep raw mode, mode=%v", m.mode)
	}
	if m.errMsg == "" {
		t.Error("expected a parse error when toggling with invalid JSON")
	}
}

func TestMalformedSeedOpensRawMode(t *testing.T) {
	m := New([]byte(`{not valid`), func([]byte) error { return nil })
	if m.mode != modeRaw {
		t.Errorf("malformed seed should open raw mode, mode=%v", m.mode)
	}
	if m.errMsg == "" {
		t.Error("expected a parse error message for malformed seed")
	}
}

func TestStructuralViewWindowsToSelection(t *testing.T) {
	// A tall document must scroll so the selected line stays on screen.
	// (Reverse-video highlight isn't asserted here: lipgloss strips
	// styling when stdout is not a TTY, as under `go test`. Highlight
	// placement is covered by render_test.go.)
	var b strings.Builder
	b.WriteString("{")
	for i := range 50 {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `"key%02d":%d`, i, i)
	}
	b.WriteString("}")

	m := New([]byte(b.String()), func([]byte) error { return nil })
	m.SetSize(80, 12) // viewHeight == 8 lines

	for range 80 { // walk down to key40's key unit
		m = down(m)
	}
	view := m.View()
	if !strings.Contains(view, `"key40"`) {
		t.Errorf("selected key scrolled out of view:\n%s", view)
	}
	if strings.Contains(view, `"key00"`) {
		t.Errorf("far-off first key should have scrolled away:\n%s", view)
	}
}
