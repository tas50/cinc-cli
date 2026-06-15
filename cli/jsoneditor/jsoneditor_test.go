package jsoneditor

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func ctrlD(m Model) Model { m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlD}); return m }
func enter(m Model) Model { m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}); return m }
func esc(m Model) Model   { m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}); return m }

func TestEditorCommitsCanonicalJSON(t *testing.T) {
	m := New([]byte(`{"b":2,"a":1}`), func([]byte) error { return nil })
	// Structural mode is always valid JSON, so Ctrl-D commits immediately
	// — no preview/confirm round-trip.
	m = ctrlD(m)
	if !m.Finished() || m.Aborted() {
		t.Fatalf("expected an immediate committed edit, finished=%v aborted=%v", m.Finished(), m.Aborted())
	}
	got := string(m.Committed())
	if !strings.Contains(got, `"a": 1`) || !strings.Contains(got, `"b": 2`) {
		t.Errorf("committed JSON not canonicalised: %q", got)
	}
}

func TestEditorAbortsOnEsc(t *testing.T) {
	m := New([]byte(`{}`), func([]byte) error { return nil })
	m = esc(m)
	if !m.Finished() || !m.Aborted() {
		t.Fatalf("esc should abort: finished=%v aborted=%v", m.Finished(), m.Aborted())
	}
	if m.Committed() != nil {
		t.Error("aborted edit must not commit")
	}
}

func TestEditorKeepsEditingOnValidationError(t *testing.T) {
	m := New([]byte(`{}`), func([]byte) error { return errors.New("nope") })
	m = ctrlD(m)
	if m.Finished() {
		t.Fatal("validation failure must not finish the edit")
	}
	if !strings.Contains(m.View(), "nope") {
		t.Errorf("expected the validation error in the view, got %q", m.View())
	}
}
