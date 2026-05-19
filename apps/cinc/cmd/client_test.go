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

// clientServer starts an httptest server that serves a client index for
// org "acme" and returns the server.
func clientServer(t *testing.T, names ...string) *httptest.Server {
	t.Helper()
	index := make(map[string]string, len(names))
	for _, n := range names {
		index[n] = "https://example.test/clients/" + n
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/clients", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(index)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchClientNamesReturnsSortedNames(t *testing.T) {
	srv := clientServer(t, "worker-02", "admin", "worker-01")

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

	names, err := fetchClientNames(context.Background(), c)
	if err != nil {
		t.Fatalf("fetchClientNames: %v", err)
	}
	want := []string{"admin", "worker-01", "worker-02"}
	if !slices.Equal(names, want) {
		t.Errorf("fetchClientNames = %v, want %v", names, want)
	}
}

func TestClientListCommandEndToEnd(t *testing.T) {
	srv := clientServer(t, "worker-02", "admin", "worker-01")

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
	root.SetArgs([]string{"client", "list", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc client list: %v", err)
	}
	if got := buf.String(); got != "admin\nworker-01\nworker-02\n" {
		t.Errorf("client list output = %q, want sorted client names", got)
	}
}

func TestClientListCommandReportsConfigError(t *testing.T) {
	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"client", "list", "--config", filepath.Join(t.TempDir(), "missing.toml")})

	if err := root.Execute(); err == nil {
		t.Error("expected an error when the config file is missing")
	}
}
