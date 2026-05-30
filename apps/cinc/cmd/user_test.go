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

// userServer starts an httptest server that serves a user index at the
// top-level /users endpoint (users are global, not org-scoped) and
// returns the server.
func userServer(t *testing.T, names ...string) *httptest.Server {
	t.Helper()
	index := make(map[string]string, len(names))
	for _, n := range names {
		index[n] = "https://example.test/users/" + n
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(index)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchUserNamesReturnsSortedNames(t *testing.T) {
	srv := userServer(t, "carol", "alice", "bob")

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

	names, err := fetchUserNames(context.Background(), c)
	if err != nil {
		t.Fatalf("fetchUserNames: %v", err)
	}
	want := []string{"alice", "bob", "carol"}
	if !slices.Equal(names, want) {
		t.Errorf("fetchUserNames = %v, want %v", names, want)
	}
}

func TestUserListCommandEndToEnd(t *testing.T) {
	srv := userServer(t, "carol", "alice", "bob")

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
	root.SetArgs([]string{"user", "list", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc user list: %v", err)
	}
	if got := buf.String(); got != "alice\nbob\ncarol\n" {
		t.Errorf("user list output = %q, want sorted user names", got)
	}
}

func TestUserShowCommandEndToEnd(t *testing.T) {
	user := cinc.User{
		UserName:    "alice",
		DisplayName: "Alice Admin",
		Email:       "alice@example.test",
		FirstName:   "Alice",
		LastName:    "Admin",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/users/alice", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(user)
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
	root.SetArgs([]string{"user", "show", "alice", "--config", cfgPath, "--format", "json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc user show: %v", err)
	}

	var got cinc.User
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("show output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if got.UserName != "alice" || got.Email != "alice@example.test" {
		t.Errorf("show returned %+v, want username=alice email=alice@example.test", got)
	}
}
