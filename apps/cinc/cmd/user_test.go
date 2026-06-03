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
	"strings"
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

func TestUserCreateCommandStreamsGeneratedKey(t *testing.T) {
	var posted map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&posted)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"uri":      "https://example.test/users/anna",
			"chef_key": map[string]string{"private_key": "-----BEGIN RSA PRIVATE KEY-----\nFAKE\n-----END RSA PRIVATE KEY-----\n"},
		})
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
	root.SetArgs([]string{"user", "create", "anna",
		"--email", "anna@example.test", "--display-name", "Anna Admin",
		"--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc user create: %v", err)
	}
	if posted["username"] != "anna" || posted["email"] != "anna@example.test" {
		t.Errorf("create body = %v, want username=anna email set", posted)
	}
	if posted["create_key"] != true {
		t.Errorf("create body create_key = %v, want true", posted["create_key"])
	}
	if got := buf.String(); !strings.Contains(got, "BEGIN RSA PRIVATE KEY") {
		t.Errorf("user create output missing private key:\n%s", got)
	}
}

func TestUserCreateCommandWritesKeyFile(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"chef_key": map[string]string{"private_key": "PRIVATE-KEY-PEM\n"},
		})
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
	keyPath := filepath.Join(t.TempDir(), "anna.pem")

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"user", "create", "anna", "--key-file", keyPath, "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc user create: %v", err)
	}
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("key file not written: %v", err)
	}
	if string(data) != "PRIVATE-KEY-PEM\n" {
		t.Errorf("key file = %q", string(data))
	}
	if got := buf.String(); got != fmt.Sprintf("Created user \"anna\" (key written to %s)\n", keyPath) {
		t.Errorf("user create output = %q", got)
	}
}

func TestUserDeleteCommandEndToEnd(t *testing.T) {
	var deleted string
	mux := http.NewServeMux()
	mux.HandleFunc("/users/anna", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		deleted = "anna"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"username": "anna"})
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
	root.SetArgs([]string{"user", "delete", "anna", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc user delete: %v", err)
	}
	if deleted != "anna" {
		t.Errorf("server saw delete of %q, want anna", deleted)
	}
	if got := buf.String(); got != "Deleted user \"anna\"\n" {
		t.Errorf("user delete output = %q", got)
	}
}

func TestUserPasswordCommandEndToEnd(t *testing.T) {
	var putBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/users/anna", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(cinc.User{UserName: "anna", Email: "anna@example.test", DisplayName: "Anna Admin"})
		case http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&putBody)
			_ = json.NewEncoder(w).Encode(cinc.User{UserName: "anna", Email: "anna@example.test"})
		default:
			t.Errorf("unexpected method %q", r.Method)
		}
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
	root.SetArgs([]string{"user", "password", "anna", "--password", "s3cret!", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc user password: %v", err)
	}
	if putBody["password"] != "s3cret!" {
		t.Errorf("PUT body password = %v, want s3cret!", putBody["password"])
	}
	// The existing metadata must survive the password change.
	if putBody["email"] != "anna@example.test" {
		t.Errorf("PUT body dropped email: %v", putBody)
	}
	if got := buf.String(); got != "Updated password for user \"anna\"\n" {
		t.Errorf("user password output = %q", got)
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
