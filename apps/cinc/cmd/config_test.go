package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigValidateCommandReportsValidConfig(t *testing.T) {
	srv := configValidateServer(t, http.StatusOK)
	cfgPath := writeValidateConfig(t, fmt.Sprintf(`
[default]
client_name = "tim"
client_key = %q
cinc_server_url = "%s/organizations/acme"
`, writeTestKey(t), srv.URL))

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"config", "validate", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc config validate: %v", err)
	}
	got := out.String()
	for _, want := range []string{"is valid", "default profile [VALID]", "✓ Server URL is valid", "✓ Server is reachable"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
	}
}

func TestParseServerHost(t *testing.T) {
	ok := map[string]string{
		"https://chef.example.com":     "chef.example.com",
		"http://chef.example.com:8443": "chef.example.com",
		"https://127.0.0.1:8889":       "127.0.0.1",
	}
	for in, want := range ok {
		if got, err := parseServerHost(in); err != nil || got != want {
			t.Errorf("parseServerHost(%q) = (%q, %v), want (%q, nil)", in, got, err, want)
		}
	}
	for _, in := range []string{"", "chef.example.com", "ftp://host", "https://", "://nope"} {
		if _, err := parseServerHost(in); err == nil {
			t.Errorf("parseServerHost(%q) = nil error, want a format error", in)
		}
	}
}

func withStubResolver(t *testing.T, fn func(context.Context, string) ([]string, error)) {
	t.Helper()
	orig := resolveHost
	resolveHost = fn
	t.Cleanup(func() { resolveHost = orig })
}

func TestConfigValidateServerReachableFailsOnDNS(t *testing.T) {
	// The reachable check resolves DNS before connecting; a resolution failure
	// is reported there (and the server is never dialed).
	srv := configValidateServer(t, http.StatusOK)
	contacted := false
	srv.Config.Handler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) { contacted = true })
	withStubResolver(t, func(context.Context, string) ([]string, error) {
		return nil, fmt.Errorf("no such host")
	})

	cfgPath := writeValidateConfig(t, fmt.Sprintf(`
[default]
client_name = "tim"
client_key = %q
cinc_server_url = "%s/organizations/acme"
`, writeTestKey(t), srv.URL))

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"config", "validate", cfgPath})

	if err := root.Execute(); err == nil {
		t.Fatal("expected validation error when DNS fails")
	}
	if got := out.String(); !strings.Contains(got, "✗ Server is reachable: cannot resolve host") {
		t.Errorf("stdout = %q, want a DNS resolution failure on the reachable check", got)
	}
	if contacted {
		t.Error("the server should not be dialed when DNS resolution fails")
	}
}

func TestConfigValidateCommandPreflightsSupermarketSite(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The reachable check hits the Supermarket health endpoint via the
		// cinc-supermarket client.
		if r.URL.Path != "/api/v1/health" {
			t.Errorf("path = %q, want /api/v1/health", r.URL.Path)
		}
		hit = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	t.Cleanup(srv.Close)
	cfgPath := writeValidateConfig(t, fmt.Sprintf(`
[supermarket]
client_name = "tim"
client_key = %q
supermarket_site = "%s"
`, writeTestKey(t), srv.URL))

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"config", "validate", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("cinc config validate: %v", err)
	}
	if !hit {
		t.Fatal("supermarket reachable check did not contact the site")
	}
	if got := out.String(); !strings.Contains(got, "✓ Supermarket is reachable") {
		t.Errorf("stdout = %q, want the supermarket check to pass", got)
	}
}

func TestConfigValidateCommandReportsUnreachableServer(t *testing.T) {
	srv := configValidateServer(t, http.StatusInternalServerError)
	cfgPath := writeValidateConfig(t, fmt.Sprintf(`
[default]
client_name = "tim"
client_key = %q
cinc_server_url = "%s/organizations/acme"
`, writeTestKey(t), srv.URL))

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"config", "validate", cfgPath})

	if err := root.Execute(); err == nil {
		t.Fatal("expected validation error for an unreachable server")
	}
	got := out.String()
	for _, want := range []string{"is invalid", "default profile [INVALID]", "✗ Server is reachable"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
	}
}

func TestConfigValidateCommandReportsInvalidConfig(t *testing.T) {
	cfgPath := writeValidateConfig(t, `
[broken]
client_name = "tim"
`)

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"config", "validate", cfgPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "config validation failed") {
		t.Fatalf("error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"broken profile [INVALID]",
		"✗ Client key is configured: client_key is required",
		"✗ An endpoint is configured: configure cinc_server_url, chef_server_url, or supermarket_site",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
	}
}

func TestConfigValidateCommandFormatsChecks(t *testing.T) {
	cfgPath := writeValidateConfig(t, `
[broken]
client_name = "tim"
`)

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"config", "validate", cfgPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected validation error")
	}
	got := out.String()

	// Overall line, then the top-level checks.
	if !strings.HasPrefix(got, "Config ") || !strings.Contains(got, " is invalid\n") {
		t.Errorf("missing overall line:\n%s", got)
	}
	if !strings.Contains(got, "✓ Credentials file is valid TOML") ||
		!strings.Contains(got, "✓ At least one profile is defined") {
		t.Errorf("missing top-level checks:\n%s", got)
	}
	// A blank line separates the top-level checks from the profile block, which
	// is tagged and has indented per-profile checks.
	if !strings.Contains(got, "\n\nbroken profile [INVALID]\n") {
		t.Errorf("profile block not separated/tagged:\n%s", got)
	}
	if !strings.Contains(got, "  ✓ Client name is configured\n") {
		t.Errorf("per-profile checks not indented:\n%s", got)
	}
	// No leftover wording from the old issue-count format.
	if strings.Contains(got, "issue(s)") {
		t.Errorf("output still mentions issue counts:\n%s", got)
	}
	// The error wraps errAlreadyReported so Execute exits non-zero without
	// re-printing a generic "Error: ..." line.
	if !errors.Is(err, errAlreadyReported) {
		t.Errorf("error should wrap errAlreadyReported; got %v", err)
	}
}

func TestConfigValidateCommandSupportsJSONOutput(t *testing.T) {
	cfgPath := writeValidateConfig(t, `
[broken]
client_key = "/keys/tim.pem"
`)

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"config", "validate", cfgPath, "--format", "json"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected validation error")
	}

	var result configValidationResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if result.Valid {
		t.Error("result.valid should be false")
	}
	if len(result.TopLevel) == 0 {
		t.Error("expected top-level checks in JSON")
	}
	if len(result.Profiles) != 1 || result.Profiles[0].Name != "broken" || result.Profiles[0].Valid {
		t.Fatalf("profiles = %+v", result.Profiles)
	}
	var found bool
	for _, c := range result.Profiles[0].Checks {
		if c.Name == "Client name is configured" {
			found = true
			if c.Passed {
				t.Error("'Client name is configured' should have failed for the broken profile")
			}
		}
	}
	if !found {
		t.Error("expected a 'Client name is configured' check in the JSON output")
	}
}

func writeValidateConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func configValidateServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/acme/clients", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}
