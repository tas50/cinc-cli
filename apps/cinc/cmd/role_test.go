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

// roleServer starts an httptest server that serves a role index for org
// "acme" and returns the server.
func roleServer(t *testing.T, names ...string) *httptest.Server {
	t.Helper()
	index := make(map[string]string, len(names))
	for _, n := range names {
		index[n] = "https://example.test/roles/" + n
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/roles", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(index)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchRoleNamesReturnsSortedNames(t *testing.T) {
	srv := roleServer(t, "web", "base", "db")

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

	names, err := fetchRoleNames(context.Background(), c)
	if err != nil {
		t.Fatalf("fetchRoleNames: %v", err)
	}
	want := []string{"base", "db", "web"}
	if !slices.Equal(names, want) {
		t.Errorf("fetchRoleNames = %v, want %v", names, want)
	}
}

func TestRoleDeleteCommandEndToEnd(t *testing.T) {
	var deleted string
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/roles/web", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		deleted = "web"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "web"})
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
	root.SetArgs([]string{"role", "delete", "web", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc role delete: %v", err)
	}
	if deleted != "web" {
		t.Errorf("server saw delete of %q, want %q", deleted, "web")
	}
	if got := buf.String(); got != "Deleted role \"web\"\n" {
		t.Errorf("role delete output = %q", got)
	}
}

func TestRoleShowCommandEndToEnd(t *testing.T) {
	role := cinc.Role{
		Name:        "web",
		Description: "Web tier",
		RunList:     []string{"recipe[apache]", "recipe[base]"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/roles/web", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(role)
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
	root.SetArgs([]string{"role", "show", "web", "--config", cfgPath, "--format", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc role show: %v", err)
	}

	var got cinc.Role
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("show output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if got.Name != "web" || got.Description != "Web tier" {
		t.Errorf("show returned %+v, want name=web description=Web tier", got)
	}
	if !slices.Equal(got.RunList, []string{"recipe[apache]", "recipe[base]"}) {
		t.Errorf("show run_list = %v", got.RunList)
	}
}

func TestRoleListCommandEndToEnd(t *testing.T) {
	srv := roleServer(t, "web", "base", "db")

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
	root.SetArgs([]string{"role", "list", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc role list: %v", err)
	}
	if got := buf.String(); got != "base\ndb\nweb\n" {
		t.Errorf("role list output = %q, want sorted role names", got)
	}
}
