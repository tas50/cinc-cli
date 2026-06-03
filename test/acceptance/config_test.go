//go:build acceptance

package acceptance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigCreateNonInteractiveAgainstCincZero runs `cinc config create`
// with every value supplied as a flag (no prompts), then proves the
// written profile actually works by listing nodes through it.
func TestConfigCreateNonInteractiveAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	out := filepath.Join(t.TempDir(), "credentials")
	serverURL := "http://127.0.0.1:" + itoa(env.port) + "/organizations/acme"

	_, stderr, err := runCincRaw(env.binary, "config", "create",
		"--config", out,
		"--server-url", serverURL,
		"--client-name", "pivotal",
		"--client-key", env.adminKey,
	)
	if err != nil {
		t.Fatalf("config create failed: %v\nstderr: %s", err, stderr)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("config create did not write %s: %v", out, err)
	}

	nodes := runCinc(t, env.binary, "node", "list", "--config", out)
	if nodes != "db01\nweb01\nweb02\n" {
		t.Errorf("node list via created config = %q, want the seeded nodes", nodes)
	}
}

// TestConfigValidateAgainstCincZero validates the working acceptance
// config; preflight should reach the live server and report it valid.
func TestConfigValidateAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	out := runCinc(t, env.binary, "config", "validate", env.cfgPath, "--config", env.cfgPath)
	if !strings.Contains(out, "valid") {
		t.Errorf("config validate output = %q, want it to report the config valid", out)
	}
}
