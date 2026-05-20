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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/remote"
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

func TestNodeSSHNoClientCommand(t *testing.T) {
	runner := &recordingRunner{result: remote.CommandResult{Stdout: "ok\n"}}
	old := nodeRemoteRunner
	nodeRemoteRunner = runner
	t.Cleanup(func() { nodeRemoteRunner = old })

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"node", "ssh", "web01 web02", "uptime",
		"--no-client",
		"--ssh-user", "ubuntu",
		"--ssh-agent-socket", "/tmp/cinc-agent.sock",
		"--no-host-key-verify",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc node ssh: %v", err)
	}
	if got, want := buf.String(), "web01\tok\nweb02\tok\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	slices.SortFunc(runner.calls, func(a, b remoteCall) int {
		return strings.Compare(a.target.Host, b.target.Host)
	})
	if len(runner.calls) != 2 || runner.calls[0].target.Host != "web01" || runner.calls[1].target.Host != "web02" {
		t.Fatalf("runner calls = %+v", runner.calls)
	}
	if runner.calls[0].opts.User != "ubuntu" || runner.calls[0].opts.AgentSocket != "/tmp/cinc-agent.sock" || runner.calls[0].opts.VerifyHost {
		t.Fatalf("ssh opts = %+v", runner.calls[0].opts)
	}
}

func TestNodeSSHSearchBackedCommand(t *testing.T) {
	runner := &recordingRunner{result: remote.CommandResult{Stdout: "done\n"}}
	old := nodeRemoteRunner
	nodeRemoteRunner = runner
	t.Cleanup(func() { nodeRemoteRunner = old })

	var sawQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/search/node", func(w http.ResponseWriter, r *http.Request) {
		sawQuery = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"total":1,"start":0,"rows":[{"automatic":{"fqdn":"web01.example.test"}}]}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	cfgPath := writeCommandConfig(t, srv.URL)

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"node", "ssh", "web", "hostname",
		"--ssh-user", "ubuntu",
		"--config", cfgPath,
		"--no-host-key-verify",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc node ssh: %v", err)
	}
	if !strings.Contains(sawQuery, "tags:*web*") || !strings.Contains(sawQuery, "fqdn:*web*") {
		t.Fatalf("query = %q, want expanded Knife-style node query", sawQuery)
	}
	if len(runner.calls) != 1 || runner.calls[0].target.Host != "web01.example.test" {
		t.Fatalf("runner calls = %+v", runner.calls)
	}
	if got := buf.String(); got != "web01.example.test\tdone\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestNodeBootstrapDryRunCommand(t *testing.T) {
	cfgPath := writeCommandConfig(t, "https://cinc.example.test")
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"node", "bootstrap", "web01.example.test",
		"--node-name", "web01",
		"--ssh-user", "ubuntu",
		"--run-list", "recipe[apt],recipe[nginx]",
		"--config", cfgPath,
		"--dry-run",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc node bootstrap --dry-run: %v", err)
	}
	got := buf.String()
	for _, want := range []string{
		"curl -L 'https://omnitruck.cinc.sh/install.sh'",
		"chef_server_url \"https://cinc.example.test/organizations/acme\"",
		"node_name \"web01\"",
		"\"run_list\": [",
		"recipe[nginx]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, got)
		}
	}
}

func TestNodeBootstrapCreatesClientAndRunsRemoteCommand(t *testing.T) {
	runner := &recordingRunner{result: remote.CommandResult{Stdout: "bootstrap ok\n"}}
	old := nodeRemoteRunner
	nodeRemoteRunner = runner
	t.Cleanup(func() { nodeRemoteRunner = old })

	var clientBody string
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/clients", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		clientBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"name":"web01","validator":false}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	cfgPath := writeCommandConfig(t, srv.URL)

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"node", "bootstrap", "web01.example.test",
		"--node-name", "web01",
		"--ssh-user", "ubuntu",
		"--config", cfgPath,
		"--no-host-key-verify",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc node bootstrap: %v", err)
	}
	if !strings.Contains(clientBody, `"name":"web01"`) {
		t.Fatalf("client create body = %s", clientBody)
	}
	if !strings.Contains(clientBody, `"public_key":"-----BEGIN PUBLIC KEY-----`) {
		t.Fatalf("client create body missing generated public key: %s", clientBody)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner calls = %+v", runner.calls)
	}
	call := runner.calls[0]
	if call.target.Host != "web01.example.test" || call.opts.User != "ubuntu" || call.opts.VerifyHost {
		t.Fatalf("runner call = %+v", call)
	}
	if !strings.Contains(call.command, "-----BEGIN RSA PRIVATE KEY-----") || !strings.Contains(call.command, "cinc-client -j /etc/cinc/first-boot.json") {
		t.Fatalf("bootstrap command = %s", call.command)
	}
	if got := buf.String(); got != "Bootstrapped node \"web01\" on web01.example.test\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func writeCommandConfig(t *testing.T, serverURL string) string {
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

type recordingRunner struct {
	mu     sync.Mutex
	calls  []remoteCall
	result remote.CommandResult
}

type remoteCall struct {
	target  remote.Target
	command string
	opts    remote.SSHOptions
}

func (r *recordingRunner) Run(_ context.Context, target remote.Target, command string, opts remote.SSHOptions) remote.CommandResult {
	r.mu.Lock()
	r.calls = append(r.calls, remoteCall{target: target, command: command, opts: opts})
	result := r.result
	r.mu.Unlock()
	result.Host = target.Host
	if result.ExitCode == 0 && result.Error == "" {
		result.ExitCode = 0
	}
	if result.Stdout == "" && r.result.Stdout == "" {
		result.Stdout = "ok\n"
	}
	return result
}
