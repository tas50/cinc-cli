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

// environmentServer starts an httptest server that serves an environment
// index for org "acme" and returns the server.
func environmentServer(t *testing.T, names ...string) *httptest.Server {
	t.Helper()
	index := make(map[string]string, len(names))
	for _, n := range names {
		index[n] = "https://example.test/environments/" + n
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/environments", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(index)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchEnvironmentNamesReturnsSortedNames(t *testing.T) {
	srv := environmentServer(t, "prod", "_default", "staging")

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

	names, err := fetchEnvironmentNames(context.Background(), c)
	if err != nil {
		t.Fatalf("fetchEnvironmentNames: %v", err)
	}
	want := []string{"_default", "prod", "staging"}
	if !slices.Equal(names, want) {
		t.Errorf("fetchEnvironmentNames = %v, want %v", names, want)
	}
}

func TestEnvironmentDeleteCommandEndToEnd(t *testing.T) {
	var deleted string
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/environments/staging", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		deleted = "staging"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "staging"})
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
	root.SetArgs([]string{"environment", "delete", "staging", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc environment delete: %v", err)
	}
	if deleted != "staging" {
		t.Errorf("server saw delete of %q, want %q", deleted, "staging")
	}
	if got := buf.String(); got != "Deleted environment \"staging\"\n" {
		t.Errorf("environment delete output = %q", got)
	}
}

func TestEnvironmentListCommandEndToEnd(t *testing.T) {
	srv := environmentServer(t, "prod", "_default", "staging")

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
	root.SetArgs([]string{"environment", "list", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc environment list: %v", err)
	}
	if got := buf.String(); got != "_default\nprod\nstaging\n" {
		t.Errorf("environment list output = %q, want sorted environment names", got)
	}
}
