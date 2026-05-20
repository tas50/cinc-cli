package cmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	cinc "github.com/tas50/cinc-api"
)

// writeTestKey generates an RSA key, writes it as PEM to a temp file, and
// returns the path.
func writeTestKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	path := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// nodeServer starts an httptest server that serves a node index for org
// "acme" and returns the server.
func nodeServer(t *testing.T, names ...string) *httptest.Server {
	t.Helper()
	index := make(map[string]string, len(names))
	for _, n := range names {
		index[n] = "https://example.test/nodes/" + n
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/nodes", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(index)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchNodeNamesReturnsSortedNames(t *testing.T) {
	srv := nodeServer(t, "web02", "db01", "web01")

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

	names, err := fetchNodeNames(context.Background(), c)
	if err != nil {
		t.Fatalf("fetchNodeNames: %v", err)
	}
	want := []string{"db01", "web01", "web02"}
	if !slices.Equal(names, want) {
		t.Errorf("fetchNodeNames = %v, want %v", names, want)
	}
}

func TestNodeListCommandEndToEnd(t *testing.T) {
	srv := nodeServer(t, "web02", "db01", "web01")

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
	root.SetArgs([]string{"node", "list", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc node list: %v", err)
	}
	if got := buf.String(); got != "db01\nweb01\nweb02\n" {
		t.Errorf("node list output = %q, want sorted node names", got)
	}
}

func TestNodeDeleteCommandEndToEnd(t *testing.T) {
	var deleted string
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/nodes/web01", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		deleted = "web01"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "web01"})
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
	root.SetArgs([]string{"node", "delete", "web01", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc node delete: %v", err)
	}
	if deleted != "web01" {
		t.Errorf("server saw delete of %q, want %q", deleted, "web01")
	}
	if got := buf.String(); got != "Deleted node \"web01\"\n" {
		t.Errorf("node delete output = %q", got)
	}
}

func TestNodeListCommandReportsConfigError(t *testing.T) {
	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"node", "list", "--config", filepath.Join(t.TempDir(), "missing.toml")})

	if err := root.Execute(); err == nil {
		t.Error("expected an error when the config file is missing")
	}
}
