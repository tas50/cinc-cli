package explore

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// nodesServer returns an httptest server with a two-node index and a
// detail document for web01 carrying the attributes the summary reads.
func nodesServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	jsonHandler(mux, "/organizations/acme/nodes", `{"web01":"u","web02":"u"}`)
	jsonHandler(mux, "/organizations/acme/nodes/web01", `{
		"name": "web01",
		"chef_environment": "production",
		"policy_group": "web",
		"run_list": ["role[base]"],
		"automatic": {
			"ohai_time": 1000000000.0,
			"chef_packages": {"chef": {"version": "18.4.2"}}
		}
	}`)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// openNodes drives the model from the kind menu into the loaded Nodes
// list, returning the model and the command handleListLoaded produced
// (the summary debounce tick).
func openNodes(t *testing.T, srv *httptest.Server) (model, tea.Cmd) {
	t.Helper()
	m := modelOnKinds(t, testClient(t, srv))
	m, cmd := pressKey(t, m, tea.KeyEnter) // openList → listCmd
	return step(t, m, drain(cmd))          // listLoadedMsg → summary tick
}

func TestSummaryPanelLoadsForSelectedNode(t *testing.T) {
	m, tick := openNodes(t, nodesServer(t))

	// The debounce tick fires, then the fetch resolves into the cache.
	m, fetch := step(t, m, drain(tick)) // summaryTickMsg → summaryCmd
	m, _ = step(t, m, drain(fetch))     // summaryLoadedMsg → cache

	view, ok := m.summaryCache["web01"]
	if !ok || len(view.Fields) == 0 {
		t.Fatalf("web01 summary not cached as fields: %+v", m.summaryCache)
	}
	out := m.View()
	for _, want := range []string{"Environment", "production", "Policy Group", "web", "18.4.2"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered view missing %q\n%s", want, out)
		}
	}
}

func TestStaleSummaryTickIgnored(t *testing.T) {
	m, _ := openNodes(t, nodesServer(t))
	// A later selection has advanced the debounce counter.
	m.debounceID = 5
	// A tick from an earlier selection must not start a fetch.
	_, cmd := m.handleSummaryTick(summaryTickMsg{id: 4, name: "web01"})
	if cmd != nil {
		t.Errorf("stale tick (id 4 vs current 5) should not fetch")
	}
}

func TestSummaryNotRefetchedWhenCached(t *testing.T) {
	m, tick := openNodes(t, nodesServer(t))
	m, fetch := step(t, m, drain(tick))
	m, _ = step(t, m, drain(fetch)) // web01 now cached

	// Re-arming for the already-cached row yields no tick.
	if cmd := m.scheduleSummary(); cmd != nil {
		t.Errorf("scheduleSummary on a cached row should return nil")
	}
}

func TestListViewIsSplitAndFooterPinned(t *testing.T) {
	m, _ := openNodes(t, nodesServer(t))
	// Sized wide by modelOnKinds (100x40), so the split is active.
	out := m.View()
	if h := lipgloss.Height(out); h != 40 {
		t.Errorf("height = %d, want 40", h)
	}
	for i, ln := range strings.Split(out, "\n") {
		if w := lipgloss.Width(ln); w != 100 {
			t.Errorf("line %d width = %d, want 100", i, w)
		}
	}
	if !strings.Contains(out, "│") {
		t.Errorf("split separator missing from view:\n%s", out)
	}
	// Footer hints still sit just above the bottom border.
	lines := strings.Split(out, "\n")
	if footer := lines[len(lines)-2]; !strings.Contains(footer, "quit") {
		t.Errorf("footer not pinned; got %q", footer)
	}
}

func TestNarrowTerminalFallsBackToList(t *testing.T) {
	m, _ := openNodes(t, nodesServer(t))
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	m = next.(model)
	out := m.View()
	// Below splitMinWidth there's no separator column running the body.
	body := strings.Split(out, "\n")
	rules := 0
	for _, ln := range body {
		if strings.Count(ln, "│") > 2 { // more than the two frame walls
			rules++
		}
	}
	if rules != 0 {
		t.Errorf("narrow terminal should not draw a split separator:\n%s", out)
	}
}
