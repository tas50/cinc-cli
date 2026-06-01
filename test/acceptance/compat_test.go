//go:build acceptance

package acceptance

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestChefServerURLAliasAgainstCincZero confirms a profile that uses the
// chef-compat `chef_server_url` key (instead of `cinc_server_url`)
// resolves and runs a real command against the live server.
func TestChefServerURLAliasAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "credentials")
	cfg := fmt.Sprintf(`[default]
chef_server_url = "http://127.0.0.1:%d/organizations/acme"
client_name     = "pivotal"
client_key      = %q
`, env.port, env.adminKey)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	out := runCinc(t, env.binary, "node", "list", "--config", cfgPath)
	if out != "db01\nweb01\nweb02\n" {
		t.Errorf("node list via chef_server_url = %q, want the seeded nodes", out)
	}
}

// TestCincProfileBeatsChefProfileAgainstCincZero writes a two-profile
// config where only the "cincwins" profile points at the live acme org
// (the "chefwins" profile points at a dead port). With both CINC_PROFILE
// and CHEF_PROFILE set, CINC_PROFILE must win, so the command succeeds.
func TestCincProfileBeatsChefProfileAgainstCincZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "credentials")
	cfg := fmt.Sprintf(`[cincwins]
cinc_server_url = "http://127.0.0.1:%d/organizations/acme"
client_name     = "pivotal"
client_key      = %q

[chefwins]
cinc_server_url = "http://127.0.0.1:1/organizations/acme"
client_name     = "pivotal"
client_key      = %q
`, env.port, env.adminKey, env.adminKey)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runCincRawEnv(t,
		[]string{"CINC_PROFILE=cincwins", "CHEF_PROFILE=chefwins"},
		env.binary, "node", "list", "--config", cfgPath)
	if err != nil {
		t.Fatalf("expected CINC_PROFILE to win and the command to succeed: %v\nstderr: %s", err, stderr)
	}
	if stdout != "db01\nweb01\nweb02\n" {
		t.Errorf("node list = %q, want the acme nodes (proves cincwins profile was used)", stdout)
	}
}
