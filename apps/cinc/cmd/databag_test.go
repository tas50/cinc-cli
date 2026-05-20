package cmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	cinc "github.com/tas50/cinc-api"
)

// databagServer starts an httptest server that serves a databag index
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

func TestDataBagCreateCommandEndToEnd(t *testing.T) {
	var created string
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/data", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var payload struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal create body: %v", err)
		}
		created = payload.Name
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(body)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"databag", "create", "secrets", "--config", writeDataBagConfig(t, srv.URL)})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc databag create: %v", err)
	}
	if created != "secrets" {
		t.Errorf("server saw create of %q, want %q", created, "secrets")
	}
	if got := buf.String(); got != "Created data bag \"secrets\"\n" {
		t.Errorf("databag create output = %q", got)
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
	root.SetArgs([]string{"databag", "delete", "users", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc databag delete: %v", err)
	}
	if deleted != "users" {
		t.Errorf("server saw delete of %q, want %q", deleted, "users")
	}
	if got := buf.String(); got != "Deleted data bag \"users\"\n" {
		t.Errorf("databag delete output = %q", got)
	}
}

// databagItemServer serves GET of the supplied current item and
// records the PUT body into *gotPut, *gotPath for assertion.
func databagItemServer(t *testing.T, bag, id string, current cinc.DataBagItem, gotPut *cinc.DataBagItem, gotPath *string) *httptest.Server {
	t.Helper()
	path := "/organizations/acme/data/" + bag + "/" + id
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(current)
		case http.MethodPut:
			*gotPath = r.URL.Path
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(body, gotPut); err != nil {
				t.Fatalf("unmarshal PUT body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
		default:
			t.Errorf("unexpected method %q on %s", r.Method, r.URL.Path)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func withStubDataBagItemEditor(t *testing.T, stub func(cinc.DataBagItem) (cinc.DataBagItem, error)) {
	t.Helper()
	orig := editDataBagItem
	editDataBagItem = stub
	t.Cleanup(func() { editDataBagItem = orig })
}

func writeDataBagConfig(t *testing.T, serverURL string) string {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "credentials")
	cfg := fmt.Sprintf(`[default]
cinc_server_url = "%s/organizations/acme"
client_name     = "tim"
client_key      = %q
`, serverURL, writeTestKey(t))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func TestDataBagItemEditCommandPutsEditorResult(t *testing.T) {
	var gotPut cinc.DataBagItem
	var gotPath string
	current := cinc.DataBagItem{"id": "alice", "role": "admin"}
	srv := databagItemServer(t, "users", "alice", current, &gotPut, &gotPath)

	withStubDataBagItemEditor(t, func(in cinc.DataBagItem) (cinc.DataBagItem, error) {
		out := cinc.DataBagItem{}
		for k, v := range in {
			out[k] = v
		}
		out["role"] = "editor"
		return out, nil
	})

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"databag", "item", "edit", "users", "alice", "--config", writeDataBagConfig(t, srv.URL)})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc databag item edit: %v", err)
	}
	if gotPath != "/organizations/acme/data/users/alice" {
		t.Errorf("PUT path = %q", gotPath)
	}
	if gotPut["role"] != "editor" || gotPut["id"] != "alice" {
		t.Errorf("PUT body = %+v, want id=alice role=editor", gotPut)
	}
	if got := buf.String(); got != "Updated item \"alice\" in bag \"users\"\n" {
		t.Errorf("stdout = %q", got)
	}
}

func TestDataBagItemEditCommandSkipsPutWhenUnchanged(t *testing.T) {
	var gotPut cinc.DataBagItem
	var gotPath string
	current := cinc.DataBagItem{"id": "alice", "role": "admin"}
	srv := databagItemServer(t, "users", "alice", current, &gotPut, &gotPath)

	withStubDataBagItemEditor(t, func(in cinc.DataBagItem) (cinc.DataBagItem, error) {
		out := cinc.DataBagItem{}
		for k, v := range in {
			out[k] = v
		}
		return out, nil
	})

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"databag", "item", "edit", "users", "alice", "--config", writeDataBagConfig(t, srv.URL)})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc databag item edit (unchanged): %v", err)
	}
	if gotPath != "" {
		t.Errorf("server saw a PUT at %q for an unchanged edit", gotPath)
	}
	if got := buf.String(); got != "Item \"alice\" in bag \"users\" unchanged\n" {
		t.Errorf("stdout = %q", got)
	}
}

func TestDataBagItemEditCommandReadsFromFile(t *testing.T) {
	var gotPut cinc.DataBagItem
	var gotPath string
	current := cinc.DataBagItem{"id": "alice", "role": "admin"}
	srv := databagItemServer(t, "users", "alice", current, &gotPut, &gotPath)

	withStubDataBagItemEditor(t, func(cinc.DataBagItem) (cinc.DataBagItem, error) {
		t.Fatal("editor was invoked despite --file")
		return nil, nil
	})

	filePath := filepath.Join(t.TempDir(), "item.json")
	body, err := json.Marshal(cinc.DataBagItem{"id": "ignored-in-file", "role": "editor"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, body, 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"databag", "item", "edit", "users", "alice", "--file", filePath, "--config", writeDataBagConfig(t, srv.URL)})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc databag item edit --file: %v", err)
	}
	if gotPut["role"] != "editor" {
		t.Errorf("PUT body role = %v, want editor", gotPut["role"])
	}
	if gotPut["id"] != "alice" {
		t.Errorf("PUT body id = %v, want alice (path arg must win over file)", gotPut["id"])
	}
}

func TestDataBagItemEditCommandRejectsFileMissingID(t *testing.T) {
	var gotPut cinc.DataBagItem
	var gotPath string
	current := cinc.DataBagItem{"id": "alice"}
	srv := databagItemServer(t, "users", "alice", current, &gotPut, &gotPath)

	filePath := filepath.Join(t.TempDir(), "item.json")
	if err := os.WriteFile(filePath, []byte(`{"role": "editor"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"databag", "item", "edit", "users", "alice", "--file", filePath, "--config", writeDataBagConfig(t, srv.URL)})

	if err := root.Execute(); err == nil {
		t.Error("expected an error for a --file payload with no id")
	}
	if gotPath != "" {
		t.Errorf("server saw a PUT %q despite validation failure", gotPath)
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
	root.SetArgs([]string{"databag", "list", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc databag list: %v", err)
	}
	if got := buf.String(); got != "apps\nsecrets\nusers\n" {
		t.Errorf("databag list output = %q, want sorted databag names", got)
	}
}
