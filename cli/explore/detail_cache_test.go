package explore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// countingNodeServer serves a one-node index plus a web01 detail document,
// counting every GET of the detail endpoint so a test can assert how many
// times the node object was actually fetched.
func countingNodeServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var detailHits int32
	mux := http.NewServeMux()
	jsonHandler(mux, "/organizations/acme/nodes", `{"web01":"u"}`)
	mux.HandleFunc("/organizations/acme/nodes/web01", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&detailHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name": "web01",
			"chef_environment": "production",
			"automatic": {"platform": "ubuntu", "platform_version": "24.04"}
		}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &detailHits
}

// The summary already fetches the whole object, so it should carry that
// object's JSON for the detail/edit views to reuse — not just the curated
// fields.
func TestNodeSummaryCarriesObjectJSON(t *testing.T) {
	srv, _ := countingNodeServer(t)
	k := newNodeKind().(Summarizable)

	view, err := k.Summary(context.Background(), testClient(t, srv), "web01")
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if len(view.Fields) == 0 {
		t.Error("expected the curated fields to still be present")
	}
	if !strings.Contains(view.JSON, "web01") || !strings.Contains(view.JSON, "ubuntu") {
		t.Errorf("summary did not carry the full node JSON: %q", view.JSON)
	}
}

// cacheWeb01Summary drives the model into the loaded node list and resolves
// the web01 summary into the cache, returning the model and the detail-hit
// counter (which should read 1 at that point).
func cacheWeb01Summary(t *testing.T) (model, *int32) {
	t.Helper()
	srv, hits := countingNodeServer(t)
	m, tick := openNodes(t, srv)
	m, fetch := step(t, m, drain(tick)) // summaryTickMsg → summaryCmd
	m, _ = step(t, m, drain(fetch))     // summaryLoadedMsg → cache
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Fatalf("after summary fetch, detail hits = %d, want 1", got)
	}
	if view, ok := m.summaryCache["web01"]; !ok || view.JSON == "" {
		t.Fatalf("web01 JSON not cached: %+v", m.summaryCache["web01"])
	}
	return m, hits
}

// Opening the detail view for a node whose summary is already cached must
// reuse that fetch rather than issue a second Get.
func TestDetailReusesCachedSummary(t *testing.T) {
	m, hits := cacheWeb01Summary(t)

	next, cmd := m.openSelected()
	m = next.(model)
	m, _ = step(t, m, drain(cmd)) // detailLoadedMsg (synthetic) → content set

	if got := atomic.LoadInt32(hits); got != 1 {
		t.Errorf("opening detail refetched the node: hits = %d, want 1", got)
	}
	if m.screen != screenDetail {
		t.Errorf("screen = %v, want screenDetail", m.screen)
	}
	if !strings.Contains(m.detail.View(), "web01") {
		t.Errorf("detail content missing the node JSON:\n%s", m.detail.View())
	}
}

// Seeding the editor for a node whose summary is already cached must reuse
// that fetch too.
func TestEditSeedReusesCachedSummary(t *testing.T) {
	m, hits := cacheWeb01Summary(t)

	next, cmd := m.startEdit("web01")
	m = next.(model)
	m, _ = step(t, m, drain(cmd)) // editSeedMsg (synthetic) → openEditor

	if got := atomic.LoadInt32(hits); got != 1 {
		t.Errorf("seeding the editor refetched the node: hits = %d, want 1", got)
	}
	if m.editName != "web01" {
		t.Errorf("editName = %q, want web01", m.editName)
	}
	if m.screen != screenEditor {
		t.Errorf("screen = %v, want screenEditor", m.screen)
	}
}

// When the summary hasn't been cached yet (fast keyboarding), opening the
// detail still works by fetching once — no regression, no double fetch.
func TestDetailFetchesWhenNotCached(t *testing.T) {
	srv, hits := countingNodeServer(t)
	m, _ := openNodes(t, srv) // summary tick left pending → nothing cached

	next, cmd := m.openSelected()
	m = next.(model)
	m, _ = step(t, m, drain(cmd)) // describeCmd → detailLoadedMsg

	if got := atomic.LoadInt32(hits); got != 1 {
		t.Errorf("uncached detail open = %d fetches, want exactly 1", got)
	}
	if !strings.Contains(m.detail.View(), "web01") {
		t.Errorf("detail content missing the node JSON:\n%s", m.detail.View())
	}
}
