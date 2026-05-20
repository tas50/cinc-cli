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

// databagServer starts an httptest server that serves a data-bag index
// for org "acme" and returns the server. The Chef API exposes data bags
// under /data, not /data_bags.
func databagServer(t *testing.T, names ...string) *httptest.Server {
	t.Helper()
	index := make(map[string]string, len(names))
	for _, n := range names {
		index[n] = "https://example.test/data/" + n
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/data", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(index)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchDataBagNamesReturnsSortedNames(t *testing.T) {
	srv := databagServer(t, "users", "apps", "secrets")

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

	names, err := fetchDataBagNames(context.Background(), c)
	if err != nil {
		t.Fatalf("fetchDataBagNames: %v", err)
	}
	want := []string{"apps", "secrets", "users"}
	if !slices.Equal(names, want) {
		t.Errorf("fetchDataBagNames = %v, want %v", names, want)
	}
}

func TestDataBagDeleteCommandEndToEnd(t *testing.T) {
	var deleted string
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/data/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		deleted = "users"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "users"})
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
	root.SetArgs([]string{"data-bag", "delete", "users", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc data-bag delete: %v", err)
	}
	if deleted != "users" {
		t.Errorf("server saw delete of %q, want %q", deleted, "users")
	}
	if got := buf.String(); got != "Deleted data bag \"users\"\n" {
		t.Errorf("data-bag delete output = %q", got)
	}
}

func TestDataBagListCommandEndToEnd(t *testing.T) {
	srv := databagServer(t, "users", "apps", "secrets")

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
	root.SetArgs([]string{"data-bag", "list", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc data-bag list: %v", err)
	}
	if got := buf.String(); got != "apps\nsecrets\nusers\n" {
		t.Errorf("data-bag list output = %q, want sorted data-bag names", got)
	}
}
