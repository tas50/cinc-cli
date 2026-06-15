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

// commit drives the save flow to completion: Ctrl-D shows the preview,
// Enter confirms it.
func commit(m Model) Model { return enter(ctrlD(m)) }

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

func TestStructuralEditWholeBlock(t *testing.T) {
	m := New([]byte(`{"l":[1]}`), func([]byte) error { return nil })
	m = down(m)  // key[0] -> block[0] (the array)
	m = enter(m) // begin block edit
	if m.state != stBlockEdit {
		t.Fatalf("expected block edit, state=%v", m.state)
	}
	m.textarea.SetValue(`[2,3,4]`)
	m = ctrlD(m) // apply the block
	m = commit(m)
	got := string(m.Committed())
	if !strings.Contains(got, "2") || !strings.Contains(got, "3") || !strings.Contains(got, "4") {
		t.Errorf("block edit not committed, got %q", got)
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
	for i := 0; i < 50; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `"key%02d":%d`, i, i)
	}
	b.WriteString("}")

	m := New([]byte(b.String()), func([]byte) error { return nil })
	m.SetSize(80, 12) // viewHeight == 8 lines

	for i := 0; i < 80; i++ { // walk down to key40's key unit
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
