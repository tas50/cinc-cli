package supermarket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestSearchReturnsEntriesFromSinglePage(t *testing.T) {
	srv := searchFixtureServer(t, searchFixture{
		ExpectQuery: "nginx",
		Cookbooks: []sumEntry{
			{Name: "nginx", Maintainer: "sous-chefs", Description: "Web server"},
			{Name: "nginx_simple", Maintainer: "someone"},
		},
	})
	defer srv.Close()

	client := mustAnonymousClient(t, srv.URL)
	result, err := client.Search(context.Background(), SearchOptions{Query: "nginx"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("len = %d, want 2", len(result.Entries))
	}
	if result.Entries[0].Name != "nginx" || result.Entries[1].Name != "nginx_simple" {
		t.Fatalf("entries = %+v", result.Entries)
	}
	if result.Total != 2 {
		t.Fatalf("Total = %d, want 2", result.Total)
	}
}

func TestSearchAutoPaginates(t *testing.T) {
	all := make([]sumEntry, 0, 175)
	for i := range 175 {
		all = append(all, sumEntry{Name: fmt.Sprintf("nginx-%03d", i)})
	}
	srv := searchFixtureServer(t, searchFixture{ExpectQuery: "nginx", Cookbooks: all})
	defer srv.Close()

	client := mustAnonymousClient(t, srv.URL)
	result, err := client.Search(context.Background(), SearchOptions{Query: "nginx"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Entries) != 175 {
		t.Fatalf("len = %d, want 175", len(result.Entries))
	}
}

func TestSearchHonorsLimit(t *testing.T) {
	all := make([]sumEntry, 0, 175)
	for i := range 175 {
		all = append(all, sumEntry{Name: fmt.Sprintf("nginx-%03d", i)})
	}
	srv := searchFixtureServer(t, searchFixture{ExpectQuery: "nginx", Cookbooks: all})
	defer srv.Close()

	client := mustAnonymousClient(t, srv.URL)
	result, err := client.Search(context.Background(), SearchOptions{Query: "nginx", Limit: 25})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Entries) != 25 {
		t.Fatalf("len = %d, want 25", len(result.Entries))
	}
}

func TestSearchRejectsEmptyQuery(t *testing.T) {
	client := mustAnonymousClient(t, "https://supermarket.example.test")
	if _, err := client.Search(context.Background(), SearchOptions{}); err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestSearchVerboseAttachesLatestVersion(t *testing.T) {
	srv := searchFixtureServer(t, searchFixture{
		ExpectQuery: "ng",
		Cookbooks:   []sumEntry{{Name: "nginx", Maintainer: "sous-chefs"}},
		Universe:    map[string][]string{"nginx": {"12.0.3", "12.0.4"}},
	})
	defer srv.Close()

	client := mustAnonymousClient(t, srv.URL)
	result, err := client.Search(context.Background(), SearchOptions{Query: "ng", Verbose: true})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if result.Entries[0].LatestVersion != "12.0.4" {
		t.Fatalf("LatestVersion = %q, want 12.0.4", result.Entries[0].LatestVersion)
	}
}

type searchFixture struct {
	ExpectQuery string
	Cookbooks   []sumEntry
	Universe    map[string][]string
}

func searchFixtureServer(t *testing.T, f searchFixture) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/search":
			q := r.URL.Query()
			if got := q.Get("q"); got != f.ExpectQuery {
				t.Errorf("q = %q, want %q", got, f.ExpectQuery)
			}
			start, _ := strconv.Atoi(q.Get("start"))
			items, _ := strconv.Atoi(q.Get("items"))
			if items <= 0 {
				items = 100
			}
			start = min(start, len(f.Cookbooks))
			end := min(start+items, len(f.Cookbooks))
			page := f.Cookbooks[start:end]
			items_ := make([]map[string]string, 0, len(page))
			for _, c := range page {
				items_ = append(items_, map[string]string{
					"cookbook_name":        c.Name,
					"cookbook_maintainer":  c.Maintainer,
					"cookbook_description": c.Description,
					"cookbook":             "/api/v1/cookbooks/" + c.Name,
				})
			}
			writeJSON(w, map[string]any{
				"start": start,
				"total": len(f.Cookbooks),
				"items": items_,
			})
		case "/universe":
			out := map[string]map[string]map[string]any{}
			for name, versions := range f.Universe {
				out[name] = map[string]map[string]any{}
				for _, v := range versions {
					out[name][v] = map[string]any{
						"location_type": "supermarket",
						"location_path": "https://example.test",
					}
				}
			}
			writeJSON(w, out)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	return srv
}
