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

// groupServer starts an httptest server that serves a group index for
// org "acme" and returns the server.
func groupServer(t *testing.T, names ...string) *httptest.Server {
	t.Helper()
	index := make(map[string]string, len(names))
	for _, n := range names {
		index[n] = "https://example.test/groups/" + n
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/groups", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(index)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchGroupNamesReturnsSortedNames(t *testing.T) {
	srv := groupServer(t, "users", "admins", "clients")

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

	names, err := fetchGroupNames(context.Background(), c)
	if err != nil {
		t.Fatalf("fetchGroupNames: %v", err)
	}
	want := []string{"admins", "clients", "users"}
	if !slices.Equal(names, want) {
		t.Errorf("fetchGroupNames = %v, want %v", names, want)
	}
}

func TestGroupListCommandEndToEnd(t *testing.T) {
	srv := groupServer(t, "users", "admins", "clients")

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
	root.SetArgs([]string{"group", "list", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc group list: %v", err)
	}
	if got := buf.String(); got != "admins\nclients\nusers\n" {
		t.Errorf("group list output = %q, want sorted group names", got)
	}
}

func TestGroupShowCommandEndToEnd(t *testing.T) {
	group := cinc.Group{
		Name:      "admins",
		GroupName: "admins",
		OrgName:   "acme",
		Users:     []string{"alice", "pivotal"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/groups/admins", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(group)
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
	root.SetArgs([]string{"group", "show", "admins", "--config", cfgPath, "--format", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc group show: %v", err)
	}

	var got cinc.Group
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("show output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if got.GroupName != "admins" {
		t.Errorf("show returned groupname=%q, want admins", got.GroupName)
	}
	if !slices.Equal(got.Users, []string{"alice", "pivotal"}) {
		t.Errorf("show users = %v", got.Users)
	}
}
