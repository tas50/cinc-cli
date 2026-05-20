package cmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	cinc "github.com/tas50/cinc-api"
)

// cookbookServer starts an httptest server that serves a cookbook index
// for org "acme" and returns the server. Each cookbook in the index has a
// URL plus a single dummy version, matching the Chef API shape.
func cookbookServer(t *testing.T, names ...string) *httptest.Server {
	t.Helper()
	type entry struct {
		URL      string              `json:"url"`
		Versions []map[string]string `json:"versions"`
	}
	index := make(map[string]entry, len(names))
	for _, n := range names {
		index[n] = entry{
			URL: "https://example.test/cookbooks/" + n,
			Versions: []map[string]string{
				{"url": "https://example.test/cookbooks/" + n + "/1.0.0", "version": "1.0.0"},
			},
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/cookbooks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(index)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchCookbookNamesReturnsSortedNames(t *testing.T) {
	srv := cookbookServer(t, "nginx", "apache", "mysql")

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	c, err := cinc.NewClient(cinc.Config{
		ServerURL: srv.URL, Org: "acme", ClientName: "tim", Key: key,
	})
	if err != nil {
		t.Fatal(err)
	}

	names, err := fetchCookbookNames(context.Background(), c)
	if err != nil {
		t.Fatalf("fetchCookbookNames: %v", err)
	}
	want := []string{"apache", "mysql", "nginx"}
	if !slices.Equal(names, want) {
		t.Errorf("fetchCookbookNames = %v, want %v", names, want)
	}
}

func TestCookbookDeleteCommandEndToEnd(t *testing.T) {
	var deletedPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/cookbooks/nginx/1.0.0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		deletedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "nginx"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfgPath := filepath.Join(t.TempDir(), "credentials")
	cfg := fmt.Sprintf(`[default]
cinc_server_url = "%s/organizations/acme"
client_name     = "tim"
client_key      = %q
`, srv.URL, writeTestKey(t))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"cookbook", "delete", "nginx", "1.0.0", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc cookbook delete: %v", err)
	}
	if deletedPath != "/organizations/acme/cookbooks/nginx/1.0.0" {
		t.Errorf("server saw delete at %q", deletedPath)
	}
	if got := buf.String(); got != "Deleted cookbook \"nginx\" version 1.0.0\n" {
		t.Errorf("cookbook delete output = %q", got)
	}
}

func TestCookbookListCommandEndToEnd(t *testing.T) {
	srv := cookbookServer(t, "nginx", "apache", "mysql")

	cfgPath := filepath.Join(t.TempDir(), "credentials")
	cfg := fmt.Sprintf(`[default]
cinc_server_url = "%s/organizations/acme"
client_name     = "tim"
client_key      = %q
`, srv.URL, writeTestKey(t))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"cookbook", "list", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc cookbook list: %v", err)
	}
	if got := buf.String(); got != "apache\nmysql\nnginx\n" {
		t.Errorf("cookbook list output = %q, want sorted cookbook names", got)
	}
}
