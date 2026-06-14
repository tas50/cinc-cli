package cmd

import (
	"bytes"
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
	if got := out.String(); !strings.Contains(got, "is valid (1 profile(s))") {
		t.Fatalf("stdout = %q", got)
	}
}

func TestConfigValidateCommandPreflightsSupermarketSite(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The preflight now reaches the Supermarket via the cinc-supermarket
		// client's health check rather than a hand-rolled cookbooks GET.
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
		t.Fatal("supermarket preflight endpoint was not contacted")
	}
}

func TestConfigValidateCommandReportsPreflightFailure(t *testing.T) {
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

	err := root.Execute()
	if err == nil {
		t.Fatal("expected preflight validation error")
	}
	got := out.String()
	for _, want := range []string{
		"is invalid",
		"default\tcinc_server_url\tpreflight failed:",
	} {
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
	if !strings.Contains(err.Error(), "config validation failed with") {
		t.Fatalf("error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"is invalid",
		"broken\tclient_key\tclient_key is required",
		"broken\tendpoint\tconfigure cinc_server_url, chef_server_url, or supermarket_site",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
	}
}

func TestConfigValidateCommandFormatsIssues(t *testing.T) {
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
	// Header ends in a colon, not the old "(N issue(s))".
	if !strings.Contains(got, "is invalid:\n") || strings.Contains(got, "issue(s)") {
		t.Errorf("header not reformatted:\n%s", got)
	}
	// Issues are indented and numbered.
	if !strings.Contains(got, "  1. ") || !strings.Contains(got, "  2. ") {
		t.Errorf("issues not numbered/indented:\n%s", got)
	}
	// The error wraps errAlreadyReported, which is how Execute suppresses the
	// second generic "Error: ..." line while still exiting non-zero.
	if !errors.Is(err, errAlreadyReported) {
		t.Errorf("error should wrap errAlreadyReported so Execute does not re-print it; got %v", err)
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

	err := root.Execute()
	if err == nil {
		t.Fatal("expected validation error")
	}
	got := out.String()
	for _, want := range []string{`"valid": false`, `"profile": "broken"`, `"field": "client_name"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
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
