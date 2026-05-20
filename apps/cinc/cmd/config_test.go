package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigValidateCommandReportsValidConfig(t *testing.T) {
	cfgPath := writeValidateConfig(t, `
[default]
client_name = "tim"
client_key = "/keys/tim.pem"
cinc_server_url = "https://cinc.example.test/organizations/acme"
`)

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
