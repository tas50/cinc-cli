package explore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The Users kind renders a curated facts panel in the summary pane, not
// raw JSON, so an operator scanning users sees the human details at a
// glance.
func TestUserKindSummaryShowsFields(t *testing.T) {
	mux := http.NewServeMux()
	jsonHandler(mux, "/users/alice", `{
		"username": "alice",
		"display_name": "Alice Liddell",
		"email": "alice@example.com",
		"first_name": "Alice",
		"last_name": "Liddell"
	}`)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	k, ok := newUserKind().(Summarizable)
	if !ok {
		t.Fatal("user kind is not Summarizable")
	}
	view, err := k.Summary(context.Background(), testClient(t, srv), "alice")
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if view.JSON != "" {
		t.Errorf("user summary fell back to JSON: %q", view.JSON)
	}
	got := map[string]string{}
	for _, f := range view.Fields {
		got[f.Label] = f.Value
	}
	want := map[string]string{
		"Display Name": "Alice Liddell",
		"Email":        "alice@example.com",
		"First Name":   "Alice",
		"Last Name":    "Liddell",
	}
	for label, val := range want {
		if got[label] != val {
			t.Errorf("field %q = %q, want %q", label, got[label], val)
		}
	}
}
