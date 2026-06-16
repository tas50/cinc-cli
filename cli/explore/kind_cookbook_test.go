package explore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	cinc "github.com/tas50/cinc-api"
)

func TestLatestCookbookVersion(t *testing.T) {
	versions := []cinc.CookbookVersion{{Version: "1.0.0"}, {Version: "1.2.0"}, {Version: "1.1.5"}}
	if got := latestCookbookVersion(versions); got != "1.2.0" {
		t.Errorf("latestCookbookVersion = %q, want 1.2.0", got)
	}
	if got := latestCookbookVersion(nil); got != "" {
		t.Errorf("latestCookbookVersion(nil) = %q, want empty", got)
	}
}

// A cookbook shows its version count and newest version on hover.
func TestCookbookKindSummary(t *testing.T) {
	mux := http.NewServeMux()
	jsonHandler(mux, "/organizations/acme/cookbooks",
		`{"nginx":{"versions":[{"version":"1.0.0"},{"version":"1.2.0"}]}}`)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	view, err := cookbookKind{}.Summary(context.Background(), testClient(t, srv), "nginx")
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	got := map[string]string{}
	for _, f := range view.Fields {
		got[f.Label] = f.Value
	}
	if got["Versions"] != "2" {
		t.Errorf("Versions = %q, want 2", got["Versions"])
	}
	if got["Latest"] != "1.2.0" {
		t.Errorf("Latest = %q, want 1.2.0", got["Latest"])
	}
}

// A cookbook version shows its identity and manifest file count.
func TestCookbookVersionsKindSummary(t *testing.T) {
	mux := http.NewServeMux()
	jsonHandler(mux, "/organizations/acme/cookbooks/nginx/1.2.0",
		`{"cookbook_name":"nginx","name":"nginx-1.2.0","version":"1.2.0",
		  "all_files":[{"name":"recipes/default.rb"},{"name":"metadata.rb"}]}`)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	k := cookbookVersionsKind{name: "nginx"}
	view, err := k.Summary(context.Background(), testClient(t, srv), "1.2.0")
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	got := map[string]string{}
	for _, f := range view.Fields {
		got[f.Label] = f.Value
	}
	if got["Version"] != "1.2.0" {
		t.Errorf("Version = %q, want 1.2.0", got["Version"])
	}
	if got["Files"] != "2" {
		t.Errorf("Files = %q, want 2", got["Files"])
	}
}
