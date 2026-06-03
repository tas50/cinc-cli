//go:build acceptance

package acceptance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfigFile writes raw TOML to a temp credentials file and
// returns its path. Unlike writeAcceptanceConfig it does no chef-zero
// wiring, since `cinc config validate` is local-only.
func writeConfigFile(t *testing.T, contents string) string {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "credentials")
	if err := os.WriteFile(cfgPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

// TestConfigValidateValidAgainstBinary runs the real cinc binary against
// a well-formed credentials file and asserts the command reports success
// in both human and JSON output.
func TestConfigValidateValidAgainstBinary(t *testing.T) {
	binary := buildCinc(t)
	cfgPath := writeConfigFile(t, `[default]
cinc_server_url = "https://example.test/organizations/acme"
client_name     = "tester"
client_key      = "/tmp/tester.pem"
`)

	human := runCinc(t, binary, "config", "validate", "--config", cfgPath)
	if !strings.Contains(human, "is valid") || !strings.Contains(human, "1 profile(s)") {
		t.Errorf("config validate (human) = %q, want valid with 1 profile", human)
	}

	jsonOut := runCinc(t, binary, "config", "validate", "--config", cfgPath, "--format", "json")
	for _, want := range []string{`"valid": true`, `"profiles": 1`} {
		if !strings.Contains(jsonOut, want) {
			t.Errorf("config validate (json) missing %q\ngot: %s", want, jsonOut)
		}
	}
}

// TestConfigValidateInvalidAgainstBinary runs the real cinc binary
// against a credentials file missing required fields and asserts the
// command exits non-zero and names the offending fields.
func TestConfigValidateInvalidAgainstBinary(t *testing.T) {
	binary := buildCinc(t)
	cfgPath := writeConfigFile(t, `[default]
cinc_server_url = "https://example.test/organizations/acme"
`)

	stdout, _, err := runCincRaw(binary, "config", "validate", "--config", cfgPath)
	if err == nil {
		t.Fatalf("config validate should exit non-zero for an invalid config\nstdout: %s", stdout)
	}
	if !strings.Contains(stdout, "is invalid") {
		t.Errorf("config validate (human) = %q, want invalid report", stdout)
	}
	for _, want := range []string{"client_name", "client_key"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("config validate output missing issue for %q\ngot: %s", want, stdout)
		}
	}
}
