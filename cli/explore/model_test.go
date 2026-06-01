package explore

import (
	"net/http"
	"net/http/httptest"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// nodesMux serves a node index and a single node document.
func nodesMux(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	jsonHandler(mux, "/organizations/acme/nodes", `{"web01":"u","web02":"u","db01":"u"}`)
	jsonHandler(mux, "/organizations/acme/nodes/web01", `{"name":"web01","run_list":["recipe[apache]"]}`)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestKindMenuOpensListSortedByName(t *testing.T) {
	m := modelOnKinds(t, testClient(t, nodesMux(t)))

	// Nodes is the first kind; Enter opens its list.
	m, cmd := pressKey(t, m, 13) // enter
	if m.screen != screenList {
		t.Fatalf("screen = %v, want list", m.screen)
	}
	m, _ = step(t, m, drain(cmd))

	got := names(m.filteredRows())
	want := []string{"db01", "web01", "web02"}
	if !equal(got, want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func TestEnterOpensDetailForLeafKind(t *testing.T) {
	m := modelOnKinds(t, testClient(t, nodesMux(t)))
	m, cmd := pressKey(t, m, 13)
	m, _ = step(t, m, drain(cmd))

	// Cursor on db01 (sorted first); move to web01 then open detail.
	m.cursor = 1 // web01
	m, cmd = pressKey(t, m, 13)
	if m.screen != screenDetail {
		t.Fatalf("screen = %v, want detail", m.screen)
	}
	m, _ = step(t, m, drain(cmd))
	if m.detailName != "web01" {
		t.Errorf("detailName = %q, want web01", m.detailName)
	}
	if got := m.detail.View(); !contains(got, "apache") {
		t.Errorf("detail did not render node body, got %q", got)
	}
}

func TestListFilterNarrowsRows(t *testing.T) {
	m := modelOnKinds(t, testClient(t, nodesMux(t)))
	m, cmd := pressKey(t, m, 13)
	m, _ = step(t, m, drain(cmd))

	m, _ = pressRune(t, m, '/') // enter filter mode
	if !m.filtering {
		t.Fatal("expected filtering mode")
	}
	m, _ = step(t, m, keyRunes("web"))
	got := names(m.filteredRows())
	if !equal(got, []string{"web01", "web02"}) {
		t.Errorf("filtered rows = %v, want web01,web02", got)
	}
}

func TestStaleListResponseDropped(t *testing.T) {
	m := modelOnKinds(t, testClient(t, nodesMux(t)))
	m, cmd := pressKey(t, m, 13)
	m, _ = step(t, m, drain(cmd)) // reqID 1 applied, rows present
	before := len(m.rows)

	// A response stamped with an old reqID must be ignored.
	m, _ = step(t, m, listLoadedMsg{reqID: 0, rows: nil})
	if len(m.rows) != before {
		t.Errorf("stale response mutated rows: %d → %d", before, len(m.rows))
	}
}

func TestDrillDownPushAndPop(t *testing.T) {
	mux := http.NewServeMux()
	jsonHandler(mux, "/organizations/acme/data", `{"creds":"u","apps":"u"}`)
	jsonHandler(mux, "/organizations/acme/data/creds", `{"aws":"u","db":"u"}`)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	m := modelOnKinds(t, testClient(t, srv))
	m.kindCursor = 6 // Data Bags
	m, cmd := pressKey(t, m, 13)
	m, _ = step(t, m, drain(cmd))
	if got := names(m.filteredRows()); !equal(got, []string{"apps", "creds"}) {
		t.Fatalf("bag rows = %v", got)
	}

	// Enter on "creds" (cursor 1) drills into items.
	m.cursor = 1
	m, cmd = pressKey(t, m, 13)
	if len(m.stack) != 1 {
		t.Fatalf("stack depth = %d, want 1", len(m.stack))
	}
	m, _ = step(t, m, drain(cmd))
	if got := names(m.filteredRows()); !equal(got, []string{"aws", "db"}) {
		t.Fatalf("item rows = %v, want aws,db", got)
	}
	if m.crumb() != "Data Bags › creds" {
		t.Errorf("crumb = %q, want 'Data Bags › creds'", m.crumb())
	}

	// Esc pops back to the bag list, restoring the cursor.
	m, _ = pressKey(t, m, 27) // esc
	if len(m.stack) != 0 {
		t.Fatalf("stack depth after pop = %d, want 0", len(m.stack))
	}
	if m.crumb() != "Data Bags" {
		t.Errorf("crumb after pop = %q", m.crumb())
	}
	if m.cursor != 1 {
		t.Errorf("cursor not restored: %d, want 1", m.cursor)
	}
}

func TestEscAtTopLevelReturnsToKindMenu(t *testing.T) {
	m := modelOnKinds(t, testClient(t, nodesMux(t)))
	m, cmd := pressKey(t, m, 13)
	m, _ = step(t, m, drain(cmd))
	m, _ = pressKey(t, m, 27) // esc
	if m.screen != screenKinds {
		t.Errorf("screen = %v, want kinds", m.screen)
	}
}

// ----- small assertion helpers ----------------------------------------

func names(rows []Row) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Name
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// keyRunes builds a KeyMsg that types s in one batch, which the
// textinput model accepts as pasted-style input.
func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
