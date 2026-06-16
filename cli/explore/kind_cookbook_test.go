package explore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestCookbookVersionSummaryFields(t *testing.T) {
	cb := &cinc.Cookbook{
		Version:          "1.2.0",
		AllFilesManifest: []cinc.CookbookFileRef{{Name: "recipes/default.rb"}, {Name: "metadata.rb"}},
		Metadata: cinc.CookbookMetadata{
			Description: "Installs and configures nginx",
			Maintainer:  "Sous Chefs",
			License:     "Apache-2.0",
		},
	}
	var order []string
	got := map[string]string{}
	for _, f := range cookbookVersionSummaryFields(cb) {
		got[f.Label] = f.Value
		order = append(order, f.Label)
	}
	want := map[string]string{
		"Version":     "1.2.0",
		"Description": "Installs and configures nginx",
		"Maintainer":  "Sous Chefs",
		"License":     "Apache-2.0",
		"Files":       "2",
	}
	for label, val := range want {
		if got[label] != val {
			t.Errorf("field %q = %q, want %q", label, got[label], val)
		}
	}
	if !reflect.DeepEqual(order, []string{"Version", "Description", "Maintainer", "License", "Files"}) {
		t.Errorf("field order = %v", order)
	}
}

// A version with no metadata still renders, falling back to em dashes rather
// than blank or missing rows.
func TestCookbookVersionSummaryFieldsNoMetadata(t *testing.T) {
	got := map[string]string{}
	for _, f := range cookbookVersionSummaryFields(&cinc.Cookbook{Version: "1.0.0"}) {
		got[f.Label] = f.Value
	}
	if got["Description"] != "—" || got["Maintainer"] != "—" || got["License"] != "—" {
		t.Errorf("expected em dashes for absent metadata, got %+v", got)
	}
}

// A cookbook version's summary draws its description, maintainer, and license
// from the metadata the server returns alongside the file manifest.
func TestCookbookVersionsKindSummary(t *testing.T) {
	mux := http.NewServeMux()
	jsonHandler(mux, "/organizations/acme/cookbooks/nginx/1.2.0",
		`{"cookbook_name":"nginx","name":"nginx-1.2.0","version":"1.2.0",
		  "all_files":[{"name":"recipes/default.rb"},{"name":"metadata.rb"}],
		  "metadata":{"description":"Installs and configures nginx","maintainer":"Sous Chefs","license":"Apache-2.0"}}`)
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
	if got["Description"] != "Installs and configures nginx" {
		t.Errorf("Description = %q", got["Description"])
	}
	if got["Files"] != "2" {
		t.Errorf("Files = %q, want 2", got["Files"])
	}
}
