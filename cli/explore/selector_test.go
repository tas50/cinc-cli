package explore

import (
	"strings"
	"testing"
)

// The selected row — in both the kind menu and the object table — should
// carry the pronounced cursor marker, and never the old thin chevron.
func TestSelectedRowUsesPronouncedMarker(t *testing.T) {
	m := model{styles: newStyles()}

	choice := m.renderChoice("nodes", true)
	if !strings.Contains(choice, "▶") {
		t.Errorf("selected kind missing pronounced marker, got %q", choice)
	}
	if strings.Contains(choice, "❯") {
		t.Errorf("selected kind still uses the old chevron, got %q", choice)
	}

	m.cur = cookbookKind{}
	m.height = 24
	rows := []Row{{Name: "apache", Cells: []string{"apache", "1.0.0"}}}
	table := m.renderTable(rows)
	if !strings.Contains(table, "▶") {
		t.Errorf("selected table row missing pronounced marker, got %q", table)
	}
	if strings.Contains(table, "❯") {
		t.Errorf("selected table row still uses the old chevron, got %q", table)
	}
}

// Unselected rows keep a two-column blank prefix so they stay aligned
// with the marked row.
func TestUnselectedRowKeepsAlignmentPrefix(t *testing.T) {
	m := model{styles: newStyles()}
	choice := m.renderChoice("nodes", false)
	if strings.Contains(choice, "▶") {
		t.Errorf("unselected kind should not carry the marker, got %q", choice)
	}
	if !strings.Contains(choice, "  nodes") {
		t.Errorf("unselected kind lost its alignment prefix, got %q", choice)
	}
}
